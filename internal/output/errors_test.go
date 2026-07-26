package output_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/andreswebs/codelens/internal/output"
	"github.com/andreswebs/codelens/internal/terr"
)

func TestEmitError_JSON_Coded(t *testing.T) {
	err := terr.Newf("parse_error", 65, "run print-log-command", "bad log")

	var buf bytes.Buffer
	output.EmitError(&buf, err)

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

func TestEmitError_AlwaysJSON(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
	}{
		{"coded", terr.Newf("parse_error", 65, "run print-log-command", "bad log"), "parse_error"},
		{"plain", errors.New("boom"), "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			output.EmitError(&buf, tt.err)

			var env struct {
				SchemaVersion int  `json:"schema_version"`
				OK            bool `json:"ok"`
				Error         struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if e := json.Unmarshal(buf.Bytes(), &env); e != nil {
				t.Fatalf("stderr is not a JSON error envelope: %v\ngot: %s", e, buf.String())
			}
			if env.SchemaVersion != output.SchemaVersion {
				t.Errorf("schema_version = %d, want %d", env.SchemaVersion, output.SchemaVersion)
			}
			if env.OK {
				t.Errorf("ok = true, want false")
			}
			if env.Error.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", env.Error.Code, tt.wantCode)
			}
			if env.Error.Message != tt.err.Error() {
				t.Errorf("message = %q, want %q", env.Error.Message, tt.err.Error())
			}
		})
	}
}

func TestEmitError_Details(t *testing.T) {
	base := terr.Newf("parse_error", 65, "", "bad entry")
	err := base.WithDetails(map[string]any{"entry": 4, "line": "foo"})

	var buf bytes.Buffer
	output.EmitError(&buf, err)

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
		{"coded exit 65", terr.Newf("empty_log", 65, "", "empty log"), 65},
		{"generic", errors.New("boom"), 70},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := output.ExitCodeFor(tt.err); got != tt.want {
				t.Errorf("ExitCodeFor(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
