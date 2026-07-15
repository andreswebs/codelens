package main

import (
	"context"
	"fmt"

	"github.com/andreswebs/codelens/internal/version"
	"github.com/urfave/cli/v3"
)

// versionAction is the version subcommand's action (wired through the meta
// table). It prints the plain build version from internal/version.Current() to
// stdout and exits 0. The root command's --version flag reports the same value
// (main.go sets Version to version.Current()), so the flag and the subcommand
// share one source of truth. The output is the bare version string, not a JSON
// envelope, because it is meant to be read or captured directly.
func versionAction(_ context.Context, cmd *cli.Command) error {
	_, err := fmt.Fprintln(cmd.Root().Writer, version.Current())
	return err
}
