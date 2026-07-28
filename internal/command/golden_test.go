package command

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// update, when set via `go test -update`, regenerates the golden files from the
// current output instead of asserting against them. Normalization runs before a
// golden is written, so a regenerated golden can never contain a volatile value;
// even so, a regenerated golden must be reviewed by hand before it is committed.
// The flag is declared exactly once in this package: a second flag.Bool with the
// same name would panic the test binary at registration.
var update = flag.Bool("update", false, "regenerate internal/command golden files")

// authorsFixture is the shared input log for the authors goldens; its origin and
// expected result are documented in testdata/README.md.
const authorsFixture = "testdata/authors.log"

// tmpToken is the stable stand-in for a t.TempDir() path. A scenario that must
// name a filesystem path (a missing --log or --group target) writes tmpToken in
// its args; run() swaps it for the real temp dir before invoking Run and swaps
// it back out of both captured streams before comparison. Normalizing a volatile
// path to a fixed token is what keeps the log_open_failed / input_file_open_failed
// goldens stable rather than flaky; ADR 0007 calls this discipline load-bearing.
const tmpToken = "<TMPDIR>"

// goldenCase is one in-process scenario. Run is invoked with args (WITHOUT the
// program name) and stdin, and all three artifacts are compared against golden
// files: testdata/<name>.out (stdout), testdata/<name>.err (stderr), and
// testdata/<name>.exit (the exit code as decimal + newline). An empty stderr is
// goldened as an EMPTY file, never an absent one; a missing golden fails with a
// message naming -update, so absence can never pass as an empty assertion.
//
// This is an in-process harness only: it drives the Run(args, deps) delegate with
// buffer streams. ADR 0007 makes an exec-based end-to-end suite conditional on
// process-level behavior in-process tests cannot reach (signal handling and the
// 130/143 exits, subprocess lifecycles, child exit-status passthrough). codelens
// catches no signals and spawns no subprocesses, so it has none of that surface;
// the omission of an exec suite is therefore a decision, not a gap.
type goldenCase struct {
	name  string   // golden basename, also the subtest name
	args  []string // process args WITHOUT the program name
	stdin string   // stdin content streamed to Run
}

