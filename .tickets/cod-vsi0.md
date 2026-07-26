---
id: cod-vsi0
status: open
deps: [cod-2x18]
links: [cod-pp0d, cod-8eis, cod-q42s, cod-uavr, cod-2x18]
created: 2026-07-26T18:40:26Z
type: task
priority: 1
assignee: Andre Silva
tags: [codelens, adr-adoption, breaking]
---
# ADR adoption 4/6: adopt the silent logging posture and remove --debug

Adoption ticket 4 of 6 in the ADR standardization set (plan:
.local/planning/adr-adoption.md, decisions: .local/planning/responses.txt).
Adopt the ADR 0005 silent logging posture: remove `--debug` and the slog logger
entirely.

Depends on cod-2x18 for sequencing only (both touch `cmd/codelens`; no logical
dependency). Runs BEFORE the `internal/command` move (cod-q42s) so that ticket
does not relocate code this one deletes.

codelens carries the standardization ADRs shifted by one, because
`docs/adr/0001-keep-churn-and-effort-separate.md` predates them. LOGGING IS
LOCAL ADR 0005, not 0004; 0004 is CLI structure. Cite the local numbers.

## Why

ADR 0005 (local) defines two conformant postures. A tool that talks to external
systems or runs long carries a leveled logger; a single-shot computational tool
may instead stay SILENT, where coded errors are the failure channel and
advisories flow through a machine-readable warning channel on stderr. The ADR
also states plainly: "Unused logging infrastructure is a defect under this ADR,
not a default."

codelens is a single-shot log analyzer. It opens no network connections, spawns
no subprocesses, runs no daemon, and is strictly read-only (it never runs git).
Owner decision (responses.txt, Q3): silent posture, remove `--debug`.

Two further facts make the current `--debug` logger non-conformant rather than
merely unnecessary:

1. It logs the same error the envelope renders one line later, which ADR 0005's
   invariant "log records never duplicate error-envelope content" forbids
   directly.
2. It is a root flag that no `Descriptor.Flags` or `metaCommand.Flags` entry
   declares, so `codelens schema --command CMD` never mentions it. It is
   undiscoverable at runtime, which defeats the schema-discovery contract that
   the rest of the CLI is built around.

The warning channel the silent posture relies on ALREADY EXISTS:
`output.EmitWarning`, wired into analyses through `analysis.Opts.Warn`
(`internal/analysis/analysis.go:72-92`) and bound at
`cmd/codelens/commands.go:188-190`. Nothing needs to be built to replace
`--debug`; the replacement is already in service.

This is a user-visible breaking change: an invocation passing `--debug` starts
failing with exit 64.

## Verified current state (re-verify before editing)

- `cmd/codelens/main.go:9` imports `log/slog`.
- `cmd/codelens/main.go:35`: `var debug bool`.
- `cmd/codelens/main.go:45`: `Flags: globalFlags(&format, &debug)`.
- `cmd/codelens/main.go:76-79`: the only logging in the tool,
  `if debug { slog.New(slog.NewJSONHandler(stderr, nil)).Error("command failed", "error", err) }`,
  immediately before `output.EmitError(stderr, err)` at `:81`.
- `cmd/codelens/commands.go:44-46`: the `globalFlags` doc comment mentioning
  `debug`.
- `cmd/codelens/commands.go:47`: `func globalFlags(format *string, debug *bool) []cli.Flag`.
- `cmd/codelens/commands.go:102-106`: the `--debug` `cli.BoolFlag` with
  `Destination: debug`.
- `cmd/codelens/main_test.go:101-111`: `TestRun_DebugFlag_Parsed`.
- `cmd/codelens/main_test.go:113-131`: `TestRun_DebugTraceOnlyUnderDebug`.

`log/slog` is imported nowhere else. `internal/version/version.go:5` imports
`runtime/debug`, which is unrelated and must NOT be touched.

## Design

## Code changes

1. `cmd/codelens/main.go`:
   - delete the `log/slog` import at `:9`
   - delete `var debug bool` at `:35`
   - change `:45` to `Flags: globalFlags(&format)`
   - delete the comment at `:76-77` and the `if debug { ... }` block at
     `:78-79`, leaving `output.EmitError(stderr, err)` and
     `return output.ExitCodeFor(err)` as the whole tail of `run`
   - update `run`'s doc comment at `:28-32` if it mentions debug gating
2. `cmd/codelens/commands.go`:
   - change `:47` to `func globalFlags(format *string) []cli.Flag`
   - delete the `--debug` `cli.BoolFlag` at `:102-106`
   - update the doc comment at `:44-46`, which currently says "format and debug
     are bound to the caller's destinations so the top level can render errors
     and gate diagnostics even when a subcommand's Action never runs"; it should
     explain why `format` alone is bound (so `EmitError` has a valid format even
     when flag parsing fails before the Action runs)

After the change the tool imports no logging package at all. That is the point
of the silent posture: the absence is the conformance evidence.

## Test changes

- Delete `TestRun_DebugFlag_Parsed` (`cmd/codelens/main_test.go:101-111`).
- Delete `TestRun_DebugTraceOnlyUnderDebug` (`:113-131`).
- Add a test PINNING the removal, so a future reader cannot mistake the absence
  for an oversight and so a re-added flag trips a test. Model it on the existing
  helper `runUsageError` in `cmd/codelens/usage_error_test.go:13-41`, which
  already asserts exit 64, empty stdout, a decoded envelope, and a non-empty
  hint:

  ```go
  // TestRun_DebugFlagRemoved pins the ADR 0005 silent posture: codelens
  // carries no logging infrastructure, so --debug is not a flag and is
  // rejected as one.
  func TestRun_DebugFlagRemoved(t *testing.T) {
      code, _ := runUsageError(t, "authors", "--debug")
      if code != "unknown_flag" {
          t.Errorf("code = %q, want unknown_flag", code)
      }
  }
  ```

  Verify the flag position: `--debug` was a ROOT flag, so check both
  `codelens --debug authors` and `codelens authors --debug` reject it, since
  urfave resolves root flags through the subcommand's lineage either way.

