---
id: cod-q42s
status: closed
deps: [cod-vsi0]
links: [cod-pp0d, cod-8eis, cod-uavr, cod-2x18, cod-vsi0]
created: 2026-07-26T18:42:01Z
type: task
priority: 1
assignee: Andre Silva
tags: [codelens, adr-adoption]
---
# ADR adoption 5/6: move the delegate into internal/command behind the Deps seam

Adoption ticket 5 of 6 in the ADR standardization set (plan:
.local/planning/adr-adoption.md, decisions: .local/planning/responses.txt).
Move the CLI delegate out of `package main` into a new `internal/command`
package behind the `Deps` seam, and relocate the sanctioned usage classifier
into the framework interior.

Depends on cod-vsi0 (silent posture), which deletes code this ticket would
otherwise move. This is the largest diff in the set; it is mechanical, not
semantic, and the byte-identical goldens are the safety net.

codelens carries the standardization ADRs shifted by one, because
`docs/adr/0001-keep-churn-and-effort-separate.md` predates them. Cite the LOCAL
numbers: 0002 exit-code taxonomy, 0003 error handling, 0004 CLI structure,
0005 logging, 0006 output contract, 0007 CLI testing. CLI STRUCTURE IS 0004.

## Why

ADR 0004 (local) specifies a one-line `main`, an `internal/command` package
whose exported entry point is `Run(args []string, deps Deps) int` with no
framework type in its signature, and a framework interior confined to that
package's files. ADR 0007 further specifies that golden scenarios "invoke the
delegate (`Run(args, deps)`)". Today `run()` lives in `package main` with
positional writers, and there is no `internal/command` and no `Deps`.

ADR 0004 also places the sanctioned usage-error classifier (the one
string-matching carve-out from ADR 0003) in the framework interior. It currently
lives in `internal/output`, which is the output layer, not the interior.

Owner decision (responses.txt, Q2 and Q5): adopt the delegate; codelens builds
`internal/command` fresh, so there is no directory rename.

## Reference implementation

`~/cookiecutters/go-cookiecutter/{{cookiecutter.project_name}}/internal/command/`:
read `run.go` (the contract), `root.go` (root construction, `neutralize`),
`usage.go` (the classifier), `errors.go` (CLI-surface sentinels), and
`registry.go` (`ExitCodes` as data) before editing. Also read
`cmd/{{cookiecutter.project_name}}/main.go` for the one-line `main`.

## Verified current state (re-verify before editing)

- `cmd/codelens/main.go:33`:
  `func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int`,
  argv-style args INCLUDING the program name.
- `cmd/codelens/main.go:96`:
  `os.Exit(run(os.Args, os.Stdin, os.Stdout, os.Stderr))`.
- No `internal/command`, no `Deps`.

The framework interior currently in `package main`:

| Item | Location |
| --- | --- |
| `cli.VersionPrinter` global override in `init` | `main.go:22-26` |
| root `cli.Command` construction | `main.go:38-61` |
| `CommandNotFound` hook capturing the unknown name | `main.go:50-52` |
| `OnUsageError` assignment | `main.go:57` |
| `ExitErrHandler` no-op | `main.go:60` |
| `root.Run` invocation | `main.go:63` |
| unknown-command synthesis (inline `terr.New`) | `main.go:64-71` |
| exit boundary (`output.EmitError`, `output.ExitCodeFor`) | `main.go:81-82` |
| `onUsageError` helper | `main.go:91-93` |
| `globalFlags` | `commands.go:47-108` |
| `analysisCommands`, `perCommandFlags`, `toCLIFlag`, `actionFor` | `commands.go:114-203` |
| `pipelineConfig`, `parseDefinition`, `openLog`, `analysisOpts`, `effectiveParams` | `commands.go:211-346` |
| `truncate` | `commands.go:352-364` |
| `metaCommand`, `metaCommands`, projections, `lookupMeta`, `metaCLICommands`, `metaCommandSummaries`, `allCommandNames` | `metacommands.go` |
| `schemaAction`, `analysisNames` | `schema.go:32-61` |
| `printLogCommandAction` and helpers | `printlogcommand.go:36` onward |

