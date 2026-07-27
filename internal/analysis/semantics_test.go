package analysis

import "testing"

func TestValidSemantic(t *testing.T) {
	for _, s := range Semantics() {
		if !ValidSemantic(s) {
			t.Errorf("ValidSemantic(%q) = false, want true for a declared semantic", s)
		}
	}
	for _, s := range []string{"", "string", "FILEPATH", "team", "unknown"} {
		if ValidSemantic(s) {
			t.Errorf("ValidSemantic(%q) = true, want false", s)
		}
	}
}

func TestSemantics_Closed(t *testing.T) {
	want := []string{
		"filepath", "person", "date", "commit_id", "text", "label",
		"flag", "count", "loc", "percentage", "ratio", "duration_months",
	}
	got := Semantics()
	if len(got) != len(want) {
		t.Fatalf("Semantics() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Semantics()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestSemanticsOf_FlagGating pins D15: a flag-gated column is omitted when its
// flag is in the omit set and listed otherwise, while an ungated column is always
// present.
func TestSemanticsOf_FlagGating(t *testing.T) {
	d := Descriptor{
		RowSchema: []Column{
			{Name: "entity", Semantic: SemanticFilepath},
			{Name: "degree", Semantic: SemanticPercentage},
			{Name: "shared_revisions", Semantic: SemanticCount, FlagGated: "verbose"},
		},
	}

	full := SemanticsOf(d, nil)
	if len(full) != 3 {
		t.Fatalf("SemanticsOf(nil omit) = %v, want 3 entries", full)
	}
	if full["shared_revisions"] != SemanticCount {
		t.Errorf("shared_revisions = %q, want count", full["shared_revisions"])
	}

	gated := SemanticsOf(d, map[string]bool{"verbose": true})
	if len(gated) != 2 {
		t.Fatalf("SemanticsOf(verbose omitted) = %v, want 2 entries", gated)
	}
	if _, ok := gated["shared_revisions"]; ok {
		t.Errorf("shared_revisions present when verbose omitted; want absent")
	}
	if gated["entity"] != SemanticFilepath || gated["degree"] != SemanticPercentage {
		t.Errorf("ungated columns dropped: %v", gated)
	}
}
