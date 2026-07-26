---
id: cod-8eis
status: closed
deps: [cod-uavr]
links: [cod-pp0d, cod-q42s, cod-uavr, cod-2x18, cod-vsi0]
created: 2026-07-26T18:37:18Z
type: task
priority: 1
assignee: Andre Silva
tags: [codelens, adr-adoption]
---
# ADR adoption 2/6: port the terr registry half (E, registering New, Newf, All, Is)

Adoption ticket 2 of 6 in the ADR standardization set (plan:
.local/planning/adr-adoption.md, decisions: .local/planning/responses.txt).
Port the registry half of `internal/terr` so it matches the reference
implementation exactly, and clean up the pre-migration test fixtures that block
it.

Depends on cod-uavr (unique error codes). A registering `New` that panics on a
duplicate code CANNOT land before that ticket: `cmd/codelens` links three
duplicate code groups today and would panic at init.

codelens carries the standardization ADRs shifted by one, because
`docs/adr/0001-keep-churn-and-effort-separate.md` predates them. Cite the LOCAL
numbers: 0002 exit-code taxonomy, 0003 error handling, 0004 CLI structure,
0005 logging, 0006 output contract, 0007 CLI testing.

## Why

ADR 0003: "Where the tool exposes runtime self-description (a `schema`
command), error codes are enumerable as data, populated at sentinel
construction, so the documented error surface cannot drift from the real one."
codelens has the details half of terr but no registry, so `terr.All()` does not
exist and nothing can enumerate the real error surface. Owner decision
(responses.txt, Q4): full conformance to the reference terr surface, including
the type name `E`, a registering `New` that panics on any duplicate code, and
`Is` matching by code. Do NOT adopt the origin-matching variant that the draft
plan floated.

## Reference implementation

`~/cookiecutters/go-cookiecutter/{{cookiecutter.project_name}}/internal/terr/terr.go`
is the target shape verbatim (modulo the import path). Read it before editing.

## Verified current state (re-verify before editing)

`internal/terr/terr.go` is the details half only:

| Identifier | Line | Status |
| --- | --- | --- |
| `Coded` interface | 8 | matches reference |
| `Detailed` interface | 17 | matches reference |
| `Error` struct | 23 | reference names it `E` |
| `New(code, exit, hint, msg)` | 38 | does not register |
| `Error()` | 44 | matches |
| `Wrap(err)` | 53 | copy, matches |
| `Unwrap()` | 60 | matches |
| `Code()` | 63 | matches |
| `ExitCode()` | 66 | matches |
| `Hint()` | 69 | matches |
| `ErrorDetails()` | 72 | matches |
| `WithDetails(details)` | 77 | copy, matches |

Missing: the package-level registry, a registering `New`, `Newf`, `All()`, and
a custom `Is`.

The only reference to the concrete type name outside the package is
`internal/gitlog/parse_test.go:187`:
`func assertCoded(t *testing.T, err error, want *terr.Error, wantExit int)`.
Every sentinel declaration uses an inferred type (`var ErrX = terr.New(...)`),
so the rename touches three files total.

Consequence of the missing `Is`, verified by reading the code: because `Wrap`
and `WithDetails` return a new pointer and `Unwrap` yields the cause rather
than the origin, `errors.Is(ErrInvalidGroup.WithDetails(...), ErrInvalidGroup)`
is FALSE today. Nothing depends on that comparison, so adding `Is` is additive
and closes a latent trap. The two existing sentinel `errors.Is` assertions are
`internal/analysis/*_test.go` against `churn.ErrMissingMetrics` (returned
uncopied) and `internal/gitlog/tokenize_test.go:183` (a plain error).

## Design

## Target shape

Mirror the reference exactly:

```go
// E is the concrete coded error. Declare sentinels with New, attach a cause
// with Wrap, and attach structured payloads with WithDetails.
type E struct {
    code, msg, hint string
    exit            int
    cause           error
    details         any
}

var (
    _ Coded    = (*E)(nil)
    _ Detailed = (*E)(nil)
)

var registry []*E

// New creates a sentinel E and registers it for enumeration via All. It
// panics when code is already registered: duplicate registration is an
// init-time programmer error, and crashing at startup is the correct outcome.
func New(code string, exit int, hint, msg string) *E

// Newf creates an E without registering it, formatting the message from
// format and args. Use it for one-off per-invocation errors whose class does
// not belong in the enumerable inventory.
func Newf(code string, exit int, hint, format string, args ...any) *E

// All returns a copy of every error registered via New, in registration
// order. It backs the schema command's error inventory and the exit-code
// conformance test.
func All() []*E

// Is reports whether target is an E with the same code, so copies produced by
// Wrap and WithDetails still match their sentinel under errors.Is.
func (e *E) Is(target error) bool
```

