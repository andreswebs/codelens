package command

import (
	"context"
	"fmt"

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

// runRoot builds the root command, neutralizes urfave's exit-and-help defaults,
// and runs it against args. args arrives WITHOUT the program name, so the root
// name is re-prepended for urfave's parser. A recognized-but-unknown command is
// captured by CommandNotFound and turned into the ErrUnknownCommand sentinel;
// any other error is returned unchanged for Run's exit boundary to resolve.
func runRoot(ctx context.Context, args []string, deps Deps) error {
	var format string
	var unknownCmd string

	root := &cli.Command{
		Name:      "codelens",
		Usage:     "mine a git log and run evolutionary code analyses",
		Version:   version.Current(),
		Reader:    deps.In,
		Writer:    deps.Out,
		ErrWriter: deps.Err,
		Flags:     globalFlags(&format),
		Commands:  append(analysisCommands(deps.In), metaCLICommands()...),
		// urfave routes an unrecognized command to its help topic; capturing it
		// here suppresses that and lets the top level classify it as a usage
		// error.
		CommandNotFound: func(_ context.Context, _ *cli.Command, name string) {
			unknownCmd = name
		},
	}
	neutralize(root)

	err := root.Run(ctx, append([]string{root.Name}, args...))
	if unknownCmd != "" {
		return ErrUnknownCommand.WithDetails(map[string]string{"command": unknownCmd})
	}
	return err
}

// neutralize sets the two exit-and-help hooks on cmd and every subcommand,
// recursively, so they are configured in exactly one place rather than at each
// construction site. Neither hook is inherited by urfave, hence the walk. The
// reasoning each hook carries is load-bearing:
//
//   - OnUsageError returns the parse error unchanged, suppressing urfave's
//     default "Incorrect Usage" banner and command-help dump on a flag-parse or
//     missing-required-flag error, so stdout stays results-only and the coded
//     error envelope (rendered by Run from the returned error) is the sole
//     diagnostic.
//   - ExitErrHandler is a no-op, so urfave returns errors from Run instead of
//     calling os.Exit; the top level owns exit-code mapping and error rendering.
func neutralize(cmd *cli.Command) {
	cmd.OnUsageError = onUsageError
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
	for _, sub := range cmd.Commands {
		neutralize(sub)
	}
}

// onUsageError returns err unchanged. See neutralize for why the hook exists; it
// is a named function so neutralize can install the same behavior everywhere.
func onUsageError(_ context.Context, _ *cli.Command, err error, _ bool) error {
	return err
}
