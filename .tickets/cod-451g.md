---
id: cod-451g
status: closed
deps: []
links: [cod-5t5t, cod-12yk]
created: 2026-07-15T00:58:10Z
type: task
priority: 2
assignee: Andre Silva
tags: [codelens, architecture, deepening]
---
# Bring meta commands onto the descriptor spine (introspectable schema)

Architecture-review candidate 4. Analyses derive their `cli.Command`, help, schema, and
exit codes from one `Descriptor`. The three meta commands (`schema`, `version`,
`print-log-command`) do not: each is built by a bespoke builder func, and
`metaSummaries()` re-declares Name/Summary/ExitCodes for the command list. Name is
declared twice and the exit codes live disconnected from the command. Worse, meta
commands are **not introspectable** - `schema --command print-log-command` is a usage
error, so an agent cannot discover `--after` / `--command` at runtime, contradicting
the "runtime schema is the source of truth" ethos (cli-design.md 8).

Decided shape (full introspectable schema, cmd-local type): a `metaCommand` struct in
package `main` is the single source per meta command, projecting to the `cli.Command`,
the command-list `CommandSummary`, and a full `CommandSchema` served by
`schema --command <meta>`. Meta commands stay a distinct type (not
`analysis.Descriptor`, whose `Run(mods, opts)` and `RowSchema` do not fit them).

Skills: /tdd (new introspection tests first), /golang (idioms), /llm-coding (surgical,
single source, verifiable success).

## Design

### `metaCommand` (new, package main)

```go
// metaCommand is the single source for a non-analysis command's identity: it
// declares the command's name, summary, flags, and error/exit codes once, then
// projects to the cli wiring, the schema command list, and the full schema. Meta
// commands are not analyses (no log input, no rows), so they are a distinct type
// rather than an analysis.Descriptor.
type metaCommand struct {
    Name       string
    Summary    string
    Flags      []analysis.Flag  // declared in the analysis.Flag shape; reused by toCLIFlag
    ErrorCodes []string
    ExitCodes  []int
    Action     cli.ActionFunc
}

func (m metaCommand) command() *cli.Command      // Name, Usage=Summary, OnUsageError, toCLIFlag(Flags), Action
func (m metaCommand) summary() analysis.CommandSummary  // Name, Summary, ExitCodes
func (m metaCommand) schema() analysis.CommandSchema    // analysis.MetaSchema(...)
```

The single table (co-located with the meta actions):

```go
func metaCommands() []metaCommand {
    return []metaCommand{
        {Name: "print-log-command", Summary: printLogCommandUsage,
         Flags: []analysis.Flag{{Name: "after", Type: "string",
            Desc: "limit history to commits after `DATE` (YYYY-MM-dd)"}},
         ErrorCodes: []string{"usage_error"}, ExitCodes: []int{0, 2},
         Action: printLogCommandAction},
        {Name: "schema", Summary: schemaUsage,
         Flags: []analysis.Flag{{Name: "command", Type: "string",
            Desc: "describe a single `CMD` in full (flags, row schema, codes)"}},
         ErrorCodes: []string{"usage_error"}, ExitCodes: []int{0, 2},
         Action: schemaAction},
        {Name: "version", Summary: versionUsage,
         ExitCodes: []int{0}, Action: versionAction},
    }
}
```

The existing builder funcs `schemaCommand`/`versionCommand`/`printLogCommand` dissolve
into this table; their bodies become the named actions `schemaAction` /
`versionAction` / `printLogCommandAction`, kept in their current files
(`schema.go`/`version_cmd.go`/`printlogcommand.go`) next to their helpers
(`emitLogCommand`, etc.). `toCLIFlag` (already in `commands.go`) is reused, so the
`--after`/`--command` flags' Usage text comes from the flag `Desc` exactly as for
analyses.

### `analysis.MetaSchema` (new, internal/analysis/schema.go)

Meta commands have no `Descriptor`, so add a constructor that builds a `CommandSchema`
from explicit parts and normalizes slices to non-nil (Aliases and RowSchema are empty
for meta commands). `Schema(d Descriptor)` is left untouched.

```go
// MetaSchema builds a CommandSchema for a non-analysis command (no aliases, no row
// schema). Analyses use Schema(d) instead.
func MetaSchema(command, summary string, flags []Flag, errorCodes []string, exitCodes []int) CommandSchema
```

### Wiring

- `main.go`: `Commands: append(analysisCommands(stdin), metaCLICommands()...)` where
  `metaCLICommands()` maps `metaCommands()` -> `.command()`. (Replaces the three
  builder calls.)
- `schema.go`: `metaSummaries()` becomes a projection `metaCommands()` -> `.summary()`
  (keep the name or rename to `metaCommandSummaries()`; update the one caller and the
  conformance test). `schemaAction` resolves meta commands after analyses:

```go
if d, ok := analysis.Lookup(name); ok {
    return output.EmitJSON(w, analysis.Schema(d))
}
if m, ok := lookupMeta(name); ok {   // linear scan over metaCommands()
    return output.EmitJSON(w, m.schema())
}
return errUnknownSchemaCommand.WithDetails(...)  // known_commands now includes meta names
```

