---
id: cod-2x18
status: closed
deps: [cod-8eis]
links: [cod-pp0d, cod-8eis, cod-q42s, cod-uavr, cod-vsi0]
created: 2026-07-26T18:39:30Z
type: task
priority: 1
assignee: Andre Silva
tags: [codelens, adr-adoption]
---
# ADR adoption 3/6: reconcile the descriptor error-code enumeration with the terr registry

Adoption ticket 3 of 6 in the ADR standardization set (plan:
.local/planning/adr-adoption.md, decisions: .local/planning/responses.txt).
Reconcile the hand-maintained per-command error-code enumeration with the terr
registry, and add the tool-level baseline and global inventory the schema is
missing.

Depends on cod-8eis (terr registry: `terr.All()` must exist).

codelens carries the standardization ADRs shifted by one, because
`docs/adr/0001-keep-churn-and-effort-separate.md` predates them. Cite the LOCAL
numbers: 0002 exit-code taxonomy, 0003 error handling, 0004 CLI structure,
0005 logging, 0006 output contract, 0007 CLI testing.

## Why

ADR 0003 requires error codes to be "enumerable as data, populated at sentinel
construction, so the documented error surface cannot drift from the real one."
Today the enumeration is hand-maintained per-command descriptor data, declared
in two places and disconnected from the sentinels, with no test tying a declared
code to a real one. Owner decisions: Q1 option A (register at construction,
cross-check declared codes against `terr.All()`, add a global inventory to
`schema`) and Q1b option A1 (keep per-command lists analysis-specific, add a
tool-level `common_error_codes` field).

`schema_version` stays at 1 by owner decision. The two new fields are additive,
so a consumer that ignores unknown keys is unaffected.

## Verified current state (re-verify before editing)

The declaration sites:

- `internal/analysis/analysis.go:109-112`: `Descriptor.ErrorCodes []string` and
  `ExitCodes []int`, with the doc comment "ErrorCodes lists the terr codes the
  analysis may return".
- All twenty analyses declare `ExitCodes: []int{0, 64, 65, 70, 74}` and an
  `ErrorCodes` list drawn from only five values:
  - `empty_log` alone: `authors.go:34`, `communication.go:42`, `coupling.go:55`,
    `entityeffort.go:39`, `fragmentation.go:36`, `maindevbyrevs.go:44`,
    `parse.go:45`, `revisions.go:32`, `soc.go:37`, `summary.go:32`
  - `empty_log`, `missing_metrics`: `abschurn.go:39`, `authorchurn.go:38`,
    `entitychurn.go:38`, `maindev.go:41`, `ownership.go:38`,
    `refactoringmaindev.go:42`
  - `empty_log`, `invalid_time_now`: `codeage.go:53`
  - `empty_log`, `missing_messages`, `invalid_expression`: `messages.go:64`
- `cmd/codelens/metacommands.go:20-21`: `metaCommand.ErrorCodes`/`ExitCodes`,
  declared at `:39-40` and `:49-50` (the error codes there are the renamed ones
  from cod-uavr: `invalid_after_date` and `unknown_schema_command`).

The projection: `internal/analysis/schema.go:56-57` copies the lists into
`CommandSchema`; `:76-77` does the same for meta commands via `MetaSchema`;
`:110` carries `ExitCodes` into the command-list summary.

The guards: `cmd/codelens/schemacodes_test.go:126-173`
(`TestExitCodesRegistered_AllCommands`) asserts the schema surfaces exactly the
declared lists; `:68-100` (`TestExitCodes_DeclaredCodesAreExercised`) ties every
declared exit code to an observed exit through `run()`. NOTHING ties a declared
error code to a real sentinel.

The drift, measured: the union of every declared error code is six values,
against nineteen reachable codes before cod-uavr (twenty-three after). The
declared `ExitCodes` already admit 64/65/70/74, so the undeclared codes are
reachable by construction: a bad `--format` emits `unknown_format`, a bad
`--fields` path emits `invalid_field`, a bad `--include` glob emits
`invalid_glob`, an unreadable `--log` emits `log_open_failed`. The per-command
lists are in practice "codes specific to this analysis plus the log-parse code",
and nothing says so.

Codes that reach the envelope WITHOUT a terr sentinel (as of this ticket; the
first three become sentinels in cod-q42s):

- `unknown_flag`, `invalid_value`, `missing_required_flag` from the usage
  classifier table at `internal/output/errors.go:106-112`
- `internal_error`, the uncoded fallback at `internal/output/errors.go:80`

## Design

## 1. Document the field semantics

`internal/analysis/analysis.go:109` and `cmd/codelens/metacommands.go:20`: state
explicitly that these lists carry only the codes DISTINCTIVE to that command,
beyond the common input and option codes reported as `common_error_codes`. The
current wording ("the terr codes the analysis may return") is false and is what
lets the drift read as a bug rather than a convention.

## 2. `common_error_codes` on the command schema

