---
id: cod-uavr
status: open
deps: []
links: [cod-pp0d, cod-8eis, cod-q42s, cod-2x18, cod-vsi0]
created: 2026-07-26T18:21:56Z
type: task
priority: 1
assignee: Andre Silva
tags: [codelens, adr-adoption, breaking]
---
# ADR adoption 1/6: make every terr error code unique

Adoption ticket 1 of 6 in the ADR standardization set (plan:
.local/planning/adr-adoption.md, decisions: .local/planning/responses.txt).
Make every terr error code unique, so a registering `terr.New` that panics on
duplicate codes can land in the next ticket.

codelens carries the standardization ADRs shifted by one, because
`docs/adr/0001-keep-churn-and-effort-separate.md` predates them. Cite the LOCAL
numbers: 0002 exit-code taxonomy, 0003 error handling, 0004 CLI structure,
0005 logging, 0006 output contract, 0007 CLI testing.

## Why

ADR 0003 requires error codes to be stable machine codes that a consumer
branches on, and requires the documented error surface not to drift from the
real one. Today three codes are each carried by more than one sentinel, which
means a consumer branching on the code cannot tell three unrelated failures
apart, and it blocks the terr registry port: the reference `terr.New` panics on
a duplicate code, and `cmd/codelens` links every duplicate group, so a
registering `New` would panic at init.

Owner decision (responses.txt, Q4): error codes are unique per sentinel. This
is a breaking change to the published error-code surface and lands in this
adoption pass. `schema_version` stays at 1 by owner decision, so the envelope
carries no version signal for this change; the CHANGELOG is the record.

## Verified current state (re-verify before editing)

Nineteen `terr.New` call sites in production code. The three duplicate groups:

- `usage_error` (exit 64) x3:
  - `internal/output/format.go:19` `errUnknownFormat` - unknown `--format` value
  - `cmd/codelens/schema.go:19` `errUnknownSchemaCommand` - unknown
    `schema --command` value
  - `cmd/codelens/printlogcommand.go:30` `errBadAfter` - malformed `--after`
- `parse_error` (exit 65) x2:
  - `internal/gitlog/errors.go:10` `ErrParse` - malformed log entry
  - `internal/gitlog/errors.go:28` `ErrControlChar` - disallowed control char
- `io_error` (exit 74) x2:
  - `cmd/codelens/commands.go:25` `errLogOpen` - `--log` path unreadable
  - `cmd/codelens/commands.go:35` `errFileOpen` - `--group` / `--team-map` path
    unreadable

In every group the exit code agrees; only the hint, the message, and the
`details` payload differ.

The remaining sentinels keep their codes unchanged: `empty_log`
(`internal/gitlog/errors.go:19`), `invalid_field`
(`internal/output/fields.go:17`), `invalid_expression` and `missing_messages`
(`internal/analysis/messages.go:21`, `:32`), `invalid_time_now`
(`internal/analysis/codeage.go:20`), `missing_metrics`
(`internal/analysis/churn/churn.go:18`), `invalid_temporal_period` and
`invalid_temporal_date` (`internal/transform/temporal/temporal.go:25`, `:33`),
`invalid_group` (`internal/transform/group/group.go:34`), `invalid_team_map`
(`internal/transform/teammap/teammap.go:28`), `invalid_glob`
(`internal/transform/filter/filter.go:26`), and the inline `unknown_command`
at `cmd/codelens/main.go:65`.

## Design

## Renames

All three groups resolve by RENAME, not collapse: in each group the sites are
genuinely different failures with different hints and different `details`
payloads, so a consumer needs to tell them apart. Exit codes are unchanged.

| Location | Identifier | Old code | New code | Exit |
| --- | --- | --- | --- | --- |
| `internal/gitlog/errors.go:28` | `ErrControlChar` | `parse_error` | `invalid_control_char` | 65 |
| `internal/output/format.go:19` | `errUnknownFormat` | `usage_error` | `unknown_format` | 64 |
| `cmd/codelens/schema.go:19` | `errUnknownSchemaCommand` | `usage_error` | `unknown_schema_command` | 64 |
| `cmd/codelens/printlogcommand.go:30` | `errBadAfter` | `usage_error` | `invalid_after_date` | 64 |
| `cmd/codelens/commands.go:25` | `errLogOpen` | `io_error` | `log_open_failed` | 74 |
| `cmd/codelens/commands.go:35` | `errFileOpen` | `io_error` | `input_file_open_failed` | 74 |

`ErrParse` keeps `parse_error`. Go identifiers are unchanged; only the code
strings move. Per ADR 0003, each sentinel's declaration must still document why
it carries its exit code, next to the code itself; adjust the existing doc
comments so they read correctly with the new code names.

## Descriptor declarations

`cmd/codelens/metacommands.go` declares `ErrorCodes` per meta command:

- `:39` `print-log-command`: `[]string{"usage_error"}` becomes
  `[]string{"invalid_after_date"}`
- `:49` `schema`: `[]string{"usage_error"}` becomes
  `[]string{"unknown_schema_command"}`

No `analysis.Descriptor` declares a renamed code (their `ErrorCodes` lists draw
only on `empty_log`, `missing_metrics`, `invalid_time_now`, `missing_messages`,
`invalid_expression`), so no descriptor file changes. Verify this with a grep
before concluding it.

