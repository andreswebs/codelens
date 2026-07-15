package analysis

import (
	"errors"
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/analysis/churn"
	"github.com/andreswebs/codelens/internal/model"
)

// authorChurnRows asserts the result is an ok author-churn envelope and returns
// its rows as the concrete row type so cases can index into them.
func authorChurnRows(t *testing.T, rows any) []authorChurnRow {
	t.Helper()
	got, ok := rows.([]authorChurnRow)
	if !ok {
		t.Fatalf("rows is %T, want []authorChurnRow", rows)
	}
	return got
}

func TestAuthorChurn_PerAuthorSums(t *testing.T) {
	// Author x makes two distinct revisions (r1 touches two entities, r3 one);
	// author y makes one. Loc sums roll up per author and commits count distinct
	// revisions, so x's shared revision r1 counts once.
	mods := []model.Modification{
		churnMod("A", "r1", "2024-01-01", "x", 10, 2),
		churnMod("B", "r1", "2024-01-01", "x", 5, 1),
		churnMod("C", "r2", "2024-01-01", "y", 1, 3),
		churnMod("A", "r3", "2024-01-02", "x", 7, 0),
	}

	res, err := runAuthorChurn(mods, Opts{})
	if err != nil {
		t.Fatalf("runAuthorChurn returned error: %v", err)
	}

	rows := authorChurnRows(t, res)
	want := []authorChurnRow{
		{Author: "x", Added: 22, Deleted: 3, Commits: 2},
		{Author: "y", Added: 1, Deleted: 3, Commits: 1},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestAuthorChurn_SortAuthorAsc(t *testing.T) {
	// Authors are emitted in ascending name order; the added tiebreaker only
	// matters when two rows share an author, which cannot happen after grouping,
	// so this pins the primary key ordering.
	mods := []model.Modification{
		churnMod("A", "r1", "2024-01-01", "carol", 1, 1),
		churnMod("A", "r2", "2024-01-01", "alice", 1, 1),
		churnMod("A", "r3", "2024-01-01", "bob", 1, 1),
	}

	res, err := runAuthorChurn(mods, Opts{})
	if err != nil {
		t.Fatalf("runAuthorChurn returned error: %v", err)
	}

	rows := authorChurnRows(t, res)
	gotAuthors := []string{rows[0].Author, rows[1].Author, rows[2].Author}
	wantAuthors := []string{"alice", "bob", "carol"}
	if !reflect.DeepEqual(gotAuthors, wantAuthors) {
		t.Errorf("authors = %v, want %v", gotAuthors, wantAuthors)
	}
}

func TestAuthorChurn_MissingMetrics(t *testing.T) {
	// A message-only log (no numstat) has modifications but no loc data, so the
	// churn guard rejects it with missing_metrics (exit code 3).
	mods := []model.Modification{
		{Entity: "A", Rev: "r1", Date: "2024-01-01", Author: "x", HasLoc: false},
	}

	_, err := runAuthorChurn(mods, Opts{})
	if !errors.Is(err, churn.ErrMissingMetrics) {
		t.Fatalf("runAuthorChurn error = %v, want ErrMissingMetrics", err)
	}
	if got := churn.ErrMissingMetrics.ExitCode(); got != 3 {
		t.Errorf("ErrMissingMetrics exit code = %d, want 3", got)
	}
}

func TestAuthorChurn_DescriptorRegistered(t *testing.T) {
	d := authorChurnDescriptor()
	if d.Name != "author-churn" {
		t.Errorf("Name = %q, want %q", d.Name, "author-churn")
	}
	if len(d.Aliases) != 0 {
		t.Errorf("Aliases = %v, want none", d.Aliases)
	}
	wantCols := []string{"author", "added", "deleted", "commits"}
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
