package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/analysis"
)

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
