package gitlog

import "github.com/andreswebs/codelens/internal/terr"

// The parser's coded errors. All are input errors (exit 3): the log is data
// supplied by the user, so a failure to parse it is never an internal fault.
var (
	// ErrParse marks a structurally invalid log entry. It is wrapped with the
	// offending entry index and line and carries them as structured details.
	ErrParse = terr.New(
		"parse_error", 3,
		"generate the log with `codelens print-log-command`",
		"failed to parse git log",
	)

	// ErrEmptyLog marks input that contains no entries at all (empty or
	// whitespace-only). A well-formed log whose entries happen to touch no files
	// is not empty and does not trigger this error.
	ErrEmptyLog = terr.New(
		"empty_log", 3,
		"provide a non-empty git2 log on stdin or via --log",
		"the log is empty",
	)

	// ErrControlChar marks input containing a disallowed control character (such
	// as NUL). Tab is permitted because numstat fields are tab-separated; the
	// line terminator is consumed by the tokenizer and never reaches this check.
	ErrControlChar = terr.New(
		"parse_error", 3,
		"the input contains disallowed control characters",
		"invalid control character in input",
	)
)
