package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/analysis"
	"github.com/andreswebs/codelens/internal/output"
)

// exitScenario names a declared exit code and a way to produce it, so the
// conformance test can tie each declared code to a real observed exit rather
// than to prose. Every code declared by any command (analysis or meta) must
// have a scenario, and every scenario must be declared by at least one command;
// TestExitCodes_DeclaredCodesAreExercised enforces both directions.
type exitScenario struct {
	// code is the declared exit code this scenario produces.
	code int
	// observe returns the exit code actually produced. Most scenarios drive a
	// real run() invocation; the internal-fault code (70) has no well-formed CLI
	// input and instead exercises the same resolver run() applies to its error.
	observe func(t *testing.T) int
}

// exitScenarios maps each exit code the taxonomy (ADR 0002) lets codelens
// produce to a concrete way to observe it. 0/64/65/74 are driven end to end
// through run(); 70 is the internal-fault fallback, unreachable from
// well-formed input, so it exercises output.ExitCodeFor on an uncoded error,
// which is exactly what run() calls to resolve its exit.
func exitScenarios() []exitScenario {
	return []exitScenario{
		{code: 0, observe: func(t *testing.T) int {
			return runExit(t, []string{"codelens", "authors"}, sampleLog)
		}},
		{code: 64, observe: func(t *testing.T) int {
			return runExit(t, []string{"codelens", "authors", "--nope"}, "")
		}},
		{code: 65, observe: func(t *testing.T) int {
			return runExit(t, []string{"codelens", "authors"}, "")
		}},
		{code: 74, observe: func(t *testing.T) int {
			missing := filepath.Join(t.TempDir(), "missing.log")
			return runExit(t, []string{"codelens", "authors", "--log", missing}, "")
		}},
		{code: 70, observe: func(*testing.T) int {
			return output.ExitCodeFor(errors.New("unexpected internal fault"))
		}},
	}
}

// runExit executes run() with the given args and stdin and returns its exit
// code, discarding stdout and stderr.
func runExit(t *testing.T, args []string, stdin string) int {
	t.Helper()
	var stdout, stderr bytes.Buffer
	return run(args, strings.NewReader(stdin), &stdout, &stderr)
}

// TestExitCodes_DeclaredCodesAreExercised is the ADR 0002 conformance guard that
// ties declarations to real exits. Every exit code any command declares must be
// produced by a scenario and observed to match, and every scenario's code must
// be declared by some command, so neither a newly declared code nor a dropped
// one can slip past unexercised.
func TestExitCodes_DeclaredCodesAreExercised(t *testing.T) {
	declared := map[int]bool{}
	for _, d := range analysis.All() {
		for _, c := range d.ExitCodes {
			declared[c] = true
		}
	}
	for _, m := range metaCommands() {
		for _, c := range m.ExitCodes {
			declared[c] = true
		}
	}

	scenarios := exitScenarios()
	covered := map[int]bool{}
	for _, s := range scenarios {
		covered[s.code] = true
		t.Run(exitName(s.code), func(t *testing.T) {
			if !declared[s.code] {
				t.Fatalf("scenario exercises exit %d, which no command declares", s.code)
			}
			if got := s.observe(t); got != s.code {
				t.Errorf("observed exit = %d, want %d", got, s.code)
			}
		})
	}

	for code := range declared {
		if !covered[code] {
			t.Errorf("declared exit %d is never exercised by a scenario", code)
		}
	}
}

// exitName is a stable subtest label for an exit code.
func exitName(code int) string {
	switch code {
	case 0:
		return "success_0"
	case 64:
		return "usage_64"
	case 65:
		return "data_65"
	case 70:
		return "internal_70"
	case 74:
		return "io_74"
	default:
		return "code"
	}
}

