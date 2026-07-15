package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/output"
)

func TestEmitWarning_Shape(t *testing.T) {
	var buf bytes.Buffer
	output.EmitWarning(&buf, "low_signal", "few revisions", "raise --min-revs",
		map[string]any{"entities": 3})
	output.EmitWarning(&buf, "another", "second advisory", "", nil)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (one per warning); output:\n%s", len(lines), buf.String())
	}

	var first struct {
		SchemaVersion int            `json:"schema_version"`
		Level         string         `json:"level"`
		Code          string         `json:"code"`
		Message       string         `json:"message"`
		Hint          string         `json:"hint"`
		Details       map[string]any `json:"details"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first line is not JSON: %v\n%s", err, lines[0])
	}
	if first.SchemaVersion != output.SchemaVersion {
		t.Errorf("schema_version = %d, want %d", first.SchemaVersion, output.SchemaVersion)
	}
	if first.Level != "warning" {
		t.Errorf("level = %q, want %q", first.Level, "warning")
	}
	if first.Code != "low_signal" {
		t.Errorf("code = %q, want %q", first.Code, "low_signal")
	}
	if first.Message != "few revisions" {
		t.Errorf("message = %q, want %q", first.Message, "few revisions")
	}
	if first.Hint != "raise --min-revs" {
		t.Errorf("hint = %q, want %q", first.Hint, "raise --min-revs")
	}
	if got, ok := first.Details["entities"].(float64); !ok || got != 3 {
		t.Errorf("details.entities = %v, want 3", first.Details["entities"])
	}

	// The second line must independently parse as JSON (valid NDJSON stream).
	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("second line is not JSON: %v\n%s", err, lines[1])
	}
}

func TestEmitWarning_OmitsEmptyHintAndDetails(t *testing.T) {
	var buf bytes.Buffer
	output.EmitWarning(&buf, "code", "msg", "", nil)

	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	if _, ok := raw["hint"]; ok {
		t.Errorf("empty hint should be omitted, got: %s", buf.String())
	}
	if _, ok := raw["details"]; ok {
		t.Errorf("nil details should be omitted, got: %s", buf.String())
	}
	if _, ok := raw["ok"]; ok {
		t.Errorf("diagnostic must not carry an ok field, got: %s", buf.String())
	}
}
