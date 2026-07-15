package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// wantLogCommand is the exact extended-git2 (with %s subject) log command the
// helper must emit by default. Pinning the full string guards the log format
// contract: codelens's parser consumes precisely this shape. The default reads
// the current branch (no --all) and applies .mailmap (--use-mailmap).
const wantLogCommand = "git log --numstat --date=short --pretty=format:'--%h--%ad--%aN--%s' --no-renames --use-mailmap"

// wantLogCommandAll is the emitted command with the --all opt-in, which restores
// the all-refs behavior by inserting --all right after "git log".
const wantLogCommandAll = "git log --all --numstat --date=short --pretty=format:'--%h--%ad--%aN--%s' --no-renames --use-mailmap"

func TestPrintLogCommand_DefaultOmitsAllUsesMailmap(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "print-log-command"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	got := strings.TrimRight(stdout.String(), "\n")
	if got != wantLogCommand {
		t.Errorf("stdout = %q\nwant     %q", got, wantLogCommand)
	}
	if !strings.Contains(got, "--use-mailmap") {
		t.Errorf("default command must include --use-mailmap, got %q", got)
	}
	if strings.Contains(got, "--all") {
		t.Errorf("default command must not include --all, got %q", got)
	}
	for _, want := range []string{"--numstat", "--no-renames", "--date=short", "--%h--%ad--%aN--%s"} {
		if !strings.Contains(got, want) {
			t.Errorf("default command missing %q, got %q", want, got)
		}
	}
}

func TestPrintLogCommand_AllFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "print-log-command", "--all"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	got := strings.TrimRight(stdout.String(), "\n")
	if got != wantLogCommandAll {
		t.Errorf("stdout = %q\nwant     %q", got, wantLogCommandAll)
	}
	if !strings.Contains(got, "--use-mailmap") {
		t.Errorf("--all command must still include --use-mailmap, got %q", got)
	}
}

func TestPrintLogCommand_AfterStillWorks(t *testing.T) {
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

func TestPrintLogCommand_AllAndAfter(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "print-log-command", "--all", "--after", "2025-01-01"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	want := wantLogCommandAll + " --after=2025-01-01"
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
