---
id: cod-pp0d
status: open
deps: [cod-q42s]
links: [cod-8eis, cod-q42s, cod-uavr, cod-2x18, cod-vsi0]
created: 2026-07-26T18:44:40Z
type: task
priority: 2
assignee: Andre Silva
tags: [codelens, adr-adoption, testing]
---
# ADR adoption 6/6: complete the ADR 0007 golden triple harness

Adoption ticket 6 of 6 in the ADR standardization set (plan:
.local/planning/adr-adoption.md, decisions: .local/planning/responses.txt).
Complete the ADR 0007 in-process golden harness: golden the full
stdout/stderr/exit triple per scenario, and cover the error scenarios that are
currently asserted field by field.

Depends on cod-q42s (`internal/command`), so the harness is written against
`Run(args, deps)` once rather than written and then converted.

codelens carries the standardization ADRs shifted by one, because
`docs/adr/0001-keep-churn-and-effort-separate.md` predates them. Cite the LOCAL
numbers: 0002 exit-code taxonomy, 0003 error handling, 0004 CLI structure,
0005 logging, 0006 output contract, 0007 CLI testing. CLI TESTING IS 0007.

## Why

ADR 0007 (local) requires: "scenarios invoke the delegate (`Run(args, deps)`)
with buffer streams and compare THREE artifacts per scenario against golden
files: stdout, stderr, and the exit code", with an `-update` flag, normalization
for volatile values, and conformance obligations expressed through the harness
("every code in the exit-code registry is exercised by at least one scenario,
every observed exit is a member of the registry, and error scenarios cover the
envelope shape").

Today's harness goldens stdout ONLY, covers success cases only, and asserts exit
0 and empty stderr inline. Error envelopes are asserted field by field in
separate tests, so no error scenario's stderr bytes are pinned. That matters
especially now: cod-uavr renamed six published error codes and cod-q42s reshaped
the error path (the classifier moved and now wraps in `Run`). Those changes
currently rest on field assertions, not on reviewable golden diffs.

This ticket is the safety net the rest of the set was verified against
informally. Landing it last means every earlier change becomes a reviewable
golden diff from here on.

## Verified current state (line numbers predate cod-q42s; the files move to internal/command)

- `cmd/codelens/e2e_authors_test.go:15`: `var update = flag.Bool("update", false,
  "regenerate cmd/codelens golden files")`.
- `:20`: `const authorsFixture = "testdata/authors.log"`.
- `:25-29`: `type e2eCase struct { name string; args []string; golden string }`.
- `:34-42`: `authorsCases`, seven scenarios (json, ndjson, csv, table, fields,
  rows2, schema), all success.
- `:48-85`: `TestE2E_Authors`. Asserts exit 0 and empty stderr INLINE (`:59-64`),
  then compares stdout to `testdata/<golden>` (`:66-82`). One artifact
  goldened.
- `:92-127`: `TestE2E_Authors_JSONReviewed`, a semantic guard on the JSON golden
  independent of the byte comparison. KEEP THIS. Its purpose is to catch an
  unreviewed `-update` that changed analysis semantics consistently across every
  golden; a triple harness does not replace it.
- `cmd/codelens/testdata/`: nine files (`authors.log`, `authors.json`,
  `authors.ndjson`, `authors.csv`, `authors.table`, `authors.fields.json`,
  `authors.rows2.json`, `authors.schema.json`, `README.md`).

Error scenarios currently asserted field by field, with no golden:

- `cmd/codelens/usage_error_test.go`: helper `runUsageError` at `:13-41` (asserts
  exit 64, empty stdout, decoded envelope, non-empty hint), then four tests:
  `TestUsage_UnknownFlag` (`:43-48`, `authors --nope` -> `unknown_flag`),
  `TestUsage_UnknownSubcommand` (`:50-55`, `frobnicate` -> `unknown_command`),
  `TestUsage_InvalidIntFlag` (`:57-62`, `authors --rows abc` ->
  `invalid_value`), `TestUsage_MessagesMissingExpression` (`:64-69`, `messages`
  -> `missing_required_flag`).
- `cmd/codelens/error_format_test.go:15-49`:
  `TestError_FormatText_StillJSONEnvelope`, three sub-cases (`text`, `table`,
  `json`) forcing a data error (exit 65) on empty stdin, asserting the envelope
  is JSON regardless of `--format`.
- `cmd/codelens/pipeline_e2e_test.go:219-220`: an I/O error envelope assertion
  (the code there is the renamed one from cod-uavr).
- `cmd/codelens/schemacodes_test.go:34-53`: `exitScenarios()`, which already
  enumerates one observation per declared exit code (0, 64, 65, 74 through
  `run()`; 70 through `output.ExitCodeFor` on an uncoded error, since 70 is
  unreachable from well-formed input).
- `cmd/codelens/e2e_coupling_warn_test.go`: a warning-channel scenario, so a
  case where stderr is NON-EMPTY on a SUCCESSFUL run. This is the scenario that
  most needs stderr goldened, and the one the current stdout-only harness cannot
  express.

## Design

## Harness shape

Extend the existing case struct to a triple. One golden file per artifact keeps
diffs readable and lets a reviewer see at a glance which stream changed:

```go
// goldenCase is one in-process scenario: Run is invoked with args (without the
// program name) and stdin, and all three artifacts are compared against
// golden files: testdata/<name>.out, testdata/<name>.err, and
// testdata/<name>.exit.
type goldenCase struct {
    name  string   // also the golden basename
    args  []string // without the program name
    stdin string   // or a fixture path; see below
}
```

Alternatives are acceptable (a single golden file holding all three artifacts
with delimiters, or a `.txtar` archive). Choose one and document the choice in
`testdata/README.md`. Prefer three files: `git diff` on a stderr-only change
then touches one file, which is the reviewability ADR 0007 is after ("change the
code, run with `-update`, review the golden diff, and the diff is the release
note's evidence").

An empty stderr is goldened as an EMPTY FILE, not an absent one. An absent
golden must fail with the "run `go test -update`" message the current harness
already gives at `e2e_authors_test.go:78`; silently treating absence as empty
would let a scenario pass while asserting nothing.

Keep the `-update` flag. It is declared once per test binary
(`e2e_authors_test.go:15`); with the tests now consolidated in
`internal/command`, make sure it is declared exactly once across the package or
the binary will panic on duplicate flag registration.

## Scenarios to cover

Convert the seven existing `authorsCases` to the triple, keeping their golden
content for stdout byte-identical (their `.err` goldens are empty files and
their `.exit` goldens are `0`). Then ADD, as goldened scenarios:

1. The four usage errors from `usage_error_test.go`: `authors --nope`,
   `frobnicate`, `authors --rows abc`, `messages`. Each pins exit 64 and the
   exact stderr envelope bytes, including the code and hint.
2. The three `--format` sub-cases from `error_format_test.go`: a forced data
   error (empty stdin) under `text`, `table`, and `json`, pinning exit 65 and an
   identical JSON envelope in all three, which is the always-JSON-errors
   decision that test exists to defend.
3. The renamed-code scenarios from cod-uavr, one per code, so every renamed code
   has a golden envelope: `unknown_format` (`--format bogus authors`),
   `unknown_schema_command` (`schema --command bogus`), `invalid_after_date`
   (`print-log-command --after notadate`), `log_open_failed`
   (`--log <MISSING> authors`), `input_file_open_failed`
   (`--group <MISSING> authors`), `invalid_control_char` (a log fixture
   containing a NUL byte).
4. The warning scenario from `e2e_coupling_warn_test.go`: exit 0, non-empty
   stdout, and a NON-EMPTY stderr carrying the warning envelope. This is the
   case the current harness structurally cannot cover.
5. `schema` with no `--command` (the command list, including the `errors`
   inventory added in cod-2x18), so the inventory has golden coverage.

Existing field-by-field tests may be deleted where a golden fully subsumes them,
but KEEP any test asserting MEANING rather than bytes:
`TestE2E_Authors_JSONReviewed` (`e2e_authors_test.go:92-127`) stays, and the
`error_format_test.go` assertion that stderr is valid JSON (rather than merely
matching a golden) is worth keeping as a decoded check, since a golden of
malformed JSON would still match itself.

## Normalization

ADR 0007: "working-directory paths, ephemeral ports, and similar volatile values
are replaced with stable tokens before comparison. Normalization discipline is
load-bearing: an unnormalized volatile value makes goldens flaky, which is the
harness's main failure mode."

The concrete volatile values in codelens:

- `t.TempDir()` paths, which appear in the `details` of `log_open_failed` and
  `input_file_open_failed` envelopes (and in the `message`, since the wrapped
  `os.Open` error carries the path). See `schemacodes_test.go:46-47` for an
  existing scenario that builds such a path. Normalize to a stable token such as
  `<TMPDIR>`.
- Any absolute repo path if one leaks into an envelope.
- `version.Current()` if a scenario captures `--version` output: it varies with
  the build (`internal/version/version.go` returns the ldflags override, the
  module version, or `dev`). Either normalize it or do not golden that scenario;
  `cmd/codelens/main_test.go:28-41` already asserts it exactly against
  `version.Current()`, which is the better test for that one case. Leave it as
  is.

Normalize by applying an ordered list of (pattern, token) replacements to both
artifacts before comparison AND before writing under `-update`, so goldens never
contain a volatile value in the first place.

## Conformance obligations through the harness

ADR 0007 wants the conformance guards expressed through the harness rather than
alongside it. Rework `schemacodes_test.go`'s `TestExitCodes_DeclaredCodesAreExercised`
(`:68-100`) so:

- every code in `internal/command.ExitCodes` (declared in cod-q42s) is exercised
  by at least one goldened scenario, read from the scenario table rather than
  from a parallel `exitScenarios()` list;
- every exit observed by any scenario is a member of `ExitCodes`;
- exit 70 remains the documented exception: it is unreachable from well-formed
  CLI input, so it keeps its `output.ExitCodeFor(errors.New(...))` observation
  with the comment explaining why (already at `:30-33` and `:49-51`). Do not
  invent an artificial scenario to reach it.

If the scenario table becomes the single source for both the goldens and the
exit-code conformance guard, say so in a comment: that is the property that
stops the two from drifting.

## testdata/README.md

The existing `cmd/codelens/testdata/README.md` documents the fixture origin and
the expected result. Extend it: the golden naming scheme, the three artifacts
per scenario, the `-update` workflow, the normalization tokens and what each
stands for, and the rule that a regenerated golden must be reviewed by hand
before the owner commits it.

## Documentation

- Append a `docs/specs/learnings.md` entry describing the harness shape,
  the normalization tokens, and the `-update` workflow.
- No `CHANGELOG.md` entry: this ticket changes no user-visible behavior. If the
  CHANGELOG has an `Internal` heading, a one-line note is welcome.
- Markdownlint every markdown file touched:
  `markdownlint-cli2 --config .markdownlint.yaml --fix <FILE>`.

## Out of scope

- No exec-based end-to-end suite. ADR 0007 makes it conditional on
  process-level behavior that in-process tests cannot reach (signals and 130/143,
  subprocess lifecycles, child exit-status passthrough). codelens catches no
  signals and spawns no subprocesses, so it has none of that. Record this
  reasoning in a comment so the omission reads as a decision.
- No change to any command's behavior, output, or exit codes. If a golden
  surprises you, the bug is in the code or in an earlier ticket; investigate,
  do not paper over it with `-update`.
- Do not delete `TestE2E_Authors_JSONReviewed`.

## Acceptance Criteria

- Every scenario in the harness compares three artifacts against golden files:
  stdout, stderr, and the exit code. An empty stderr is an empty golden file,
  and a MISSING golden fails with a message naming the `-update` flag.
- `go test ./internal/command/ -update` regenerates all three artifacts for
  every scenario, and running the suite immediately afterwards passes.
- Goldened scenarios cover: the seven existing success cases with byte-identical
  stdout; the four usage errors (`unknown_flag`, `unknown_command`,
  `invalid_value`, `missing_required_flag`); the three `--format` data-error
  cases; one scenario per code renamed in cod-uavr (`unknown_format`,
  `unknown_schema_command`, `invalid_after_date`, `log_open_failed`,
  `input_file_open_failed`, `invalid_control_char`); the coupling warning case
  with non-empty stderr on a zero exit; and `schema` with no `--command`.
- No golden contains a temp-dir path, an absolute repo path, or any other
  volatile value. Verified by grepping the goldens for `/tmp`, `/Users`,
  `/private`, and the repo path.
- Normalization is applied both before comparison and before writing under
  `-update`, so a volatile value cannot enter a golden.
- The exit-code conformance guard reads the scenario table, asserts every code
  in `internal/command.ExitCodes` is exercised by a scenario and every observed
  exit is a member, and keeps exit 70's documented `output.ExitCodeFor`
  exception with its reasoning.
- `TestE2E_Authors_JSONReviewed` survives and passes.
- The `-update` flag is registered exactly once in the package.
- `testdata/README.md` documents the naming scheme, the three artifacts, the
  `-update` workflow, the normalization tokens, and the review-by-hand rule.
- A comment records why there is no exec-based end-to-end suite (no signal
  handling, no subprocesses).
- No production code changed: `git diff` touches only test files, `testdata/`,
  and documentation.
- `make build` passes (it runs `validate`: `fmt-check`, `vet`, `lint`, `test`)
  before the ticket closes.
- DO NOT COMMIT. The owner owns all git operations: no commits, no branches,
  no staging. Leave the work tree dirty for review.

