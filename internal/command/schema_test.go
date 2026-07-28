package command

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
	Errors []struct {
		Code     string `json:"code"`
		ExitCode int    `json:"exit_code"`
		Hint     string `json:"hint"`
	} `json:"errors"`
	AggregationRoles map[string]string `json:"aggregation_roles"`
}

// schemaCmd mirrors the `schema --command CMD` envelope the tests inspect.
type schemaCmd struct {
	SchemaVersion int      `json:"schema_version"`
	OK            bool     `json:"ok"`
	Command       string   `json:"command"`
	Summary       string   `json:"summary"`
	Aliases       []string `json:"aliases"`
	Shape         string   `json:"shape"`
	Flags         []struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Desc string `json:"desc"`
	} `json:"flags"`
	RowSchema []struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Semantic string `json:"semantic"`
		Desc     string `json:"desc"`
	} `json:"row_schema"`
	ErrorCodes       []string          `json:"error_codes"`
	CommonErrorCodes []string          `json:"common_error_codes"`
	ExitCodes        []int             `json:"exit_codes"`
	AggregationRoles map[string]string `json:"aggregation_roles"`
}

func TestSchema_List(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"schema"}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})
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
	for _, name := range []string{"authors", "schema", "print-log-command"} {
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

	// The list form publishes the full semantic-to-role catalog: one entry per
	// member of the closed semantic vocabulary. This is the wire-level
	// exhaustiveness check the golden pins.
	if len(list.AggregationRoles) != len(analysis.Semantics()) {
		t.Errorf("aggregation_roles has %d entries, want %d", len(list.AggregationRoles), len(analysis.Semantics()))
	}
	for _, s := range analysis.Semantics() {
		role, ok := list.AggregationRoles[string(s)]
		if !ok {
			t.Errorf("aggregation_roles missing semantic %q", s)
			continue
		}
		if want := analysis.AggRoleOf(s); role != string(want) {
			t.Errorf("aggregation_roles[%q] = %q, want %q", s, role, want)
		}
	}
}

func TestSchema_Command_Authors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"schema", "--command", "authors"}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})
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
	if !equalInts(got.ExitCodes, []int{0, 64, 65, 70, 74}) {
		t.Errorf("exit_codes = %v, want [0 64 65 70 74]", got.ExitCodes)
	}

	// The per-command form carries only the roles its own columns use: authors
	// declares filepath (entity) and count (n_authors, n_revs) columns.
	wantRoles := map[string]string{"filepath": "dimension", "count": "additive"}
	if len(got.AggregationRoles) != len(wantRoles) {
		t.Errorf("aggregation_roles = %v, want %v", got.AggregationRoles, wantRoles)
	}
	for s, r := range wantRoles {
		if got.AggregationRoles[s] != r {
			t.Errorf("aggregation_roles[%q] = %q, want %q", s, got.AggregationRoles[s], r)
		}
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
			code := Run([]string{"schema", "--command", alias}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})
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
	if code := Run([]string{"schema", "--command", "no-such-alias"}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr}); code != 64 {
		t.Fatalf("unknown alias exit = %d, want 64", code)
	}
}

func TestSchema_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"schema", "--command", "nope"}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})
	if code != 64 {
		t.Fatalf("exit code = %d, want 64; stderr:\n%s", code, stderr.String())
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
	if env.OK || env.Error.Code != "unknown_schema_command" {
		t.Errorf("error = %+v, want ok=false code=unknown_schema_command", env.Error)
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
		code := Run([]string{"schema", "--command", d.Name}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})
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
			if c.Semantic == "" || !analysis.ValidSemantic(analysis.Semantic(c.Semantic)) {
				t.Errorf("%q: column %q has semantic %q, not a member of %v", d.Name, c.Name, c.Semantic, analysis.Semantics())
			}
		}
		if len(got.ExitCodes) == 0 {
			t.Errorf("%q: exit_codes is empty", d.Name)
		}
		if !analysis.ValidShape(analysis.Shape(got.Shape)) {
			t.Errorf("%q: shape %q is not a member of the closed set %v", d.Name, got.Shape, analysis.Shapes())
		}

		// aggregation_roles is exactly the roles of the DECLARED columns:
		// every row_schema semantic (flag-gated included, since the schema
		// declares the full untransformed vocabulary) is covered, and no
		// semantic outside the declared set appears.
		declared := map[string]bool{}
		for _, c := range got.RowSchema {
			declared[c.Semantic] = true
			role, ok := got.AggregationRoles[c.Semantic]
			if !ok {
				t.Errorf("%q: aggregation_roles missing semantic %q (column %q)", d.Name, c.Semantic, c.Name)
				continue
			}
			if want := analysis.AggRoleOf(analysis.Semantic(c.Semantic)); role != string(want) {
				t.Errorf("%q: aggregation_roles[%q] = %q, want %q", d.Name, c.Semantic, role, want)
			}
		}
		for s := range got.AggregationRoles {
			if !declared[s] {
				t.Errorf("%q: aggregation_roles has %q, not a declared column semantic", d.Name, s)
			}
		}
	}
}

// TestSchema_MetaShapes pins the D5 shape declarations for the meta commands:
// print-log-command reports shape "text" so an agent learns its stdout is a bare
// command line, and schema omits the key entirely because its stdout is an
// introspection envelope, not an analysis result.
func TestSchema_MetaShapes(t *testing.T) {
	t.Run("print-log-command", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"schema", "--command", "print-log-command"}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr:\n%s", code, stderr.String())
		}
		var got schemaCmd
		if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
			t.Fatalf("not a schema envelope: %v", err)
		}
		if analysis.Shape(got.Shape) != analysis.ShapeText {
			t.Errorf("shape = %q, want %q", got.Shape, analysis.ShapeText)
		}
	})

	t.Run("schema omits shape", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"schema", "--command", "schema"}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr:\n%s", code, stderr.String())
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
			t.Fatalf("not a JSON object: %v", err)
		}
		if _, ok := m["shape"]; ok {
			t.Errorf("schema --command schema should omit the shape key; got %s", stdout.String())
		}
	})
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
