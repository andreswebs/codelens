package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/analysis"
)

// schemaList mirrors the `schema` (no --command) envelope the tests inspect.
type schemaList struct {
	SchemaVersion int  `json:"schema_version"`
	OK            bool `json:"ok"`
	Commands      []struct {
		Name      string   `json:"name"`
		Aliases   []string `json:"aliases"`
		Summary   string   `json:"summary"`
		ExitCodes []int    `json:"exit_codes"`
	} `json:"commands"`
}

// schemaCmd mirrors the `schema --command CMD` envelope the tests inspect.
type schemaCmd struct {
	SchemaVersion int      `json:"schema_version"`
	OK            bool     `json:"ok"`
	Command       string   `json:"command"`
	Summary       string   `json:"summary"`
	Aliases       []string `json:"aliases"`
	Flags         []struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Desc string `json:"desc"`
	} `json:"flags"`
	RowSchema []struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Desc string `json:"desc"`
	} `json:"row_schema"`
	ErrorCodes []string `json:"error_codes"`
	ExitCodes  []int    `json:"exit_codes"`
}

func TestSchema_List(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "schema"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}

	var list schemaList
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
		t.Fatalf("stdout is not a schema list envelope: %v\n%s", err, stdout.String())
	}
	if !list.OK || len(list.Commands) == 0 {
		t.Fatalf("list = %+v, want ok with commands", list)
	}

	byName := map[string]int{}
	for _, c := range list.Commands {
		byName[c.Name]++
	}
	for _, name := range []string{"authors", "schema", "print-log-command", "version"} {
		if byName[name] == 0 {
			t.Errorf("command %q missing from schema list", name)
		}
	}

	for _, c := range list.Commands {
		if c.Name == "authors" {
			if len(c.ExitCodes) == 0 {
				t.Errorf("authors exit_codes empty, want the analysis set")
			}
		}
	}
}

func TestSchema_Command_Authors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "schema", "--command", "authors"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}

	var got schemaCmd
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a schema envelope: %v\n%s", err, stdout.String())
	}
	if !got.OK || got.Command != "authors" {
		t.Fatalf("schema = %+v, want ok/authors", got)
	}
	if len(got.Flags) != 0 {
		t.Errorf("authors flags = %+v, want none", got.Flags)
	}

	wantCols := map[string]bool{"entity": false, "n_authors": false, "n_revs": false}
	for _, c := range got.RowSchema {
		if _, ok := wantCols[c.Name]; ok {
			wantCols[c.Name] = true
		}
		if c.Desc == "" {
			t.Errorf("column %q has empty desc", c.Name)
		}
	}
	for name, seen := range wantCols {
		if !seen {
			t.Errorf("row_schema missing column %q", name)
		}
	}

	if !contains(got.ErrorCodes, "empty_log") {
		t.Errorf("error_codes = %v, want to include empty_log", got.ErrorCodes)
	}
	if !equalInts(got.ExitCodes, []int{0, 2, 3, 1}) {
		t.Errorf("exit_codes = %v, want [0 2 3 1]", got.ExitCodes)
	}
}

// TestSchema_Command_Alias verifies --command resolves through the alias index.
// authors carries no alias yet, so this asserts the negative (an unknown alias
// is rejected) and, for any analysis that does declare one, that the alias
// resolves to the same canonical schema.
func TestSchema_Command_Alias(t *testing.T) {
	var found bool
	for _, d := range analysis.All() {
		for _, alias := range d.Aliases {
			found = true
			var stdout, stderr bytes.Buffer
			code := run([]string{"codelens", "schema", "--command", alias}, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("schema --command %q exit = %d, want 0; stderr:\n%s", alias, code, stderr.String())
			}
			var got schemaCmd
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatalf("alias %q: not a schema envelope: %v", alias, err)
			}
			if got.Command != d.Name {
				t.Errorf("alias %q resolved to command %q, want %q", alias, got.Command, d.Name)
			}
		}
	}
	if !found {
		t.Log("no aliased analysis registered yet; alias resolution asserted only negatively")
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"codelens", "schema", "--command", "no-such-alias"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("unknown alias exit = %d, want 2", code)
	}
}

func TestSchema_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "schema", "--command", "nope"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stderr:\n%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on error, got:\n%s", stdout.String())
	}

	var env struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Details struct {
				KnownCommands []string `json:"known_commands"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
		t.Fatalf("stderr is not a JSON error envelope: %v\n%s", err, stderr.String())
	}
	if env.OK || env.Error.Code != "usage_error" {
		t.Errorf("error = %+v, want ok=false code=usage_error", env.Error)
	}
	if !contains(env.Error.Details.KnownCommands, "authors") {
		t.Errorf("known_commands = %v, want to list authors", env.Error.Details.KnownCommands)
	}
}

// TestSchema_Conformance guards Phase 4 additions: every registered analysis
// must expose a non-empty, fully documented row schema and exit-code set.
func TestSchema_Conformance(t *testing.T) {
	for _, d := range analysis.All() {
		var stdout, stderr bytes.Buffer
		code := run([]string{"codelens", "schema", "--command", d.Name}, strings.NewReader(""), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("schema --command %q exit = %d, want 0; stderr:\n%s", d.Name, code, stderr.String())
		}

		var got schemaCmd
		if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
			t.Fatalf("%q: not a schema envelope: %v", d.Name, err)
		}
		if len(got.RowSchema) == 0 {
			t.Errorf("%q: row_schema is empty", d.Name)
		}
		for _, c := range got.RowSchema {
			if c.Name == "" || c.Type == "" || c.Desc == "" {
				t.Errorf("%q: column %+v is not fully documented", d.Name, c)
			}
		}
		if len(got.ExitCodes) == 0 {
			t.Errorf("%q: exit_codes is empty", d.Name)
		}
	}
}

func contains(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func equalInts(a, b []int) bool {
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