- Add a guard that no logging package is imported anywhere in the module, so the
  posture cannot regress silently. A small test walking the module's imports
  (or, if that reads as overkill, a `make` check) asserting no non-test file
  imports `log` or `log/slog`. Prefer the test, so `make build` catches it.

## ADR record

ADR 0005 defines both postures but does not record which one codelens chose.
Add that record so the absence of a logger is documented as a decision rather
than an omission. Either:

- append a short "Posture chosen" subsection to `docs/adr/0005-logging.md`
  stating that codelens adopts the silent posture, with the reason (single-shot
  computational tool; coded errors are the failure channel; advisories flow
  through `output.EmitWarning`), or
- add a companion note under `docs/adr/` following the existing naming.

Prefer amending 0005 in place; the ADR set is codelens-local and the posture is
part of the decision, not a new one.

## Documentation sweep

Every one of these was verified to reference `--debug`:

- `docs/cli-design.md:101` (the global-flags table row), `:308` (the exit-70
  row, "bug; only path that prints a trace (with `--debug`)"), `:432` (the
  design-principles table, "Traces only under `--debug`")
- `README.md:258` (the exit-code table's exit-70 row, "a bug; the only path that
  prints a trace, and only under `--debug`")
- `docs/skills/codelens/references/operating.md:222` (the exit-code table's
  exit-70 row, "a bug; prints a trace only under `--debug`")
- `docs/specs/001-initial-implementation/requirements.md:241-243`: TWO EARS
  requirements mandating a debug option:
  "WHILE a debug option is enabled, the system shall include diagnostic detail
  (such as stack traces) on the error stream." and "WHILE a debug option is not
  enabled, the system shall not expose internal stack traces to the user."
  Retire or restate them. Retiring is correct: the second requirement's intent
  (never leak traces to users) is now unconditionally true, so restate it as an
  unconditional requirement and drop the first. Note the adjacent block at
  `:236-239` still states the PRE-MIGRATION exit codes (2 usage, 3 input, 1
  unexpected), superseded by ADR 0002; fixing those while the paragraph is open
  is welcome but optional, and if done must be called out in the ticket notes.
- `docs/specs/001-initial-implementation/plan.md:117` (the global-flag list),
  `:292` ("only under `--debug`")
- `docs/specs/learnings.md:60-64`: an entry describing the `--debug` slog
  behavior. APPEND a superseding entry; do not rewrite the existing one, that
  file is a log.

Do NOT touch `docs/research/code-maat.md:310`, which is upstream reference
material describing code-maat, not codelens.

Append a `CHANGELOG.md` entry under the existing `## [Unreleased]` section
(created in cod-uavr) under a breaking-change heading: `--debug` removed, no
replacement needed, advisories already flow through the warning channel, and an
invocation passing `--debug` now exits 64 with `unknown_flag`.

Markdownlint every markdown file touched:
`markdownlint-cli2 --config .markdownlint.yaml --fix <FILE>`.

## Out of scope

- Do NOT add `--log-level`, `--quiet`, `CODELENS_LOG_LEVEL`, handler selection,
  or a `logctx` package. That is the leveled posture, explicitly rejected.
- Do not change the warning channel, `output.EmitWarning`, or
  `analysis.Opts.Warn`.
- Do not change any error envelope, exit code, or result envelope.
- Do not create `internal/command` (cod-q42s).

## Acceptance Criteria

- `grep -rn '"log/slog"\|"log"' --include='*.go' .` returns nothing outside
  tests (and `internal/version/version.go`'s `runtime/debug` import is
  untouched).
- `grep -rn 'debug' --include='*.go' cmd/ internal/` returns only
  `internal/version/version.go`'s `runtime/debug` usage and
  `internal/analysis/parse.go`'s "debug/interop" prose.
- `codelens --debug authors` and `codelens authors --debug` both exit 64 with an
  `unknown_flag` error envelope on stderr and empty stdout, pinned by a test.
- A test (not just a convention) fails if any non-test file imports `log` or
  `log/slog`.
- `TestRun_DebugFlag_Parsed` and `TestRun_DebugTraceOnlyUnderDebug` are gone.
- Every remaining test passes unmodified, and every success and error envelope
  is byte-identical to before the ticket. `git diff --stat
  cmd/codelens/testdata/` is empty: no golden changes.
- `docs/adr/0005-logging.md` (or a companion note) records that codelens adopts
  the silent posture, with the reason.
- `grep -rn 'debug' README.md docs/cli-design.md docs/skills/ docs/specs/` finds
  no reference presenting `--debug` as a current flag. Historical entries in
  `docs/specs/learnings.md` may remain if a superseding entry follows them.
- The two EARS requirements at
  `docs/specs/001-initial-implementation/requirements.md:241-243` are retired or
  restated.
- `CHANGELOG.md` records the removal as breaking.
- Every markdown file touched is markdownlint clean against
  `.markdownlint.yaml`.
- `make build` passes (it runs `validate`: `fmt-check`, `vet`, `lint`, `test`)
  before the ticket closes.
- DO NOT COMMIT. The owner owns all git operations: no commits, no branches,
  no staging. Leave the work tree dirty for review.

