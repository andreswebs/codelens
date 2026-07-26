package command

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/version"
)

func TestRun_NoArgs_PrintsHelp_ExitZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "USAGE") {
		t.Errorf("stdout does not contain usage:\n%s", stdout.String())
	}
}

// TestRun_Version_Flag pins the --version flag to the bare version string
// reported by internal/version.Current() (plus a trailing newline), with nothing
// on stderr. Bare output is exact by design: it is trivial to capture and compare
// in scripts, and version.Current() is the single source.
func TestRun_Version_Flag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--version"}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty, got:\n%s", stderr.String())
	}
	if want := version.Current() + "\n"; stdout.String() != want {
		t.Errorf("stdout = %q, want exactly %q", stdout.String(), want)
	}
}

// TestRun_VersionSubcommand_UnknownExit64 pins the removal of the version
// subcommand: `codelens version` is now an unknown command, classified as a usage
// error (exit 64) with the unknown_command envelope, exactly like any other
// non-command.
func TestRun_VersionSubcommand_UnknownExit64(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"version"}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})

	if code != 64 {
		t.Fatalf("exit code = %d, want 64; stderr:\n%s", code, stderr.String())
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

func TestRun_UnknownCommand_UsageExit64(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"bogus"}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})

	if code != 64 {
		t.Fatalf("exit code = %d, want 64", code)
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

// TestRun_DebugFlagRemoved pins the ADR 0005 (local) silent posture: codelens
// carries no logging infrastructure, so --debug is not a flag and is rejected as
// one. It was a root flag, so urfave resolves it through the subcommand lineage
// whether it appears before or after the subcommand name; both positions must be
// rejected with unknown_flag.
func TestRun_DebugFlagRemoved(t *testing.T) {
	for _, args := range [][]string{
		{"--debug", "authors"},
		{"authors", "--debug"},
	} {
		code, _ := runUsageError(t, args...)
		if code != "unknown_flag" {
			t.Errorf("run(%v) code = %q, want unknown_flag", args, code)
		}
	}
}
