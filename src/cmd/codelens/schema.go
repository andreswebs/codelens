package main

import (
	"context"

	"github.com/andreswebs/codelens/internal/analysis"
	"github.com/andreswebs/codelens/internal/output"
	"github.com/andreswebs/codelens/internal/terr"
	"github.com/urfave/cli/v3"
)

// schemaUsage is the one-line summary of the schema command, reused for its own
// entry in the command list so the list stays complete and self-describing.
const schemaUsage = "describe commands and their I/O contract (machine-readable)"

// versionUsage is the one-line summary of the version command, reused for its
// entry in the schema command list (via the meta table) so the list and the
// wired subcommand cannot drift.
const versionUsage = "print the codelens build version"

// errUnknownSchemaCommand marks a --command value that names no known analysis.
// It is a usage error (exit 2) and carries the resolvable command names in its
// details so a caller can recover without a second round trip.
var errUnknownSchemaCommand = terr.New(
	"usage_error", 2,
	"run `codelens schema` to list all commands",
	"unknown command",
)

// schemaAction runs the schema introspection command. With no --command it
// emits the full command list (analyses plus meta commands); with --command CMD
// it emits CMD's complete schema, resolving analyses (canonical names and
// aliases) through the registry and meta commands through their table. Analysis
// schemas are derived from the registered descriptors and meta schemas from the
// meta table, so the schema cannot drift from the commands' actual flags and
// output.
func schemaAction(_ context.Context, cmd *cli.Command) error {
	w := cmd.Root().Writer
	name := cmd.String("command")
	if name == "" {
		return output.EmitJSON(w, analysis.List(analysis.All(), metaCommandSummaries()))
	}

	if d, ok := analysis.Lookup(name); ok {
		return output.EmitJSON(w, analysis.Schema(d))
	}
	if m, ok := lookupMeta(name); ok {
		return output.EmitJSON(w, m.schema())
	}
	return errUnknownSchemaCommand.WithDetails(map[string]any{
		"command":        name,
		"known_commands": allCommandNames(),
	})
}

// analysisNames returns the canonical names of every registered analysis, in
// registry order, as the analysis half of the unknown-command recovery hint
// (allCommandNames adds the meta names and sorts).
func analysisNames() []string {
	all := analysis.All()
	names := make([]string, len(all))
	for i, d := range all {
		names[i] = d.Name
	}
	return names
}
