// Package terr defines codelens's coded-error model: errors that carry a
// stable string code, a process exit code, and a user-facing hint. Commands
// wrap these with context and the top level recovers them with errors.As to
// render a structured error envelope and choose an exit code.
package terr

import "fmt"

// Coded is an error that also reports a stable code, an exit code, and a hint.
type Coded interface {
	error
	Code() string
	ExitCode() int
	Hint() string
}

// Detailed is an optional interface for errors that carry structured details,
// surfaced in the error envelope's "details" field.
type Detailed interface {
	ErrorDetails() any
}

// E is the concrete coded error. Declare sentinels with New, attach a cause
// with Wrap, and attach structured payloads with WithDetails.
type E struct {
	code    string
	exit    int
	hint    string
	msg     string
	cause   error
	details any
}

var (
	_ Coded    = (*E)(nil)
	_ Detailed = (*E)(nil)
)

// registry holds every sentinel created via New, in registration order. It
// backs All, which the schema command and the exit-code conformance test read.
var registry []*E

// New creates a sentinel E and registers it for enumeration via All. It panics
// when code is already registered: duplicate registration is an init-time
// programmer error, and crashing at startup is the correct outcome.
func New(code string, exit int, hint, msg string) *E {
	for _, r := range registry {
		if r.code == code {
			panic(fmt.Sprintf("terr: duplicate error code %q", code))
		}
	}
	e := &E{code: code, exit: exit, hint: hint, msg: msg}
	registry = append(registry, e)
	return e
}

// Newf creates an E without registering it, formatting the message from format
// and args. Use it for one-off per-invocation errors whose class does not
// belong in the enumerable inventory.
func Newf(code string, exit int, hint, format string, args ...any) *E {
	return &E{code: code, exit: exit, hint: hint, msg: fmt.Sprintf(format, args...)}
}

// All returns a copy of every error registered via New, in registration order.
// It backs the schema command's error inventory and the exit-code conformance
// test.
func All() []*E {
	out := make([]*E, len(registry))
	copy(out, registry)
	return out
}

// Error returns the message; when a cause is present it is appended as
// "message: cause".
func (e *E) Error() string {
	if e.cause != nil {
		return e.msg + ": " + e.cause.Error()
	}
	return e.msg
}

// Wrap returns a copy of the error with err set as its cause. The receiver is
// left unchanged so package-level sentinels stay reusable.
func (e *E) Wrap(err error) *E {
	c := *e
	c.cause = err
	return &c
}

// Unwrap returns the cause, or nil, so errors.Is/As traverse the chain.
func (e *E) Unwrap() error { return e.cause }

// Is reports whether target is an E with the same code, so copies produced by
// Wrap and WithDetails still match their sentinel under errors.Is.
func (e *E) Is(target error) bool {
	t, ok := target.(*E)
	return ok && t.code == e.code
}

// Code returns the stable error code.
func (e *E) Code() string { return e.code }

// ExitCode returns the process exit code associated with the error.
func (e *E) ExitCode() int { return e.exit }

// Hint returns a user-facing hint for resolving the error, or "" if none.
func (e *E) Hint() string { return e.hint }

// ErrorDetails returns the structured details attached with WithDetails, or nil.
func (e *E) ErrorDetails() any { return e.details }

// WithDetails returns a copy of the error carrying the given structured
// details. The receiver is left unchanged so package-level sentinels stay
// reusable.
func (e *E) WithDetails(details any) *E {
	c := *e
	c.details = details
	return &c
}
