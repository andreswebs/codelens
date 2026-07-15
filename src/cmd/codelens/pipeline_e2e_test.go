package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemp writes content to a uniquely named file in the test's temp dir and
// returns its path, for feeding --group / --team-map definitions to run().
func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// runAuthors executes the authors analysis with args over log on stdin and
// decodes the JSON envelope, failing on a non-zero exit or non-empty stderr.
func runAuthors(t *testing.T, log string, args ...string) authorsEnvelope {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"codelens", "authors"}, args...), strings.NewReader(log), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty on success, got:\n%s", stderr.String())
	}
	var env authorsEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout.String())
	}
	return env
}

// TestE2E_Pipeline_Group proves --group is honored end-to-end: src/foo.go is
// remapped to its layer and lib/bar.go, matching no rule, is dropped before the
// authors analysis runs.
func TestE2E_Pipeline_Group(t *testing.T) {
	const log = `--a1--2024-01-01--Alice--c1
10	0	src/foo.go
5	0	lib/bar.go

--b2--2024-01-02--Bob--c2
3	0	src/foo.go
`
	groupFile := writeTemp(t, "layers.txt", "src => SrcLayer\n")

	env := runAuthors(t, log, "--group", groupFile)

	if env.RowCount != 1 || len(env.Rows) != 1 {
		t.Fatalf("row_count=%d rows=%d, want 1/1 (lib/bar.go dropped, src/foo.go grouped)", env.RowCount, len(env.Rows))
	}
	got := env.Rows[0]
	if got.Entity != "SrcLayer" {
		t.Errorf("entity = %q, want SrcLayer (grouping not applied)", got.Entity)
	}
	if got.NAuthors != 2 || got.NRevs != 2 {
		t.Errorf("SrcLayer = n_authors %d / n_revs %d, want 2/2", got.NAuthors, got.NRevs)
	}
}

// TestE2E_Pipeline_Temporal proves --temporal-period is honored end-to-end: two
// same-day commits to src/foo.go collapse into one logical change within a
// 1-day window, so the entity shows a single author and revision.
func TestE2E_Pipeline_Temporal(t *testing.T) {
	const log = `--a1--2024-01-01--Alice--c1
10	0	src/foo.go

--b2--2024-01-01--Bob--c2
3	0	src/foo.go
`
	// Without temporal collapsing the entity would show 2 authors / 2 revs.
	base := runAuthors(t, log)
	if base.Rows[0].NAuthors != 2 || base.Rows[0].NRevs != 2 {
		t.Fatalf("baseline = %+v, want 2 authors / 2 revs before collapsing", base.Rows[0])
	}

	env := runAuthors(t, log, "--temporal-period", "1")

	if env.RowCount != 1 || len(env.Rows) != 1 {
		t.Fatalf("row_count=%d rows=%d, want 1/1", env.RowCount, len(env.Rows))
	}
	got := env.Rows[0]
	if got.Entity != "src/foo.go" {
		t.Errorf("entity = %q, want src/foo.go", got.Entity)
	}
	if got.NAuthors != 1 || got.NRevs != 1 {
		t.Errorf("src/foo.go = n_authors %d / n_revs %d, want 1/1 (window collapsed)", got.NAuthors, got.NRevs)
	}
}

// TestE2E_Pipeline_TeamMap proves --team-map is honored end-to-end: both authors
// map to the same team, so the shared entity reports a single distinct author.
func TestE2E_Pipeline_TeamMap(t *testing.T) {
	const log = `--a1--2024-01-01--Alice--c1
10	0	src/foo.go

--b2--2024-01-02--Bob--c2
3	0	src/foo.go
`
	teamFile := writeTemp(t, "team-map.csv", "author,team\nAlice,Core\nBob,Core\n")

	env := runAuthors(t, log, "--team-map", teamFile)

	if env.RowCount != 1 || len(env.Rows) != 1 {
		t.Fatalf("row_count=%d rows=%d, want 1/1", env.RowCount, len(env.Rows))
	}
	got := env.Rows[0]
	if got.NAuthors != 1 || got.NRevs != 2 {
		t.Errorf("src/foo.go = n_authors %d / n_revs %d, want 1/2 (authors merged into one team)", got.NAuthors, got.NRevs)
	}
}

