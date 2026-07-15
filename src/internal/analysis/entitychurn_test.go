package analysis

import (
	"errors"
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/analysis/churn"
	"github.com/andreswebs/codelens/internal/model"
)

// entityChurnRows asserts the result is an ok entity-churn envelope and returns
// its rows as the concrete row type so cases can index into them.
func entityChurnRows(t *testing.T, rows any) []entityChurnRow {
	t.Helper()
	got, ok := rows.([]entityChurnRow)
	if !ok {
		t.Fatalf("rows is %T, want []entityChurnRow", rows)
	}
	return got
}

func TestEntityChurn_PerEntitySums(t *testing.T) {
	// Entity A is touched by two distinct revisions (r1, r3); B and C once each.
	// Loc sums roll up per entity and commits count distinct revisions, so A's
	// two revisions count as two commits.
	mods := []model.Modification{
		churnMod("A", "r1", "2024-01-01", "x", 10, 2),
		churnMod("B", "r1", "2024-01-01", "x", 5, 1),
		churnMod("C", "r2", "2024-01-01", "y", 1, 3),
		churnMod("A", "r3", "2024-01-02", "x", 7, 0),
	}

	res, err := runEntityChurn(mods, Opts{})
	if err != nil {
		t.Fatalf("runEntityChurn returned error: %v", err)
	}

	rows := entityChurnRows(t, res)
	want := []entityChurnRow{
		{Entity: "A", Added: 17, Deleted: 2, Commits: 2},
		{Entity: "B", Added: 5, Deleted: 1, Commits: 1},
		{Entity: "C", Added: 1, Deleted: 3, Commits: 1},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestEntityChurn_SortAddedDesc(t *testing.T) {
	// Rows are ordered by added lines descending; entities with equal added lines
	// keep ascending entity order (stable over the ascending-key grouping).
	mods := []model.Modification{
		churnMod("low", "r1", "2024-01-01", "x", 1, 0),
		churnMod("high", "r2", "2024-01-01", "x", 100, 0),
		churnMod("mid", "r3", "2024-01-01", "x", 50, 0),
		churnMod("tieB", "r4", "2024-01-01", "x", 50, 0),
	}

	res, err := runEntityChurn(mods, Opts{})
	if err != nil {
		t.Fatalf("runEntityChurn returned error: %v", err)
	}

	rows := entityChurnRows(t, res)
	gotEntities := []string{rows[0].Entity, rows[1].Entity, rows[2].Entity, rows[3].Entity}
	wantEntities := []string{"high", "mid", "tieB", "low"}
	if !reflect.DeepEqual(gotEntities, wantEntities) {
		t.Errorf("entities = %v, want %v", gotEntities, wantEntities)
	}
}

func TestEntityChurn_MissingMetrics(t *testing.T) {
	// A message-only log (no numstat) has modifications but no loc data, so the
	// churn guard rejects it with missing_metrics (exit code 3).
	mods := []model.Modification{
		{Entity: "A", Rev: "r1", Date: "2024-01-01", Author: "x", HasLoc: false},
	}

	_, err := runEntityChurn(mods, Opts{})
	if !errors.Is(err, churn.ErrMissingMetrics) {
		t.Fatalf("runEntityChurn error = %v, want ErrMissingMetrics", err)
	}
	if got := churn.ErrMissingMetrics.ExitCode(); got != 3 {
		t.Errorf("ErrMissingMetrics exit code = %d, want 3", got)
	}
}

func TestEntityChurn_DescriptorRegistered(t *testing.T) {
	d := entityChurnDescriptor()
	if d.Name != "entity-churn" {
		t.Errorf("Name = %q, want %q", d.Name, "entity-churn")
	}
	if len(d.Aliases) != 0 {
		t.Errorf("Aliases = %v, want none", d.Aliases)
	}
	wantCols := []string{"entity", "added", "deleted", "commits"}
	if len(d.RowSchema) != len(wantCols) {
		t.Fatalf("RowSchema has %d columns, want %d", len(d.RowSchema), len(wantCols))
	}
	for i, name := range wantCols {
		if d.RowSchema[i].Name != name {
			t.Errorf("RowSchema[%d].Name = %q, want %q", i, d.RowSchema[i].Name, name)
		}
	}
	if d.Run == nil {
		t.Error("Run is nil")
	}
}