// goldenCases is the single source of truth for both the golden comparison
// (TestGolden) and the exit-code conformance guard
// (TestExitCodes_DeclaredCodesAreExercised). Deriving both from one table is the
// property that stops the goldens and the conformance set from drifting: a new
// scenario is covered by the goldens and counted toward exit-code coverage at
// once, and an exit a scenario stops producing disappears from both.
//
// Coverage spans every exit code the taxonomy lets codelens produce except 70
// (unreachable from well-formed input; see the conformance guard):
//
//   - 0  the four authors success variants, the coupling warning, schema list,
//     the two temporal-period scenarios
//   - 64 the four usage errors, removed_format_flag, unknown_schema_command,
//     invalid_after_date
//   - 65 empty_log, invalid_control_char
//   - 74 log_open_failed, input_file_open_failed
func goldenCases(t *testing.T) []goldenCase {
	t.Helper()

	in, err := os.ReadFile(authorsFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	authorsLog := string(in)

	// sampleGitLog is a minimal valid git2(+subject) log for scenarios that must
	// get past the log parser to reach a later failure (removed_format_flag,
	// input_file_open_failed): an empty stdin would short-circuit to empty_log
	// before the flag is rejected.
	const sampleGitLog = "--a1--2024-01-01--Alice--c1\n10\t0\tsrc/foo.go\n"

	// nulLog embeds a NUL byte in an otherwise valid record to force the
	// disallowed-control-character data error.
	nulLog := "--a1--2024-01-01--Alice--c1\n10\t0\tsrc/\x00foo.go\n"

	return []goldenCase{
		// Four authors success variants: the JSON envelope plus --fields, --rows,
		// and schema --command. Their .err goldens are empty and their .exit
		// goldens are 0.
		{"authors_json", []string{"authors"}, authorsLog},
		{"authors_fields", []string{"--fields", "rows.entity", "authors"}, authorsLog},
		{"authors_rows2", []string{"--rows", "2", "authors"}, authorsLog},
		{"authors_schema", []string{"schema", "--command", "authors"}, ""},

		// Four usage errors (exit 64). Each pins the exact stderr envelope,
		// including code and hint.
		{"usage_unknown_flag", []string{"authors", "--nope"}, ""},
		{"usage_unknown_command", []string{"frobnicate"}, ""},
		{"usage_invalid_value", []string{"authors", "--rows", "abc"}, ""},
		{"usage_missing_required_flag", []string{"messages"}, ""},

		// empty_log (exit 65): an empty stdin is a data error. Keeps the exit-65
		// and empty_log coverage the deleted format_error_* scenarios were
		// incidentally providing (all three were the same empty_log envelope).
		{"empty_log", []string{"authors"}, ""},

		// removed_format_flag (exit 64): pins that --format is gone, classified as
		// an unknown_flag usage error. Without this, a re-introduction of the flag
		// would pass the suite (D2). Needs a valid log on stdin or it
		// short-circuits to empty_log before the flag is rejected.
		{"removed_format_flag", []string{"--format", "json", "authors"}, sampleGitLog},

		// One scenario per error code renamed in cod-uavr, so each renamed code
		// carries a golden envelope.
		{"unknown_schema_command", []string{"schema", "--command", "bogus"}, ""},
		{"invalid_after_date", []string{"print-log-command", "--after", "notadate"}, ""},
		{"log_open_failed", []string{"--log", tmpToken + "/missing.log", "authors"}, ""},
		{"input_file_open_failed", []string{"--group", tmpToken + "/missing.txt", "authors"}, sampleGitLog},
		{"invalid_control_char", []string{"authors"}, nulLog},

		// The coupling warning: exit 0, a non-empty stdout envelope, and a
		// NON-EMPTY stderr carrying the warning envelope. This is the scenario a
		// stdout-only harness structurally cannot express.
		{"coupling_warning", []string{"coupling"}, weakCouplingLog(5, 12)},

		// coupling_warning_revs (exit 0): the sibling attribution to
		// coupling_warning. Here a.go and b.go are coupled at 100% but co-change
		// only 3 times, so --min-revs and --min-shared-revs bind while
		// --min-coupling is satisfied. Freezes that the hint names the revision
		// thresholds and never blames --min-coupling (cod-7l0q).
		{"coupling_warning_revs", []string{"coupling"}, weakCouplingLog(3, 0)},

		// schema with no --command: the command list, including the errors
		// inventory, so the inventory has golden coverage.
		{"schema_list", []string{"schema"}, ""},

		// authors_grouped (exit 0): pins the D4 semantic degradation (grouped entity
		// becomes a label, not a filepath) and the transforms record together. The
		// group fixture must exist, so it is a committed testdata file referenced by
		// relative path (the tmpToken harness only names files it does not create).
		{"authors_grouped", []string{"--group", "testdata/layers.group", "authors"}, authorsLog},

		// coupling_verbose (exit 0): pins the D15 flag-gated semantics. --verbose adds
		// the three revision-detail columns, so the semantics map carries 7 entries,
		// not the 4 of a plain coupling run.
		{"coupling_verbose", []string{"coupling", "--verbose"}, weakCouplingLog(8, 2)},

		// revisions_temporal (exit 0): --temporal-period on a non-changeset
		// analysis with an additive column. The .err file is the point: it pins
		// the temporal_period_recounts warning, naming the affected columns in
		// details. No other golden exercises --temporal-period at all.
		{"revisions_temporal", []string{"--temporal-period", "3", "revisions"}, dailyPairLog(8)},

		// soc_temporal (exit 0): the negative assertion proving ChangesetBased
		// works. sum-of-coupling reinterprets a revision as a logical change set
		// by design, so the same transform must warn NOTHING (an empty .err).
		{"soc_temporal", []string{"--temporal-period", "3", "sum-of-coupling"}, dailyPairLog(8)},
	}
}

// dailyPairLog builds a git2+subject log where a.go and b.go co-change once per
// day for `days` consecutive days starting 2024-01-01. Spreading the commits
// over distinct days gives --temporal-period real windows to collapse: with
// period 3 the pair appears in more overlapping windows than it has commits, so
// a temporal run produces non-empty rows for both revisions (window counts) and
// sum-of-coupling (per-window co-change gains above the default threshold).
func dailyPairLog(days int) string {
	var b strings.Builder
	for i := 0; i < days; i++ {
		date := time.Date(2024, 1, 1+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		fmt.Fprintf(&b, "--%07x--%s--Alice--c%d\n", i+1, date, i+1)
		b.WriteString("1\t0\ta.go\n1\t0\tb.go\n\n")
	}
	return b.String()
}

// weakCouplingLog builds a git2+subject log where a.go and b.go co-change in
// `shared` commits and each also changes alone in `alone` further commits. With
// shared=5 and alone=12 each entity has 17 revisions, so the pair's degree is
// 5/17 = 29%, just under the default --min-coupling 30: a candidate pair that is
// nonetheless filtered out, which is what triggers the coupling_all_filtered
// warning.
func weakCouplingLog(shared, alone int) string {
	var b strings.Builder
	n := 0
	entry := func(files ...string) {
		n++
		fmt.Fprintf(&b, "--%07x--2024-01-01--Alice--c%d\n", n, n)
		for _, f := range files {
			fmt.Fprintf(&b, "1\t0\t%s\n", f)
		}
		b.WriteString("\n")
	}
	for i := 0; i < shared; i++ {
		entry("a.go", "b.go")
	}
	for i := 0; i < alone; i++ {
		entry("a.go")
	}
	for i := 0; i < alone; i++ {
		entry("b.go")
	}
	return b.String()
}

// run drives Run for the scenario against buffer streams and returns the
// normalized stdout and stderr plus the exit code. When the args carry tmpToken,
// it is swapped for a fresh t.TempDir() before the invocation and normalized back
// out of both streams afterward, so a temp path can never reach a golden.
func (c goldenCase) run(t *testing.T) (stdout, stderr string, exit int) {
	t.Helper()

	args := c.args
	tmpDir := ""
	if argsNeedTmp(c.args) {
		tmpDir = t.TempDir()
		args = make([]string, len(c.args))
		for i, a := range c.args {
			args[i] = strings.ReplaceAll(a, tmpToken, tmpDir)
		}
	}

	var out, errb bytes.Buffer
	exit = Run(args, Deps{In: strings.NewReader(c.stdin), Out: &out, Err: &errb})
	return normalize(out.String(), tmpDir), normalize(errb.String(), tmpDir), exit
}

// argsNeedTmp reports whether any arg carries the temp-dir token.
func argsNeedTmp(args []string) bool {
	for _, a := range args {
		if strings.Contains(a, tmpToken) {
			return true
		}
	}
	return false
}

// normalize replaces the volatile temp-dir path with tmpToken. It runs before
// comparison AND before a golden is written under -update, so no golden can ever
// hold a volatile value in the first place.
func normalize(s, tmpDir string) string {
	if tmpDir != "" {
		s = strings.ReplaceAll(s, tmpDir, tmpToken)
	}
	return s
}

// TestGolden drives every scenario in-process and compares its stdout, stderr,
// and exit code against golden files. Run with -update to regenerate the goldens
// after an intentional surface change, then review the diff by hand: the diff is
// the release note's evidence.
func TestGolden(t *testing.T) {
	for _, c := range goldenCases(t) {
		t.Run(c.name, func(t *testing.T) {
			stdout, stderr, exit := c.run(t)
			checkArtifact(t, c.name+".out", []byte(stdout))
			checkArtifact(t, c.name+".err", []byte(stderr))
			checkArtifact(t, c.name+".exit", []byte(strconv.Itoa(exit)+"\n"))
		})
	}
}

// checkArtifact compares got against testdata/<base>, or rewrites it under
// -update. An absent golden is a hard failure naming -update, so a scenario can
// never silently pass by asserting against nothing (an empty stderr is an empty
// file, not an absent one).
func checkArtifact(t *testing.T, base string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", base)

	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run `go test -update` to create it): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s mismatch\n got:\n%s\nwant:\n%s", path, got, want)
	}
}

