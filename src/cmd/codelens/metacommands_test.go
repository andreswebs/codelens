package main

import (
	"testing"
)

// TestSchema_Command_PrintLogCommand asserts the print-log-command helper is now
// introspectable: `schema --command print-log-command` returns a valid schema
// surfacing its --after flag and its usage-error/exit-code contract, rather than
// a usage error.
func TestSchema_Command_PrintLogCommand(t *testing.T) {
	got := schemaOf(t, "print-log-command")

	if got.Command != "print-log-command" {
		t.Errorf("Command = %q, want %q", got.Command, "print-log-command")
	}
	if !hasFlag(got, "after") {
		t.Errorf("flags = %+v, want to include after", got.Flags)
	}
	if len(got.RowSchema) != 0 {
		t.Errorf("row_schema = %+v, want none for a meta command", got.RowSchema)
	}
	if !equalStrings(got.ErrorCodes, []string{"usage_error"}) {
		t.Errorf("error_codes = %v, want [usage_error]", got.ErrorCodes)
	}
	if !equalInts(got.ExitCodes, []int{0, 2}) {
		t.Errorf("exit_codes = %v, want [0 2]", got.ExitCodes)
	}
}

// TestSchema_Command_Schema asserts the schema command describes itself,
// surfacing its --command flag and codes.
func TestSchema_Command_Schema(t *testing.T) {
	got := schemaOf(t, "schema")

	if got.Command != "schema" {
		t.Errorf("Command = %q, want %q", got.Command, "schema")
	}
	if !hasFlag(got, "command") {
		t.Errorf("flags = %+v, want to include command", got.Flags)
	}
	if !equalStrings(got.ErrorCodes, []string{"usage_error"}) {
		t.Errorf("error_codes = %v, want [usage_error]", got.ErrorCodes)
	}
	if !equalInts(got.ExitCodes, []int{0, 2}) {
		t.Errorf("exit_codes = %v, want [0 2]", got.ExitCodes)
	}
}

// TestSchema_Command_Version asserts the version command is introspectable with
// no flags, no row schema, and only the success exit code.
func TestSchema_Command_Version(t *testing.T) {
	got := schemaOf(t, "version")

	if got.Command != "version" {
		t.Errorf("Command = %q, want %q", got.Command, "version")
	}
	if len(got.Flags) != 0 {
		t.Errorf("flags = %+v, want none", got.Flags)
	}
	if len(got.RowSchema) != 0 {
		t.Errorf("row_schema = %+v, want none", got.RowSchema)
	}
	if !equalInts(got.ExitCodes, []int{0}) {
		t.Errorf("exit_codes = %v, want [0]", got.ExitCodes)
	}
}

// TestMetaCommands_SchemaFlagsMatchWiredFlags guards the single-source
// invariant: the flags a meta command declares (and thus wires into its
// cli.Command via toCLIFlag) are exactly the flags its schema surfaces.
func TestMetaCommands_SchemaFlagsMatchWiredFlags(t *testing.T) {
	for _, m := range metaCommands() {
		t.Run(m.Name, func(t *testing.T) {
			got := schemaOf(t, m.Name)

			want := make(map[string]bool, len(m.Flags))
			for _, f := range m.Flags {
				want[f.Name] = false
			}
			for _, f := range got.Flags {
				if _, ok := want[f.Name]; !ok {
					t.Errorf("schema flag %q not declared in metaCommand", f.Name)
					continue
				}
				want[f.Name] = true
			}
			for name, seen := range want {
				if !seen {
					t.Errorf("declared flag %q missing from schema", name)
				}
			}

			cmd := m.command()
			if cmd.Name != m.Name {
				t.Errorf("command().Name = %q, want %q", cmd.Name, m.Name)
			}
			if len(cmd.Flags) != len(m.Flags) {
				t.Errorf("wired %d cli flags, want %d", len(cmd.Flags), len(m.Flags))
			}
		})
	}
}

func hasFlag(s schemaCmd, name string) bool {
	for _, f := range s.Flags {
		if f.Name == name {
			return true
		}
	}
	return false
}
