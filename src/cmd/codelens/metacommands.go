package main

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis"
	"github.com/urfave/cli/v3"
)

// metaCommand is the single source for a non-analysis command's identity: it
// declares the command's name, summary, flags, and error/exit codes once, then
// projects to the cli wiring, the schema command list, and the full schema. Meta
// commands are not analyses (no log input, no rows), so they are a distinct type
// rather than an analysis.Descriptor, whose Run(mods, opts) and RowSchema do not
// fit them.
type metaCommand struct {
	Name       string
	Summary    string
	Flags      []analysis.Flag // declared in the analysis.Flag shape; reused by toCLIFlag
	ErrorCodes []string
	ExitCodes  []int
	Action     cli.ActionFunc
}

// metaCommands is the single table describing every non-analysis command. Its
// entries drive the cli wiring, the schema command list, and the per-command
// schema, so a meta command's identity is declared in exactly one place. The
// action bodies live next to their helpers in schema.go and
// printlogcommand.go.
func metaCommands() []metaCommand {
	return []metaCommand{
		{
			Name:    "print-log-command",
			Summary: printLogCommandUsage,
			Flags: []analysis.Flag{
				{Name: "after", Type: "string", Desc: "limit history to commits after `DATE` (YYYY-MM-DD)"},
				{Name: "all", Type: "bool", Default: false, Desc: "include all refs (default: current branch only)"},
			},
			ErrorCodes: []string{"usage_error"},
			ExitCodes:  []int{0, 2},
			Action:     printLogCommandAction,
		},
		{
			Name:    "schema",
			Summary: schemaUsage,
			Flags: []analysis.Flag{
				{Name: "command", Type: "string", Desc: "describe a single `CMD` in full (flags, row schema, codes)"},
			},
			ErrorCodes: []string{"usage_error"},
			ExitCodes:  []int{0, 2},
			Action:     schemaAction,
		},
	}
}

// command projects the meta command to its cli.Command, reusing toCLIFlag so a
// flag's Usage text comes from its Desc exactly as it does for analyses.
func (m metaCommand) command() *cli.Command {
	flags := make([]cli.Flag, 0, len(m.Flags))
	for _, f := range m.Flags {
		flags = append(flags, toCLIFlag(f))
	}
	return &cli.Command{
		Name:         m.Name,
		Usage:        m.Summary,
		OnUsageError: onUsageError,
		Flags:        flags,
		Action:       m.Action,
	}
}

// summary projects the meta command to its entry in the schema command list.
func (m metaCommand) summary() analysis.CommandSummary {
	return analysis.CommandSummary{
		Name:      m.Name,
		Summary:   m.Summary,
		ExitCodes: m.ExitCodes,
	}
}

// schema projects the meta command to its full, self-describing schema, served
// by `schema --command <meta>`.
func (m metaCommand) schema() analysis.CommandSchema {
	return analysis.MetaSchema(m.Name, m.Summary, m.Flags, m.ErrorCodes, m.ExitCodes)
}

// metaCLICommands maps the meta table to the cli.Commands the root wires in.
func metaCLICommands() []*cli.Command {
	metas := metaCommands()
	cmds := make([]*cli.Command, 0, len(metas))
	for _, m := range metas {
		cmds = append(cmds, m.command())
	}
	return cmds
}

// metaCommandSummaries maps the meta table to the command-list summaries the
// schema command merges with the analysis descriptors.
func metaCommandSummaries() []analysis.CommandSummary {
	metas := metaCommands()
	summaries := make([]analysis.CommandSummary, 0, len(metas))
	for _, m := range metas {
		summaries = append(summaries, m.summary())
	}
	return summaries
}

// lookupMeta resolves a name to its meta command via a linear scan over the
// table. Meta commands are few and have no aliases, so a scan is clearer than an
// index.
func lookupMeta(name string) (metaCommand, bool) {
	for _, m := range metaCommands() {
		if m.Name == name {
			return m, true
		}
	}
	return metaCommand{}, false
}

// allCommandNames returns every command name the binary exposes (analyses plus
// meta commands), sorted for a stable unknown-command recovery hint.
func allCommandNames() []string {
	names := analysisNames()
	for _, m := range metaCommands() {
		names = append(names, m.Name)
	}
	sort.Strings(names)
	return names
}
