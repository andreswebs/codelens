package command

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// runUsageError executes run with args, asserts an exit-64 usage error with an
// empty stdout, and returns the decoded error code and hint from the stderr
// envelope.
func runUsageError(t *testing.T, args ...string) (code, hint string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	exit := Run(args, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})
	if exit != 64 {
		t.Fatalf("exit code = %d, want 64; stderr:\n%s", exit, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on a usage error, got:\n%s", stdout.String())
	}

	var env struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
			Hint string `json:"hint"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
		t.Fatalf("stderr is not a JSON error envelope: %v\n%s", err, stderr.String())
	}
	if env.OK {
		t.Errorf("error envelope ok = true, want false")
	}
	if env.Error.Hint == "" {
		t.Errorf("usage error should carry a hint, got envelope:\n%s", stderr.String())
	}
	return env.Error.Code, env.Error.Hint
}

// The four usage-error codes (unknown_flag, unknown_command, invalid_value,
// missing_required_flag) are pinned byte for byte by the golden harness
// (golden_test.go), which subsumes the field-by-field assertions that once lived
// here. runUsageError is retained because main_test.go's TestRun_DebugFlagRemoved
// still uses it to assert a classification (that --debug is rejected as
// unknown_flag) rather than an exact envelope.
