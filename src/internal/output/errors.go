package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/andreswebs/codelens/internal/terr"
)

// exitUsage, exitInput, and exitInternal are the non-success process exit codes
// from the taxonomy (cli-design.md §7.2). exitInput is carried by coded errors
// themselves; the other two are resolved here.
const (
	exitUsage    = 2
	exitInternal = 1
)

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

// EmitError writes a structured description of err to w. With format "text" it
// renders "✗ <message>" and an optional "  hint:" line; otherwise it emits the
// JSON error envelope. Code, hint, and details come from the error's coded and
// detailed interfaces when present, falling back to usage classification for
// uncoded CLI-framework errors and to an internal-error code otherwise.
//
// The write is best-effort: w is the diagnostic sink (stderr) and a failure to
// write there is unrecoverable, so the write error is intentionally discarded.
func EmitError(w io.Writer, format string, err error) {
	_, _ = io.WriteString(w, render(format, detailFor(err)))
}

// render turns a resolved error detail into the bytes to write, in either the
// text or JSON form. A JSON marshal failure (not expected for these types)
// falls back to the text rendering so an error is never swallowed silently.
func render(format string, d *errorDetail) string {
	if format == "text" {
		s := fmt.Sprintf("✗ %s\n", d.Message)
		if d.Hint != "" {
			s += fmt.Sprintf("  hint: %s\n", d.Hint)
		}
		return s
	}
	env := errorEnvelope{SchemaVersion: SchemaVersion, OK: false, Error: d}
	b, err := json.Marshal(env)
	if err != nil {
		return render("text", d)
	}
	return string(b) + "\n"
}

// detailFor derives the rendered error detail from err, preferring a coded
// error's own code/hint/details and falling back to usage or internal codes.
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

	if code, hint := classifyUsageError(err); code != "" {
		d.Code = code
		d.Hint = hint
		return d
	}

	d.Code = "internal_error"
	return d
}

// ExitCodeFor resolves the process exit code for err: 0 for nil, a coded
// error's own exit code, 2 for a classified usage error, and 1 otherwise.
func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var coded terr.Coded
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	if code, _ := classifyUsageError(err); code != "" {
		return exitUsage
	}
	return exitInternal
}

// usageClasses maps substrings of the urfave/cli v3 parsing messages to the
// coded error each represents. Every entry is an exit-2 usage error; the code
// distinguishes the class (an unknown/undefined flag, a bad flag value, or a
// missing required flag) so an agent can react to the specific failure rather
// than a single opaque "usage_error". Order is significant: the first matching
// marker wins, so more specific markers precede more general ones.
var usageClasses = []struct{ marker, code, hint string }{
	{"flag provided but not defined", "unknown_flag", "run `codelens <command> --help` to list valid flags"},
	{"no such flag", "unknown_flag", "run `codelens <command> --help` to list valid flags"},
	{"invalid value", "invalid_value", "run `codelens <command> --help` for accepted flag values"},
	{"Required flag", "missing_required_flag", "run `codelens <command> --help` to see required flags"},
	{"not set", "missing_required_flag", "run `codelens <command> --help` to see required flags"},
}

// classifyUsageError reports the usage code and hint when err's message matches
// a known CLI-framework parsing error, or ("", "") when it does not. Unknown
// commands are classified upstream (they never reach the framework's flag
// parser) and so are not covered here.
func classifyUsageError(err error) (code, hint string) {
	msg := err.Error()
	for _, c := range usageClasses {
		if strings.Contains(msg, c.marker) {
			return c.code, c.hint
		}
	}
	return "", ""
}