Add to `CommandSchema` (`internal/analysis/schema.go:14-24`):

```go
// CommonErrorCodes lists the codes any invocation of this tool can produce
// (input, option, and output-layer failures), declared once at tool level.
// ErrorCodes carries only the codes distinctive to this command, so an agent
// enumerates the full reachable surface as CommonErrorCodes + ErrorCodes.
CommonErrorCodes []string `json:"common_error_codes"`
```

Populate it from ONE declaration, not per descriptor. Put the declaration where
it can be verified: it must list exactly the codes any analysis run can produce
regardless of which analysis. Derive the membership from the tree, do not copy
this list blindly. Expected members after cod-uavr:

`parse_error`, `empty_log`, `invalid_control_char`, `unknown_format`,
`invalid_field`, `invalid_glob`, `invalid_group`, `invalid_team_map`,
`invalid_temporal_period`, `invalid_temporal_date`, `log_open_failed`,
`input_file_open_failed`, `unknown_flag`, `invalid_value`,
`missing_required_flag`, `internal_error`, `unknown_command`.

Note `empty_log` and `parse_error` are currently ALSO declared per analysis
(every descriptor lists `empty_log`). Decide one way and be consistent: either
they are common (drop `empty_log` from all twenty descriptors, which makes many
`ErrorCodes` lists empty) or they stay per-command as the log-input codes and
are excluded from the common list. RECOMMENDATION: keep them per-command and
exclude them from `common_error_codes`, because the owner decision was to make
no descriptor edits ("no descriptor edits to the code"). Document the choice in
the `common_error_codes` declaration's comment.

Both `Schema(d Descriptor)` and `MetaSchema(...)` must populate the field, so
meta commands report the same baseline. Normalize to non-nil like every other
slice field (`nonNilStrings`), so it marshals as `[]` not `null`.

Route it so `MetaSchema`'s signature does not grow a seventh positional
parameter if that reads badly; setting the field inside both constructors from
the single declaration is cleaner than threading it through call sites.

## 3. Global error inventory on the command list

Add to `CommandList` (`internal/analysis/schema.go:38-42`), mirroring the
reference's `SchemaError` in
`~/cookiecutters/go-cookiecutter/{{cookiecutter.project_name}}/internal/command/commands.go`:

```go
// Errors is the tool's full error inventory, derived from the terr registry so
// it cannot drift from the sentinels that actually exist.
Errors []SchemaError `json:"errors"`

type SchemaError struct {
    Code     string `json:"code"`
    ExitCode int    `json:"exit_code"`
    Hint     string `json:"hint,omitempty"`
}
```

Build it from `terr.All()`. Sort by code for a stable envelope (registration
order depends on package init order, which is not a contract). This is the
clause of ADR 0003 the whole ticket exists to satisfy: the inventory is
populated at sentinel construction and cannot drift.

Watch the import direction: `internal/analysis` already imports
`internal/output` (`schema.go:6`). Importing `internal/terr` from
`internal/analysis` is fine (terr imports only `fmt`). Verify no cycle with
`go build ./...` rather than by inspection.

The non-sentinel codes (`unknown_flag`, `invalid_value`,
`missing_required_flag`, `internal_error`) cannot come from `terr.All()` in this
ticket. Either include them in the inventory from the allowlist below with a
marker, or leave the inventory sentinel-only and let `common_error_codes` carry
them. RECOMMENDATION: leave the inventory sentinel-derived and pure; the
allowlist plus `common_error_codes` covers the rest, and cod-q42s converts three
of the four to real sentinels, which shrinks the gap to `internal_error` alone.
State the choice in the field's doc comment.

## 4. The cross-check conformance test

Add to `cmd/codelens/schemacodes_test.go` (it moves to `internal/command` in
cod-q42s; write it so the move is mechanical):

`TestErrorCodes_DeclaredCodesExist`: for every code declared by any
`analysis.Descriptor`, any `metaCommand`, or the `common_error_codes`
declaration, assert it is either present in `terr.All()` or a member of an
explicit allowlist of non-sentinel codes. The test must fail when a descriptor
declares a code no sentinel carries.

Declare the allowlist as DATA next to the usage classifier in
`internal/output/errors.go`, exported for the test, and assert in the same test
that the allowlist matches the classifier table (`usageClasses` at `:106-112`)
plus `internal_error`, so the two cannot drift. That is the second half of the
guard: without it the allowlist becomes a place to hide a missing sentinel.

Consider also the reverse direction: every sentinel in `terr.All()` should
appear in at least one command's declared list or in `common_error_codes`, so a
sentinel cannot exist unreported. Add it if it passes; if a sentinel is
legitimately unreachable from any command, that is itself worth knowing, so
report it as a failure and resolve it rather than weakening the test.

## 5. Goldens and docs

- Regenerate with `go test ./cmd/codelens/ -update` and REVIEW the diff by
  hand. Expect `cmd/codelens/testdata/authors.schema.json` to gain
  `common_error_codes`; expect any command-list golden to gain `errors`.