The panic message follows the reference: `terr: duplicate error code %q`.
Duplicate detection is a linear scan over `registry`; the sentinel count is
under thirty, so no map is warranted.

The existing internal field name `wrapped` may either stay or be renamed to the
reference's `cause`; it is unexported and invisible outside the package. Prefer
aligning with the reference so future diffs against it stay small.

`Is` matching on code is safe only because cod-uavr made codes unique. If that
ticket is not closed, STOP: this one will panic at init.

## Rename

- `internal/terr/terr.go`: `Error` -> `E` throughout, including the
  `_ Coded = (*Error)(nil)` / `_ Detailed = (*Error)(nil)` assertions at
  `:33-34` and every method receiver.
- `internal/gitlog/parse_test.go:187`: `want *terr.Error` -> `want *terr.E`.
- `internal/terr/terr_test.go`: any explicit type reference.

Note the method named `Error()` (the `error` interface implementation) is
unrelated to the type name and stays.

## Fixture cleanup (rider)

These fixtures use exit code 3, a value from before the exit-code migration
(production sentinels use 0/64/65/70/74 only). More importantly, under a
registering `New` they register DUPLICATE codes inside a single test binary,
which would panic, so they must move to `terr.Newf` regardless of the exit
value:

- `internal/terr/terr_test.go`: `:13` (`"c"`, 3), `:17-18` (asserts exit 3),
  `:30` (`"c"`, 3), `:37` (`"parse_error"`, 3), `:54` (`"c"`, 3), `:59`
  (`"parse_error"`, 3), `:67` (`"parse_error"`, 3). Three `"c"` registrations
  and four `"parse_error"` registrations in one binary.
- `internal/output/errors_test.go`: `:15`, `:55`, `:92` all
  `terr.New("parse_error", 3, ...)`. Three registrations of one code in one
  binary. Note `:38-39` and `:55` also ASSERT the code `parse_error`; keep the
  assertions consistent with whatever the fixture uses.
- `internal/output/errors_test.go:121` uses the code `"data_error"`, which no
  production sentinel carries. Change it to a real code.

Convert all of these to `terr.Newf` with production exit codes (65 for the
`parse_error`-shaped fixtures, 64 or 70 for the abstract `"c"` fixtures) and
update the exit-code assertions at `terr_test.go:17-18` to match. `Newf` does
not register, so duplicates are fine and the test binaries stay panic-free.

Where a test genuinely needs a REGISTERED sentinel (the new `All()` and
duplicate-panic tests), declare it with `New` and a code unique within that
test binary, for example `terr_test_alpha`.

## New unit tests (internal/terr)

1. `All()` returns registered sentinels in registration order, and the returned
   slice is a copy (mutating it does not affect a later `All()`).
2. `Newf` does not register: an `All()` length snapshot is unchanged after a
   `Newf` call, and `Newf`'s formatting works (`%q`/`%d` args land in the
   message).
3. `errors.Is(sentinel.Wrap(inner), sentinel)` is true.
4. `errors.Is(sentinel.WithDetails(x), sentinel)` is true.
5. `errors.Is(sentinel.Wrap(inner).WithDetails(x), sentinel)` is true (chained
   copies).
6. `New` panics on a duplicate code. Use a `defer func() { recover() }()`
   helper; register a fresh unique code first, then re-register it.
7. Copy semantics still hold: `Wrap` and `WithDetails` do not mutate the
   receiver (the existing `:79-81` assertion covers `WithDetails`; add the
   `Wrap` case).

## Whole-binary registry guard

Add a test in `cmd/codelens` (package `main`) that links the entire binary and
asserts the registry is coherent:

- `terr.All()` returns exactly 18 sentinels. That is the production count after
  cod-uavr: 3 in `internal/gitlog/errors.go`, 1 in `internal/output/fields.go`,
  1 in `internal/output/format.go`, 2 in `internal/analysis/messages.go`, 1 in
  `internal/analysis/codeage.go`, 1 in `internal/analysis/churn/churn.go`, 2 in
  `internal/transform/temporal/temporal.go`, 1 in
  `internal/transform/group/group.go`, 1 in
  `internal/transform/teammap/teammap.go`, 1 in
  `internal/transform/filter/filter.go`, 2 in `cmd/codelens/commands.go`, 1 in
  `cmd/codelens/schema.go`, 1 in `cmd/codelens/printlogcommand.go`. RECOUNT
  from the tree rather than trusting this number; assert the count you verify,
  and write the derivation in a comment so a future reader can re-derive it.
  (Later tickets raise this count: cod-q42s promotes the inline
  `unknown_command` at `cmd/codelens/main.go:65` to a sentinel and adds three
  classifier sentinels.)
