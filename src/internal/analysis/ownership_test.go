package analysis

import (
	"errors"
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/analysis/churn"
	"github.com/andreswebs/codelens/internal/model"
)

// ownershipRows asserts the result is an ok entity-ownership envelope and
// returns its rows as the concrete row type so cases can index into them.
func ownershipRows(t *testing.T, rows any) []ownershipRow {
	t.Helper()
	got, ok := rows.([]ownershipRow)
	if !ok {
		t.Fatalf("rows is %T, want []ownershipRow", rows)
	}
	return got
}

func TestOwnership_PerEntityPerAuthorRows(t *testing.T) {
	// Entity A is touched by two authors (alice, bob); each author's loc rolls up
	// across their revisions of the entity, yielding one row per (entity, author).
	mods := []model.Modification{
		churnMod("A", "r1", "2024-01-01", "alice", 10, 2),
		churnMod("A", "r2", "2024-01-02", "bob", 5, 1),
		churnMod("A", "r3", "2024-01-03", "alice", 3, 0),
		churnMod("B", "r4", "2024-01-04", "bob", 7, 4),
	}

	res, err := runOwnership(mods, Opts{})
	if err != nil {
		t.Fatalf("runOwnership returned error: %v", err)
	}

	rows := ownershipRows(t, res)
	want := []ownershipRow{
		{Entity: "A", Author: "alice", Added: 13, Deleted: 2},
		{Entity: "A", Author: "bob", Added: 5, Deleted: 1},
		{Entity: "B", Author: "bob", Added: 7, Deleted: 4},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestOwnership_SortEntityAsc(t *testing.T) {
	// Rows are ordered by entity ascending; within an entity, authors keep
	// ascending order, so the output is fully deterministic regardless of the
	// order modifications arrive in.
	mods := []model.Modification{
		churnMod("zeta", "r1", "2024-01-01", "carol", 1, 0),
		churnMod("alpha", "r2", "2024-01-01", "bob", 2, 0),
		churnMod("alpha", "r3", "2024-01-01", "alice", 4, 0),
		churnMod("mid", "r4", "2024-01-01", "dave", 3, 0),
	}

	res, err := runOwnership(mods, Opts{})
	if err != nil {
		t.Fatalf("runOwnership returned error: %v", err)
	}

	rows := ownershipRows(t, res)
	type pair struct{ entity, author string }
	got := make([]pair, len(rows))
	for i, r := range rows {
		got[i] = pair{r.Entity, r.Author}
	}
	want := []pair{
		{"alpha", "alice"},
		{"alpha", "bob"},
		{"mid", "dave"},
		{"zeta", "carol"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entity/author order = %v, want %v", got, want)
	}
}

func TestOwnership_MissingMetrics(t *testing.T) {
	// A message-only log (no numstat) has modifications but no loc data, so the
	// churn guard rejects it with missing_metrics (exit code 3).
	mods := []model.Modification{
		{Entity: "A", Rev: "r1", Date: "2024-01-01", Author: "x", HasLoc: false},
	}

	_, err := runOwnership(mods, Opts{})
	if !errors.Is(err, churn.ErrMissingMetrics) {
		t.Fatalf("runOwnership error = %v, want ErrMissingMetrics", err)
	}
	if got := churn.ErrMissingMetrics.ExitCode(); got != 3 {
		t.Errorf("ErrMissingMetrics exit code = %d, want 3", got)
	}
}

func TestOwnership_DescriptorRegistered(t *testing.T) {
	d := ownershipDescriptor()
	if d.Name != "entity-ownership" {
		t.Errorf("Name = %q, want %q", d.Name, "entity-ownership")
	}
	if len(d.Aliases) != 0 {
		t.Errorf("Aliases = %v, want none", d.Aliases)
	}
	wantCols := []string{"entity", "author", "added", "deleted"}
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
