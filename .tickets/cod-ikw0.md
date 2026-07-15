---
id: cod-ikw0
status: closed
deps: []
links: []
created: 2026-07-14T21:18:36Z
type: task
priority: 1
assignee: Andre Silva
tags: [codelens, architecture, deepening]
---
# Deepen result envelope: rows-only Run + output.NewResult, wire Params

Architecture-review candidate 1 (top recommendation). Narrow `analysis.Descriptor.Run`
to return `(any, error)`; concentrate success-envelope construction in a new
`output.NewResult` applied by the dispatcher (`actionFor`), sourcing the analysis name
from `d.Name`; populate the currently-dead `Result.Params` from the effective flags.

Skills: /tdd (test-first, red-green-refactor), /golang (idioms, table-driven tests),
/llm-coding (surgical changes, no scope creep, verifiable success).

## Design

### Problem

The success envelope is rebuilt at 19 call sites. Every `runX` ends with the same
tail, hard-coding its own analysis name as a string literal that duplicates
`Descriptor.Name` and can drift from it:

```go
return output.Result{
    SchemaVersion: output.SchemaVersion,
    OK:            true,
    Analysis:      "absolute-churn", // duplicates Descriptor.Name
    RowCount:      len(rows),
    Rows:          rows,
}, nil
```

