---
id: cod-vdx3
status: closed
deps: [cod-g7yh]
links: []
created: 2026-07-14T03:37:08Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-0]
---
# P0-1 remove greet; wire root app + exit-code mapping

Remove the greet placeholder and wire the root command so main() maps returned errors to exit codes via output.ExitCodeFor and emits structured errors to stderr. Subcommands are added in P2-3.

Edit: src/cmd/codelens/main.go. New: src/cmd/codelens/main_test.go.

Docs (repo-root relative):

- Plan task: docs/specs/001-initial-implementation/plan.md (Phase 0)
- Design: docs/cli-design.md
- Requirements: docs/specs/001-initial-implementation/requirements.md
- CLI framework: docs/research/urfave-cli.reference.md (urfave/cli v3)
- Go style: /golang skill. TDD: /tdd skill (vertical slices, one test -> one impl).
Design ref: cli-design.md section 4 (command surface), 7 (errors). Depends on P0-3 (ExitCodeFor/EmitError).

## Design

Refactor main into a testable run function:

- func run(args []string, stdout, stderr io.Writer) int
    builds the root *cli.Command (Name "codelens", Usage, Version version.Current()), executes with args; on error: output.EmitError(stderr, format, err); return output.ExitCodeFor(err); else 0.
- func main() { os.Exit(run(os.Args, os.Stdout, os.Stderr)) }
- Keep slog to stderr but gate verbose/trace on a --debug persistent flag (traces only under --debug; otherwise one-line coded error).
- Root has no analysis subcommands yet; --help and --version work. A persistent --format flag (default json) determines EmitError format ("text" vs json).

TDD cases (main_test.go, call run() with buffers):

1. TestRun_NoArgs_PrintsHelp_ExitZero: run(["codelens"]) -> 0, stdout contains usage.
2. TestRun_Version: run(["codelens","--version"]) -> 0, stdout contains version.Current().
3. TestRun_UnknownCommand_UsageExit2: run(["codelens","bogus"]) -> 2, stderr JSON envelope ok:false code unknown/usage.
4. TestRun_DebugFlag_Parsed: run(["codelens","--debug","--help"]) -> 0 (flag accepted).

## Acceptance Criteria

- greet command removed; no dead placeholder code.
- run() returns 0 for help/version, 2 for unknown command; errors emitted to stderr writer.
- All 4 cases pass; make validate green.

## Notes

**2026-07-14T10:29:49Z**

Removed greet placeholder; wired testable run(args, stdout, stderr) int in src/cmd/codelens/main.go. main() = os.Exit(run(...)). Root cli.Command has persistent --format (default json) and --debug flags, no analysis subcommands yet (added in P2-3). Error handling: ExitErrHandler set to a no-op so Run returns errors to run() instead of urfave calling os.Exit; run() then maps via output.EmitError + output.ExitCodeFor. Key urfave/cli v3 finding: an unrecognized command is routed to the help topic ('No help topic for X'), NOT to classifyUsageError, so it would NOT map to exit 2. Fix: set CommandNotFound hook to capture the unknown name (it fires even with zero subcommands and suppresses the help-topic error, Run returns nil), then synthesize a terr.New("usage_error",2,...).WithDetails({command}) in run(). --debug gates a verbose slog trace (bound to the injected stderr writer, not global default) on top of the coded envelope. 5 tests in main_test.go all green; make build green.
