package output_test

import (
	"testing"

	"github.com/andreswebs/codelens/internal/output"
)

func TestNewResult_Invariants(t *testing.T) {
	rows := []int{1, 2, 3}
	r := output.NewResult(output.Meta{Analysis: "coupling", Shape: "table"}, rows)

	if r.SchemaVersion != output.SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", r.SchemaVersion, output.SchemaVersion)
	}
	if !r.OK {
		t.Error("OK = false, want true")
	}
	if r.Analysis != "coupling" {
		t.Errorf("Analysis = %q, want %q", r.Analysis, "coupling")
	}
	if r.RowCount != 3 {
		t.Errorf("RowCount = %d, want 3", r.RowCount)
	}
	if r.Shape != "table" {
		t.Errorf("Shape = %q, want %q", r.Shape, "table")
	}
	got, ok := r.Payload.([]int)
	if !ok || len(got) != 3 {
		t.Errorf("Payload = %#v, want passthrough []int{1,2,3}", r.Payload)
	}
}

func TestNewResult_EmptySlice(t *testing.T) {
	r := output.NewResult(output.Meta{Analysis: "authors", Shape: "table"}, []int{})
	if r.RowCount != 0 {
		t.Errorf("RowCount = %d, want 0 for empty slice", r.RowCount)
	}
	if r.Payload == nil {
		t.Error("Payload = nil, want the empty slice passed through")
	}
}

func TestNewResult_NilRows(t *testing.T) {
	r := output.NewResult(output.Meta{Analysis: "authors", Shape: "table"}, nil)
	if r.RowCount != 0 {
		t.Errorf("RowCount = %d, want 0 for nil rows", r.RowCount)
	}
}

func TestRowLen(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int
	}{
		{"nil", nil, 0},
		{"empty slice", []string{}, 0},
		{"three ints", []int{1, 2, 3}, 3},
		{"non-slice", 42, 0},
		{"string is not a row slice", "abc", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := output.RowLen(tc.in); got != tc.want {
				t.Errorf("RowLen(%#v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
