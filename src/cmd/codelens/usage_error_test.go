package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// runUsageError executes run with args, asserts an exit-2 usage error with an
// empty stdout, and returns the decoded error code and hint from the stderr
// envelope.
func runUsageError(t *testing.T, args ...string) (code, hint string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	exit := run(append([]string{"codelens"}, args...), strings.NewReader(""), &stdout, &stderr)
	if exit != 2 {
		t.Fatalf("exit code = %d, want 2; stderr:\n%s", exit, stderr.String())
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

func TestUsage_UnknownFlag(t *testing.T) {
	code, _ := runUsageError(t, "authors", "--nope")
	if code != "unknown_flag" {
		t.Errorf("code = %q, want unknown_flag", code)
	}
}

func TestUsage_UnknownSubcommand(t *testing.T) {
	code, _ := runUsageError(t, "frobnicate")
	if code != "unknown_command" {
		t.Errorf("code = %q, want unknown_command", code)
	}
}

func TestUsage_InvalidIntFlag(t *testing.T) {
	code, _ := runUsageError(t, "authors", "--rows", "abc")
	if code != "invalid_value" {
		t.Errorf("code = %q, want invalid_value", code)
	}
}

func TestUsage_MessagesMissingExpression(t *testing.T) {
	code, _ := runUsageError(t, "messages")
	if code != "missing_required_flag" {
		t.Errorf("code = %q, want missing_required_flag", code)
	}
}
