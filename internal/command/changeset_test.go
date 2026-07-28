package command

import (
	"testing"

	"github.com/andreswebs/codelens/internal/analysis"
)

// TestChangesetBased_Conformance pins ChangesetBased to exactly coupling and
// sum-of-coupling. The field is not derivable from column semantics (soc is an
// additive count that is SUPPOSED to count windows), so a new coupling-family
// analysis must take a position here rather than inheriting the zero value
// unnoticed.
func TestChangesetBased_Conformance(t *testing.T) {
	want := map[string]bool{
		"coupling":        true,
		"sum-of-coupling": true,
	}
	for _, d := range analysis.All() {
		if d.ChangesetBased != want[d.Name] {
			t.Errorf("%q: ChangesetBased = %v, want %v", d.Name, d.ChangesetBased, want[d.Name])
		}
	}
}