// TestExitCodesRegistered_AllCommands is the conformance guard for the command
// exit/error-code contract. Every registered analysis must declare a non-empty
// exit-code set that includes success (0) and a non-empty error-code set, and
// `schema --command CMD` must surface exactly those codes. Every meta command
// must likewise carry its exit-code set into the `schema` command list. A future
// analysis added without codes trips this test.
func TestExitCodesRegistered_AllCommands(t *testing.T) {
	for _, d := range analysis.All() {
		t.Run(d.Name, func(t *testing.T) {
			if len(d.ExitCodes) == 0 {
				t.Fatalf("%q: descriptor ExitCodes is empty", d.Name)
			}
			if !containsInt(d.ExitCodes, 0) {
				t.Errorf("%q: ExitCodes = %v, want to include success code 0", d.Name, d.ExitCodes)
			}
			if len(d.ErrorCodes) == 0 {
				t.Errorf("%q: descriptor ErrorCodes is empty", d.Name)
			}

			got := schemaOf(t, d.Name)
			if !equalInts(got.ExitCodes, d.ExitCodes) {
				t.Errorf("%q: schema exit_codes = %v, want %v", d.Name, got.ExitCodes, d.ExitCodes)
			}
			if !equalStrings(got.ErrorCodes, d.ErrorCodes) {
				t.Errorf("%q: schema error_codes = %v, want %v", d.Name, got.ErrorCodes, d.ErrorCodes)
			}
		})
	}

	list := schemaListOf(t)
	byName := map[string][]int{}
	for _, c := range list.Commands {
		byName[c.Name] = c.ExitCodes
	}
	for _, m := range metaCommands() {
		t.Run(m.Name, func(t *testing.T) {
			listCodes, ok := byName[m.Name]
			if !ok {
				t.Fatalf("%q: missing from the schema command list", m.Name)
			}
			if !equalInts(listCodes, m.ExitCodes) {
				t.Errorf("%q: schema list exit codes = %v, want %v", m.Name, listCodes, m.ExitCodes)
			}

			got := schemaOf(t, m.Name)
			if !equalInts(got.ExitCodes, m.ExitCodes) {
				t.Errorf("%q: schema exit_codes = %v, want %v", m.Name, got.ExitCodes, m.ExitCodes)
			}
			if !equalStrings(got.ErrorCodes, nonNilMetaStrings(m.ErrorCodes)) {
				t.Errorf("%q: schema error_codes = %v, want %v", m.Name, got.ErrorCodes, m.ErrorCodes)
			}
		})
	}
}

// nonNilMetaStrings mirrors the schema builder's slice normalization so the
// conformance guard compares against the same [] the schema surfaces for a meta
// command with no declared error codes.
func nonNilMetaStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// TestSchema_ReportsDeclaredErrorCodes spot-checks that the schema surfaces each
// command's declared error codes: coupling reports empty_log, and messages
// reports both of its extra failure modes.
func TestSchema_ReportsDeclaredErrorCodes(t *testing.T) {
	if got := schemaOf(t, "coupling"); !contains(got.ErrorCodes, "empty_log") {
		t.Errorf("coupling error_codes = %v, want to include empty_log", got.ErrorCodes)
	}

	got := schemaOf(t, "messages")
	for _, code := range []string{"missing_messages", "invalid_expression"} {
		if !contains(got.ErrorCodes, code) {
			t.Errorf("messages error_codes = %v, want to include %q", got.ErrorCodes, code)
		}
	}
}

// schemaOf runs `codelens schema --command name` and decodes the emitted schema
// envelope, failing the test on a non-zero exit or malformed output.
func schemaOf(t *testing.T, name string) schemaCmd {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "schema", "--command", name}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("schema --command %q exit = %d, want 0; stderr:\n%s", name, code, stderr.String())
	}
	var got schemaCmd
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("%q: stdout is not a schema envelope: %v\n%s", name, err, stdout.String())
	}
	return got
}

// schemaListOf runs `codelens schema` (no --command) and decodes the command
// list envelope, failing the test on a non-zero exit or malformed output.
func schemaListOf(t *testing.T) schemaList {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "schema"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("schema exit = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	var got schemaList
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a schema list envelope: %v\n%s", err, stdout.String())
	}
	return got
}

func containsInt(s []int, want int) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
