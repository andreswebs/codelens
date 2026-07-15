package analysis

import (
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// mainDevByRevsRows asserts the result is an ok main-developer-by-revisions
// envelope and returns its rows as the concrete row type so cases can index
// into them.
func mainDevByRevsRows(t *testing.T, rows any) []mainDevByRevsRow {
	t.Helper()
	got, ok := rows.([]mainDevByRevsRow)
	if !ok {
		t.Fatalf("rows is %T, want []mainDevByRevsRow", rows)
	}
	return got
}

func TestMainDevByRevs_PicksMaxReviser(t *testing.T) {
	// alice contributes 2 of the entity's 3 revisions, bob 1; alice is the
	// main developer by revision count. Added is her rev count, TotalAdded the
	// entity total.
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice"},
		{Entity: "a.txt", Rev: "r2", Author: "alice"},
		{Entity: "a.txt", Rev: "r3", Author: "bob"},
	}

	res, err := runMainDevByRevs(mods, Opts{})
	if err != nil {
		t.Fatalf("runMainDevByRevs returned error: %v", err)
	}

	rows := mainDevByRevsRows(t, res)
	want := []mainDevByRevsRow{
		{Entity: "a.txt", MainDev: "alice", Added: 2, TotalAdded: 3, Ownership: 0.67},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestMainDevByRevs_Ownership(t *testing.T) {
	// 5 of 10 revisions by the main developer yields ownership 0.5.
	mods := make([]model.Modification, 0, 10)
	for i := 0; i < 5; i++ {
		mods = append(mods, model.Modification{Entity: "a.txt", Rev: "r", Author: "alice"})
	}
	for i := 0; i < 5; i++ {
		mods = append(mods, model.Modification{Entity: "a.txt", Rev: "r", Author: "bob"})
	}

	res, err := runMainDevByRevs(mods, Opts{})
	if err != nil {
		t.Fatalf("runMainDevByRevs returned error: %v", err)
	}

	rows := mainDevByRevsRows(t, res)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	// alice and bob tie at 5; the ascending-author tiebreak keeps alice.
	if rows[0].MainDev != "alice" {
		t.Errorf("MainDev = %q, want alice (ascending-author tiebreak)", rows[0].MainDev)
	}
	if rows[0].Ownership != 0.5 {
		t.Errorf("Ownership = %v, want 0.5", rows[0].Ownership)
	}
}

func TestMainDevByRevs_SortEntityAsc(t *testing.T) {
	// Rows are ordered by entity ascending, regardless of input order.
	mods := []model.Modification{
		{Entity: "b.txt", Rev: "r1", Author: "carol"},
		{Entity: "a.txt", Rev: "r2", Author: "carol"},
	}

	res, err := runMainDevByRevs(mods, Opts{})
	if err != nil {
		t.Fatalf("runMainDevByRevs returned error: %v", err)
	}

	rows := mainDevByRevsRows(t, res)
	gotEntities := []string{rows[0].Entity, rows[1].Entity}
	wantEntities := []string{"a.txt", "b.txt"}
	if !reflect.DeepEqual(gotEntities, wantEntities) {
		t.Errorf("entities = %v, want %v", gotEntities, wantEntities)
	}
}

func TestMainDevByRevs_Empty(t *testing.T) {
	res, err := runMainDevByRevs(nil, Opts{})
	if err != nil {
		t.Fatalf("runMainDevByRevs returned error: %v", err)
	}
	rows := mainDevByRevsRows(t, res)
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want empty", rows)
	}
}

func TestMainDevByRevs_DescriptorRegistered(t *testing.T) {
	d := mainDevByRevsDescriptor()
	if d.Name != "main-developer-by-revisions" {
		t.Errorf("Name = %q, want %q", d.Name, "main-developer-by-revisions")
	}
	wantAliases := []string{"main-dev-by-revs"}
	if !reflect.DeepEqual(d.Aliases, wantAliases) {
		t.Errorf("Aliases = %v, want %v", d.Aliases, wantAliases)
	}
	wantCols := []string{"entity", "main_dev", "added", "total_added", "ownership"}
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
