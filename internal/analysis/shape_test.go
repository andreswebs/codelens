package analysis

import "testing"

func TestValidShape(t *testing.T) {
	for _, s := range Shapes() {
		if !ValidShape(s) {
			t.Errorf("ValidShape(%q) = false, want true for a declared shape", s)
		}
	}
	// tree/graph/matrix/series were retracted or deferred (cod-2le4): they are no
	// longer declared shapes, so they must read as invalid. A future author who
	// implements the graph shape must remove "graph" from this negative list in
	// the same change that declares it (see ADR 0008), rather than being confused
	// by a test that appears to forbid their new shape.
	for _, s := range []Shape{"", "rows", "TABLE", "unknown", "tree", "graph", "matrix", "series"} {
		if ValidShape(s) {
			t.Errorf("ValidShape(%q) = true, want false", s)
		}
	}
}

func TestShapes_Closed(t *testing.T) {
	want := []Shape{ShapeTable, ShapeText}
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