Line numbers above predate cod-vsi0, which removes the `--debug` plumbing
(`main.go:35`, `:45`, `:76-79`, `commands.go:47`, `:102-106`). Re-verify.

The classifier: `usageClasses` and `classifyUsageError` at
`internal/output/errors.go:100-126`, consumed as a fallback inside
`output.EmitError` (`:74-78`) and `output.ExitCodeFor` (`:94-96`). Its tests are
`internal/output/errors_test.go:55-56`, `:114-133` (`TestExitCodeFor`), and
`TestEmitError_UsageErrorClassified` at `:135` onward.

Test churn: 45 `run(` references across eleven test files in `cmd/codelens`
(`commands_test.go` 7, `main_test.go` 7, `schema_test.go` 6,
`printlogcommand_test.go` 5, `schemacodes_test.go` 3, `pipeline_e2e_test.go` 3,
`e2e_authors_test.go` 2, `e2e_coupling_warn_test.go` 1, `error_format_test.go`
1, `usage_error_test.go` 1, plus helper definitions). Every call site passes
`"codelens"` as `args[0]`. `cmd/codelens/testdata/` holds nine golden files plus
a README.

## Design

## File split

Mirror the reference. Contract in `run.go`, framework interior everywhere else,
so replacing urfave replaces the interior files and keeps `run.go` and the
tests.

- `internal/command/run.go` (CONTRACT, no urfave import):

  ```go
  type Deps struct {
      In  io.Reader
      Out io.Writer
      Err io.Writer
  }

  func Run(args []string, deps Deps) int
  ```

  `Run` calls an interior `runRoot(ctx, args, deps)`, then owns the exit
  boundary: resolve the coded error, classify usage errors when the error is not
  already coded, `output.EmitError(deps.Err, err)`, return
  `output.ExitCodeFor(err)`. codelens catches no signals, so there is no 130/143
  override.

  A doc comment on the package must state the split: `run.go` is the
  framework-free contract, the interior is replaceable, no framework type
  appears in any exported identifier.

