package analysis

import (
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// revisionRows asserts the result is an ok revisions envelope and returns its
// rows as the concrete row type so cases can index into them.
func revisionRows(t *testing.T, rows any) []revisionsRow {
	t.Helper()
	got, ok := rows.([]revisionsRow)
	if !ok {
		t.Fatalf("rows is %T, want []revisionsRow", rows)
	}
	return got
}

func TestRevisions_CountsDistinctRevs(t *testing.T) {
	// Three rows across two distinct revisions of the same entity: the same
	// revision touching the entity twice must not be double-counted.
	mods := []model.Modification{
		mod("A", "r1", "x"),
		mod("A", "r1", "x"),
		mod("A", "r2", "y"),
	}

	res, err := runRevisions(mods, Opts{})
	if err != nil {
		t.Fatalf("runRevisions returned error: %v", err)
	}

	rows := revisionRows(t, res)
	want := []revisionsRow{{Entity: "A", NRevs: 2}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestRevisions_SortDescTieByEntity(t *testing.T) {
	// hot has two revisions and sorts first. alpha and beta each have one
	// revision, so entity name breaks the tie (alpha before beta).
	mods := []model.Modification{
		mod("beta", "r1", "x"),
		mod("alpha", "r2", "x"),
		mod("hot", "r3", "y"),
		mod("hot", "r4", "y"),
	}

	res, err := runRevisions(mods, Opts{})
	if err != nil {
		t.Fatalf("runRevisions returned error: %v", err)
	}

	rows := revisionRows(t, res)
	want := []revisionsRow{
		{Entity: "hot", NRevs: 2},
		{Entity: "alpha", NRevs: 1},
		{Entity: "beta", NRevs: 1},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestRevisions_Empty(t *testing.T) {
	res, err := runRevisions(nil, Opts{})
	if err != nil {
		t.Fatalf("runRevisions returned error: %v", err)
	}

	rows := revisionRows(t, res)
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want empty", rows)
	}
}

func TestRevisions_DescriptorRegistered(t *testing.T) {
	d := revisionsDescriptor()
	if d.Name != "revisions" {
		t.Errorf("Name = %q, want %q", d.Name, "revisions")
	}
	if len(d.Aliases) != 0 {
		t.Errorf("Aliases = %v, want none", d.Aliases)
	}
	wantCols := []string{"entity", "n_revs"}
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