- `docs/cli-design.md:363` shows a schema example with
  `"error_codes": ["parse_error", "empty_log"]`; extend the example with
  `common_error_codes` and document both fields where the schema envelope is
  described.
- `docs/skills/codelens/references/operating.md:48` describes what
  `schema --command` returns ("summary, flags, row_schema, error_codes,
  exit_codes"); add `common_error_codes`, and document the `errors` inventory
  on the bare `schema` command. This file is the canonical operations
  reference; an agent reads it to learn the envelope, so an omission here
  defeats the ticket.
- Append a `CHANGELOG.md` entry under the existing `## [Unreleased]` section
  (created in cod-uavr): the two additive schema fields, and that
  `schema_version` stays at 1 because they are additive.
- Markdownlint every markdown file touched:
  `markdownlint-cli2 --config .markdownlint.yaml --fix <FILE>`.

## Out of scope

- No descriptor `ErrorCodes` edits (owner decision A1: the per-command lists
  stay analysis-specific).
- Do not make the per-command lists complete (that was option A2, rejected).
- Do not tag sentinels with the commands that can emit them (option B,
  rejected).
- Do not touch `--debug` (cod-vsi0) or create `internal/command` (cod-q42s).

## Acceptance Criteria

- `codelens schema --command CMD` emits `common_error_codes` for every command,
  analyses and meta commands alike, marshalled as `[]` rather than `null` when
  empty.
- `codelens schema` (no `--command`) emits an `errors` inventory of
  `{code, exit_code, hint}` derived from `terr.All()`, sorted by code.
- `Descriptor.ErrorCodes` and `metaCommand.ErrorCodes` doc comments state that
  the lists are command-specific and that the baseline lives in
  `common_error_codes`.
- `TestErrorCodes_DeclaredCodesExist` passes, and FAILS when a descriptor
  declares a code with no sentinel. Demonstrate this during development with a
  deliberate local edit; do not commit the edit.
- The non-sentinel allowlist is declared as data next to the usage classifier,
  and a test asserts it equals the `usageClasses` codes plus `internal_error`,
  so adding a classifier entry without updating the allowlist fails.
- `TestExitCodesRegistered_AllCommands` and
  `TestExitCodes_DeclaredCodesAreExercised` in
  `cmd/codelens/schemacodes_test.go` still pass.
- Goldens regenerated with `go test ./cmd/codelens/ -update`; the diff contains
  ONLY the two additive fields, reviewed by hand. No row data, no exit code, and
  no error code changed.
- `schema_version` is still 1 (`internal/output/types.go:8` unchanged).
- `docs/cli-design.md` and
  `docs/skills/codelens/references/operating.md` document both new fields;
  `CHANGELOG.md` records them; every markdown file touched is markdownlint
  clean against `.markdownlint.yaml`.
- `make build` passes (it runs `validate`: `fmt-check`, `vet`, `lint`, `test`)
  before the ticket closes.
- DO NOT COMMIT. The owner owns all git operations: no commits, no branches,
  no staging. Leave the work tree dirty for review.


## Notes

**2026-07-26T19:10:16Z**

Reconciled the descriptor error-code enumeration with the terr registry per ADR 0003. Added two additive schema fields (schema_version stays 1): (1) common_error_codes on CommandSchema — a tool-level baseline of the 15 codes any invocation can produce (input/global-option/output-layer failures), declared once as analysis.commonErrorCodes and copied into both Schema() and MetaSchema(); a command's error_codes now carries only its distinctive codes. (2) errors inventory on CommandList (schema, no --command) — {code,exit_code,hint} for all 18 registered sentinels, built from terr.All() and sorted by code, so it cannot drift from the sentinels. Followed the recommendation to make NO descriptor edits: empty_log stays per-analysis and is excluded from the baseline; parse_error IS in the baseline (no descriptor declares it, and the reverse-direction guard requires every sentinel be declared somewhere). unknown_command is deliberately excluded from the baseline: it is a terr.Newf (unregistered) pre-dispatch error and would fail the sentinel-or-allowlist guard. Guards: TestErrorCodes_DeclaredCodesExist (every declared code is a terr.All() sentinel or in output.NonSentinelCodes; reverse: every sentinel is declared somewhere) and TestNonSentinelAllowlist_MatchesClassifier (NonSentinelCodes == output.UsageClassCodes()+InternalErrorCode, so a new classifier entry without an allowlist update fails). Both fail-closed; demonstrated during dev then reverted. New data in internal/output/errors.go: NonSentinelCodes, InternalErrorCode, UsageClassCodes(). Docs: cli-design.md, operating.md, CHANGELOG.md, learnings.md. Golden authors.schema.json regenerated (only the additive field). NOTE: the working tree also carries uncommitted work from closed deps cod-8eis/cod-uavr (owner owns commits); left dirty for review per instructions. make build green.
