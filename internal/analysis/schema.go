package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/output"
	"github.com/andreswebs/codelens/internal/terr"
)

// commonErrorCodes is the tool-level error baseline: the codes any invocation of
// codelens can produce regardless of which command runs, drawn from the input
// parser, the global option flags, and the output layer. It is declared once
// here and copied into every CommandSchema (analyses and meta commands alike),
// so a command's ErrorCodes carries only its DISTINCTIVE codes and an agent
// enumerates the full reachable surface as CommonErrorCodes + ErrorCodes.
//
// Membership is derived from the command tree, not copied from prose:
//   - the log-parse and log-content codes (parse_error, invalid_control_char)
//     and every global-flag/output code (unknown_format, invalid_field,
//     invalid_glob, invalid_group, invalid_team_map, invalid_temporal_period,
//     invalid_temporal_date, log_open_failed, input_file_open_failed) are
//     sentinels reachable on any run;
//   - the usage-class codes (unknown_flag, invalid_value, missing_required_flag)
//     are sentinels registered in internal/command, and internal_error is the
//     one non-sentinel fallback any run can also surface (output.NonSentinelCodes).
//
// empty_log is deliberately EXCLUDED: it stays declared per analysis (every
// descriptor lists it) so no descriptor edits are needed, and duplicating it
// here would double-report it. The command-specific meta codes
// (unknown_schema_command, invalid_after_date) and the pre-dispatch
// unknown_command are likewise excluded: they are not reachable from an
// arbitrary command invocation. A conformance test pins every entry here to a
// terr sentinel or output.NonSentinelCodes, so this list cannot drift.
var commonErrorCodes = []string{
	"input_file_open_failed",
	"internal_error",
	"invalid_control_char",
	"invalid_field",
	"invalid_glob",
	"invalid_group",
	"invalid_team_map",
	"invalid_temporal_date",
	"invalid_temporal_period",
	"invalid_value",
	"log_open_failed",
	"missing_required_flag",
	"parse_error",
	"unknown_flag",
	"unknown_format",
}

// CommonErrorCodes returns a copy of the tool-level error baseline (see
// commonErrorCodes). It lets a conformance test enumerate the baseline without
// exporting the backing slice.
func CommonErrorCodes() []string {
	out := make([]string, len(commonErrorCodes))
	copy(out, commonErrorCodes)
	return out
}

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
	// CommonErrorCodes lists the codes any invocation of this tool can produce
	// (input, option, and output-layer failures), declared once at tool level.
	// ErrorCodes carries only the codes distinctive to this command, so an agent
	// enumerates the full reachable surface as CommonErrorCodes + ErrorCodes.
	CommonErrorCodes []string `json:"common_error_codes"`
	ExitCodes        []int    `json:"exit_codes"`
}

// SchemaError is one entry in the tool's error inventory: a stable code, the
// process exit code it resolves to, and its user-facing hint (omitted when
// empty). It mirrors a terr sentinel projected for the schema.
type SchemaError struct {
	Code     string `json:"code"`
	ExitCode int    `json:"exit_code"`
	Hint     string `json:"hint,omitempty"`
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
	// Errors is the tool's full error inventory, derived from the terr registry
	// so it cannot drift from the sentinels that actually exist. It is
	// sentinel-only: the non-sentinel codes (output.NonSentinelCodes) are
	// reported per command via CommonErrorCodes, not here. Sorted by code for a
	// stable envelope, since registration order is not a contract.
	Errors []SchemaError `json:"errors"`
}

// Schema builds the full schema object for descriptor d. Slice fields are
// normalised to non-nil so they marshal as [] rather than null and an agent can
// iterate them unconditionally.
func Schema(d Descriptor) CommandSchema {
	return CommandSchema{
		SchemaVersion:    output.SchemaVersion,
		OK:               true,
		Command:          d.Name,
		Summary:          d.Summary,
		Aliases:          nonNilStrings(d.Aliases),
		Flags:            nonNilFlags(d.Flags),
		RowSchema:        nonNilColumns(d.RowSchema),
		ErrorCodes:       nonNilStrings(d.ErrorCodes),
		CommonErrorCodes: CommonErrorCodes(),
		ExitCodes:        nonNilInts(d.ExitCodes),
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
		SchemaVersion:    output.SchemaVersion,
		OK:               true,
		Command:          command,
		Summary:          summary,
		Aliases:          []string{},
		Flags:            nonNilFlags(flags),
		RowSchema:        []Column{},
		ErrorCodes:       nonNilStrings(errorCodes),
		CommonErrorCodes: CommonErrorCodes(),
		ExitCodes:        nonNilInts(exitCodes),
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
		Errors:        errorInventory(),
	}
}

// errorInventory projects every registered terr sentinel to a SchemaError,
// sorted by code so the envelope is stable regardless of package init order.
// Building it from terr.All() is what keeps the documented error surface from
// drifting from the sentinels that actually exist (ADR 0003).
func errorInventory() []SchemaError {
	all := terr.All()
	errs := make([]SchemaError, 0, len(all))
	for _, e := range all {
		errs = append(errs, SchemaError{Code: e.Code(), ExitCode: e.ExitCode(), Hint: e.Hint()})
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].Code < errs[j].Code })
	return errs
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
