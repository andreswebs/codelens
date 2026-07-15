package pipeline_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/pipeline"
	"github.com/andreswebs/codelens/internal/terr"
	"github.com/andreswebs/codelens/internal/transform/filter"
	"github.com/andreswebs/codelens/internal/transform/group"
)

// mustFilter compiles include/exclude globs or fails the test.
func mustFilter(t *testing.T, includes, excludes []string) filter.Spec {
	t.Helper()
	spec, err := filter.Compile(includes, excludes)
	if err != nil {
		t.Fatalf("filter.Compile(%v, %v) returned error: %v", includes, excludes, err)
	}
	return spec
}

// TestPipeline_FilterBeforeGroup proves the filter stage runs before grouping:
// a glob dropping `**/Migrations/**` removes those files while they still carry
// raw paths, so the layer they would have grouped into never sees them. Both
// files would otherwise group to SrcLayer, so filtering afterward could not
// distinguish them.
func TestPipeline_FilterBeforeGroup(t *testing.T) {
	mods := []model.Modification{
		{Entity: "src/app.go", Rev: "r1", Date: "2024-01-01", Author: "Alice"},
		{Entity: "src/Migrations/0001.go", Rev: "r2", Date: "2024-01-02", Author: "Bob"},
	}
	cfg := pipeline.Config{
		FilterSpec: mustFilter(t, nil, []string{"**/Migrations/**"}),
		GroupSpecs: mustSpecs(t, "src => SrcLayer"),
	}

	got, err := pipeline.Apply(mods, cfg)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (Migrations file filtered before grouping)", len(got))
	}
	if got[0].Entity != "SrcLayer" {
		t.Errorf("entity = %q, want SrcLayer", got[0].Entity)
	}
	if got[0].Rev != "r1" {
		t.Errorf("rev = %q, want r1 (the non-Migrations change)", got[0].Rev)
	}
}

// mustSpecs compiles a text-form grouping definition or fails the test.
func mustSpecs(t *testing.T, def string) []group.Spec {
	t.Helper()
	specs, err := group.Parse(strings.NewReader(def), "text")
	if err != nil {
		t.Fatalf("group.Parse(%q) returned error: %v", def, err)
	}
	return specs
}

// TestPipeline_NoOps asserts that an empty Config is a pass-through: every stage
// is skipped and the records are returned unchanged.
func TestPipeline_NoOps(t *testing.T) {
	mods := []model.Modification{
		{Entity: "src/a.go", Rev: "r1", Date: "2024-01-01", Author: "Alice"},
		{Entity: "docs/x.md", Rev: "r1", Date: "2024-01-01", Author: "Bob"},
	}

	got, err := pipeline.Apply(mods, pipeline.Config{})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !reflect.DeepEqual(got, mods) {
		t.Errorf("empty Config changed the records:\n got: %+v\nwant: %+v", got, mods)
	}
}

// TestPipeline_Order runs grouping and team mapping together and asserts both
// stages applied: unmatched entities are dropped and remapped to their layer,
// and every surviving author is substituted with their team.
func TestPipeline_Order(t *testing.T) {
	mods := []model.Modification{
		{Entity: "src/a.go", Rev: "r1", Date: "2024-01-01", Author: "Alice"},
		{Entity: "src/b.go", Rev: "r1", Date: "2024-01-01", Author: "Bob"},
		{Entity: "docs/x.md", Rev: "r1", Date: "2024-01-01", Author: "Alice"},
	}
	cfg := pipeline.Config{
		GroupSpecs: mustSpecs(t, "src => SrcLayer"),
		TeamMap:    map[string]string{"Alice": "TeamA", "Bob": "TeamB"},
	}

	got, err := pipeline.Apply(mods, cfg)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (docs/x.md dropped by grouping)", len(got))
	}

	wantAuthors := map[string]bool{"TeamA": true, "TeamB": true}
	for i, m := range got {
		if m.Entity != "SrcLayer" {
			t.Errorf("row %d entity = %q, want SrcLayer (grouping not applied)", i, m.Entity)
		}
		if !wantAuthors[m.Author] {
			t.Errorf("row %d author = %q, want a mapped team (team map not applied)", i, m.Author)
		}
	}
}

// TestPipeline_GroupThenTemporal proves grouping runs before temporal windowing:
// two distinct files that group to the same layer on the same day collapse to a
// single record in a 1-day window. Without grouping first, temporal dedup would
// keep them as two separate entities.
func TestPipeline_GroupThenTemporal(t *testing.T) {
	mods := []model.Modification{
		{Entity: "src/a.go", Rev: "r1", Date: "2024-01-01", Author: "Alice"},
		{Entity: "src/b.go", Rev: "r2", Date: "2024-01-01", Author: "Bob"},
	}
	cfg := pipeline.Config{
		GroupSpecs:     mustSpecs(t, "src => SrcLayer"),
		TemporalPeriod: 1,
	}

	got, err := pipeline.Apply(mods, cfg)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (both files grouped to SrcLayer, deduped in the window)", len(got))
	}
	if got[0].Entity != "SrcLayer" {
		t.Errorf("entity = %q, want SrcLayer", got[0].Entity)
	}
	if got[0].Rev != "2024-01-01" {
		t.Errorf("rev = %q, want the window's latest day 2024-01-01", got[0].Rev)
	}
}

// TestPipeline_TemporalError surfaces the temporal stage's failure: a record
// with a malformed date is propagated as a coded input error so the command
// layer can classify its exit code.
func TestPipeline_TemporalError(t *testing.T) {
	mods := []model.Modification{
		{Entity: "src/a.go", Rev: "r1", Date: "not-a-date", Author: "Alice"},
	}

	_, err := pipeline.Apply(mods, pipeline.Config{TemporalPeriod: 1})
	if err == nil {
		t.Fatal("Apply with an invalid date returned nil error, want failure")
	}
	var coded terr.Coded
	if !errors.As(err, &coded) {
		t.Fatalf("error is not coded: %v", err)
	}
	if coded.Code() != "invalid_temporal_date" || coded.ExitCode() != 3 {
		t.Errorf("error code/exit = %q/%d, want invalid_temporal_date/3", coded.Code(), coded.ExitCode())
	}
}
