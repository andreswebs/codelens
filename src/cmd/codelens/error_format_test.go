package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestError_FormatText_StillJSONEnvelope pins the always-JSON error decision end
// to end: --format governs results on stdout, never diagnostics on stderr, so a
// forced error under --format text still yields the JSON error envelope (there is
// no "✗ <message>" text path). An empty stdin makes the log unparseable, which is
// the input error being forced here.
func TestError_FormatText_StillJSONEnvelope(t *testing.T) {
	for _, format := range []string{"text", "table", "json"} {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := run(
				[]string{"codelens", "--format", format, "authors"},
				strings.NewReader(""), &stdout, &stderr,
			)
			if exit != 3 {
				t.Fatalf("exit code = %d, want 3; stderr:\n%s", exit, stderr.String())
			}
			if strings.Contains(stderr.String(), "✗") {
				t.Errorf("stderr should not use the removed text error path, got:\n%s", stderr.String())
			}

			var env struct {
				SchemaVersion int  `json:"schema_version"`
				OK            bool `json:"ok"`
				Error         struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
				t.Fatalf("stderr is not a JSON error envelope under --format %s: %v\n%s",
					format, err, stderr.String())
			}
			if env.OK {
				t.Errorf("error envelope ok = true, want false")
			}
			if env.Error.Code == "" || env.Error.Message == "" {
				t.Errorf("error envelope missing code/message, got:\n%s", stderr.String())
			}
		})
	}
}
