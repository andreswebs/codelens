package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/version"
)

func TestRun_NoArgs_PrintsHelp_ExitZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens"}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "USAGE") {
		t.Errorf("stdout does not contain usage:\n%s", stdout.String())
	}
}

func TestRun_Version_ExitZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "--version"}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if want := version.Current(); !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout %q does not contain version %q", stdout.String(), want)
	}
}

func TestRun_UnknownCommand_UsageExit2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "bogus"}, strings.NewReader(""), &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
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
	if env.OK {
		t.Errorf("error envelope ok = true, want false")
	}
	if env.Error.Code != "unknown_command" {
		t.Errorf("error code = %q, want unknown_command", env.Error.Code)
	}
}

func TestRun_DebugFlag_Parsed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "--debug", "--help"}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "USAGE") {
		t.Errorf("stdout does not contain usage:\n%s", stdout.String())
	}
}

func TestRun_DebugTraceOnlyUnderDebug(t *testing.T) {
	const traceMarker = "command failed"

	var withStderr bytes.Buffer
	if code := run([]string{"codelens", "--debug", "bogus"}, strings.NewReader(""), &bytes.Buffer{}, &withStderr); code != 2 {
		t.Fatalf("--debug exit code = %d, want 2", code)
	}
	if !strings.Contains(withStderr.String(), traceMarker) {
		t.Errorf("--debug stderr should contain a diagnostic trace:\n%s", withStderr.String())
	}

	var withoutStderr bytes.Buffer
	if code := run([]string{"codelens", "bogus"}, strings.NewReader(""), &bytes.Buffer{}, &withoutStderr); code != 2 {
		t.Fatalf("no-debug exit code = %d, want 2", code)
	}
	if strings.Contains(withoutStderr.String(), traceMarker) {
		t.Errorf("stderr without --debug should not contain a trace:\n%s", withoutStderr.String())
	}
}
