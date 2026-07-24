package analysis

import (
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// socResultRows runs the sum-of-coupling analysis and returns its rows as the
// concrete row type, asserting the row type along the way.
func socResultRows(t *testing.T, mods []model.Modification, opts Opts) []socRow {
	t.Helper()
	rows, err := runSoc(mods, opts)
	if err != nil {
		t.Fatalf("runSoc returned error: %v", err)
	}
	got, ok := rows.([]socRow)
	if !ok {
		t.Fatalf("rows is %T, want []socRow", rows)
	}
	return got
}

// socPairs builds a log where entity changes together with a distinct filler in
// n separate revisions. Each 2-entity change set contributes 1 to every member,
// so entity's sum-of-coupling is n while each single-use filler's is 1.
func socPairs(entity, prefix string, n int) []model.Modification {
	mods := make([]model.Modification, 0, n*2)
	for i := 1; i <= n; i++ {
		rev := prefix + string(rune('0'+i))
		filler := prefix + "f" + string(rune('0'+i))
		mods = append(mods, cmod(entity, rev), cmod(filler, rev))
	}
	return mods
}

// TestSoc_AccumulatesPerRev: every entity in a size-3 change set gains k-1 == 2.
func TestSoc_AccumulatesPerRev(t *testing.T) {
	mods := []model.Modification{
		cmod("A", "r1"), cmod("B", "r1"), cmod("C", "r1"),
	}
	opts := Opts{MinRevs: 1}

	rows := socResultRows(t, mods, opts)

	want := []socRow{
		{Entity: "C", Soc: 2},
		{Entity: "B", Soc: 2},
		{Entity: "A", Soc: 2},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

// TestSoc_StrictMinRevs: an entity whose soc equals min-revs is excluded (strict
// >), while one exceeding it is kept. A size-6 set gives its members soc 5; a
// size-7 set gives soc 6. With min-revs 5, only the size-7 members survive.
func TestSoc_StrictMinRevs(t *testing.T) {
	mods := []model.Modification{
		// size-6 change set: each member gains 5.
		cmod("d", "r1"), cmod("x1", "r1"), cmod("x2", "r1"),
		cmod("x3", "r1"), cmod("x4", "r1"), cmod("x5", "r1"),
		// size-7 change set: each member gains 6.
		cmod("k", "r2"), cmod("y1", "r2"), cmod("y2", "r2"),
		cmod("y3", "r2"), cmod("y4", "r2"), cmod("y5", "r2"), cmod("y6", "r2"),
	}
	opts := Opts{MinRevs: 5}

	rows := socResultRows(t, mods, opts)

	for _, r := range rows {
		if r.Soc <= 5 {
			t.Errorf("row %+v has soc <= min-revs, should have been excluded", r)
		}
		if r.Entity == "d" {
			t.Errorf("entity %q (soc == min-revs) should be excluded", r.Entity)
		}
	}
	var keptK bool
	for _, r := range rows {
		if r.Entity == "k" {
			keptK = true
			if r.Soc != 6 {
				t.Errorf("soc(k) = %d, want 6", r.Soc)
			}
		}
	}
	if !keptK {
		t.Errorf("entity %q (soc > min-revs) should be kept; rows = %+v", "k", rows)
	}
	if len(rows) != 7 {
		t.Errorf("row_count = %d, want 7 (size-7 set members)", len(rows))
	}
}

// TestSoc_SortDesc pins the ordering: primary soc descending, entity descending
// as the tie-break. A=6, B=4, C=4 yields [A, C, B]. Fillers (soc 1) are dropped.
func TestSoc_SortDesc(t *testing.T) {
	var mods []model.Modification
	mods = append(mods, socPairs("A", "a", 6)...)
	mods = append(mods, socPairs("B", "b", 4)...)
	mods = append(mods, socPairs("C", "c", 4)...)
	opts := Opts{MinRevs: 3}

	rows := socResultRows(t, mods, opts)

	want := []socRow{
		{Entity: "A", Soc: 6},
		{Entity: "C", Soc: 4},
		{Entity: "B", Soc: 4},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

// TestSocDescriptor pins the registered contract: canonical name, soc alias, the
// min-revs flag, the two-column row schema, and the parity error/exit codes.
func TestSocDescriptor(t *testing.T) {
	d := socDescriptor()
	if d.Name != "sum-of-coupling" {
		t.Errorf("Name = %q, want %q", d.Name, "sum-of-coupling")
	}
	if !reflect.DeepEqual(d.Aliases, []string{"soc"}) {
		t.Errorf("Aliases = %v, want [soc]", d.Aliases)
	}
	if len(d.Flags) != 1 || d.Flags[0].Name != "min-revs" || d.Flags[0].Default != 5 {
		t.Errorf("Flags = %+v, want a single min-revs=5 flag", d.Flags)
	}
	wantCols := []Column{
		{Name: "entity", Type: "string", Desc: "module path"},
		{Name: "soc", Type: "int", Desc: "number of shared transactions"},
	}
	if !reflect.DeepEqual(d.RowSchema, wantCols) {
		t.Errorf("RowSchema = %+v, want %+v", d.RowSchema, wantCols)
	}
}
