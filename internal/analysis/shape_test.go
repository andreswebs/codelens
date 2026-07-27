package analysis

import "testing"

func TestValidShape(t *testing.T) {
	for _, s := range Shapes() {
		if !ValidShape(s) {
			t.Errorf("ValidShape(%q) = false, want true for a declared shape", s)
		}
	}
	for _, s := range []string{"", "rows", "TABLE", "unknown"} {
		if ValidShape(s) {
			t.Errorf("ValidShape(%q) = true, want false", s)
		}
	}
}

func TestShapes_Closed(t *testing.T) {
	want := []string{"table", "tree", "graph", "matrix", "series", "text"}
	got := Shapes()
	if len(got) != len(want) {
		t.Fatalf("Shapes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Shapes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
