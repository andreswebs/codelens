package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/analysis"
)

// sampleLog is a two-entity git2(+subject) log: foo.go is touched by both
// authors across two commits, bar.go by one. The authors analysis therefore
// emits two rows (foo.go first: more authors), exercising row ordering and
// --rows truncation.
const sampleLog = `--a1b2c3--2024-01-01--Alice--first commit
10	2	foo.go
5	0	bar.go

--d4e5f6--2024-01-02--Bob--second commit
3	1	foo.go
`

// authorsEnvelope is the subset of the JSON success envelope the command tests
// assert against.
type authorsEnvelope struct {
	SchemaVersion int    `json:"schema_version"`
	OK            bool   `json:"ok"`
	Analysis      string `json:"analysis"`
	RowCount      int    `json:"row_count"`
	TotalCount    int    `json:"total_count"`
	Truncated     bool   `json:"truncated"`
	Rows          []struct {
		Entity   string `json:"entity"`
		NAuthors int    `json:"n_authors"`
		NRevs    int    `json:"n_revs"`
	} `json:"rows"`
}

func TestCmd_Authors_FromStdin_JSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "authors"}, strings.NewReader(sampleLog), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}

	var env authorsEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout.String())
	}
	if !env.OK || env.Analysis != "authors" {
		t.Fatalf("envelope ok=%v analysis=%q, want true/authors", env.OK, env.Analysis)
	}
	if env.RowCount != 2 || len(env.Rows) != 2 {
		t.Fatalf("row_count=%d rows=%d, want 2/2", env.RowCount, len(env.Rows))
	}
	if env.Rows[0].Entity != "foo.go" || env.Rows[0].NAuthors != 2 || env.Rows[0].NRevs != 2 {
		t.Errorf("row[0] = %+v, want foo.go/2/2", env.Rows[0])
	}
	if env.Rows[1].Entity != "bar.go" || env.Rows[1].NAuthors != 1 {
		t.Errorf("row[1] = %+v, want bar.go/1", env.Rows[1])
	}
}

func TestCmd_Alias_Resolves(t *testing.T) {
	// authors carries no alias, so its only invocation name is the canonical
	// one. Genuine alias resolution is exercised in Phase 4 once an aliased
	// analysis is registered.
	d, ok := analysis.Lookup("authors")
	if !ok {
		t.Fatal("authors analysis is not registered")
	}
	if len(d.Aliases) != 0 {
		t.Errorf("authors aliases = %v, want none", d.Aliases)
	}

	// An unknown command/alias is a usage error (exit 2), not a silent no-op.
	var stdout, stderr bytes.Buffer
	if code := run([]string{"codelens", "authorz"}, strings.NewReader(sampleLog), &stdout, &stderr); code != 2 {
		t.Fatalf("unknown alias exit code = %d, want 2; stderr:\n%s", code, stderr.String())
	}
}

func TestCmd_LogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.log")
	if err := os.WriteFile(path, []byte(sampleLog), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// Empty stdin proves the log is read from the file, not the pipe.
	code := run([]string{"codelens", "authors", "--log", path}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}

	var env authorsEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout.String())
	}
	if env.RowCount != 2 || env.Rows[0].Entity != "foo.go" {
		t.Errorf("--log result = %+v, want 2 rows led by foo.go", env)
	}
}

func TestCmd_MissingLog_EmptyStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "authors"}, strings.NewReader(""), &stdout, &stderr)

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
	if errEnv.OK || errEnv.Error.Code != "empty_log" {
		t.Errorf("error envelope = %+v, want ok=false code=empty_log", errEnv)
	}
}

func TestCmd_Rows_Truncates(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "authors", "--rows", "1"}, strings.NewReader(sampleLog), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}

	var env authorsEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout.String())
	}
	if env.RowCount != 1 || len(env.Rows) != 1 {
		t.Fatalf("row_count=%d rows=%d, want 1/1", env.RowCount, len(env.Rows))
	}
	if env.TotalCount != 2 || !env.Truncated {
		t.Errorf("total_count=%d truncated=%v, want 2/true", env.TotalCount, env.Truncated)
	}
	if env.Rows[0].Entity != "foo.go" {
		t.Errorf("kept row = %q, want foo.go (highest-ranked after sort)", env.Rows[0].Entity)
	}
}

// paramsEnvelope decodes only the params object so the dispatch tests can assert
// which effective flags a result echoes without pinning the rows.
type paramsEnvelope struct {
	Analysis string         `json:"analysis"`
	Params   map[string]any `json:"params"`
}

func TestCmd_FlaggedAnalysis_EchoesEffectiveParams(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// --min-coupling is overridden; the rest keep their declared defaults. params
	// must document every declared flag with the value actually applied.
	code := run(
		[]string{"codelens", "coupling", "--min-coupling", "42"},
		strings.NewReader(sampleLog), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}

	var env paramsEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout.String())
	}
	if env.Params == nil {
		t.Fatalf("coupling result has no params object, want effective flags echoed")
	}
	// JSON numbers decode as float64 through map[string]any.
	want := map[string]float64{
		"min-revs":           5,
		"min-shared-revs":    5,
		"min-coupling":       42, // overridden
		"max-coupling":       100,
		"max-changeset-size": 30,
	}
	for name, wantVal := range want {
		got, ok := env.Params[name].(float64)
		if !ok {
			t.Errorf("params[%q] = %#v, want number %v", name, env.Params[name], wantVal)
			continue
		}
		if got != wantVal {
			t.Errorf("params[%q] = %v, want %v", name, got, wantVal)
		}
	}
	if v, ok := env.Params["verbose"].(bool); !ok || v {
		t.Errorf("params[verbose] = %#v, want false", env.Params["verbose"])
	}
}

func TestCmd_FlaglessAnalysis_OmitsParams(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "authors"}, strings.NewReader(sampleLog), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout.String())
	}
	if _, present := raw["params"]; present {
		t.Errorf("flagless analysis emitted a params key, want it omitted\n%s", stdout.String())
	}
}
