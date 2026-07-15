// Package output builds and emits codelens's result and error envelopes. Every
// command renders through it: successful runs marshal a Result, failures render
// an error envelope and resolve a process exit code from the error's kind.
package output

// SchemaVersion is the version of the output envelope contract. It is bumped
// only on a breaking change to the envelope shape.
const SchemaVersion = 1

// Result is the success envelope wrapping one analysis's rows. Params echoes
// the effective tuning options so a result is self-documenting; TotalCount and
// Truncated are populated only when --rows caps the output, and are omitted
// otherwise so an uncapped result is unambiguous.
type Result struct {
	SchemaVersion int            `json:"schema_version"`
	OK            bool           `json:"ok"`
	Analysis      string         `json:"analysis"`
	Params        map[string]any `json:"params,omitempty"`
	RowCount      int            `json:"row_count"`
	TotalCount    int            `json:"total_count,omitempty"`
	Truncated     bool           `json:"truncated,omitempty"`
	Rows          any            `json:"rows"`
}
