package analysis

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// parseResultRows asserts the result is an ok parse envelope and returns its
// rows as the concrete row type so cases can index into them.
func parseResultRows(t *testing.T, rows any) []parseRow {
	t.Helper()
	got, ok := rows.([]parseRow)
	if !ok {
		t.Fatalf("rows is %T, want []parseRow", rows)
	}
	return got
}

func intPtr(v int) *int { return &v }

func TestParse_LogOrderPreserved(t *testing.T) {
	// Entities are supplied in an order that a sorted analysis would rearrange
	// (z, a, m); parse must emit them verbatim, one row per modification.
	mods := []model.Modification{
		{Entity: "z", Rev: "r1", Date: "2024-01-01", Author: "x", Message: "third", LocAdded: 1, LocDeleted: 0, HasLoc: true},
		{Entity: "a", Rev: "r2", Date: "2024-01-02", Author: "y", Message: "first", LocAdded: 2, LocDeleted: 3, HasLoc: true},
		{Entity: "m", Rev: "r2", Date: "2024-01-02", Author: "y", Message: "first", LocAdded: 0, LocDeleted: 5, HasLoc: true},
	}

	res, err := runParse(mods, Opts{})
	if err != nil {
		t.Fatalf("runParse returned error: %v", err)
	}

	rows := parseResultRows(t, res)
	want := []parseRow{
		{Entity: "z", Rev: "r1", Date: "2024-01-01", Author: "x", Message: "third", LocAdded: intPtr(1), LocDeleted: intPtr(0)},
		{Entity: "a", Rev: "r2", Date: "2024-01-02", Author: "y", Message: "first", LocAdded: intPtr(2), LocDeleted: intPtr(3)},
		{Entity: "m", Rev: "r2", Date: "2024-01-02", Author: "y", Message: "first", LocAdded: intPtr(0), LocDeleted: intPtr(5)},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestParse_LocOmittedWhenAbsent(t *testing.T) {
	// A record without numstat (HasLoc false) carries no loc pointers and, once
	// marshaled, no loc_added/loc_deleted keys; a record with numstat carries
	// both as integers.
	mods := []model.Modification{
		{Entity: "noloc", Rev: "r1", Date: "2024-01-01", Author: "x", Message: "-", HasLoc: false},
		{Entity: "withloc", Rev: "r2", Date: "2024-01-02", Author: "y", Message: "m", LocAdded: 4, LocDeleted: 2, HasLoc: true},
	}

	res, err := runParse(mods, Opts{})
	if err != nil {
		t.Fatalf("runParse returned error: %v", err)
	}

	rows := parseResultRows(t, res)
	if rows[0].LocAdded != nil || rows[0].LocDeleted != nil {
		t.Errorf("row without numstat has loc pointers: added=%v deleted=%v", rows[0].LocAdded, rows[0].LocDeleted)
	}
	if rows[1].LocAdded == nil || *rows[1].LocAdded != 4 {
		t.Errorf("row with numstat loc_added = %v, want 4", rows[1].LocAdded)
	}
	if rows[1].LocDeleted == nil || *rows[1].LocDeleted != 2 {
		t.Errorf("row with numstat loc_deleted = %v, want 2", rows[1].LocDeleted)
	}

	b, err := json.Marshal(rows[0])
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if _, present := got["loc_added"]; present {
		t.Errorf("loc_added present in %s, want omitted", b)
	}
	if _, present := got["loc_deleted"]; present {
		t.Errorf("loc_deleted present in %s, want omitted", b)
	}
}

func TestParse_BinaryFlag(t *testing.T) {
	// A binary modification (git numstat "-"/"-") is marked binary and reports
	// zero loc; a text modification omits the binary key entirely.
	mods := []model.Modification{
		{Entity: "img.png", Rev: "r1", Date: "2024-01-01", Author: "x", Message: "add image", LocAdded: 0, LocDeleted: 0, Binary: true, HasLoc: true},
		{Entity: "main.go", Rev: "r1", Date: "2024-01-01", Author: "x", Message: "add image", LocAdded: 3, LocDeleted: 1, HasLoc: true},
	}

	res, err := runParse(mods, Opts{})
	if err != nil {
		t.Fatalf("runParse returned error: %v", err)
	}

	rows := parseResultRows(t, res)
	if !rows[0].Binary {
		t.Errorf("binary row Binary = false, want true")
	}

	b, err := json.Marshal(rows[1])
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if _, present := got["binary"]; present {
		t.Errorf("binary present in %s for a text row, want omitted", b)
	}
}

func TestParse_Empty(t *testing.T) {
	res, err := runParse(nil, Opts{})
	if err != nil {
		t.Fatalf("runParse returned error: %v", err)
	}

	rows := parseResultRows(t, res)
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want empty", rows)
	}
}

func TestParse_DescriptorRegistered(t *testing.T) {
	d := parseDescriptor()
	if d.Name != "parse" {
		t.Errorf("Name = %q, want %q", d.Name, "parse")
	}
	wantAliases := []string{"identity"}
	if !reflect.DeepEqual(d.Aliases, wantAliases) {
		t.Errorf("Aliases = %v, want %v", d.Aliases, wantAliases)
	}
	wantCols := []string{"entity", "rev", "date", "author", "message", "loc_added", "loc_deleted", "binary"}
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

func TestParse_AliasIdentity(t *testing.T) {
	// The terse code-maat original "identity" resolves to the parse descriptor.
	d, ok := Lookup("identity")
	if !ok {
		t.Fatalf("Lookup(%q) not found", "identity")
	}
	if d.Name != "parse" {
		t.Errorf("Lookup(%q).Name = %q, want %q", "identity", d.Name, "parse")
	}
}
