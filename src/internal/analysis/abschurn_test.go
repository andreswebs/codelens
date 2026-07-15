package analysis

import (
	"errors"
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/analysis/churn"
	"github.com/andreswebs/codelens/internal/model"
)

// churnMod is a terse constructor for the Modification fields the churn
// analyses read: identity plus loc metrics (HasLoc set, matching a git2 log).
func churnMod(entity, rev, date, author string, added, deleted int) model.Modification {
	return model.Modification{
		Entity:     entity,
		Rev:        rev,
		Date:       date,
		Author:     author,
		LocAdded:   added,
		LocDeleted: deleted,
		HasLoc:     true,
	}
}

// absChurnRows asserts the result is an ok absolute-churn envelope and returns
// its rows as the concrete row type so cases can index into them.
func absChurnRows(t *testing.T, rows any) []absChurnRow {
	t.Helper()
	got, ok := rows.([]absChurnRow)
	if !ok {
		t.Fatalf("rows is %T, want []absChurnRow", rows)
	}
	return got
}

func TestAbsChurn_PerDateSums(t *testing.T) {
	// Two dates. On the first, two revisions (r1 twice, r2 once) sum their loc
	// and count as two distinct commits; on the second, one revision.
	mods := []model.Modification{
		churnMod("A", "r1", "2024-01-01", "x", 10, 2),
		churnMod("B", "r1", "2024-01-01", "x", 5, 1),
		churnMod("C", "r2", "2024-01-01", "y", 1, 3),
		churnMod("A", "r3", "2024-01-02", "y", 7, 0),
	}

	res, err := runAbsChurn(mods, Opts{})
	if err != nil {
		t.Fatalf("runAbsChurn returned error: %v", err)
	}

	rows := absChurnRows(t, res)
	want := []absChurnRow{
		{Date: "2024-01-01", Added: 16, Deleted: 6, Commits: 2},
		{Date: "2024-01-02", Added: 7, Deleted: 0, Commits: 1},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestAbsChurn_SortDateAsc(t *testing.T) {
	mods := []model.Modification{
		churnMod("A", "r3", "2024-03-01", "x", 1, 1),
		churnMod("A", "r1", "2024-01-01", "x", 1, 1),
		churnMod("A", "r2", "2024-02-01", "x", 1, 1),
	}

	res, err := runAbsChurn(mods, Opts{})
	if err != nil {
		t.Fatalf("runAbsChurn returned error: %v", err)
	}

	rows := absChurnRows(t, res)
	gotDates := []string{rows[0].Date, rows[1].Date, rows[2].Date}
	wantDates := []string{"2024-01-01", "2024-02-01", "2024-03-01"}
	if !reflect.DeepEqual(gotDates, wantDates) {
		t.Errorf("dates = %v, want %v", gotDates, wantDates)
	}
}

func TestAbsChurn_MissingMetrics(t *testing.T) {
	// A message-only log (no numstat) has modifications but no loc data, so the
	// churn guard rejects it with missing_metrics (exit code 3).
	mods := []model.Modification{
		{Entity: "A", Rev: "r1", Date: "2024-01-01", Author: "x", HasLoc: false},
	}

	_, err := runAbsChurn(mods, Opts{})
	if !errors.Is(err, churn.ErrMissingMetrics) {
		t.Fatalf("runAbsChurn error = %v, want ErrMissingMetrics", err)
	}
	if got := churn.ErrMissingMetrics.ExitCode(); got != 3 {
		t.Errorf("ErrMissingMetrics exit code = %d, want 3", got)
	}
}

func TestAbsChurn_DescriptorRegistered(t *testing.T) {
	d := absChurnDescriptor()
	if d.Name != "absolute-churn" {
		t.Errorf("Name = %q, want %q", d.Name, "absolute-churn")
	}
	if !reflect.DeepEqual(d.Aliases, []string{"abs-churn"}) {
		t.Errorf("Aliases = %v, want [abs-churn]", d.Aliases)
	}
	wantCols := []string{"date", "added", "deleted", "commits"}
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
