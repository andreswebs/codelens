package analysis

import (
	"errors"
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/analysis/churn"
	"github.com/andreswebs/codelens/internal/model"
)

// refMainDevRows asserts the result is an ok refactoring-main-developer
// envelope and returns its rows as the concrete row type so cases can index
// into them.
func refMainDevRows(t *testing.T, rows any) []refMainDevRow {
	t.Helper()
	got, ok := rows.([]refMainDevRow)
	if !ok {
		t.Fatalf("rows is %T, want []refMainDevRow", rows)
	}
	return got
}

func TestRefMainDev_PicksMaxRemover(t *testing.T) {
	// bob removes 20 lines from A over two revisions, alice removes 3; bob is the
	// refactoring main developer. total_removed is the sum over all authors (23).
	// The main developer is ranked by DELETED lines, so alice's larger additions
	// do not matter.
	mods := []model.Modification{
		churnMod("A", "r1", "2024-01-01", "alice", 100, 3),
		churnMod("A", "r2", "2024-01-02", "bob", 0, 12),
		churnMod("A", "r3", "2024-01-03", "bob", 0, 8),
	}

	res, err := runRefMainDev(mods, Opts{})
	if err != nil {
		t.Fatalf("runRefMainDev returned error: %v", err)
	}

	rows := refMainDevRows(t, res)
	want := []refMainDevRow{
		{Entity: "A", MainDev: "bob", Removed: 20, TotalRemoved: 23, Ownership: 0.87},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestRefMainDev_TieBreaksAuthorAsc(t *testing.T) {
	// carol and alice each remove 5 lines from A. The main developer is picked by
	// max removed, and ties resolve to the lexicographically first author
	// (alice), since contributions arrive in ascending author order.
	mods := []model.Modification{
		churnMod("A", "r1", "2024-01-01", "carol", 0, 5),
		churnMod("A", "r2", "2024-01-02", "alice", 0, 5),
	}

	res, err := runRefMainDev(mods, Opts{})
	if err != nil {
		t.Fatalf("runRefMainDev returned error: %v", err)
	}

	rows := refMainDevRows(t, res)
	if rows[0].MainDev != "alice" {
		t.Errorf("MainDev = %q, want %q (tie broken by ascending author)", rows[0].MainDev, "alice")
	}
}

func TestRefMainDev_Ownership(t *testing.T) {
	// Ownership is the main developer's removed lines over the entity total,
	// rounded to two significant digits (ratio->centi-float-precision):
	// 164 / 245 = 0.6693... -> 0.67.
	mods := []model.Modification{
		churnMod("A", "r1", "2024-01-01", "alice", 0, 164),
		churnMod("A", "r2", "2024-01-02", "bob", 0, 81),
	}

	res, err := runRefMainDev(mods, Opts{})
	if err != nil {
		t.Fatalf("runRefMainDev returned error: %v", err)
	}

	rows := refMainDevRows(t, res)
	got := rows[0]
	if got.MainDev != "alice" || got.Removed != 164 || got.TotalRemoved != 245 {
		t.Fatalf("row = %+v, want main-dev alice removed 164 total 245", got)
	}
	if got.Ownership != 0.67 {
		t.Errorf("Ownership = %v, want 0.67", got.Ownership)
	}
}

func TestRefMainDev_SortEntityAsc(t *testing.T) {
	mods := []model.Modification{
		churnMod("charlie.go", "r1", "2024-01-01", "x", 0, 3),
		churnMod("alpha.go", "r2", "2024-01-01", "x", 0, 1),
		churnMod("bravo.go", "r3", "2024-01-01", "x", 0, 2),
	}

	res, err := runRefMainDev(mods, Opts{})
	if err != nil {
		t.Fatalf("runRefMainDev returned error: %v", err)
	}

	rows := refMainDevRows(t, res)
	gotEntities := []string{rows[0].Entity, rows[1].Entity, rows[2].Entity}
	wantEntities := []string{"alpha.go", "bravo.go", "charlie.go"}
	if !reflect.DeepEqual(gotEntities, wantEntities) {
		t.Errorf("entities = %v, want %v", gotEntities, wantEntities)
	}
}

func TestRefMainDev_MissingMetrics(t *testing.T) {
	// A message-only log (no numstat) has modifications but no loc data, so the
	// churn guard rejects it with missing_metrics (exit code 3).
	mods := []model.Modification{
		{Entity: "A", Rev: "r1", Date: "2024-01-01", Author: "x", HasLoc: false},
	}

	_, err := runRefMainDev(mods, Opts{})
	if !errors.Is(err, churn.ErrMissingMetrics) {
		t.Fatalf("runRefMainDev error = %v, want ErrMissingMetrics", err)
	}
	if got := churn.ErrMissingMetrics.ExitCode(); got != 3 {
		t.Errorf("ErrMissingMetrics exit code = %d, want 3", got)
	}
}

func TestRefMainDev_DescriptorRegistered(t *testing.T) {
	d := refMainDevDescriptor()
	if d.Name != "refactoring-main-developer" {
		t.Errorf("Name = %q, want %q", d.Name, "refactoring-main-developer")
	}
	if len(d.Aliases) != 1 || d.Aliases[0] != "refactoring-main-dev" {
		t.Errorf("Aliases = %v, want [refactoring-main-dev]", d.Aliases)
	}
	wantCols := []string{"entity", "main_dev", "removed", "total_removed", "ownership"}
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
