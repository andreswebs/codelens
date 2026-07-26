package command

import "github.com/andreswebs/codelens/internal/terr"

// ErrUnknownCommand marks an invocation naming a command the binary does not
// expose. It is a usage error (exit 64): the caller mistyped or invented a
// command. The offending name is attached as a detail at the call site. As a
// registered package-level sentinel it appears in terr.All() and so in the
// schema error inventory (ADR 0003), unlike the per-invocation error it replaced.
var ErrUnknownCommand = terr.New(
	"unknown_command", 64,
	"run `codelens --help` to list available commands",
	"unknown command",
)

// errLogOpen marks a failure to open the file named by --log. It is an I/O
// error (exit 74): opening a user-supplied path failed, which is an I/O fault
// rather than a data or usage error.
var errLogOpen = terr.New(
	"log_open_failed", 74,
	"check that the --log path exists and is readable",
	"cannot open log file",
)

// errFileOpen marks a failure to open a user-supplied auxiliary file (--group
// or --team-map). Like --log, an unreadable path is an I/O error (exit 74): the
// path is user-supplied, so opening it failing is an I/O fault, never an
// internal one. The offending flag and path are attached as details.
var errFileOpen = terr.New(
	"input_file_open_failed", 74,
	"check that the file path exists and is readable",
	"cannot open input file",
)

// errUnknownSchemaCommand marks a --command value that names no known analysis.
// It is a usage error (exit 64) and carries the resolvable command names in its
// details so a caller can recover without a second round trip.
var errUnknownSchemaCommand = terr.New(
	"unknown_schema_command", 64,
	"run `codelens schema` to list all commands",
	"unknown command",
)

// errBadAfter marks an --after value that is not a well-formed YYYY-MM-DD date.
// It is a usage error (exit 64): the caller passed a malformed flag value.
var errBadAfter = terr.New(
	"invalid_after_date", 64,
	"pass the date as YYYY-MM-DD, e.g. --after=2024-01-01",
	"invalid --after date",
)
