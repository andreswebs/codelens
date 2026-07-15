package churn

import (
	"errors"
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// TestRequireLoc_ErrorsWhenAbsent verifies the metrics guard: a log whose
// modifications all lack loc data (a message-only log) is rejected with
// ErrMissingMetrics, whose exit code is 3 (input error).
func TestRequireLoc_ErrorsWhenAbsent(t *testing.T) {
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "1", Author: "alice", HasLoc: false},
		{Entity: "b.txt", Rev: "1", Author: "alice", HasLoc: false},
	}

	err := RequireLoc(mods)
	if !errors.Is(err, ErrMissingMetrics) {
		t.Fatalf("RequireLoc() error = %v, want ErrMissingMetrics", err)
	}
	if got := ErrMissingMetrics.ExitCode(); got != 3 {
		t.Errorf("ErrMissingMetrics exit code = %d, want 3", got)
	}
}

// TestRequireLoc_OKWhenPresent verifies the guard passes when at least one
// modification carries loc data (the git2 case), and also that a genuinely
// empty log is not a metrics error (empty handling belongs upstream).
func TestRequireLoc_OKWhenPresent(t *testing.T) {
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "1", Author: "alice", HasLoc: false},
		{Entity: "b.txt", Rev: "1", Author: "alice", LocAdded: 3, HasLoc: true},
	}
	if err := RequireLoc(mods); err != nil {
		t.Errorf("RequireLoc() with loc present = %v, want nil", err)
	}
	if err := RequireLoc(nil); err != nil {
		t.Errorf("RequireLoc(nil) = %v, want nil", err)
	}
}

// TestSumByGroup_SumsAndDistinctCommits verifies per-group added/deleted sums
// and that Commits counts distinct revisions (two revs touching a group -> 2),
// with groups returned in ascending key order.
func TestSumByGroup_SumsAndDistinctCommits(t *testing.T) {
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice", LocAdded: 10, LocDeleted: 2, HasLoc: true},
		{Entity: "a.txt", Rev: "r1", Author: "alice", LocAdded: 5, LocDeleted: 1, HasLoc: true},
		{Entity: "a.txt", Rev: "r2", Author: "bob", LocAdded: 1, LocDeleted: 3, HasLoc: true},
		{Entity: "b.txt", Rev: "r2", Author: "bob", LocAdded: 7, LocDeleted: 0, HasLoc: true},
	}

	got := SumByGroup(mods, func(m model.Modification) string { return m.Entity })
	want := []GroupChurn{
		{Group: "a.txt", Added: 16, Deleted: 6, Commits: 2},
		{Group: "b.txt", Added: 7, Deleted: 0, Commits: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SumByGroup() = %+v, want %+v", got, want)
	}
}

// TestSumByGroup_BinaryCountsZero verifies binary rows (added/deleted
// normalized to 0 by the parser) contribute nothing to the loc sums while
// still counting toward the distinct-commit total.
func TestSumByGroup_BinaryCountsZero(t *testing.T) {
	mods := []model.Modification{
		{Entity: "img.png", Rev: "r1", Author: "alice", Binary: true, HasLoc: true},
		{Entity: "img.png", Rev: "r2", Author: "bob", LocAdded: 4, LocDeleted: 1, HasLoc: true},
	}

	got := SumByGroup(mods, func(m model.Modification) string { return m.Entity })
	want := []GroupChurn{
		{Group: "img.png", Added: 4, Deleted: 1, Commits: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SumByGroup() = %+v, want %+v", got, want)
	}
}

// TestByEntityAuthorContrib verifies grouping by entity then by author, with
// added/deleted summed per (entity, author) and both levels in ascending key
// order.
func TestByEntityAuthorContrib(t *testing.T) {
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice", LocAdded: 10, LocDeleted: 2, HasLoc: true},
		{Entity: "a.txt", Rev: "r2", Author: "alice", LocAdded: 5, LocDeleted: 0, HasLoc: true},
		{Entity: "a.txt", Rev: "r3", Author: "bob", LocAdded: 1, LocDeleted: 4, HasLoc: true},
		{Entity: "b.txt", Rev: "r4", Author: "bob", LocAdded: 7, LocDeleted: 3, HasLoc: true},
	}

	got := ByEntityAuthorContrib(mods)
	want := []EntityContribs{
		{Entity: "a.txt", Contribs: []AuthorContrib{
			{Author: "alice", Added: 15, Deleted: 2},
			{Author: "bob", Added: 1, Deleted: 4},
		}},
		{Entity: "b.txt", Contribs: []AuthorContrib{
			{Author: "bob", Added: 7, Deleted: 3},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ByEntityAuthorContrib() = %+v, want %+v", got, want)
	}
}
