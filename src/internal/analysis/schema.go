package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/output"
)

// CommandSchema is the full, self-describing contract for one command: its
// identity, the flags and columns it exposes, and the error and exit codes it
// may produce. It is built purely from a Descriptor (see Schema) so it can never
// drift from the command's actual behaviour. This is what lets an agent learn a
// command entirely at runtime (cli-design.md §8).
type CommandSchema struct {
	SchemaVersion int      `json:"schema_version"`
	OK            bool     `json:"ok"`
	Command       string   `json:"command"`
	Summary       string   `json:"summary"`
	Aliases       []string `json:"aliases"`
	Flags         []Flag   `json:"flags"`
	RowSchema     []Column `json:"row_schema"`
	ErrorCodes    []string `json:"error_codes"`
	ExitCodes     []int    `json:"exit_codes"`
}

// CommandSummary is one entry in the command list: the minimal description an
// agent needs to discover a command and know how it can exit. Meta commands
// (schema, print-log-command, version) appear here alongside the analyses.
type CommandSummary struct {
	Name      string   `json:"name"`
	Aliases   []string `json:"aliases"`
	Summary   string   `json:"summary"`
	ExitCodes []int    `json:"exit_codes"`
}

// CommandList is the `schema` (no --command) envelope: every command the binary
// exposes, ordered by name.
type CommandList struct {
	SchemaVersion int              `json:"schema_version"`
	OK            bool             `json:"ok"`
	Commands      []CommandSummary `json:"commands"`
}

// Schema builds the full schema object for descriptor d. Slice fields are
// normalised to non-nil so they marshal as [] rather than null and an agent can
// iterate them unconditionally.
func Schema(d Descriptor) CommandSchema {
	return CommandSchema{
		SchemaVersion: output.SchemaVersion,
		OK:            true,
		Command:       d.Name,
		Summary:       d.Summary,
		Aliases:       nonNilStrings(d.Aliases),
		Flags:         nonNilFlags(d.Flags),
		RowSchema:     nonNilColumns(d.RowSchema),
		ErrorCodes:    nonNilStrings(d.ErrorCodes),
		ExitCodes:     nonNilInts(d.ExitCodes),
	}
}

// MetaSchema builds a CommandSchema for a non-analysis command (schema,
// version, print-log-command). Such commands have no Descriptor: they take no
// log input and emit no rows, so their aliases and row schema are always empty.
// The explicit parts are carried through and every slice is normalised to
// non-nil so the schema marshals as [] rather than null, matching Schema(d).
// Analyses use Schema(d) instead.
func MetaSchema(command, summary string, flags []Flag, errorCodes []string, exitCodes []int) CommandSchema {
	return CommandSchema{
		SchemaVersion: output.SchemaVersion,
		OK:            true,
		Command:       command,
		Summary:       summary,
		Aliases:       []string{},
		Flags:         nonNilFlags(flags),
		RowSchema:     []Column{},
		ErrorCodes:    nonNilStrings(errorCodes),
		ExitCodes:     nonNilInts(exitCodes),
	}
}

// List builds the command-list envelope from the given analysis descriptors and
// any extra (meta) command summaries, ordering every entry by name. The
// descriptors are passed in rather than read from the registry so callers can
// list a controlled set (and so the builder stays trivially testable).
func List(analyses []Descriptor, extra []CommandSummary) CommandList {
	commands := make([]CommandSummary, 0, len(analyses)+len(extra))
	for _, d := range analyses {
		commands = append(commands, summaryOf(d))
	}
	for _, s := range extra {
		s.Aliases = nonNilStrings(s.Aliases)
		s.ExitCodes = nonNilInts(s.ExitCodes)
		commands = append(commands, s)
	}
	sort.Slice(commands, func(i, j int) bool { return commands[i].Name < commands[j].Name })

	return CommandList{
		SchemaVersion: output.SchemaVersion,
		OK:            true,
		Commands:      commands,
	}
}

// summaryOf projects a descriptor to its command-list entry.
func summaryOf(d Descriptor) CommandSummary {
	return CommandSummary{
		Name:      d.Name,
		Aliases:   nonNilStrings(d.Aliases),
		Summary:   d.Summary,
		ExitCodes: nonNilInts(d.ExitCodes),
	}
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilInts(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}

func nonNilFlags(s []Flag) []Flag {
	if s == nil {
		return []Flag{}
	}
	return s
}

func nonNilColumns(s []Column) []Column {
	if s == nil {
		return []Column{}
	}
	return s
}