- `internal/command/root.go`: root `cli.Command` construction (from
  `main.go:38-61`), the `CommandNotFound` capture, the `cli.VersionPrinter`
  override (from `main.go:22-26`), and a recursive `neutralize(cmd)` that sets
  `ExitErrHandler` and `OnUsageError` on the root and every subcommand. The
  reference's `neutralize` replaces today's pattern of assigning
  `OnUsageError: onUsageError` at each construction site
  (`commands.go:124`, `metacommands.go:66`); prefer the recursive walk, and
  delete the per-site assignments so the hook is set in exactly one place. The
  existing comments at `main.go:53-60` explain WHY each hook exists (suppressing
  urfave's help-topic routing, its "Incorrect Usage" banner, and its `os.Exit`);
  carry that reasoning across, it is the load-bearing part.
- `internal/command/commands.go`, `metacommands.go`, `schema.go`,
  `printlogcommand.go`: the bodies of the current `cmd/codelens` files of the
  same name, unchanged except for package clause and any identifier that must be
  exported.
- `internal/command/errors.go`: the CLI-surface sentinels, gathered from where
  they are scattered today:
  - `ErrUnknownCommand` (`unknown_command`, 64), PROMOTED from the inline
    `terr.New` at `main.go:64-71`. It currently constructs a fresh error per
    invocation, so it is not registered and does not appear in `terr.All()`.
    As a package-level sentinel with `.WithDetails(map[string]string{"command":
    name})` applied at the call site, it registers and becomes enumerable. This
    is a real ADR 0003 improvement, not just a move.
  - the relocated `errLogOpen` (`log_open_failed`) and `errFileOpen`
    (`input_file_open_failed`) from `commands.go:25`, `:35`
  - the relocated `errUnknownSchemaCommand` (`unknown_schema_command`) from
    `schema.go:19`
  - the relocated `errBadAfter` (`invalid_after_date`) from
    `printlogcommand.go:30`

  Each keeps its doc comment explaining why it carries its exit code (ADR 0003).
- `internal/command/usage.go`: the relocated classifier plus THREE sentinels,
  not the reference's single `ErrUsage`, so codelens's finer classes survive:
  `ErrUnknownFlag` (`unknown_flag`, 64), `ErrInvalidValue` (`invalid_value`,
  64), `ErrMissingRequiredFlag` (`missing_required_flag`, 64), each with the
  hint currently in the `usageClasses` table. The marker-to-sentinel table
  keeps today's ORDER, which is significant (the comment at
  `internal/output/errors.go:103-105` says the first matching marker wins, so
  specific markers precede general ones). Keep `strings.Contains` matching, not
  the reference's `strings.HasPrefix`: codelens's current markers ("no such
  flag", "not set") match mid-message, and switching to prefix matching would
  silently reclassify errors as `internal_error`.
- `internal/command/registry.go`: `var ExitCodes = []int{0, 64, 65, 70, 74}`
  declared as data, with a comment mapping each code to its meaning per ADR
  0002, replacing the test-time union at
  `cmd/codelens/schemacodes_test.go:69-79`.

`unknown_format` STAYS in `internal/output/format.go`. It is an output-layer
failure, not a CLI-surface one; moving it would put an output concern in the
framework interior. Same for `invalid_field` in `internal/output/fields.go`.

## `main`

```go
// Command codelens mines a git log and runs evolutionary code analyses,
// emitting structured JSON by default.
package main

import (
    "os"

    "github.com/andreswebs/codelens/internal/command"
)

// main is the only place the real process environment is touched: it hands the
// arguments and streams to the delegate and exits with its code. It never
// inspects errors; the exit boundary lives in internal/command.
func main() {
    os.Exit(command.Run(os.Args[1:], command.Deps{
        In:  os.Stdin,
        Out: os.Stdout,
        Err: os.Stderr,
    }))
}
```

Note `os.Args[1:]`: `Run` takes args WITHOUT the program name, unlike today's
`run`. `runRoot` re-prepends `root.Name` before calling urfave's `Run`, as the
reference does.

## Classifier relocation and the output layer

- Delete `usageClasses` and `classifyUsageError` from
  `internal/output/errors.go:100-126`.
- `output.EmitError`'s `detailFor` (`:60-82`): drop the usage branch at
  `:74-78`, keep the `internal_error` fallback at `:80`.
- `output.ExitCodeFor` (`:86-98`): drop the usage branch at `:94-96`, so an
  uncoded error returns `exitInternal` (70). `exitUsage` at `:17` becomes unused;
  delete it if the linter flags it, and update the comment at `:13-15`
  accordingly.
- `Run` wraps before emitting, so the CLI boundary behaves identically: when
  `errors.As(err, &coded)` fails, run the classifier and replace `err` with the
  wrapped sentinel. Follow the reference's `run.go:36-46`.
- Move the classifier tests from `internal/output/errors_test.go` into
  `internal/command`, keeping the same assertions:
  `TestEmitError_UsageErrorClassified` (`:135` onward) and the usage rows of
  `TestExitCodeFor` (`:114-133`) and `TestEmitError_AlwaysJSON` (`:55-56`). The
  remaining `internal/output` tests keep the coded and `internal_error` cases,
  and `TestExitCodeFor` keeps its `{"generic", errors.New("boom"), 70}` row.

Behavior at the CLI boundary must be byte-identical: same codes, same hints,
same exit codes. `cmd/codelens/usage_error_test.go` (moving to
`internal/command`) is the proof; its four assertions (`unknown_flag`,
`unknown_command`, `invalid_value`, `missing_required_flag`) must pass unchanged.

## Test migration

Move all eleven `cmd/codelens/*_test.go` files and `cmd/codelens/testdata/` into
`internal/command`. Convert every `run(` call site:

```go
// before
code := run([]string{"codelens", "authors"}, strings.NewReader(sampleLog), &stdout, &stderr)
// after
code := Run([]string{"authors"}, Deps{In: strings.NewReader(sampleLog), Out: &stdout, Err: &stderr})
```

The `"codelens"` element DISAPPEARS. Missing that at one site produces a
confusing failure (the first real arg gets eaten as the program name), so
convert mechanically and let the suite catch stragglers.

Helpers to convert once, which then fix many sites: `runExit`
(`schemacodes_test.go:57-61`), `schemaOf` (`:203-215`), `schemaListOf`
(`:219-231`), `runUsageError` (`usage_error_test.go:13-41`). The golden
comparison in `e2e_authors_test.go:48-85` and the `-update` flag at `:15` move
as-is; `authorsFixture` at `:20` is the relative path `testdata/authors.log`,
which stays correct once `testdata/` moves with the tests.

`cmd/codelens` keeps only `main.go`. It may end up with no test file at all;
that is correct, since `main` is one line with nothing to assert.

## No-leak guard (ADR 0004)

ADR 0004: "The no-leak rule is cheap to verify (an import grep or a small lint
test) but must be kept honest; one leaked framework type in an exported
signature re-welds the tool to the framework." Add a test asserting:

