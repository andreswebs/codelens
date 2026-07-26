// Package command is the CLI delegate: it builds the command tree, runs it, and
// owns the process exit boundary. The package is split so the CLI framework can
// be replaced without touching the tool's contract.
//
//   - run.go is the framework-free contract. It declares Deps and Run and imports
//     no CLI framework package; no framework type appears in any exported
//     identifier of this package.
//   - every other file is the framework interior (root construction, the command
//     wiring, the usage-error classifier, and the CLI-surface sentinels). It is
//     replaceable in one place; run.go and the tests do not move when it changes.
package command

import (
	"context"
	"errors"
	"io"

	"github.com/andreswebs/codelens/internal/output"
	"github.com/andreswebs/codelens/internal/terr"
)

// Deps carries the process environment the delegate needs: the input stream and
// the result and diagnostic sinks. main wires these to the real os streams; a
// test wires buffers. No framework type appears here, so Run's signature does
// not weld the tool to the CLI framework (ADR 0004).
type Deps struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// Run builds and executes the command tree against args (the process arguments
// WITHOUT the program name) and returns the process exit code. It owns the exit
// boundary: it resolves the returned error, renders it as the coded error
// envelope on deps.Err (output.EmitError), and maps it to an exit code
// (output.ExitCodeFor). codelens catches no signals, so there is no 130/143
// override. Results go to deps.Out; diagnostics go to deps.Err.
func Run(args []string, deps Deps) int {
	err := runRoot(context.Background(), args, deps)
	if err == nil {
		return 0
	}
	err = resolve(err)
	output.EmitError(deps.Err, err)
	return output.ExitCodeFor(err)
}

// resolve returns the error to emit. A coded error is emitted unchanged. An
// uncoded error is run through the usage classifier: a match becomes the coded
// usage error (its raw framework message preserved, so the envelope is
// byte-identical to before the classifier moved out of internal/output); a
// non-match is returned unchanged for output.EmitError to stamp as
// internal_error. The classifier is the one sanctioned string-matching carve-out
// from ADR 0003, and it lives in the framework interior (usage.go) beside the
// markers it depends on.
func resolve(err error) error {
	var coded terr.Coded
	if errors.As(err, &coded) {
		return err
	}
	if classified := classifyUsage(err); classified != nil {
		return classified
	}
	return err
}