- The unknown-command recovery hint (`analysisNames()`) extends to include meta command
  names (`allCommandNames()`), so the hint stays complete.

### Behavior changes (intended)

- NEW: `schema --command schema|version|print-log-command` returns a `CommandSchema`
  (exit 0) instead of `usage_error` (exit 2). No existing test asserts the old error,
  so nothing regresses; this is the added capability.
- The unknown-command error's `known_commands` list now includes the meta names.
- Unchanged: the command LIST output (meta summaries identical), and `version` /
  `print-log-command` stdout.

### TDD plan (/tdd)

1. Red: add tests that `schema --command print-log-command` surfaces the `after` flag,
   `error_codes` `[usage_error]`, `exit_codes` `[0,2]`; that `schema --command version`
   surfaces `exit_codes` `[0]` and empty flags/row_schema; reuse the `schemaOf` helper
   (decodes `CommandSchema`). Add an `analysis.MetaSchema` unit test (non-nil
   normalization). These fail against the current tree.
2. Green: add `metaCommand` + `metaCommands()` + projections + `MetaSchema`; rewire
   `main.go`/`schema.go`; convert builders to named actions.
3. Strengthen the candidate-2 conformance guard (`schemacodes_test.go`): now that meta
   commands are introspectable, verify each meta command via `schemaOf` (exit/error
   codes match the `metaCommands()` entry), in addition to the command-list check.
4. `make build` green; `TestSchema_List`, `version_cmd_test`, `printlogcommand_test`,
   `e2e_authors_test` stay green unchanged.

### Docs

Update `docs/skills/codelens/references/operating.md` (and `cli-design.md` if it states
meta commands are not introspectable) to note `schema --command` now describes meta
commands too. Markdownlint the edits.

### Out of scope

- No merge of `effort` into `churn` (candidate 5).
- No change to analysis descriptors, the analysis registry, or `Schema(d)`.
- No attempt to derive exit codes from runtime behavior; they remain declared on the
  `metaCommand` (as they are for analyses on the `Descriptor`).

### Files touched

```text
cmd/codelens/metacommands.go (new)    metaCommand type, metaCommands(), projections, lookupMeta, metaCLICommands
cmd/codelens/main.go                  Commands assembly via metaCommands()
cmd/codelens/schema.go                metaSummaries -> projection; schemaAction meta lookup; allCommandNames
cmd/codelens/version_cmd.go           builder -> versionAction
cmd/codelens/printlogcommand.go       builder -> printLogCommandAction
internal/analysis/schema.go           add MetaSchema
cmd/codelens/schemacodes_test.go      strengthen meta conformance via schemaOf
cmd/codelens/*_test.go                new meta-introspection tests
internal/analysis/schema_test.go      MetaSchema test
docs/skills/codelens/references/operating.md   note meta introspection
```

## Acceptance Criteria

- A single `metaCommand` table is the source for each meta command; Name/Summary/
  ExitCodes are declared once and drive the cli wiring, the command list, and the
  schema. No meta command's identity is declared in two places.
- `schema --command schema|version|print-log-command` returns a valid `CommandSchema`
  (exit 0) surfacing that command's flags, error codes, and exit codes; the flag set
  matches the wired `cli.Flag`s (same `toCLIFlag` source).
- `analysis.MetaSchema` builds a normalized `CommandSchema`; `analysis.Schema(d)` is
  unchanged.
- The command-list output and `version`/`print-log-command` stdout are unchanged; the
  conformance guard verifies every meta command's codes through `schema` as well as the
  list.
- Operating docs note meta-command introspection; markdownlint clean.
- `make build` green (validate + compile).

## Notes

**2026-07-15T01:12:05Z**

Meta commands (schema/version/print-log-command) now sit on a single-source spine. New cmd/codelens/metacommands.go defines a metaCommand struct + metaCommands() table that projects to (a) the cli.Command wiring via .command() (reusing toCLIFlag), (b) the schema command-list summary via .summary(), and (c) the full introspection schema via .schema(). New analysis.MetaSchema() builds a normalized CommandSchema for non-analysis commands (empty aliases/row_schema); analysis.Schema(d) untouched. The three builder funcs dissolved into named actions (schemaAction/versionAction/printLogCommandAction) kept next to their helpers. NEW capability: 'schema --command schema|version|print-log-command' now returns a CommandSchema (exit 0) instead of usage_error; unknown --command hint's known_commands merges analysis+meta names via allCommandNames() (sorted). metaSummaries() renamed to metaCommandSummaries() (now a projection). Conformance guard in schemacodes_test.go verifies each meta command through both the list and schema --command. TDD: red tests written first (analysis MetaSchema unit test + cmd meta-introspection tests), then green. make build clean; command-list and version/print-log-command stdout unchanged. Docs updated: operating.md + cli-design.md sec 8 note meta introspection.
