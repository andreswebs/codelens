---
id: cod-5t5t
status: closed
deps: []
links: [cod-451g]
created: 2026-07-15T03:40:57Z
type: chore
priority: 3
assignee: Andre Silva
tags: [codelens, cli, friction]
---
# Drop the version subcommand; make --version print bare version

`codelens` reports its version two ways that disagree: the root `--version` flag
prints `codelens version v0.0.1` (urfave/cli's default template) while the
`version` subcommand prints the bare `v0.0.1`. Collapse to one: **drop the
`version` subcommand** and make `--version` print the bare version string.

## Decision

Keep only the `--version` flag; it prints bare `v0.0.1` (via
`internal/version.Current()`). The subcommand is redundant surface. Bare output is
chosen over the prefixed form because it is trivial to capture and compare in
scripts with nothing to strip, and `version.Current()` stays the single source of
truth.

## Context: meta-command spine

The `version` subcommand was recently brought onto the meta-command spine (see
ticket `cod-451g`): it lives as an entry in `metaCommands()` in
`src/cmd/codelens/metacommands.go`, its action is `versionAction` in
`src/cmd/codelens/version_cmd.go`, and `schema --command version` returns its
schema. Dropping it partially unwinds that for this one command. Link this ticket
to `cod-451g` for context and update its conformance guard accordingly.

## Implementation

1. Remove the `version` entry from `metaCommands()` in
   `src/cmd/codelens/metacommands.go`. After removal, `metaCommands()` contains
   `print-log-command` and `schema` only.
2. Delete `src/cmd/codelens/version_cmd.go` (`versionAction`) and its
   `versionUsage` constant. Remove the now-unused `internal/version` import from
   that file only; `main.go` still imports it for the root `Version` field.
3. Make `--version` print bare. `main.go` sets `Version: version.Current()`.
   urfave/cli v3 renders the flag via a package-level `cli.VersionPrinter`. Set it
   (in `run()` or an `init()` in `main.go`) to print just the version:

   ```go
   cli.VersionPrinter = func(cmd *cli.Command) {
       _, _ = fmt.Fprintln(cmd.Root().Writer, cmd.Root().Version)
   }
   ```

   Confirm the exact `cli.VersionPrinter` signature against urfave/cli v3.10.1
   (`go doc github.com/urfave/cli/v3.VersionPrinter`) before wiring; adjust if the
   installed version differs. The flag must write to the command's configured
   writer (so tests capturing stdout keep working), not `os.Stdout` directly.
4. The unknown-command recovery hint (`allCommandNames()` in `metacommands.go`)
   automatically stops listing `version` once it leaves the table; no separate
   edit. Confirm no other code hard-codes the string `"version"` as a command.

## Behavior changes (intended)

- `codelens version` becomes an unknown command (exit 2, `unknown_command`
  envelope), same as any other non-command.
- `codelens --version` prints `v0.0.1` (bare) and exits 0.
- `schema --command version` becomes a usage error (the command no longer exists);
  `schema` no longer lists `version`.

## TDD plan (/tdd)

1. Update `src/cmd/codelens/version_cmd_test.go`:
   - Remove `TestVersion_Subcommand` (the subcommand is gone).
   - Change `TestVersion_Flag` to assert the stdout is **exactly** the bare
     `version.Current()` (plus trailing newline), not merely `Contains`, so it
     pins the bare format. Keep it asserting exit 0.
   - Rename/rehome the test file if it no longer concerns a subcommand (e.g. fold
     the flag test into an existing `main`/CLI test file per repo convention).
2. Update the conformance guard `src/cmd/codelens/schemacodes_test.go`: its
   meta-command portion iterates `metaCommandSummaries()` / `metaCommands()`;
   dropping `version` is reflected automatically, but confirm the test does not
   assert a fixed meta-command count that now changes.
3. Add/adjust a test that `codelens version` returns exit 2 with the
   `unknown_command` code, matching the existing unknown-command handling in
   `main.go` (`unknownCmd` path).

Write the failing assertion first (bare `--version` output), then flip the printer;
one behavior per cycle.

## Files touched

```text
src/cmd/codelens/main.go                 set cli.VersionPrinter to bare output
src/cmd/codelens/metacommands.go         remove version from metaCommands()
src/cmd/codelens/version_cmd.go          delete
src/cmd/codelens/version_cmd_test.go     drop subcommand test; pin bare --version
src/cmd/codelens/schemacodes_test.go     confirm meta conformance still holds
docs/skills/codelens/references/operating.md   remove `version` helper + `schema --command version` mentions
docs/cli-design.md                       update if it documents the version subcommand
```

## Acceptance criteria

- `codelens --version` prints exactly `v0.0.1` (bare, from `version.Current()`) and
  exits 0.
- There is no `version` subcommand: `codelens version` exits 2 with the
  `unknown_command` envelope; `schema` does not list it.
- Exactly one code path produces the version string; `version.Current()` remains
  the single source.
- `make build` is green (validate + compile), all tests pass.
- operating.md (and cli-design.md if applicable) no longer reference a `version`
  subcommand; Markdown passes markdownlint per project standard.

## References

- `src/cmd/codelens/main.go`, `metacommands.go`, `version_cmd.go`
- `src/cmd/codelens/version_cmd_test.go`, `schemacodes_test.go`
- `internal/version/version.go` (`Current()`)
- Related: `cod-451g` (meta-command spine)
- Skills: `/golang` (urfave idioms), `/tdd`, `/llm-coding` (surgical deletion)

## Notes

**2026-07-15T04:31:31Z**

Dropped the version subcommand; --version now prints the bare version string. Set a package-level cli.VersionPrinter in an init() in main.go to write cmd.Root().Version (bare) to cmd.Root().Writer, replacing urfave's 'codelens version <v>' template. Removed the version entry from metaCommands(), deleted version_cmd.go (versionAction) and the versionUsage const in schema.go. version.Current() stays the single source. Behavior: 'codelens version' is now unknown_command exit 2; schema no longer lists version; 'schema --command version' is a usage error. Tests: deleted duplicate version_cmd_test.go, strengthened the flag test in main_test.go to pin exact bare output (+newline), added TestRun_VersionSubcommand_UnknownExit2, dropped TestSchema_Command_Version, removed 'version' from the schema-list check. The schemacodes_test.go conformance guard iterates metaCommands() so it adjusted automatically (no fixed meta count). Docs updated: cli-design.md command surface + audit table + meta-schema note, operating.md helper mentions. make build green; markdownlint clean (repo-local .markdownlint.yaml).
