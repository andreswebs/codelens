// Package terr defines codelens's coded-error model: errors that carry a
// stable string code, a process exit code, and a user-facing hint. Commands
// wrap these with context and the top level recovers them with errors.As to
// render a structured error envelope and choose an exit code.
package terr

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

// Error is the concrete coded error. Construct it with New and wrap it with
// fmt.Errorf("%w: ...") to add context while preserving the code via errors.As.
type Error struct {
	code    string
	exit    int
	hint    string
	msg     string
	wrapped error
	details any
}

var (
	_ Coded    = (*Error)(nil)
	_ Detailed = (*Error)(nil)
)

// New returns a coded error with the given code, exit code, hint, and message.
func New(code string, exit int, hint, msg string) *Error {
	return &Error{code: code, exit: exit, hint: hint, msg: msg}
}

// Error returns the message; when a wrapped error is present it is appended as
// "message: wrapped".
func (e *Error) Error() string {
	if e.wrapped != nil {
		return e.msg + ": " + e.wrapped.Error()
	}
	return e.msg
}

// Wrap returns a copy of the error with err set as its wrapped cause. The
// receiver is left unchanged so package-level sentinels stay reusable.
func (e *Error) Wrap(err error) *Error {
	c := *e
	c.wrapped = err
	return &c
}

// Unwrap returns the wrapped cause, or nil, so errors.Is/As traverse the chain.
func (e *Error) Unwrap() error { return e.wrapped }

// Code returns the stable error code.
func (e *Error) Code() string { return e.code }

// ExitCode returns the process exit code associated with the error.
func (e *Error) ExitCode() int { return e.exit }

// Hint returns a user-facing hint for resolving the error, or "" if none.
func (e *Error) Hint() string { return e.hint }

// ErrorDetails returns the structured details attached with WithDetails, or nil.
func (e *Error) ErrorDetails() any { return e.details }

// WithDetails returns a copy of the error carrying the given structured
// details. The receiver is left unchanged so package-level sentinels stay
// reusable.
func (e *Error) WithDetails(details any) *Error {
	c := *e
	c.details = details
	return &c
}
