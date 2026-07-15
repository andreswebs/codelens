---
id: cod-12yk
status: open
deps: []
links: [cod-451g, cod-btfg]
created: 2026-07-15T03:40:57Z
type: feature
priority: 2
assignee: Andre Silva
tags: [codelens, cli, feature, friction]
---
# print-log-command: emit --use-mailmap, drop --all default

The `git log` command emitted by `codelens print-log-command` has two defaults
that hurt the resulting analyses: it never applies `.mailmap` (so author aliases
inflate ownership and communication maps), and it hardcodes `--all` (so every ref,
including unmerged branch tips and cross-branch merges, pollutes the history).
Fix both in the emitted command.

## Current state

`src/cmd/codelens/printlogcommand.go`:

```go
const logCommandBase = "git log --all --numstat --date=short --pretty=format:'--%h--%ad--%aN--%s' --no-renames"
```

`emitLogCommand` appends `--after=DATE` when `--after` is passed. The command is
declared as a meta command in `metaCommands()` (`src/cmd/codelens/metacommands.go`)
with an `after` string flag.

## Decision

1. **`--use-mailmap`: emit by default.** Add `--use-mailmap` to the base command
   unconditionally. It is a safe no-op when the repo has no `.mailmap`, and
   correctly collapses author aliases when one exists. No opt-out flag (YAGNI).
2. **`--all`: drop from the default; add an opt-in `--all` flag.** The default
   command reads the checked-out branch's history (what users expect, and what
   code-maat does), eliminating the "commits from unmerged branches / dated after
   HEAD" surprise. `print-log-command --all` restores the all-refs behavior.
3. **Merge commits: leave in** (no `--no-merges`). They carry no `--numstat`, so
   they do not affect file-level analyses; their subjects carry real signal.
   Dropping `--all` already removes the worst cross-branch merge noise.

## Implementation

- Change `logCommandBase` to drop `--all` and add `--use-mailmap`:

  ```go
  const logCommandBase = "git log --numstat --date=short --pretty=format:'--%h--%ad--%aN--%s' --no-renames --use-mailmap"
  ```

  Keep `--no-renames` (the parser depends on it) and the four-field pretty format
  unchanged.
- Add an `all` bool flag to the `print-log-command` entry in `metaCommands()`
  (`metacommands.go`), described as "include all refs (default: current branch
  only)". Meta-command flags are declared in the `analysis.Flag` shape and wired via
  `toCLIFlag`, so a `{Name: "all", Type: "bool", Default: false, Desc: "..."}`
  entry is sufficient.
- Thread it through the action: `printLogCommandAction` reads `cmd.Bool("all")` and
  passes it to `emitLogCommand`. Update `emitLogCommand(w io.Writer, after string,
  all bool)` to insert `--all` (right after `git log`) when `all` is true. Keep the
  existing `--after` validation/append logic unchanged.
- The `--after` flag and its `errBadAfter` usage-error behavior are unchanged.

## `.mailmap` vs `--team-map` guidance (#9, rides here)

Document, in `docs/skills/codelens/references/operating.md` and the knowledge/
communication cards in `references/catalog.md`, the alias-resolution ladder:

- **Zero-config:** the emitted log now uses `--use-mailmap`, so a repo `.mailmap`
  collapses aliases automatically. This is the recommended first step.
- **Escalation:** when there is no `.mailmap` (or team-level rollup is wanted), use
  `--team-map` to map authors (or aliases) to canonical identities/teams. In the
  test-drive, mapping each alias to a canonical name via `--team-map` collapsed 34
  -> 24 authors and removed a spurious self-tie in the communication network.

Also add a short note to the `--all` effects: `print-log-command` now defaults to
the current branch; pass `--all` for cross-branch history, at the cost of merge/
branch-tip noise.

## TDD plan (/tdd)

`printlogcommand_test.go` already tests the emitted string and `--after`. Extend it:

1. `TestPrintLogCommand_DefaultOmitsAllUsesMailmap`: default output contains
   `--use-mailmap`, does **not** contain `--all`, still contains `--numstat`,
   `--no-renames`, and the four-field pretty format. (Adjust the existing default
   assertion, which currently expects `--all`.)
2. `TestPrintLogCommand_AllFlag`: with `--all`, output contains `--all` (and still
   `--use-mailmap`).
3. `TestPrintLogCommand_AfterStillWorks`: `--after 2025-01-01` appends
   `--after=2025-01-01`; combines correctly with `--all`.
4. `TestPrintLogCommand_BadAfter`: unchanged usage-error (exit 2) behavior.
5. `schema --command print-log-command` now surfaces the `all` flag (extend the
   meta-introspection assertion from `cod-451g`).

Write the failing default-output assertion first (mailmap present, `--all`
absent), then change the const; then add the `--all` flag and its test.

## Files touched

```text
src/cmd/codelens/printlogcommand.go       logCommandBase (drop --all, add --use-mailmap); emitLogCommand(all bool)
src/cmd/codelens/metacommands.go          add `all` bool flag to print-log-command entry
src/cmd/codelens/printlogcommand_test.go  default/mailmap/--all/after cases
docs/skills/codelens/references/operating.md   canonical-workflow log line; --all note; alias ladder
docs/skills/codelens/references/catalog.md     knowledge/communication cards: mailmap + --team-map
docs/cli-design.md                         update if it pins the print-log-command output (§5)
```

## Acceptance criteria

- `codelens print-log-command` emits a `git log` that includes `--use-mailmap` and
  does **not** include `--all` by default; `--numstat`, `--no-renames`, `--date=
  short`, and the `--%h--%ad--%aN--%s` format are unchanged.
- `codelens print-log-command --all` emits the same command plus `--all`.
- `--after=DATE` still validates (exit 2 on a malformed date) and appends
  correctly, alone or with `--all`.
- `schema --command print-log-command` lists the `after` and `all` flags.
- operating.md/catalog.md document the `--use-mailmap` default, the `--all` opt-in,
  and the `.mailmap` -> `--team-map` alias ladder; Markdown passes markdownlint per
  project standard.
- `make build` green.

## References

- `src/cmd/codelens/printlogcommand.go` (`logCommandBase`, `emitLogCommand`)
- `src/cmd/codelens/metacommands.go` (`print-log-command` entry, `toCLIFlag`)
- `src/cmd/codelens/printlogcommand_test.go`
- `docs/cli-design.md` §5 (log command), `references/operating.md` (canonical
  workflow, pipeline transforms), `references/catalog.md` (knowledge/communication)
- Related: `cod-451g` (meta-command introspection: `schema --command` must surface
  the new `all` flag)
- Skills: `/golang`, `/tdd`, `/llm-coding` (surgical: only the two default changes
  plus one opt-in flag)