// TestE2E_Authors_JSONReviewed guards the JSON golden's meaning independently of
// the byte comparison: the authors result must carry exactly the four fixture
// entities, ranked with the multi-author git2 parser first. An unreviewed
// -update that changed the analysis semantics consistently across every golden
// would still fail here, so the triple harness does not replace it.
func TestE2E_Authors_JSONReviewed(t *testing.T) {
	in, err := os.ReadFile(authorsFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"authors"}, Deps{In: bytes.NewReader(in), Out: &stdout, Err: &stderr}); code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}

	var env struct {
		Analysis string `json:"analysis"`
		RowCount int    `json:"row_count"`
		Rows     []struct {
			Entity   string `json:"entity"`
			NAuthors int    `json:"n_authors"`
			NRevs    int    `json:"n_revs"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not a result envelope: %v\n%s", err, stdout.String())
	}

	if env.Analysis != "authors" {
		t.Errorf("analysis = %q, want authors", env.Analysis)
	}
	if env.RowCount != 4 || len(env.Rows) != 4 {
		t.Fatalf("row_count = %d, len(rows) = %d, want 4", env.RowCount, len(env.Rows))
	}

	top := env.Rows[0]
	if top.Entity != "src/code_maat/parsers/git2.clj" || top.NAuthors != 2 || top.NRevs != 2 {
		t.Errorf("top row = %+v, want git2.clj with n_authors=2 n_revs=2", top)
	}
}
