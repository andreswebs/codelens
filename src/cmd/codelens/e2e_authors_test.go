package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update, when set via `go test -update`, regenerates the end-to-end golden
// files from the current output instead of asserting against them. Regenerated
// goldens must be reviewed by hand before being committed.
var update = flag.Bool("update", false, "regenerate cmd/codelens golden files")

// authorsFixture is the shared input log for the authors end-to-end goldens; it
// is read once and streamed as stdin to each run(). Origin and the expected
// result are documented in testdata/README.md.
const authorsFixture = "testdata/authors.log"

// e2eCase is one end-to-end golden: run() is invoked with args (argv, including
// the program name) against the authors fixture on stdin, and its stdout is
// compared byte-for-byte with testdata/<golden>.
type e2eCase struct {
	name   string
	args   []string
	golden string
}

// authorsCases enumerates the authors slice across every output format plus the
// --fields, --rows, and schema variants. Global flags precede the subcommand
// name, the position the design documents as canonical.
var authorsCases = []e2eCase{
	{"json", []string{"codelens", "authors"}, "authors.json"},
	{"ndjson", []string{"codelens", "--format", "ndjson", "authors"}, "authors.ndjson"},
	{"csv", []string{"codelens", "--format", "csv", "authors"}, "authors.csv"},
	{"table", []string{"codelens", "--format", "table", "authors"}, "authors.table"},
	{"fields", []string{"codelens", "--fields", "rows.entity", "authors"}, "authors.fields.json"},
	{"rows2", []string{"codelens", "--rows", "2", "authors"}, "authors.rows2.json"},
	{"schema", []string{"codelens", "schema", "--command", "authors"}, "authors.schema.json"},
}

// TestE2E_Authors drives run() end-to-end for each format and variant, asserting
// a clean exit (code 0, empty stderr) and comparing stdout to the committed
// golden. Run with -update to regenerate the goldens after an intentional
// surface change.
func TestE2E_Authors(t *testing.T) {
	in, err := os.ReadFile(authorsFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	for _, tc := range authorsCases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.args, bytes.NewReader(in), &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr should be empty on success, got:\n%s", stderr.String())
			}

			got := stdout.Bytes()
			goldenPath := filepath.Join("testdata", tc.golden)

			if *update {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (run `go test -update` to create it): %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("output does not match %s\n got:\n%s\nwant:\n%s", goldenPath, got, want)
			}
		})
	}
}

// TestE2E_Authors_JSONReviewed guards the JSON golden's meaning independently of
// the byte comparison: the authors result must carry exactly the four fixture
// entities, ranked with the multi-author git2 parser first. An unreviewed
// -update that changed the analysis semantics would fail here even if every
// golden was rewritten consistently.
func TestE2E_Authors_JSONReviewed(t *testing.T) {
	in, err := os.ReadFile(authorsFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"codelens", "authors"}, bytes.NewReader(in), &stdout, &stderr); code != 0 {
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
