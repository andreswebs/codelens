package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/version"
)

// TestVersion_Subcommand checks that the version subcommand prints the build
// version reported by internal/version.Current() to stdout and exits 0, with no
// diagnostics on stderr.
func TestVersion_Subcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "version"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty, got:\n%s", stderr.String())
	}
	want := version.Current()
	if got := strings.TrimRight(stdout.String(), "\n"); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

// TestVersion_Flag checks that the --version flag reports the same build version
// as internal/version.Current(), so the flag and the subcommand share one
// source of truth.
func TestVersion_Flag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "--version"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	if want := version.Current(); !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout = %q, want it to contain %q", stdout.String(), want)
	}
}
