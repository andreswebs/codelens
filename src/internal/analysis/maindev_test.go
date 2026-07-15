package analysis

import (
	"errors"
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/analysis/churn"
	"github.com/andreswebs/codelens/internal/model"
)

// mainDevRows asserts the result is an ok main-developer envelope and returns
// its rows as the concrete row type so cases can index into them.
func mainDevRows(t *testing.T, rows any) []mainDevRow {
	t.Helper()
	got, ok := rows.([]mainDevRow)
	if !ok {
		t.Fatalf("rows is %T, want []mainDevRow", rows)
	}
	return got
}

func TestMainDev_PicksMaxAdder(t *testing.T) {
	// alice adds 15 lines to A over two revisions, bob adds 1; alice is the main
	// developer. total_added is the sum over all authors (16).
	mods := []model.Modification{
		churnMod("A", "r1", "2024-01-01", "alice", 10, 2),
		churnMod("A", "r2", "2024-01-02", "alice", 5, 0),
		churnMod("A", "r3", "2024-01-03", "bob", 1, 4),
	}

	res, err := runMainDev(mods, Opts{})
	if err != nil {
		t.Fatalf("runMainDev returned error: %v", err)
	}

	rows := mainDevRows(t, res)
	want := []mainDevRow{
		{Entity: "A", MainDev: "alice", Added: 15, TotalAdded: 16, Ownership: 0.94},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestMainDev_TieBreaksAuthorAsc(t *testing.T) {
	// carol and alice each add 5 lines to A. The main developer is picked by max
	// added, and ties resolve to the lexicographically first author (alice),
	// since contributions arrive in ascending author order.
	mods := []model.Modification{
		churnMod("A", "r1", "2024-01-01", "carol", 5, 0),
		churnMod("A", "r2", "2024-01-02", "alice", 5, 0),
	}

	res, err := runMainDev(mods, Opts{})
	if err != nil {
		t.Fatalf("runMainDev returned error: %v", err)
	}

	rows := mainDevRows(t, res)
	if rows[0].MainDev != "alice" {
		t.Errorf("MainDev = %q, want %q (tie broken by ascending author)", rows[0].MainDev, "alice")
	}
}

func TestMainDev_OwnershipCentiRatio(t *testing.T) {
	// Ownership is the main developer's added lines over the entity total,
	// rounded to two significant digits (ratio->centi-float-precision):
	// 164 / 245 = 0.6693... -> 0.67.
	mods := []model.Modification{
		churnMod("A", "r1", "2024-01-01", "alice", 164, 0),
		churnMod("A", "r2", "2024-01-02", "bob", 81, 0),
	}

	res, err := runMainDev(mods, Opts{})
	if err != nil {
		t.Fatalf("runMainDev returned error: %v", err)
	}

	rows := mainDevRows(t, res)
	got := rows[0]
	if got.MainDev != "alice" || got.Added != 164 || got.TotalAdded != 245 {
		t.Fatalf("row = %+v, want main-dev alice added 164 total 245", got)
	}
	if got.Ownership != 0.67 {
		t.Errorf("Ownership = %v, want 0.67", got.Ownership)
	}
}

func TestMainDev_SortEntityAsc(t *testing.T) {
	mods := []model.Modification{
		churnMod("charlie.go", "r1", "2024-01-01", "x", 3, 0),
		churnMod("alpha.go", "r2", "2024-01-01", "x", 1, 0),
		churnMod("bravo.go", "r3", "2024-01-01", "x", 2, 0),
	}

	res, err := runMainDev(mods, Opts{})
	if err != nil {
		t.Fatalf("runMainDev returned error: %v", err)
	}

	rows := mainDevRows(t, res)
	gotEntities := []string{rows[0].Entity, rows[1].Entity, rows[2].Entity}
	wantEntities := []string{"alpha.go", "bravo.go", "charlie.go"}
	if !reflect.DeepEqual(gotEntities, wantEntities) {
		t.Errorf("entities = %v, want %v", gotEntities, wantEntities)
	}
}

func TestMainDev_MissingMetrics(t *testing.T) {
	// A message-only log (no numstat) has modifications but no loc data, so the
	// churn guard rejects it with missing_metrics (exit code 3).
	mods := []model.Modification{
		{Entity: "A", Rev: "r1", Date: "2024-01-01", Author: "x", HasLoc: false},
	}

	_, err := runMainDev(mods, Opts{})
	if !errors.Is(err, churn.ErrMissingMetrics) {
		t.Fatalf("runMainDev error = %v, want ErrMissingMetrics", err)
	}
	if got := churn.ErrMissingMetrics.ExitCode(); got != 3 {
		t.Errorf("ErrMissingMetrics exit code = %d, want 3", got)
	}
}

func TestMainDev_DescriptorRegistered(t *testing.T) {
	d := mainDevDescriptor()
	if d.Name != "main-developer" {
		t.Errorf("Name = %q, want %q", d.Name, "main-developer")
	}
	if len(d.Aliases) != 1 || d.Aliases[0] != "main-dev" {
		t.Errorf("Aliases = %v, want [main-dev]", d.Aliases)
	}
	wantCols := []string{"entity", "main_dev", "added", "total_added", "ownership"}
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
