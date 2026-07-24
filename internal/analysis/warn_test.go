package analysis

import "testing"

func TestOptsWarn_NilIsNoOp(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("warn with a nil sink panicked: %v", r)
		}
	}()
	var o Opts // zero value: Warn is nil
	o.warn("code", "message", "hint", map[string]any{"k": "v"})
}

func TestOptsWarn_ForwardsToSink(t *testing.T) {
	var gotCode, gotMsg, gotHint string
	var gotDetails any
	o := Opts{Warn: func(code, message, hint string, details any) {
		gotCode, gotMsg, gotHint, gotDetails = code, message, hint, details
	}}

	o.warn("low_signal", "few revisions", "raise --min-revs", 42)

	if gotCode != "low_signal" || gotMsg != "few revisions" || gotHint != "raise --min-revs" {
		t.Errorf("warn forwarded (%q, %q, %q), want (low_signal, few revisions, raise --min-revs)",
			gotCode, gotMsg, gotHint)
	}
	if gotDetails != 42 {
		t.Errorf("details = %v, want 42", gotDetails)
	}
}
