package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/andreswebs/codelens/internal/terr"
	"github.com/urfave/cli/v3"
)

// logCommandBase is the git command that produces a codelens-compatible log:
// the git2 format extended with the commit subject (%s) so every analysis,
// including messages, runs on a single log shape. See cli-design.md section 5.
// It reads the checked-out branch's history by default (the --all opt-in
// restores all refs) and applies .mailmap so author aliases collapse.
const logCommandBase = "git log --numstat --date=short --pretty=format:'--%h--%ad--%aN--%s' --no-renames --use-mailmap"

// dateLayout is the YYYY-MM-DD form accepted by --after; it is git's
// --date=short shape and time zero for code-age.
const dateLayout = "2006-01-02"

// printLogCommandUsage is the one-line summary of the print-log-command helper,
// reused for its entry in the schema command list so the two cannot drift.
const printLogCommandUsage = "print the git log command that generates a compatible log"

// errBadAfter marks an --after value that is not a well-formed YYYY-MM-DD date.
// It is a usage error (exit 2): the caller passed a malformed flag value.
var errBadAfter = terr.New(
	"usage_error", 2,
	"pass the date as YYYY-MM-DD, e.g. --after=2024-01-01",
	"invalid --after date",
)

// printLogCommandAction is the print-log-command helper's action (wired through
// the meta table). It emits the exact git log command that generates a
// codelens-compatible (extended git2 + subject) log, so neither a human nor an
// agent has to memorize the format. The output is the plain command line on
// stdout, not a JSON envelope, because it is meant to be copied and run.
func printLogCommandAction(_ context.Context, cmd *cli.Command) error {
	return emitLogCommand(cmd.Root().Writer, cmd.String("after"), cmd.Bool("all"))
}

// emitLogCommand writes the log command to w. When all is true it inserts --all
// (right after "git log") to read every ref rather than the current branch. It
// appends an --after=DATE window when after is non-empty; a non-empty after that
// is not a valid YYYY-MM-DD date is a usage error and nothing is written.
func emitLogCommand(w io.Writer, after string, all bool) error {
	command := logCommandBase
	if all {
		command = "git log --all" + command[len("git log"):]
	}
	if after != "" {
		if t, err := time.Parse(dateLayout, after); err != nil || t.Format(dateLayout) != after {
			bad := errBadAfter.WithDetails(map[string]string{"after": after})
			if err != nil {
				return bad.Wrap(err)
			}
			return bad
		}
		command += " --after=" + after
	}
	_, err := fmt.Fprintln(w, command)
	return err
}