- No two entries share a code.
- Every entry's `ExitCode()` is a member of {0, 64, 65, 70, 74}, the codes ADR
  0002 lets codelens produce. Declare that set locally in the test; the shared
  `ExitCodes` data declaration arrives in cod-q42s.
- Every entry's `Code()` is non-empty and `snake_case` (ADR 0003 requires a
  `snake_case` string), checked with a small regexp.

## Stale wording (rider)

Both call ADR 0002 "the family-wide taxonomy". The local ADR is titled
"Exit code taxonomy" and carries no family-wide framing:

- `internal/output/errors.go:14`: "resolved here from the family-wide taxonomy
  (ADR 0002)" -> "resolved here from the exit-code taxonomy (ADR 0002)".
- `README.md:250`: "Exit codes follow the family-wide taxonomy in ADR 0002"
  -> "Exit codes follow the exit-code taxonomy in ADR 0002".

Markdownlint the README edit:
`markdownlint-cli2 --config .markdownlint.yaml --fix README.md`.

## Out of scope

- No production behavior change. No error code, exit code, hint, message, or
  envelope byte changes in this ticket.
- Do not add the `schema` error inventory or `common_error_codes` (cod-2x18).
- Do not touch `--debug` (cod-vsi0) or create `internal/command` (cod-q42s).
- Do not add per-command tagging to sentinels; the registry is flat.

## Acceptance Criteria

- `internal/terr/terr.go` matches the reference surface: type `E`, a
  registering `New` that panics with `terr: duplicate error code %q`, `Newf`,
  `All()`, and `Is` matching by code. Every exported identifier carries a doc
  comment.
- `grep -rn 'terr\.Error' --include='*.go' .` returns nothing.
- The new `internal/terr` unit tests cover: registration order, `All()` copy
  semantics, `Newf` not registering, `errors.Is` through `Wrap`, through
  `WithDetails`, and through both chained, the duplicate-code panic, and `Wrap`
  copy semantics.
- The whole-binary guard in `cmd/codelens` asserts the verified sentinel count,
  no duplicate codes, every exit code in {0, 64, 65, 70, 74}, and every code
  `snake_case`. It fails if a sentinel is added without a unique code.
- No `terr.New` call in any test registers a code that another `terr.New` call
  in the same test binary registers; the exit-3 fixtures are gone and every
  fixture uses `Newf` or a binary-unique registered code.
- `internal/output/errors_test.go` no longer references the non-existent code
  `data_error`.
- No production behavior change: every other test in the suite passes
  unmodified, and every golden file under `cmd/codelens/testdata/` is
  byte-identical (`git diff --stat cmd/codelens/testdata/` is empty).
- Neither `internal/output/errors.go` nor `README.md` says "family-wide";
  README is markdownlint clean against `.markdownlint.yaml`.
- `make build` passes (it runs `validate`: `fmt-check`, `vet`, `lint`, `test`)
  before the ticket closes.
- DO NOT COMMIT. The owner owns all git operations: no commits, no branches,
  no staging. Leave the work tree dirty for review.


## Notes

**2026-07-26T19:00:57Z**

Ported terr registry half to match the reference surface: renamed Error->E, added a package-level registry, a registering New that panics with 'terr: duplicate error code %q' on dup codes, non-registering Newf, All() (returns a copy in registration order), and an Is that matches by code (so Wrap/WithDetails copies match their sentinel under errors.Is). Field wrapped->cause.

CRITICAL FINDING (not called out in the ticket's fixture list): cmd/codelens/main.go:65 calls terr.New('unknown_command',...) at RUNTIME inside run(), not as a package-level var. Multiple cmd/codelens tests invoke unknown-command paths in the same test binary (main_test.go bogus/version x3, commands_test.go authorz), so a registering New would register once then panic on the second call. Converted it to terr.Newf (a per-invocation error, exactly Newf's purpose) with a %s format arg for the command name. Zero envelope/golden change; goldens byte-identical. cod-q42s will promote it to a real sentinel later, raising All()'s count.

Whole-binary guard (cmd/codelens/registry_guard_test.go, package main): asserts terr.All()==18, re-derived from the tree and documented in a comment with the per-file breakdown; no duplicate codes; every exit in {0,64,65,70,74}; every code snake_case. Fixtures converted: terr_test.go and output/errors_test.go exit-3/duplicate-code fixtures moved to Newf with production exit codes; output/errors_test.go 'data_error' -> real code 'empty_log'. Stale 'family-wide' wording fixed in errors.go:14 and README.md:250 only (the two the ticket scoped); docs/cli-design.md and operating.md still say it but were out of scope. make build green.
