package analysis

import (
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// fragmentationRows asserts the result is an ok fragmentation envelope and
// returns its rows as the concrete row type so cases can index into them.
func fragmentationRows(t *testing.T, rows any) []fragmentationRow {
	t.Helper()
	got, ok := rows.([]fragmentationRow)
	if !ok {
		t.Fatalf("rows is %T, want []fragmentationRow", rows)
	}
	return got
}

func TestFragmentation_SingleAuthorZero(t *testing.T) {
	// One author owns every revision, so the fractal value is 1 - 1^2 = 0.
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice"},
		{Entity: "a.txt", Rev: "r2", Author: "alice"},
	}

	res, err := runFragmentation(mods, Opts{})
	if err != nil {
		t.Fatalf("runFragmentation returned error: %v", err)
	}

	rows := fragmentationRows(t, res)
	want := []fragmentationRow{
		{Entity: "a.txt", FractalValue: 0.0, TotalRevs: 2},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestFragmentation_TwoEqualAuthors(t *testing.T) {
	// A 50/50 split gives 1 - (0.5^2 + 0.5^2) = 1 - 0.5 = 0.5.
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice"},
		{Entity: "a.txt", Rev: "r2", Author: "bob"},
	}

	res, err := runFragmentation(mods, Opts{})
	if err != nil {
		t.Fatalf("runFragmentation returned error: %v", err)
	}

	rows := fragmentationRows(t, res)
	want := []fragmentationRow{
		{Entity: "a.txt", FractalValue: 0.5, TotalRevs: 2},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestFragmentation_ThreeAuthorsRounded(t *testing.T) {
	// Three authors each with one of three revisions:
	// 1 - 3*(1/3)^2 = 1 - 1/3 = 0.6666..., rounded to 2 sig digits = 0.67.
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice"},
		{Entity: "a.txt", Rev: "r2", Author: "bob"},
		{Entity: "a.txt", Rev: "r3", Author: "carol"},
	}

	res, err := runFragmentation(mods, Opts{})
	if err != nil {
		t.Fatalf("runFragmentation returned error: %v", err)
	}

	rows := fragmentationRows(t, res)
	want := []fragmentationRow{
		{Entity: "a.txt", FractalValue: 0.67, TotalRevs: 3},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestFragmentation_SortDesc(t *testing.T) {
	// Rows sort by fractal value descending, then total revs descending. The
	// two-author entity (fractal 0.5) precedes the single-author one (0.0).
	mods := []model.Modification{
		{Entity: "solo.txt", Rev: "r1", Author: "alice"},
		{Entity: "shared.txt", Rev: "r2", Author: "alice"},
		{Entity: "shared.txt", Rev: "r3", Author: "bob"},
	}

	res, err := runFragmentation(mods, Opts{})
	if err != nil {
		t.Fatalf("runFragmentation returned error: %v", err)
	}

	rows := fragmentationRows(t, res)
	gotEntities := []string{rows[0].Entity, rows[1].Entity}
	wantEntities := []string{"shared.txt", "solo.txt"}
	if !reflect.DeepEqual(gotEntities, wantEntities) {
		t.Errorf("entities = %v, want %v (sorted by fractal value desc)", gotEntities, wantEntities)
	}
}

func TestFragmentation_SortTotalRevsDescOnTie(t *testing.T) {
	// Two entities with equal fractal value (both single-author, 0.0) break the
	// tie by total revs descending: big.txt (2 revs) before small.txt (1 rev).
	mods := []model.Modification{
		{Entity: "small.txt", Rev: "r1", Author: "alice"},
		{Entity: "big.txt", Rev: "r2", Author: "alice"},
		{Entity: "big.txt", Rev: "r3", Author: "alice"},
	}

	res, err := runFragmentation(mods, Opts{})
	if err != nil {
		t.Fatalf("runFragmentation returned error: %v", err)
	}

	rows := fragmentationRows(t, res)
	gotEntities := []string{rows[0].Entity, rows[1].Entity}
	wantEntities := []string{"big.txt", "small.txt"}
	if !reflect.DeepEqual(gotEntities, wantEntities) {
		t.Errorf("entities = %v, want %v (tie broken by total revs desc)", gotEntities, wantEntities)
	}
}

func TestFragmentation_Empty(t *testing.T) {
	res, err := runFragmentation(nil, Opts{})
	if err != nil {
		t.Fatalf("runFragmentation returned error: %v", err)
	}
	rows := fragmentationRows(t, res)
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want empty", rows)
	}
}

func TestFragmentation_DescriptorRegistered(t *testing.T) {
	d := fragmentationDescriptor()
	if d.Name != "fragmentation" {
		t.Errorf("Name = %q, want %q", d.Name, "fragmentation")
	}
	if len(d.Aliases) != 0 {
		t.Errorf("Aliases = %v, want none", d.Aliases)
	}
	wantCols := []string{"entity", "fractal_value", "total_revs"}
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