// TestE2E_Pipeline_Exclude proves --exclude is honored end-to-end and is
// repeatable: two exclude globs each drop their matching entity before the
// analysis runs, leaving only the authored source file.
func TestE2E_Pipeline_Exclude(t *testing.T) {
	const log = `--a1--2024-01-01--Alice--c1
10	0	src/app.go
5	0	src/Migrations/0001.go
2	0	src/app.g.dart

--b2--2024-01-02--Bob--c2
3	0	src/app.go
`
	env := runAuthors(t, log, "--exclude", "**/Migrations/**", "--exclude", "**/*.g.dart")

	if env.RowCount != 1 || len(env.Rows) != 1 {
		t.Fatalf("row_count=%d rows=%d, want 1/1 (generated files excluded)", env.RowCount, len(env.Rows))
	}
	if got := env.Rows[0].Entity; got != "src/app.go" {
		t.Errorf("entity = %q, want src/app.go", got)
	}
}

// TestE2E_Pipeline_IncludeThenExclude proves --include/--exclude precedence
// end-to-end: only included entities survive, and an exclude still drops one of
// them.
func TestE2E_Pipeline_IncludeThenExclude(t *testing.T) {
	const log = `--a1--2024-01-01--Alice--c1
10	0	src/Page.cs
5	0	src/Page.Designer.cs
2	0	src/app.g.dart
`
	env := runAuthors(t, log, "--include", "**/*.cs", "--exclude", "**/*.Designer.cs")

	if env.RowCount != 1 || len(env.Rows) != 1 {
		t.Fatalf("row_count=%d rows=%d, want 1/1", env.RowCount, len(env.Rows))
	}
	if got := env.Rows[0].Entity; got != "src/Page.cs" {
		t.Errorf("entity = %q, want src/Page.cs (.Designer.cs excluded, .dart not included)", got)
	}
}

// TestE2E_Pipeline_BadGlob classifies a malformed --exclude glob as a usage
// error (exit 2) with a coded error envelope on stderr.
func TestE2E_Pipeline_BadGlob(t *testing.T) {
	const log = `--a1--2024-01-01--Alice--c1
10	0	src/app.go
`
	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"codelens", "authors", "--exclude", "a[b"},
		strings.NewReader(log), &stdout, &stderr,
	)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stderr:\n%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on error, got:\n%s", stdout.String())
	}
	var errEnv struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &errEnv); err != nil {
		t.Fatalf("stderr is not a JSON error envelope: %v\n%s", err, stderr.String())
	}
	if errEnv.OK || errEnv.Error.Code != "invalid_glob" {
		t.Errorf("error envelope = %+v, want ok=false code=invalid_glob", errEnv)
	}
}

// TestE2E_Pipeline_BadFile classifies an unreadable --group path as an input
// error (exit 3) with a coded error envelope on stderr, never a stack trace.
func TestE2E_Pipeline_BadFile(t *testing.T) {
	const log = `--a1--2024-01-01--Alice--c1
10	0	src/foo.go
`
	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"codelens", "authors", "--group", filepath.Join(t.TempDir(), "missing.txt")},
		strings.NewReader(log), &stdout, &stderr,
	)
	if code != 3 {
		t.Fatalf("exit code = %d, want 3; stderr:\n%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on error, got:\n%s", stdout.String())
	}
	var errEnv struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &errEnv); err != nil {
		t.Fatalf("stderr is not a JSON error envelope: %v\n%s", err, stderr.String())
	}
	if errEnv.OK || errEnv.Error.Code != "input_error" {
		t.Errorf("error envelope = %+v, want ok=false code=input_error", errEnv)
	}
}
