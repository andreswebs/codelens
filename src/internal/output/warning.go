package output

import (
	"encoding/json"
	"io"
)

// diagnosticEnvelope is a non-fatal, machine-readable advisory written to
// stderr. It shares the envelope shape of an error but carries level:"warning"
// instead of an ok field, so a consumer can tell the two apart unambiguously,
// and it never changes the exit code. One diagnostic is emitted per line.
type diagnosticEnvelope struct {
	SchemaVersion int    `json:"schema_version"`
	Level         string `json:"level"`
	Code          string `json:"code"`
	Message       string `json:"message"`
	Hint          string `json:"hint,omitempty"`
	Details       any    `json:"details,omitempty"`
}

// EmitWarning writes one JSON diagnostic line (level "warning") to w, which is
// the diagnostic sink (stderr). Empty hint and nil details are omitted. Each
// call appends exactly one newline-terminated line, so successive warnings form
// a valid NDJSON stream.
//
// The write is best-effort, like EmitError: w is stderr and a failure to write
// a diagnostic there is unrecoverable, so both the marshal and write errors are
// intentionally discarded rather than escalated over the advisory they describe.
func EmitWarning(w io.Writer, code, message, hint string, details any) {
	env := diagnosticEnvelope{
		SchemaVersion: SchemaVersion,
		Level:         "warning",
		Code:          code,
		Message:       message,
		Hint:          hint,
		Details:       details,
	}
	b, err := json.Marshal(env)
	if err != nil {
		return
	}
	_, _ = w.Write(append(b, '\n'))
}
