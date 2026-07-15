package analysis

import (
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// summaryResultRows asserts the result is an ok summary envelope and returns its
// rows as the concrete row type so cases can index into them.
func summaryResultRows(t *testing.T, rows any) []summaryRow {
	t.Helper()
	got, ok := rows.([]summaryRow)
	if !ok {
		t.Fatalf("rows is %T, want []summaryRow", rows)
	}
	return got
}

func TestSummary_Counts(t *testing.T) {
	// Two commits (r1, r2), three distinct entities (A, B, C), four modification
	// records total, and two distinct authors (x, y).
	mods := []model.Modification{
		{Entity: "A", Rev: "r1", Author: "x"},
		{Entity: "B", Rev: "r1", Author: "x"},
		{Entity: "A", Rev: "r2", Author: "y"},
		{Entity: "C", Rev: "r2", Author: "y"},
	}

	res, err := runSummary(mods, Opts{})
	if err != nil {
		t.Fatalf("runSummary returned error: %v", err)
	}

	rows := summaryResultRows(t, res)
	want := []summaryRow{
		{Statistic: "number-of-commits", Value: 2},
		{Statistic: "number-of-entities", Value: 3},
		{Statistic: "number-of-entities-changed", Value: 4},
		{Statistic: "number-of-authors", Value: 2},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestSummary_FixedOrderAndLabels(t *testing.T) {
	mods := []model.Modification{
		{Entity: "A", Rev: "r1", Author: "x"},
	}

	res, err := runSummary(mods, Opts{})
	if err != nil {
		t.Fatalf("runSummary returned error: %v", err)
	}

	rows := summaryResultRows(t, res)
	wantLabels := []string{
		"number-of-commits",
		"number-of-entities",
		"number-of-entities-changed",
		"number-of-authors",
	}
	if len(rows) != len(wantLabels) {
		t.Fatalf("got %d rows, want %d", len(rows), len(wantLabels))
	}
	for i, label := range wantLabels {
		if rows[i].Statistic != label {
			t.Errorf("rows[%d].Statistic = %q, want %q", i, rows[i].Statistic, label)
		}
	}
}

func TestSummary_Empty(t *testing.T) {
	res, err := runSummary(nil, Opts{})
	if err != nil {
		t.Fatalf("runSummary returned error: %v", err)
	}

	rows := summaryResultRows(t, res)
	want := []summaryRow{
		{Statistic: "number-of-commits", Value: 0},
		{Statistic: "number-of-entities", Value: 0},
		{Statistic: "number-of-entities-changed", Value: 0},
		{Statistic: "number-of-authors", Value: 0},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestSummary_DescriptorRegistered(t *testing.T) {
	d := summaryDescriptor()
	if d.Name != "summary" {
		t.Errorf("Name = %q, want %q", d.Name, "summary")
	}
	if len(d.Aliases) != 0 {
		t.Errorf("Aliases = %v, want none", d.Aliases)
	}
	wantCols := []string{"statistic", "value"}
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
