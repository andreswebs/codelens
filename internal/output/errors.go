package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/andreswebs/codelens/internal/terr"
)

// exitInternal is the non-success process exit code resolved here for an error
// carrying no code of its own, from the exit-code taxonomy (ADR 0002): 70
// EX_SOFTWARE. Usage (64), data (65), and I/O (74) exits are carried by coded
// errors themselves; usage errors are classified into coded errors upstream, in
// internal/command, before they reach this resolver.
const exitInternal = 70

// InternalErrorCode is the code stamped on the uncoded internal-fault fallback
// (exit 70). It is not backed by a terr sentinel: it is the code of last resort
// for an error that carries none, so it lives here beside the resolver that
// applies it rather than in the sentinel registry.
const InternalErrorCode = "internal_error"

// NonSentinelCodes is the closed set of error codes codelens can emit that are
// not backed by a terr sentinel, and so cannot appear in terr.All(). Since the
// usage classes became registered sentinels (in internal/command), the only
// remaining member is the internal-fault fallback stamped here for an uncoded
// error. It is the allowlist a schema conformance test checks declared codes
// against once the sentinel registry has been exhausted.
var NonSentinelCodes = []string{
	InternalErrorCode,
}

type errorEnvelope struct {
	SchemaVersion int          `json:"schema_version"`
	OK            bool         `json:"ok"`
	Error         *errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
	Details any    `json:"details,omitempty"`
}

// EmitError writes err to w as the JSON error envelope, always. Code, hint, and
// details come from the error's coded and detailed interfaces when present,
// falling back to an internal-error code otherwise. Usage errors are already
// coded by the time they reach here: internal/command classifies uncoded
// framework parse errors into coded usage errors before emitting.
//
// The write is best-effort: w is the diagnostic sink (stderr) and a failure to
// write there is unrecoverable, so the write error is intentionally discarded.
func EmitError(w io.Writer, err error) {
	_, _ = io.WriteString(w, render(detailFor(err)))
}

// render marshals a resolved error detail into the JSON error envelope line. A
// marshal failure (not expected for these types) falls back to a minimal text
// line so an error is never swallowed silently.
func render(d *errorDetail) string {
	env := errorEnvelope{SchemaVersion: SchemaVersion, OK: false, Error: d}
	b, err := json.Marshal(env)
	if err != nil {
		return fmt.Sprintf("error: %s\n", d.Message)
	}
	return string(b) + "\n"
}

// detailFor derives the rendered error detail from err, preferring a coded
// error's own code/hint/details and falling back to the internal-error code.
func detailFor(err error) *errorDetail {
	d := &errorDetail{Message: err.Error()}

	var coded terr.Coded
	if errors.As(err, &coded) {
		d.Code = coded.Code()
		d.Hint = coded.Hint()
		var detailed terr.Detailed
		if errors.As(err, &detailed) {
			d.Details = detailed.ErrorDetails()
		}
		return d
	}

	d.Code = InternalErrorCode
	return d
}

// ExitCodeFor resolves the process exit code for err: 0 for nil, a coded
// error's own exit code, and 70 (internal fault) for an uncoded error. Usage
// errors are coded upstream (internal/command), so they arrive as coded errors
// and resolve to their own exit code here.
func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var coded terr.Coded
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	return exitInternal
}
