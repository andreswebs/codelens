package output_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/andreswebs/codelens/internal/output"
	"github.com/andreswebs/codelens/internal/terr"
)

func TestEmitError_JSON_Coded(t *testing.T) {
	err := terr.New("parse_error", 3, "run print-log-command", "bad log")

	var buf bytes.Buffer
	output.EmitError(&buf, "json", err)

	var env struct {
		SchemaVersion int  `json:"schema_version"`
		OK            bool `json:"ok"`
		Error         struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Hint    string `json:"hint"`
		} `json:"error"`
	}
	if e := json.Unmarshal(buf.Bytes(), &env); e != nil {
		t.Fatalf("unmarshal error envelope: %v\ngot: %s", e, buf.String())
	}
	if env.SchemaVersion != output.SchemaVersion {
		t.Errorf("schema_version = %d, want %d", env.SchemaVersion, output.SchemaVersion)
	}
	if env.OK {
		t.Errorf("ok = true, want false")
	}
	if env.Error.Code != "parse_error" {
		t.Errorf("code = %q, want %q", env.Error.Code, "parse_error")
	}
	if env.Error.Message != "bad log" {
		t.Errorf("message = %q, want %q", env.Error.Message, "bad log")
	}
	if env.Error.Hint != "run print-log-command" {
		t.Errorf("hint = %q, want %q", env.Error.Hint, "run print-log-command")
	}
}

func TestEmitError_Text_Coded(t *testing.T) {
	err := terr.New("parse_error", 3, "run print-log-command", "bad log")

	var buf bytes.Buffer
	output.EmitError(&buf, "text", err)

	want := "✗ bad log\n  hint: run print-log-command\n"
	if got := buf.String(); got != want {
		t.Errorf("EmitError text = %q, want %q", got, want)
	}
}

func TestEmitError_Details(t *testing.T) {
	base := terr.New("parse_error", 3, "", "bad entry")
	err := base.WithDetails(map[string]any{"entry": 4, "line": "foo"})

	var buf bytes.Buffer
	output.EmitError(&buf, "json", err)

	var env struct {
		Error struct {
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if e := json.Unmarshal(buf.Bytes(), &env); e != nil {
		t.Fatalf("unmarshal: %v\ngot: %s", e, buf.String())
	}
	if got := env.Error.Details["line"]; got != "foo" {
		t.Errorf("details.line = %v, want %q", got, "foo")
	}
	if got, ok := env.Error.Details["entry"].(float64); !ok || got != 4 {
		t.Errorf("details.entry = %v, want 4", env.Error.Details["entry"])
	}
}

func TestExitCodeFor(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"coded exit 3", terr.New("input_error", 3, "", "empty log"), 3},
		{"usage error", errors.New("flag provided but not defined: -bogus"), 2},
		{"wrapped usage error", fmt.Errorf("x: %w", errors.New("no such flag -q")), 2},
		{"generic", errors.New("boom"), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := output.ExitCodeFor(tt.err); got != tt.want {
				t.Errorf("ExitCodeFor(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestEmitError_UsageErrorClassified(t *testing.T) {
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
			output.EmitError(&buf, "json", errors.New(tt.msg))

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
