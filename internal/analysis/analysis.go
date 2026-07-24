// Package analysis defines the extensibility spine of codelens: an analysis
// Descriptor and the in-process registry from which the command tree, help
// text, and schema introspection are all generated. Adding a new analysis is a
// single Register call at init time; no other wiring is required.
//
// A Descriptor is pure metadata plus a Run function. Run receives the parsed
// modification records and the effective Opts and returns the analysis's rows;
// the output layer wraps them in the result envelope. Cross-cutting concerns
// handled elsewhere are deliberately kept out of Run: group/temporal/team-map
// transforms are applied by the pipeline before Run, and row truncation and
// field projection are applied by the output layer after Run.
package analysis

import (
	"github.com/andreswebs/codelens/internal/model"
)

// Column describes one output row field for schema introspection: its
// snake_case name, its JSON type, and a one-line human description. The
// RowSchema of a Descriptor is the machine-readable answer to "what columns
// does this analysis emit?".
type Column struct {
	// Name is the snake_case JSON key of the field.
	Name string `json:"name"`
	// Type is the JSON type of the field ("string", "int", ...).
	Type string `json:"type"`
	// Desc is a one-line description of the field's meaning.
	Desc string `json:"desc"`
}

// Flag describes one command flag for schema introspection and command-tree
// generation. Default is the flag's zero-configuration value and is echoed in a
// result's params so a run is self-documenting.
type Flag struct {
	// Name is the long flag name without the leading dashes.
	Name string `json:"name"`
	// Type is the flag's value type ("int", "string", "bool", ...).
	Type string `json:"type"`
	// Default is the value used when the flag is not supplied.
	Default any `json:"default"`
	// Required reports whether the flag must be supplied by the caller.
	Required bool `json:"required"`
	// Desc is a one-line description of the flag's effect.
	Desc string `json:"desc"`
}

// Opts carries the effective, parsed run options for an analysis. It is a
// superset: each analysis reads only the fields relevant to it, and the schema
// declares which flags actually apply per command. Group, temporal, and
// team-map options are absent here because those transforms are applied to the
// modification set by the pipeline before Run is called.
type Opts struct {
	// MinRevs is the minimum revisions for an entity to be included, for the
	// analyses that filter by revision count.
	MinRevs int
	// MinSharedRevs is the minimum shared revisions for a coupled pair.
	MinSharedRevs int
	// MinCoupling is the minimum coupling degree, in percent, to report a pair.
	MinCoupling int
	// MaxCoupling is the maximum coupling degree, in percent, to report a pair.
	MaxCoupling int
	// MaxChangesetSize skips change sets larger than this size in coupling.
	MaxChangesetSize int
	// Verbose adds per-pair revision detail columns to the coupling analysis.
	Verbose bool
	// TimeNow is the "time zero" date (YYYY-MM-dd) for code-age; the empty
	// value means the current UTC date.
	TimeNow string
	// Expression is the regular expression the messages analysis matches
	// against commit subjects.
	Expression string
	// Warn is an optional sink for non-fatal advisories. An analysis calls it
	// (via the nil-safe warn helper) to raise a machine-readable warning without
	// depending on the output layer or knowing about stderr; the action layer
	// wires it to output.EmitWarning. A nil Warn discards the advisory, so the
	// zero-value Opts is usable without a guard at each call site.
	Warn WarnFunc
}

// WarnFunc reports one non-fatal advisory. Its signature mirrors
// output.EmitWarning's payload (code, message, hint, details) so the action
// layer can adapt it to that emitter without analysis importing output. A
// warning never alters the exit code.
type WarnFunc func(code, message, hint string, details any)

// warn forwards a non-fatal advisory to o.Warn when a sink is set, and is a
// no-op otherwise, so analyses can raise warnings unconditionally.
func (o Opts) warn(code, message, hint string, details any) {
	if o.Warn != nil {
		o.Warn(code, message, hint, details)
	}
}

// Descriptor is the registered contract for one analysis: its identity, the
// flags and columns it exposes for introspection, the error and exit codes it
// may produce, and the Run function that executes it.
type Descriptor struct {
	// Name is the descriptive canonical command name (e.g. "sum-of-coupling").
	Name string
	// Aliases are the terse code-maat originals accepted for parity (e.g.
	// "soc"); they resolve to the same descriptor as Name.
	Aliases []string
	// Summary is a one-line description of the analysis.
	Summary string
	// Flags declares the per-command flags the analysis honours.
	Flags []Flag
	// RowSchema declares the columns each output row carries.
	RowSchema []Column
	// ErrorCodes lists the terr codes the analysis may return.
	ErrorCodes []string
	// ExitCodes lists the process exit codes the command may produce.
	ExitCodes []int
	// Run executes the analysis over the parsed modifications and effective
	// options, returning the analysis's rows (a slice; the output layer wraps
	// them in the result envelope).
	Run func(mods []model.Modification, opts Opts) (any, error)
}
