package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// wantLogCommand is the exact extended-git2 (with %s subject) log command the
// helper must emit. Pinning the full string guards the log format contract:
// codelens's parser consumes precisely this shape.
const wantLogCommand = "git log --all --numstat --date=short --pretty=format:'--%h--%ad--%aN--%s' --no-renames"

func TestPrintLogCommand_Default(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "print-log-command"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	if got := strings.TrimRight(stdout.String(), "\n"); got != wantLogCommand {
		t.Errorf("stdout = %q\nwant     %q", got, wantLogCommand)
	}
}

func TestPrintLogCommand_After(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "print-log-command", "--after", "2024-01-01"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	want := wantLogCommand + " --after=2024-01-01"
	if got := strings.TrimRight(stdout.String(), "\n"); got != want {
		t.Errorf("stdout = %q\nwant     %q", got, want)
	}
}

func TestPrintLogCommand_BadAfter(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "print-log-command", "--after", "nope"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stderr:\n%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on error, got:\n%s", stdout.String())
	}
	var env struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
		t.Fatalf("stderr is not a JSON error envelope: %v\n%s", err, stderr.String())
	}
	if env.OK || env.Error.Code != "usage_error" {
		t.Errorf("error envelope = %+v, want ok=false code=usage_error", env)
	}
}
