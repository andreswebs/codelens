package effort

import (
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// TestEffort_TotalRevsIsEntityRows verifies that TotalRevs is the number of
// rows in the entity group (the original's nrows), repeated on every author row
// of that entity. An entity with three rows yields TotalRevs=3 for each of its
// authors regardless of how the rows split across authors.
func TestEffort_TotalRevsIsEntityRows(t *testing.T) {
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice"},
		{Entity: "a.txt", Rev: "r2", Author: "alice"},
		{Entity: "a.txt", Rev: "r3", Author: "bob"},
	}

	got := ByEntity(mods)
	if len(got) != 1 {
		t.Fatalf("ByEntity() returned %d entities, want 1", len(got))
	}
	for _, ar := range got[0].Authors {
		if ar.TotalRevs != 3 {
			t.Errorf("author %q TotalRevs = %d, want 3", ar.Author, ar.TotalRevs)
		}
	}
}

// TestEffort_PerAuthorRevs verifies that each author's Revs is the number of
// rows that author contributed within the entity, and that entities and authors
// come back in ascending key order for deterministic downstream output.
func TestEffort_PerAuthorRevs(t *testing.T) {
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice"},
		{Entity: "a.txt", Rev: "r2", Author: "alice"},
		{Entity: "a.txt", Rev: "r3", Author: "bob"},
		{Entity: "b.txt", Rev: "r4", Author: "bob"},
	}

	got := ByEntity(mods)
	want := []EntityEffort{
		{Entity: "a.txt", Authors: []AuthorRevs{
			{Author: "alice", Revs: 2, TotalRevs: 3},
			{Author: "bob", Revs: 1, TotalRevs: 3},
		}},
		{Entity: "b.txt", Authors: []AuthorRevs{
			{Author: "bob", Revs: 1, TotalRevs: 1},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ByEntity() = %+v, want %+v", got, want)
	}
}

// TestEffort_CountsRowsNotDistinctRevs verifies that both Revs and TotalRevs
// count rows, not distinct revisions: two rows sharing one revision (e.g. the
// same file listed twice in a change set) count as two, matching the original's
// row-based nrows/frequencies semantics.
func TestEffort_CountsRowsNotDistinctRevs(t *testing.T) {
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice"},
		{Entity: "a.txt", Rev: "r1", Author: "alice"},
	}

	got := ByEntity(mods)
	want := []EntityEffort{
		{Entity: "a.txt", Authors: []AuthorRevs{
			{Author: "alice", Revs: 2, TotalRevs: 2},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ByEntity() = %+v, want %+v", got, want)
	}
}

// TestEffort_Empty verifies that an empty input yields no entities rather than a
// nil-versus-empty mismatch downstream.
func TestEffort_Empty(t *testing.T) {
	if got := ByEntity(nil); len(got) != 0 {
		t.Errorf("ByEntity(nil) = %+v, want empty", got)
	}
}
