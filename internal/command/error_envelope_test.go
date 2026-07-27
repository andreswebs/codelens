package command

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestError_IsAlwaysJSONEnvelope pins the always-JSON error decision: there is
// no human-facing text error path, so a forced data error yields the JSON error
// envelope on stderr and nothing on stdout. An empty stdin makes the log
// unparseable, forcing empty_log (exit 65), the data error exercised here.
func TestError_IsAlwaysJSONEnvelope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := Run([]string{"authors"}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})
	if exit != 65 {
		t.Fatalf("exit code = %d, want 65; stderr:\n%s", exit, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on error, got:\n%s", stdout.String())
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
		t.Fatalf("stderr is not a JSON error envelope: %v\n%s", err, stderr.String())
	}
	if env.OK {
		t.Errorf("error envelope ok = true, want false")
	}
	if env.Error.Code == "" || env.Error.Message == "" {
		t.Errorf("error envelope missing code/message, got:\n%s", stderr.String())
	}
}
