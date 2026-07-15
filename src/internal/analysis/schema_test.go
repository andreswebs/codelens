package analysis

import (
	"encoding/json"
	"testing"

	"github.com/andreswebs/codelens/internal/output"
)

// descriptorFixture is a descriptor exercising every schema-relevant field:
// aliases, a typed flag with a default, a documented row schema, and error/exit
// code sets. The builder tests assert these survive into the schema object.
func descriptorFixture() Descriptor {
	return Descriptor{
		Name:    "coupling",
		Aliases: []string{"logical"},
		Summary: "Logical (temporal) coupling between entity pairs",
		Flags: []Flag{
			{Name: "min-coupling", Type: "int", Default: 30, Required: false, Desc: "minimum coupling degree in percent"},
		},
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "coupled", Type: "string", Desc: "co-changing module path"},
		},
		ErrorCodes: []string{"parse_error", "empty_log"},
		ExitCodes:  []int{0, 2, 3, 1},
	}
}

func TestSchema_BuildsFromDescriptor(t *testing.T) {
	got := Schema(descriptorFixture())

	if got.SchemaVersion != output.SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", got.SchemaVersion, output.SchemaVersion)
	}
	if !got.OK {
		t.Error("OK = false, want true")
	}
	if got.Command != "coupling" {
		t.Errorf("Command = %q, want %q", got.Command, "coupling")
	}
	if got.Summary != "Logical (temporal) coupling between entity pairs" {
		t.Errorf("Summary = %q", got.Summary)
	}
	if len(got.Aliases) != 1 || got.Aliases[0] != "logical" {
		t.Errorf("Aliases = %v, want [logical]", got.Aliases)
	}
	if len(got.Flags) != 1 || got.Flags[0].Name != "min-coupling" || got.Flags[0].Default != 30 {
		t.Errorf("Flags = %+v, want one min-coupling/30 flag", got.Flags)
	}
	if len(got.RowSchema) != 2 || got.RowSchema[0].Name != "entity" {
		t.Errorf("RowSchema = %+v, want entity-led 2 columns", got.RowSchema)
	}
	if len(got.ErrorCodes) != 2 || got.ErrorCodes[0] != "parse_error" {
		t.Errorf("ErrorCodes = %v", got.ErrorCodes)
	}
	if len(got.ExitCodes) != 4 || got.ExitCodes[3] != 1 {
		t.Errorf("ExitCodes = %v, want [0 2 3 1]", got.ExitCodes)
	}
}

// TestSchema_JSONKeys guards the snake_case wire contract: the schema object
// and its nested flag/column shapes must marshal to the keys an agent reads.
func TestSchema_JSONKeys(t *testing.T) {
	b, err := json.Marshal(Schema(descriptorFixture()))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{
		"schema_version", "ok", "command", "summary", "aliases",
		"flags", "row_schema", "error_codes", "exit_codes",
	} {
		if _, ok := m[key]; !ok {
			t.Errorf("schema JSON missing key %q; got %s", key, b)
		}
	}

	var flag map[string]json.RawMessage
	var flags []json.RawMessage
	if err := json.Unmarshal(m["flags"], &flags); err != nil || len(flags) == 0 {
		t.Fatalf("flags not a non-empty array: %v (%s)", err, m["flags"])
	}
	if err := json.Unmarshal(flags[0], &flag); err != nil {
		t.Fatalf("flag[0]: %v", err)
	}
	for _, key := range []string{"name", "type", "default", "required", "desc"} {
		if _, ok := flag[key]; !ok {
			t.Errorf("flag JSON missing key %q; got %s", key, flags[0])
		}
	}

	var cols []map[string]json.RawMessage
	if err := json.Unmarshal(m["row_schema"], &cols); err != nil || len(cols) == 0 {
		t.Fatalf("row_schema not a non-empty array: %v", err)
	}
	for _, key := range []string{"name", "type", "desc"} {
		if _, ok := cols[0][key]; !ok {
			t.Errorf("column JSON missing key %q", key)
		}
	}
}