Separately, `Result.Params` ("echoes the effective tuning options so a result is
self-documenting", `output/types.go`) is declared but **never populated** anywhere in
production. `actionFor` is the only place that knows both `d` and the effective flags,
so it is the right owner.

### Target shape

- `Descriptor.Run` returns the analysis's rows only; the dispatcher wraps them.
- Envelope invariants (`SchemaVersion`, `OK`, `Analysis`, `RowCount`) live in one
  constructor in the `output` package, mirroring the already-centralized error path
  (`output.EmitError`).
- `Params` is enriched by the cmd layer (it is flag-value policy, not an envelope
  invariant).

Architectural payoff: package `analysis` stops importing `output` for the envelope
type entirely (run funcs return typed slices as `any`), so analyses produce rows and
the output layer owns the envelope. Locality: envelope knowledge concentrates; the
name literal disappears; `Params` becomes real.

### 1. `internal/analysis/analysis.go`

Change the `Run` field type:

```go
// Run executes the analysis over the parsed modifications and effective options,
// returning the analysis's rows (a slice; the output layer wraps them in the
// result envelope).
Run func(mods []model.Modification, opts Opts) (any, error)
```

- Update the `Descriptor.Run` doc and the package doc (currently says Run "returns a
  fully built output.Result").
- Remove the now-unused `internal/output` import if nothing else in the file uses it.
  (`Column`/`Flag`/`Opts` are local to this package.)

### 2. `internal/output` — new `NewResult` constructor

Add (new file `newresult.go`, or in `types.go`):

```go
// NewResult wraps an analysis's rows in a success envelope, setting the invariants
// every result shares: the current schema version, ok=true, the analysis name, and
// the row count derived from rows. rows must be a slice (nil or non-slice yields a
// zero RowCount).
func NewResult(analysis string, rows any) Result {
    return Result{
        SchemaVersion: SchemaVersion,
        OK:            true,
        Analysis:      analysis,
        RowCount:      rowLen(rows),
        Rows:          rows,
    }
}
```

- Introduce an unexported `rowLen(any) int` (reflect over a slice; guard `Kind() ==
  reflect.Slice`, else 0) and **reuse it in `truncate`** (`cmd/codelens/commands.go`
  already reflects for the same purpose) to keep a single reflection site.
  Move it to the output package so both callers share it, or keep truncate's local and
  add one here — pick the one-place option (llm-coding: no duplicated reflection).

### 3. `cmd/codelens` — effectiveParams + actionFor

`effectiveParams(cmd *cli.Command, d analysis.Descriptor) map[string]any`:

- Iterate `d.Flags`; for each, read the effective value by `f.Type`
  (`cmd.Int`/`cmd.Bool`/`cmd.String`) keyed by `f.Name` (kebab, e.g. `min-revs`).
- Include **every declared flag with its effective value** (default or overridden) so
  the result documents the thresholds actually applied.
- Return `nil` when `len(d.Flags) == 0`, so flagless analyses keep `Params` nil (and
  `omitempty` omits it — output stays byte-identical for them).
- Mirror the declared-flag pattern already in `analysisOpts`.

`actionFor` (`commands.go`): build the envelope after Run.

```go
opts := analysisOpts(cmd, d)
rows, err := d.Run(mods, opts)
if err != nil {
    return err
}
res := output.NewResult(d.Name, rows)
res.Params = effectiveParams(cmd, d)
truncate(&res, cmd.Int("rows"))
return output.Emit(cmd.Root().Writer, cmd.String("format"), res, columnNames(d), cmd.String("fields"))
```

(Compute `opts` once; today `analysisOpts(cmd, d)` is called inline in the Run call.)

### 4. The 19 analyses (`internal/analysis/*.go`)

Mechanical, per file. E.g. `abschurn.go`:

- Signature: `func runAbsChurn(mods []model.Modification, _ Opts) (output.Result, error)`
  -> `(any, error)`.
- Guard early-returns: `return output.Result{}, err` -> `return nil, err`.
- Tail: replace the `output.Result{...}` literal with `return rows, nil`.
- Remove the now-unused `internal/output` import.

Files: abschurn, authorchurn, entitychurn, ownership, maindev, refactoringmaindev,
entityeffort, soc, coupling, revisions, fragmentation, communication, codeage,
maindevbyrevs, authors, messages, summary (+ any remaining run funcs; `make build`
flags leftover imports). `Descriptor` metadata (Name/RowSchema/Flags/ExitCodes) is
unchanged.

### Parity / output bytes

- Flagless analyses: `Params` nil -> omitted -> **byte-identical** output.
- Flagged analyses only (**coupling, sum-of-coupling, code-age, messages**) gain a
  `"params": {...}` object. Update any exact-output assertion for those; struct-decode
  tests are unaffected (Go ignores unknown JSON fields).

### TDD plan (/tdd)

1. `output` NewResult test (new): asserts `schema_version == SchemaVersion`, `ok`,
   `analysis` echoed, `row_count == len(rows)` (incl. empty slice -> 0, nil -> 0),
   `rows` passthrough. Add `rowLen` edge cases (nil, non-slice -> 0).
2. Per-analysis test migration: helpers such as `absChurnRows(t, res output.Result)
   []absChurnRow` change to take the rows value (`rows any`) and assert only the row
   type + contents; drop the `OK`/`Analysis`/`RowCount` assertions (now covered once by
   the NewResult test). Callers change from `res := runX(...)` to `rows := runX(...)`.
   The existing `d.Run == nil` registration tests stay valid.
3. Dispatch-level test (`cmd/codelens`): run a flagged analysis (coupling) end-to-end
   and assert the decoded envelope has the right `analysis`, `row_count`, and a
   `params` object echoing effective flags (defaults when unset); assert a flagless
   analysis (authors) omits `params`. Extend `commands_test.go`.
4. `schema_test`/`e2e_authors_test`: expected green unchanged (schema derives from
   `Descriptor.Flags`; authors is flagless).

### Out of scope

- Candidate 3 (collapsing the group-sum analyses) is a separate ticket; do NOT merge
  analyses here.
- No new flags; `Params` only surfaces existing declared flags.

### Files touched

```text
internal/analysis/analysis.go            (Run signature + docs)
internal/output/newresult.go (new) / types.go   (NewResult, rowLen)
cmd/codelens/commands.go                 (actionFor, effectiveParams, truncate reuse)
internal/analysis/*.go                   (19 run funcs: return rows, drop output import)
internal/analysis/*_test.go              (row-only helpers)
internal/output/newresult_test.go (new)
cmd/codelens/commands_test.go            (params dispatch assertions)
```

## Acceptance Criteria

- `analysis.Descriptor.Run` returns `(any, error)`; no `runX` builds `output.Result`;
  the string-literal analysis name is gone from every analysis (name comes from
  `d.Name`).
- `output.NewResult` is the single builder of the success envelope's invariants; a
  single reflection helper computes row count (shared with `truncate`).
- Package `analysis` no longer imports `output` for the envelope type.
- `Params` is populated for the four flagged analyses (coupling, sum-of-coupling,
  code-age, messages) with effective flag values keyed by flag name; flagless analyses
  omit `params` and produce byte-identical output.
- Envelope invariants are asserted once (NewResult test) plus one dispatch-level
  params test; per-analysis tests assert rows only.
- `make build` green (validate + compile); all existing analysis/CLI behavior and JSON
  parity preserved except the intended `params` addition on the four flagged analyses.

## Notes

**2026-07-15T00:00:51Z**

Narrowed analysis.Descriptor.Run to (any, error): run funcs now return their row slice only. Added output.NewResult (single success-envelope builder) + exported output.RowLen (single row-count reflection site, reused by truncate). cmd/codelens actionFor now wraps rows via NewResult(d.Name, rows) and populates Result.Params from new effectiveParams(cmd, d), which echoes every declared flag at its effective value (defaults included). Flagless analyses return nil params -> omitempty -> byte-identical output; the four flagged analyses (coupling, sum-of-coupling, code-age, messages) now carry a params object. Envelope invariants asserted once in output/newresult_test.go; per-analysis test helpers assert rows only; added dispatch-level params tests (present for coupling, absent for authors) in cmd/codelens/commands_test.go. Package analysis no longer imports output in any run func (schema.go still uses output.SchemaVersion for the schema envelope, unrelated to the Result type). make build green.
