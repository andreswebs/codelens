package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/andreswebs/codelens/internal/output"
)

// These tests moved out of internal/output when the usage classifier moved into
// internal/command. The classifier is exercised through resolve, the same
// boundary Run applies to a returned framework error before emitting it, so the
// assertions (code, hint, and the preserved raw message) are unchanged from when
// output.EmitError/output.ExitCodeFor did the classifying inline.

func TestResolve_ExitCodeFor_UsageErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"usage error", errors.New("flag provided but not defined: -bogus"), 64},
		{"wrapped usage error", fmt.Errorf("x: %w", errors.New("no such flag -q")), 64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := output.ExitCodeFor(resolve(tt.err)); got != tt.want {
				t.Errorf("ExitCodeFor(resolve(%v)) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestResolve_EmitError_UsageErrorClassified(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		wantCode string
	}{
		{"unknown flag", "flag provided but not defined: -bogus", "unknown_flag"},
		{"no such flag", "no such flag -q", "unknown_flag"},
		{"invalid value", `invalid value "abc" for flag -rows: strconv.ParseInt: parsing "abc": invalid syntax`, "invalid_value"},
		{"required flag", `Required flag "expression" not set`, "missing_required_flag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			output.EmitError(&buf, resolve(errors.New(tt.msg)))

			var env struct {
				OK    bool `json:"ok"`
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
					Hint    string `json:"hint"`
				} `json:"error"`
			}
			if e := json.Unmarshal(buf.Bytes(), &env); e != nil {
				t.Fatalf("unmarshal: %v\ngot: %s", e, buf.String())
			}
			if env.OK {
				t.Errorf("ok = true, want false")
			}
			if env.Error.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", env.Error.Code, tt.wantCode)
			}
			if env.Error.Hint == "" {
				t.Errorf("hint should be non-empty for a usage error, got envelope: %s", buf.String())
			}
			if env.Error.Message != tt.msg {
				t.Errorf("message = %q, want the underlying text %q", env.Error.Message, tt.msg)
			}
		})
	}
}