1. `urfave/cli` is imported only by files under `internal/command`. Walk the
   module's packages (`go/packages`, or parse imports with `go/ast` over the
   tree) and fail on any other importer.
2. No exported identifier of `internal/command` names a urfave type. Inspect the
   package's exported declarations and fail if any signature or field type
   resolves into the urfave import path.

Point 2 is the one that matters and the harder one; if a full type-resolution
check reads as over-engineered, an `go/ast`-level check over exported function
signatures, struct fields, and method receivers in the package is sufficient and
should be documented as such in the test's comment.

## Exit-code registry

Replace the test-time union at `schemacodes_test.go:69-79` with a read of
`ExitCodes` from `registry.go`. Keep both directions of
`TestExitCodes_DeclaredCodesAreExercised`: every code in `ExitCodes` is
exercised by a scenario, and every scenario's code is a member. Also keep the
existing cross-check that every code any command DECLARES is in the registry, so
a descriptor cannot declare an exit code the registry omits.

## Knock-on to earlier tickets in this set

Both of these are expected and must be updated, not worked around:

- The whole-binary sentinel count assertion added in cod-8eis rises: promoting
  `ErrUnknownCommand` adds one, and the three classifier sentinels add three. If
  cod-8eis asserted 18, this ticket asserts 22. RECOUNT from the tree rather
  than trusting the arithmetic.
- The non-sentinel allowlist added in cod-2x18 shrinks: `unknown_flag`,
  `invalid_value`, and `missing_required_flag` become real sentinels, leaving
  `internal_error` as the only non-sentinel code. Update the allowlist and the
  test asserting it matches the classifier table.

If cod-2x18's `errors` inventory in the `schema` envelope is derived from
`terr.All()`, it GAINS four entries here. Regenerate the affected goldens with
`go test ./internal/command/ -update` and review the diff; that is a legitimate
schema change (the inventory got more complete), not a regression.

## Documentation

- `docs/adr/0004-cli-structure.md` needs no change; it already specifies the
  target. Do not edit an ADR to match the code.
- Update `CLAUDE.md`'s repository map only if it names `cmd/codelens` structure.
- `docs/cli-design.md` and `docs/specs/learnings.md`: append a learnings entry
  describing the seam and the args convention change (`os.Args[1:]`), since that
  is the kind of detail the next agent needs.
- `CHANGELOG.md`: this is internal restructuring with no user-visible change
  EXCEPT the richer `errors` inventory; record that under `Added`/`Changed` as
  applicable, and note the internal move under an `Internal` heading if the
  format allows.
- Markdownlint every markdown file touched.

## Out of scope

