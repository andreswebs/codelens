package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/analysis"
	"github.com/andreswebs/codelens/internal/output"
	"github.com/andreswebs/codelens/internal/terr"
)

// TestExitCodes_DeclaredCodesAreExercised is the ADR 0002 conformance guard that
// ties declarations to real exits, expressed THROUGH the golden harness rather
// than alongside it. It reads the exits observed from the goldenCases table (the
// same single source TestGolden pins), so the goldens and the exit-code coverage
// cannot drift: it asserts every exit any scenario observes is a member of the
// ExitCodes registry, and every code in the registry is exercised by some
// scenario. Exit 70 is the documented exception: it is unreachable from
// well-formed CLI input, so no artificial scenario is invented for it; instead it
// exercises output.ExitCodeFor on an uncoded error, exactly what Run calls to
// resolve its exit. The guard also checks that no descriptor declares a code the
// registry omits.
func TestExitCodes_DeclaredCodesAreExercised(t *testing.T) {
	declared := map[int]bool{}
	for _, c := range ExitCodes {
		declared[c] = true
	}

	// Every exit code any command declares must be a member of the ExitCodes
	// registry, so a descriptor cannot declare a code the registry omits.
	for _, d := range analysis.All() {
		for _, c := range d.ExitCodes {
			if !declared[c] {
				t.Errorf("analysis %q declares exit %d, absent from the ExitCodes registry", d.Name, c)
			}
		}
	}
	for _, m := range metaCommands() {
		for _, c := range m.ExitCodes {
			if !declared[c] {
				t.Errorf("meta %q declares exit %d, absent from the ExitCodes registry", m.Name, c)
			}
		}
	}

	// Observe every scenario's exit from the single golden table. Each observed
	// exit must be a registry member; the union of observed exits must reach every
	// registry code except the documented 70 exception.
	observed := map[int]bool{}
	for _, c := range goldenCases(t) {
		_, _, exit := c.run(t)
		if !declared[exit] {
			t.Errorf("scenario %q observed exit %d, which the ExitCodes registry does not list", c.name, exit)
		}
		observed[exit] = true
	}
	observed[output.ExitCodeFor(errors.New("unexpected internal fault"))] = true

	for code := range declared {
		if !observed[code] {
			t.Errorf("registry exit %d is never exercised by a scenario", code)
		}
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

// TestErrorCodes_DeclaredCodesExist is the ADR 0003 conformance guard tying the
// documented error surface to the sentinels that actually exist. Every code
// declared anywhere the schema surfaces it (an analysis descriptor, a meta
// command, or the tool-level common baseline) must be either a registered terr
// sentinel or a member of the closed non-sentinel allowlist. The reverse guard
// asserts no sentinel exists unreported: every code in terr.All() must appear in
// some command's declared list or in the common baseline. It fails when a
// descriptor declares a code no sentinel carries, or when a sentinel is
// reachable but documented nowhere.
func TestErrorCodes_DeclaredCodesExist(t *testing.T) {
	sentinels := map[string]bool{}
	for _, e := range terr.All() {
		sentinels[e.Code()] = true
	}
	allowed := map[string]bool{}
	for _, c := range output.NonSentinelCodes {
		allowed[c] = true
	}

	// declared collects every code the schema surfaces, tagged with where it was
	// declared for a legible failure. It also feeds the reverse-direction guard.
	declared := map[string]bool{}
	declare := func(t *testing.T, where, code string) {
		declared[code] = true
		if sentinels[code] || allowed[code] {
			return
		}
		t.Errorf("%s declares error code %q with no terr sentinel and not in output.NonSentinelCodes", where, code)
	}

	for _, d := range analysis.All() {
		for _, c := range d.ErrorCodes {
			declare(t, "analysis "+d.Name, c)
		}
	}
	for _, m := range metaCommands() {
		for _, c := range m.ErrorCodes {
			declare(t, "meta "+m.Name, c)
		}
	}
	for _, c := range analysis.CommonErrorCodes() {
		declare(t, "common_error_codes", c)
	}

	// unknown_command is a pre-dispatch sentinel not attributable to any command:
	// it is reported to agents only through the tool-level errors inventory
	// (terr.All(), surfaced by the `schema` command), never as a command's
	// error_codes or in the common baseline. Mark it declared so the reverse guard
	// accepts it without forcing it into a command's per-command surface.
	declared[ErrUnknownCommand.Code()] = true

	for code := range sentinels {
		if !declared[code] {
			t.Errorf("terr sentinel %q is reachable but declared by no command and absent from common_error_codes", code)
		}
	}
}

// TestNonSentinelAllowlist_MatchesClassifier pins the non-sentinel allowlist to
// the internal-fault fallback alone: the usage classes became registered
// sentinels (usage.go), so internal_error is the only code an invocation emits
// that is not backed by a terr sentinel. The cross-check then ties the
// classifier table to the registry, so a new usage class must be a real sentinel
// (and stay out of the allowlist) to pass. This is the second half of the guard
// in TestErrorCodes_DeclaredCodesExist.
func TestNonSentinelAllowlist_MatchesClassifier(t *testing.T) {
	want := []string{output.InternalErrorCode}
	got := append([]string(nil), output.NonSentinelCodes...)
	sort.Strings(want)
	sort.Strings(got)
	if !equalStrings(got, want) {
		t.Errorf("output.NonSentinelCodes = %v, want %v", got, want)
	}

	sentinels := map[string]bool{}
	for _, e := range terr.All() {
		sentinels[e.Code()] = true
	}
	for _, code := range usageClassCodes() {
		if !sentinels[code] {
			t.Errorf("classifier code %q is not a registered terr sentinel", code)
		}
		if contains(output.NonSentinelCodes, code) {
			t.Errorf("classifier code %q is a sentinel; it must not be in NonSentinelCodes", code)
		}
	}
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
	code := Run([]string{"schema", "--command", name}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})
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
	code := Run([]string{"schema"}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})
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