// TestSchema_NormalizesEmptySlices ensures a descriptor with no aliases or flags
// marshals them as [] rather than null, so an agent can iterate unconditionally.
func TestSchema_NormalizesEmptySlices(t *testing.T) {
	d := Descriptor{Name: "authors", Summary: "s", ExitCodes: []int{0}}
	got := Schema(d)
	if got.Aliases == nil {
		t.Error("Aliases is nil, want empty non-nil slice")
	}
	if got.Flags == nil {
		t.Error("Flags is nil, want empty non-nil slice")
	}

	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(m["aliases"]) != "[]" {
		t.Errorf("aliases = %s, want []", m["aliases"])
	}
	if string(m["flags"]) != "[]" {
		t.Errorf("flags = %s, want []", m["flags"])
	}
}

// TestMetaSchema_BuildsAndNormalizes checks the non-analysis schema constructor:
// it carries the explicit identity/flags/codes through and normalizes the
// alias/row-schema slices (always empty for a meta command) to non-nil.
func TestMetaSchema_BuildsAndNormalizes(t *testing.T) {
	flags := []Flag{
		{Name: "command", Type: "string", Desc: "describe a single CMD"},
	}
	got := MetaSchema("schema", "describe commands", flags, []string{"usage_error"}, []int{0, 2})

	if got.SchemaVersion != output.SchemaVersion || !got.OK {
		t.Errorf("envelope = %+v, want ok/version set", got)
	}
	if got.Command != "schema" {
		t.Errorf("Command = %q, want %q", got.Command, "schema")
	}
	if got.Summary != "describe commands" {
		t.Errorf("Summary = %q", got.Summary)
	}
	if len(got.Flags) != 1 || got.Flags[0].Name != "command" {
		t.Errorf("Flags = %+v, want one command flag", got.Flags)
	}
	if !equalStringsA(got.ErrorCodes, []string{"usage_error"}) {
		t.Errorf("ErrorCodes = %v, want [usage_error]", got.ErrorCodes)
	}
	if !equalIntsA(got.ExitCodes, []int{0, 2}) {
		t.Errorf("ExitCodes = %v, want [0 2]", got.ExitCodes)
	}
	if got.Aliases == nil {
		t.Error("Aliases is nil, want empty non-nil slice")
	}
	if got.RowSchema == nil {
		t.Error("RowSchema is nil, want empty non-nil slice")
	}

	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(m["aliases"]) != "[]" {
		t.Errorf("aliases = %s, want []", m["aliases"])
	}
	if string(m["row_schema"]) != "[]" {
		t.Errorf("row_schema = %s, want []", m["row_schema"])
	}
}

// TestMetaSchema_NilFlagsAndCodes ensures a meta command with no flags and no
// error codes (e.g. version) normalizes those slices to non-nil too.
func TestMetaSchema_NilFlagsAndCodes(t *testing.T) {
	got := MetaSchema("version", "print the build version", nil, nil, []int{0})
	if got.Flags == nil {
		t.Error("Flags is nil, want empty non-nil slice")
	}
	if got.ErrorCodes == nil {
		t.Error("ErrorCodes is nil, want empty non-nil slice")
	}
	if !equalIntsA(got.ExitCodes, []int{0}) {
		t.Errorf("ExitCodes = %v, want [0]", got.ExitCodes)
	}
}

func equalStringsA(a, b []string) bool {
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

func equalIntsA(a, b []int) bool {
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

// TestList_MergesAndSorts checks the command list combines the passed analysis
// descriptors with extra (meta) summaries and orders every entry by name.
func TestList_MergesAndSorts(t *testing.T) {
	analyses := []Descriptor{
		{Name: "revisions", Summary: "r", ExitCodes: []int{0, 2, 3, 1}},
		{Name: "authors", Aliases: []string{"a"}, Summary: "au", ExitCodes: []int{0, 2, 3, 1}},
	}
	extra := []CommandSummary{
		{Name: "schema", Summary: "sc", ExitCodes: []int{0, 2}},
	}

	list := List(analyses, extra)
	if list.SchemaVersion != output.SchemaVersion || !list.OK {
		t.Errorf("list envelope = %+v, want ok/version set", list)
	}

	names := make([]string, len(list.Commands))
	for i, c := range list.Commands {
		names[i] = c.Name
	}
	want := []string{"authors", "revisions", "schema"}
	if len(names) != len(want) {
		t.Fatalf("commands = %v, want %v", names, want)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("commands[%d] = %q, want %q", i, names[i], n)
		}
	}
	if list.Commands[0].Aliases == nil {
		t.Error("authors aliases nil, want normalized slice")
	}
}
