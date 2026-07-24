package output_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/andreswebs/codelens/internal/output"
)

// marshalString marshals v with the same encoder EmitJSON uses and returns the
// bytes as a string (without the trailing newline), for shape assertions.
func marshalString(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}

func TestEmitJSON_WritesCompactWithNewline(t *testing.T) {
	type payload struct {
		A int    `json:"a"`
		B string `json:"b"`
	}

	var buf bytes.Buffer
	if err := output.EmitJSON(&buf, payload{A: 1, B: "x"}); err != nil {
		t.Fatalf("EmitJSON: %v", err)
	}

	want := `{"a":1,"b":"x"}` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("EmitJSON wrote %q, want %q", got, want)
	}
}
