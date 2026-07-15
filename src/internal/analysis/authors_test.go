package analysis

import (
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// mod is a terse constructor for the Modification fields the authors analysis
// reads; the churn/loc fields are irrelevant here and left zero.
func mod(entity, rev, author string) model.Modification {
	return model.Modification{Entity: entity, Rev: rev, Author: author}
}

// authorRows asserts the result is an ok authors envelope and returns its rows
// as the concrete row type so cases can index into them.
func authorRows(t *testing.T, rows any) []authorsRow {
	t.Helper()
	got, ok := rows.([]authorsRow)
	if !ok {
		t.Fatalf("rows is %T, want []authorsRow", rows)
	}
	return got
}

func TestAuthors_CountsDistinctAuthors(t *testing.T) {
	mods := []model.Modification{
		mod("A", "r1", "x"),
		mod("A", "r2", "x"),
		mod("A", "r3", "y"),
	}

	res, err := runAuthors(mods, Opts{})
	if err != nil {
		t.Fatalf("runAuthors returned error: %v", err)
	}

	rows := authorRows(t, res)
	want := []authorsRow{{Entity: "A", NAuthors: 2, NRevs: 3}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestAuthors_MultipleEntities_SortDesc(t *testing.T) {
	mods := []model.Modification{
		mod("solo", "r1", "x"),
		mod("crowd", "r2", "a"),
		mod("crowd", "r3", "b"),
		mod("crowd", "r4", "c"),
	}

	res, err := runAuthors(mods, Opts{})
	if err != nil {
		t.Fatalf("runAuthors returned error: %v", err)
	}

	rows := authorRows(t, res)
	want := []authorsRow{
		{Entity: "crowd", NAuthors: 3, NRevs: 3},
		{Entity: "solo", NAuthors: 1, NRevs: 1},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestAuthors_TieBrokenByRevsThenEntity(t *testing.T) {
	// zeta and alpha each have one author; zeta has more revisions so it sorts
	// first. beta ties alpha on both author and rev count, so entity name
	// breaks the tie (alpha before beta).
	mods := []model.Modification{
		mod("alpha", "r1", "x"),
		mod("beta", "r2", "x"),
		mod("zeta", "r3", "y"),
		mod("zeta", "r4", "y"),
	}

	res, err := runAuthors(mods, Opts{})
	if err != nil {
		t.Fatalf("runAuthors returned error: %v", err)
	}

	rows := authorRows(t, res)
	want := []authorsRow{
		{Entity: "zeta", NAuthors: 1, NRevs: 2},
		{Entity: "alpha", NAuthors: 1, NRevs: 1},
		{Entity: "beta", NAuthors: 1, NRevs: 1},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestAuthors_Empty(t *testing.T) {
	res, err := runAuthors(nil, Opts{})
	if err != nil {
		t.Fatalf("runAuthors returned error: %v", err)
	}

	rows := authorRows(t, res)
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want empty", rows)
	}
}

func TestAuthors_DescriptorRegistered(t *testing.T) {
	d := authorsDescriptor()
	if d.Name != "authors" {
		t.Errorf("Name = %q, want %q", d.Name, "authors")
	}
	if len(d.Aliases) != 0 {
		t.Errorf("Aliases = %v, want none", d.Aliases)
	}
	wantCols := []string{"entity", "n_authors", "n_revs"}
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
