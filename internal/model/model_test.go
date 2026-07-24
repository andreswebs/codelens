package model

import "testing"

// TestModification_ZeroValue documents the zero value of a Modification: empty
// strings, zero LOC counts, and false flags. The real coverage of field
// population comes from the parser tests; this is a light shape guard.
func TestModification_ZeroValue(t *testing.T) {
	var m Modification

	if m.Entity != "" {
		t.Errorf("Entity = %q, want empty", m.Entity)
	}
	if m.Rev != "" {
		t.Errorf("Rev = %q, want empty", m.Rev)
	}
	if m.Date != "" {
		t.Errorf("Date = %q, want empty", m.Date)
	}
	if m.Author != "" {
		t.Errorf("Author = %q, want empty", m.Author)
	}
	if m.Message != "" {
		t.Errorf("Message = %q, want empty", m.Message)
	}
	if m.LocAdded != 0 {
		t.Errorf("LocAdded = %d, want 0", m.LocAdded)
	}
	if m.LocDeleted != 0 {
		t.Errorf("LocDeleted = %d, want 0", m.LocDeleted)
	}
	if m.Binary {
		t.Error("Binary = true, want false")
	}
	if m.HasLoc {
		t.Error("HasLoc = true, want false")
	}
}

// TestOptions_ZeroValue documents that a zero Options has an empty
// InputEncoding, which callers treat as the UTF-8 default.
func TestOptions_ZeroValue(t *testing.T) {
	var o Options

	if o.InputEncoding != "" {
		t.Errorf("InputEncoding = %q, want empty", o.InputEncoding)
	}
}
