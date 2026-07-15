---
id: cod-f0zl
status: closed
deps: [cod-joym, cod-3ksh, cod-vdx3]
links: []
created: 2026-07-14T03:42:54Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-2]
---
# P2-3 cmd: registry-driven command tree + input wiring

Generate the urfave/cli command tree from the analysis registry: one subcommand per descriptor (plus aliases), global flags, stdin-default input, pipeline invocation, and result emission.

New files: src/cmd/codelens/commands.go (build commands from registry), edit main.go. Tests: commands_test.go.

Docs: plan.md (Phase 2), design cli-design.md sections 4 (surface), 6 (output), 8 (schema); reference docs/research/code-maat.md section 6 (authors), docs/research/urfave-cli.reference.md (CLI framework). Skills: /golang /tdd /llm-coding.
Design ref: cli-design.md 4.1 (aliases), 4.2/4.3 (flags), 5 (input). Depends on P2-1 (registry), P1-3 (parser), P0-1 (root app), P0-3 (emit).

## Design

- func analysisCommands() []*cli.Command : for each analysis.All(), build a*cli.Command{ Name, Aliases, Usage:Summary, Flags: global+descriptor flags, Action }.
- Global flags (persistent/shared): --log (string, default "" => stdin; "-" => stdin), --input-encoding (default UTF-8), --format (json|ndjson|csv|table, default json), --fields (string), --rows (int, 0=all), --group + --group-format(text|json,default text), --team-map + --team-map-format(csv|json,default csv), --temporal-period (int, 0=off), --debug (bool).
- Per-command flags come from Descriptor.Flags (coupling thresholds, --time-now, --expression, --verbose) - only attached to commands that declare them.
- Action: read log (stdin or --log file) -> gitlog.Parse -> pipeline transforms (group/temporal/teammap; stubs until Phase 3, wire the calls now guarded by flag presence) -> build analysis.Opts from flags -> descriptor.Run -> apply --rows truncation (set TotalCount/Truncated) -> emit via output in the chosen format (P2-4). Errors returned to run() (P0-1) for exit-code mapping.
- Input reading helper: func readLog(cmd) (io.Reader, error) opening file or os.Stdin; --input-encoding decoding.

TDD cases (commands_test.go via run() with buffers + stdin):

1. TestCmd_Authors_FromStdin_JSON: pipe a small git2 log to `authors` -> exit 0, stdout parses to a Result with analysis "authors" and expected rows.
2. TestCmd_Alias_Resolves: invoking a known alias runs the same analysis (use a later analysis's alias in Phase 4; for now assert `authors` has none and unknown alias -> exit 2).
3. TestCmd_LogFile: --log testdata file works equivalently to stdin.
4. TestCmd_MissingLog_EmptyStdin: empty stdin -> empty_log error, exit 3.
5. TestCmd_Rows_Truncates: --rows 1 on a 2-row result -> 1 row emitted, total_count 2, truncated true.

## Acceptance Criteria

- One subcommand per registered analysis with correct flags; global flags parsed; stdin default; --log file works.
- --rows sets truncation metadata; errors map to exit codes.
- Cases 1-5 pass (some formats deferred to P2-4 but json path works here); make validate green.

## Notes

**2026-07-14T11:08:11Z**

Built the registry-driven command tree in src/cmd/codelens/commands.go: analysisCommands() builds one cli.Command per analysis.All() descriptor (Name+Aliases+Summary+per-command flags+Action). Global flags (--log, --input-encoding, --format, --fields, --rows, --group[-format], --team-map[-format], --temporal-period, --debug) live on the ROOT command; urfave inherits non-Local flags into subcommand flag sets and cmd.lookupFlag walks the lineage, so subcommand Actions read them via cmd.String/Int/Bool and they parse after the subcommand name. run() gained a stdin io.Reader param (main.go + main_test.go updated); root.Reader=stdin and the Action closes over stdin. Action pipeline: openLog (stdin default, --log FILE, '-'=stdin, read-only open; open failure -> coded input_error exit 3) -> gitlog.Parse -> descriptor.Run -> truncate(&res, --rows) -> emitResult. truncate() uses reflect to cap any []row slice, setting RowCount=n, TotalCount=total, Truncated=true. Per-command Opts are read only for flags the descriptor declares (analysisOpts gates by d.Flags) so no analysis picks up another's threshold. DEFERRED (respecting ticket boundaries): --format only honors json (emitResult falls back to JSON for all formats) and --format enum validation -> P2-4/cod-9eay; suppressing urfave's help-on-flag-error stdout dump -> cod-ic8y; group/temporal/team-map transforms wired but not applied -> P3-1/cod-0xx4. 5 TDD cases in commands_test.go all green; make build clean.