## Tests to update

Contract assertions (must change):

- `cmd/codelens/pipeline_e2e_test.go:219-220` asserts `io_error`. Determine
  which sentinel that scenario hits (it drives an unreadable auxiliary or log
  path) and assert the corresponding new code.
- `cmd/codelens/schema_test.go:175-176` asserts `usage_error` for an unknown
  `schema --command`; becomes `unknown_schema_command`.
- `cmd/codelens/printlogcommand_test.go:100-101` asserts `usage_error` for a
  malformed `--after`; becomes `invalid_after_date`.
- `cmd/codelens/metacommands_test.go:26-27` and `:45-46` assert the declared
  `ErrorCodes` lists; update to match the new meta declarations.

Fixture strings (update for accuracy, not correctness):

- `internal/analysis/schema_test.go:142`, `:156` pass `"usage_error"` as a
  `MetaSchema` fixture. Change to a code that still exists so the fixture does
  not name a removed code.
- `internal/analysis/schema_test.go:25`, `:54` use `"parse_error"`, which
  survives; leave them.

Do NOT touch the exit-3 fixtures in `internal/terr/terr_test.go` or
`internal/output/errors_test.go`; those belong to the next ticket.

Note a latent weakness this ticket fixes: `internal/gitlog/parse_test.go:187`
`assertCoded` compares by `Code()`, so `assertCoded(t, err, ErrControlChar, 65)`
at `:111` passes today even if `ErrParse` were returned. After the rename that
assertion becomes meaningful. No edit needed; it starts working.

## Goldens

`cmd/codelens/testdata/authors.schema.json` carries `error_codes:
["empty_log"]`, which is unchanged. Verify no golden contains a renamed code
(`grep -rl 'usage_error\|io_error' cmd/codelens/testdata/`). If one does,
regenerate with `go test ./cmd/codelens/ -update` and review the diff by hand.

## Documentation sweep

- `docs/cli-design.md:306` (exit-64 code list), `:307` (exit-65 list), `:309`
  (exit-74 list): replace the renamed codes. `:266` and `:363` are envelope and
  schema examples that mention `parse_error`, which survives; check them
  anyway. `:434` mentions `parse_error`; survives.
- `docs/skills/codelens/references/operating.md`: check for any code list or
  code mention and update.
- `README.md`: the exit-code table at `:253-260` names meanings, not codes;
  verify before concluding no change is needed.
- `docs/specs/learnings.md`: APPEND a superseding entry; do not rewrite
  existing entries, that file is a log.
- Do not touch `docs/research/code-maat.md` (upstream reference material).

Run markdownlint on every markdown file touched:
`markdownlint-cli2 --config .markdownlint.yaml --fix <FILE>` (the project
config exists at the repo root; use it, not the global one).

## CHANGELOG

`CHANGELOG.md` does not exist yet. Create it in Keep a Changelog format with an
`## [Unreleased]` section and a `### Changed` (breaking) entry listing every
old-to-new code mapping from the table above, stating that `schema_version`
stays at 1 and that consumers branching on `usage_error` or `io_error` must
migrate. Later tickets in this set append to the same `Unreleased` section.

## Out of scope

- No change to `internal/terr` (ticket 2).
- No change to the descriptor enumeration or the schema envelope shape
  (ticket 3).
- Do not remove `--debug` (ticket 4) or create `internal/command` (ticket 5).
- No exit-code changes. Every exit code observed by the suite must be
  identical before and after.

## Acceptance Criteria

- No two `terr.New` call sites in the module share a code. Verify with
  `grep -rn -A2 'terr\.New(' --include='*.go' .` over non-test files, or a
  temporary throwaway test; the permanent guard arrives in ticket 2.
- Each renamed failure emits its new code with its pre-existing exit code:
  - `codelens --format bogus authors` -> `unknown_format`, exit 64
  - `codelens schema --command bogus` -> `unknown_schema_command`, exit 64
  - `codelens print-log-command --after notadate` -> `invalid_after_date`,
    exit 64
  - `codelens --log /nonexistent authors` -> `log_open_failed`, exit 74
  - `codelens --group /nonexistent authors` -> `input_file_open_failed`,
    exit 74
  - a log containing a NUL byte -> `invalid_control_char`, exit 65
- `grep -rn 'usage_error\|io_error' --include='*.go' .` returns nothing.
- `grep -rn 'usage_error\|io_error' README.md docs/cli-design.md docs/skills/`
  returns nothing (excluding `docs/research/` and any appended learnings entry
  that quotes the old codes historically).
- `TestExitCodesRegistered_AllCommands` and
  `TestExitCodes_DeclaredCodesAreExercised` in
  `cmd/codelens/schemacodes_test.go` pass unmodified except for the meta
  declaration changes they read.
- Every exit code the suite observes is unchanged from before the ticket.
- `CHANGELOG.md` exists and records every rename.
- Every markdown file touched is markdownlint clean against
  `.markdownlint.yaml`.
- `make build` passes (it runs `validate`: `fmt-check`, `vet`, `lint`, `test`)
  before the ticket closes.
- DO NOT COMMIT. The owner owns all git operations: no commits, no branches,
  no staging. Leave the work tree dirty for review.

