package analysis

import (
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// entityEffortRows asserts the result is an ok entity-effort envelope and
// returns its rows as the concrete row type so cases can index into them.
func entityEffortRows(t *testing.T, rows any) []entityEffortRow {
	t.Helper()
	got, ok := rows.([]entityEffortRow)
	if !ok {
		t.Fatalf("rows is %T, want []entityEffortRow", rows)
	}
	return got
}

func TestEntityEffort_Rows(t *testing.T) {
	// A single entity touched by two authors yields one row per author, each
	// carrying that author's revision count and the entity total (3).
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice"},
		{Entity: "a.txt", Rev: "r2", Author: "alice"},
		{Entity: "a.txt", Rev: "r3", Author: "bob"},
	}

	res, err := runEntityEffort(mods, Opts{})
	if err != nil {
		t.Fatalf("runEntityEffort returned error: %v", err)
	}

	rows := entityEffortRows(t, res)
	want := []entityEffortRow{
		{Entity: "a.txt", Author: "alice", AuthorRevs: 2, TotalRevs: 3},
		{Entity: "a.txt", Author: "bob", AuthorRevs: 1, TotalRevs: 3},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestEntityEffort_SortEntityThenRevsDesc(t *testing.T) {
	// Rows sort by entity ascending; within an entity, by author revs
	// descending. For b.txt, bob (2 revs) precedes alice (1 rev) even though
	// bob is lexicographically later.
	mods := []model.Modification{
		{Entity: "b.txt", Rev: "r1", Author: "alice"},
		{Entity: "b.txt", Rev: "r2", Author: "bob"},
		{Entity: "b.txt", Rev: "r3", Author: "bob"},
		{Entity: "a.txt", Rev: "r4", Author: "carol"},
	}

	res, err := runEntityEffort(mods, Opts{})
	if err != nil {
		t.Fatalf("runEntityEffort returned error: %v", err)
	}

	rows := entityEffortRows(t, res)
	want := []entityEffortRow{
		{Entity: "a.txt", Author: "carol", AuthorRevs: 1, TotalRevs: 1},
		{Entity: "b.txt", Author: "bob", AuthorRevs: 2, TotalRevs: 3},
		{Entity: "b.txt", Author: "alice", AuthorRevs: 1, TotalRevs: 3},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestEntityEffort_TieBreaksAuthorAsc(t *testing.T) {
	// When two authors have equal revs within an entity, the stable sort keeps
	// them in ascending author order (alice before bob).
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "bob"},
		{Entity: "a.txt", Rev: "r2", Author: "alice"},
	}

	res, err := runEntityEffort(mods, Opts{})
	if err != nil {
		t.Fatalf("runEntityEffort returned error: %v", err)
	}

	rows := entityEffortRows(t, res)
	gotAuthors := []string{rows[0].Author, rows[1].Author}
	wantAuthors := []string{"alice", "bob"}
	if !reflect.DeepEqual(gotAuthors, wantAuthors) {
		t.Errorf("authors = %v, want %v (tie broken by ascending author)", gotAuthors, wantAuthors)
	}
}

func TestEntityEffort_Empty(t *testing.T) {
	res, err := runEntityEffort(nil, Opts{})
	if err != nil {
		t.Fatalf("runEntityEffort returned error: %v", err)
	}
	rows := entityEffortRows(t, res)
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want empty", rows)
	}
}

func TestEntityEffort_DescriptorRegistered(t *testing.T) {
	d := entityEffortDescriptor()
	if d.Name != "entity-effort" {
		t.Errorf("Name = %q, want %q", d.Name, "entity-effort")
	}
	if len(d.Aliases) != 0 {
		t.Errorf("Aliases = %v, want none", d.Aliases)
	}
	wantCols := []string{"entity", "author", "author_revs", "total_revs"}
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