- Do not change any command's behavior, flags, help text, output, or exit codes.
- Do not add the golden triple (cod-pp0d).
- Do not extract the command tree into a framework-agnostic registry (ADR
  0004's "growth path"); the descriptor spine already covers analyses.
- Do not rename or restructure `internal/output`, `internal/analysis`, or any
  domain package beyond deleting the classifier from `internal/output`.

## Acceptance Criteria

- `cmd/codelens/main.go` is the one-line `main` and imports only `os` and
  `github.com/andreswebs/codelens/internal/command`. `cmd/codelens/` contains
  no other file.
- `internal/command.Run` has the signature `Run(args []string, deps Deps) int`
  and `Deps` has exactly `In io.Reader`, `Out io.Writer`, `Err io.Writer`.
  Neither names a framework type.
- `internal/command/run.go` does not import `urfave/cli`.
- `grep -rl urfave --include='*.go' .` matches only files under
  `internal/command/`.
- The no-leak test passes and fails when a urfave type is deliberately added to
  an exported signature. Demonstrate during development; do not commit the
  demonstration.
- `usageClasses`/`classifyUsageError` are gone from `internal/output/errors.go`;
  `output.ExitCodeFor` returns 70 for an uncoded error; the classifier lives in
  `internal/command/usage.go` as three sentinels with today's codes, hints, and
  marker order, still using `strings.Contains`.
- All four assertions in the relocated `usage_error_test.go` pass unchanged:
  `unknown_flag`, `unknown_command`, `invalid_value`,
  `missing_required_flag`, each exit 64 with a non-empty hint and empty stdout.
- `unknown_command` is a registered package-level sentinel, not an inline
  construction, and appears in `terr.All()`.
- All eleven test files and `testdata/` live under `internal/command`; all 45
  converted `Run(` call sites pass with assertions otherwise unchanged; no call
  site still passes a program-name element.
- `internal/command/registry.go` declares `ExitCodes = []int{0, 64, 65, 70, 74}`
  and the conformance test reads it instead of unioning descriptors.
- The sentinel-count assertion from cod-8eis is updated to the recounted value
  (expected 22), and cod-2x18's non-sentinel allowlist is reduced to
  `internal_error` alone with its classifier cross-check still passing.
- Every result-envelope golden is byte-identical
  (`git diff internal/command/testdata/` shows no change to any `authors.*`
  file). Any schema golden diff is confined to the `errors` inventory gaining
  the four newly registered sentinels, reviewed by hand.
- `make build` passes (it runs `validate`: `fmt-check`, `vet`, `lint`, `test`)
  before the ticket closes.
- DO NOT COMMIT. The owner owns all git operations: no commits, no branches,
  no staging. Leave the work tree dirty for review.


## Notes

**2026-07-26T19:34:41Z**

Moved the CLI delegate from package main into internal/command behind the Deps seam (ADR 0004). main is now one line handing os.Args[1:] + streams to command.Run(args []string, deps command.Deps) int; Deps={In,Out,Err} names no framework type. run.go is the framework-free contract; root/commands/metacommands/schema/printlogcommand are the interior; usage.go holds the relocated usage-error classifier (still strings.Contains, original marker order, now returns a coded error preserving the raw framework message so envelopes stay byte-identical); errors.go gathers the CLI-surface sentinels; registry.go declares ExitCodes as data. Promoted unknown_command to a registered sentinel and added 3 classifier sentinels (unknown_flag/invalid_value/missing_required_flag): whole-binary sentinel count 18->22, non-sentinel allowlist reduced to internal_error alone. output.ExitCodeFor now returns 70 for an uncoded error; the classifier is gone from internal/output. Two no-leak tests enforce ADR 0004 (import walk + ast check over exported signatures); demonstrated the ast check fails on a deliberate leak then reverted. All 13 test files + testdata moved to internal/command; 45 run() sites converted to Run(...,Deps{}) dropping the program-name arg. reverse-direction registry guard exempts pre-dispatch unknown_command (reported only via the schema errors inventory). Result-envelope goldens byte-identical (TestE2E_Authors green). make build green. NOTE: working tree was already dirty at session start (prior closed tickets' uncommitted changes incl. a regenerated authors.schema.json with common_error_codes); left those untouched. DID NOT COMMIT.
