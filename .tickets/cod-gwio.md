---
id: cod-gwio
status: open
deps: []
links: []
created: 2026-07-22T18:16:31Z
type: task
priority: 2
assignee: Andre Silva
---
# Migrate exit codes to the fleet taxonomy (ADR 0002)

## Context

`docs/adr/0002-exit-code-taxonomy.md` (already in this repo) adopts a
family-wide taxonomy: 0 success; 1 recoverable result; 2-63 optional
result sub-codes; 64 EX_USAGE; 65 EX_DATAERR; 70 EX_SOFTWARE;
74 EX_IOERR; 78 EX_CONFIG; 130/143 signals. codelens currently uses
0 success / 2 usage / 3 input / 1 internal (documented in
`docs/cli-design.md` section 7.2, now superseded). Breaking change
(v0.x); include release notes. The tool does not catch signals;
nothing to do there.

## Old to new mapping (decided; implement exactly this)

| Old | Class | New |
| --- | ----- | --- |
| 0 | success (including empty result) | 0 (unchanged) |
| 2 | usage error | 64 |
| 3 | input/data error | 65 |
| 1 | internal / unexpected | 70 |

codelens reads a log file the user supplies; open failures are
currently `input_error` exit 3. Under the new taxonomy file-open
failures are I/O: `errLogOpen` and `errFileOpen` become 74, while
content/parse errors become 65 (see the per-sentinel list). No result
sub-codes (1 or 2-63) are claimed: an analysis that finds nothing
still exits 0 with an empty result, unchanged.

## Sentinels to change (all `terr.New(code, exit, hint, msg)`; verified 2026-07-22)

Currently exit 2, change to 64:

- `internal/output/fields.go:17` `invalid_field`
- `internal/output/format.go:19` `usage_error` (unknown format)
- `internal/transform/temporal/temporal.go:25` `invalid_temporal_period`
- `internal/transform/filter/filter.go:26` `invalid_glob`
- `internal/transform/group/group.go:34` `invalid_group`
- `internal/analysis/messages.go:21` `invalid_expression`
- `internal/analysis/codeage.go:20` `invalid_time_now`
- `cmd/codelens/printlogcommand.go:30` `usage_error` (bad `--after`)
- `cmd/codelens/schema.go:19` `usage_error` (unknown schema command)
- `cmd/codelens/main.go:65` `unknown_command` (inline `terr.New`)

Currently exit 3, change to 65 (data/content errors):

- `internal/gitlog/errors.go:10` `parse_error` (`ErrParse`)
- `internal/gitlog/errors.go:19` `empty_log`
- `internal/gitlog/errors.go:28` `parse_error` (`ErrControlChar`)
- `internal/transform/temporal/temporal.go:33` `invalid_temporal_date`
- `internal/transform/teammap/teammap.go:28` `invalid_team_map`
- `internal/analysis/messages.go:32` `missing_messages`
- `internal/analysis/churn/churn.go:18` `missing_metrics`

Currently exit 3, change to 74 (file-open failures, reclassified):

- `cmd/codelens/commands.go:25` `errLogOpen` (code `input_error`;
  rename the code string to `io_error` while changing the exit, and
  update its doc comment, which currently explains why it is exit 3)
- `cmd/codelens/commands.go:35` `errFileOpen` (same treatment)

## Boundary changes (`internal/output/errors.go`)

- Constants at lines 16-19: `exitUsage` 2 becomes 64, `exitInternal` 1
  becomes 70. Update the doc comment at lines 13-15 (it cites
  cli-design.md section 7.2; cite ADR 0002 instead).
- `ExitCodeFor` (around line 86) needs no structural change; it
  resolves through the constants and coded errors.
- `usageClasses` substring table (around line 106): codes and hints
  unchanged; they all resolve to `exitUsage`, which now yields 64.

## Registry updates (declared exit-code sets)

codelens already declares exit codes as data, surfaced by the `schema`
command:

- Every analysis descriptor declares `ExitCodes: []int{0, 2, 3, 1}`
  (19 sites in `internal/analysis/*.go`: abschurn.go:40,
  authorchurn.go:39, ownership.go:39, maindev.go:42, soc.go:38,
  coupling.go:56, revisions.go:33, authors.go:35, and the rest; grep
  `ExitCodes:`). Change each to `[]int{0, 64, 65, 70, 74}` (74 because
  every analysis opens the log file via `errLogOpen`).
- Meta commands in `cmd/codelens/metacommands.go:40,50` declare
  `[]int{0, 2}`; change to `[]int{0, 64}` (add 70 only if their paths
  can reach the internal fallback; verify).
- Per the ADR a conformance test must tie declarations to real exits.
  `cmd/codelens/schemacodes_test.go` already exists and asserts
  schema-declared codes; extend or verify it so every declared code is
  exercised by at least one in-process `run()` invocation asserting
  the observed exit.

## Tests to update

Inline exit-int assertions live in (all `cmd/codelens/` unless noted):
`main_test.go`, `commands_test.go`, `usage_error_test.go`,
`error_format_test.go`, `schema_test.go`, `schemacodes_test.go`,
`printlogcommand_test.go`, `pipeline_e2e_test.go`,
`e2e_authors_test.go`, `e2e_coupling_warn_test.go`, and
`internal/output/errors_test.go`. Update expected ints per the mapping
(2 to 64, 3 to 65 or 74 per the sentinel list, 1 to 70). Golden files
under `internal/gitlog/testdata/` contain parsed output, not exit
codes; they should not change.

## Docs to update

- `docs/cli-design.md` section 7.2 (table around line 292; also the
  narrative mentions at lines 114 "exit 2" and 165 "exit 3", and the
  section 8 text). State the new table; keep the code-string column.
- `docs/skills/codelens/SKILL.md` and especially its reference
  `docs/skills/codelens/references/operating.md` (exit mentions at
  lines 89, 115, and the "Errors and exit codes" section at line 207).
- `README.md` "Errors and exit codes" section (around line 230).
- `AGENTS.md` line 19 already references the exit-code taxonomy;
  verify it points at ADR 0002 and contains no stale literals.

## Acceptance

- All sentinels carry the new exit ints; `exitUsage` is 64 and
  `exitInternal` is 70; `errLogOpen`/`errFileOpen` are 74 with code
  `io_error`.
- All 19 analysis descriptors and the meta commands declare the new
  sets; `codelens schema` output reflects them; the conformance test
  exercises every declared code.
- All listed tests updated and passing; `make build` passes from the
  repo root.
- `docs/cli-design.md`, `README.md`, and the skill docs describe only
  the new scheme (`grep -rn "exit 2\|exit 3" docs README.md` finds
  nothing stale).
- Release notes drafted (breaking change: usage 2 to 64, input 3 to
  65/74, internal 1 to 70).
- Leave committing to the repo owner; do not create git commits.
