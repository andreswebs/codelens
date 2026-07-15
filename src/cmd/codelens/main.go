// Command codelens mines a git log and runs evolutionary code analyses,
// emitting structured JSON by default.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/andreswebs/codelens/internal/output"
	"github.com/andreswebs/codelens/internal/terr"
	"github.com/andreswebs/codelens/internal/version"
	"github.com/urfave/cli/v3"
)

// init prints --version as the bare version string (no "codelens version "
// prefix from urfave's default template), so it is trivial to capture and
// compare in scripts. It writes to the command's configured writer, keeping
// stdout-capturing tests working, and version.Current() stays the single source.
func init() {
	cli.VersionPrinter = func(cmd *cli.Command) {
		_, _ = fmt.Fprintln(cmd.Root().Writer, cmd.Root().Version)
	}
}

// run builds the root command, executes it against args (argv-style, including
// the program name), and returns the process exit code. The log is read from
// stdin (unless --log names a file); results are written to stdout and
// diagnostics to stderr. A returned error is rendered as a coded error envelope
// on stderr (output.EmitError) and mapped to an exit code (output.ExitCodeFor).
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var format string
	var debug bool
	var unknownCmd string

	root := &cli.Command{
		Name:      "codelens",
		Usage:     "mine a git log and run evolutionary code analyses",
		Version:   version.Current(),
		Reader:    stdin,
		Writer:    stdout,
		ErrWriter: stderr,
		Flags:     globalFlags(&format, &debug),
		Commands:  append(analysisCommands(stdin), metaCLICommands()...),
		// urfave routes an unrecognized command to its help topic; capturing it
		// here suppresses that and lets the top level classify it as a usage
		// error. Subcommands are added in a later phase.
		CommandNotFound: func(_ context.Context, _ *cli.Command, name string) {
			unknownCmd = name
		},
		// Suppress urfave's default "Incorrect Usage" banner and help dump on a
		// flag-parse error, so stdout stays results-only and the coded error
		// envelope is the sole diagnostic. The raw error is returned unchanged
		// for the top level to classify.
		OnUsageError: onUsageError,
		// Return errors from Run instead of letting urfave call os.Exit, so the
		// top level owns exit-code mapping and error rendering.
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
	}

	err := root.Run(context.Background(), args)
	if unknownCmd != "" {
		err = terr.New(
			"unknown_command",
			2,
			"run `codelens --help` to list available commands",
			"unknown command: "+unknownCmd,
		).WithDetails(map[string]string{"command": unknownCmd})
	}
	if err == nil {
		return 0
	}

	// Verbose diagnostics are emitted only under --debug; the coded error
	// envelope is the sole output otherwise.
	if debug {
		slog.New(slog.NewJSONHandler(stderr, nil)).Error("command failed", "error", err)
	}
	output.EmitError(stderr, err)
	return output.ExitCodeFor(err)
}

// onUsageError returns err unchanged. Assigning it to a command's OnUsageError
// hook suppresses urfave's default "Incorrect Usage" banner and command-help
// dump on a flag-parse or missing-required-flag error, so stdout carries only
// results and stderr carries only the coded error envelope (rendered by the top
// level from the returned error). It is set on the root and every subcommand
// because the hook is not inherited.
func onUsageError(_ context.Context, _ *cli.Command, err error, _ bool) error {
	return err
}

func main() {
	os.Exit(run(os.Args, os.Stdin, os.Stdout, os.Stderr))
}
