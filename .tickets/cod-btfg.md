---
id: cod-btfg
status: open
deps: [cod-lrte]
links: [cod-12yk]
created: 2026-07-15T03:40:57Z
type: bug
priority: 2
assignee: Andre Silva
tags: [codelens, cli, bug, friction]
---
# Bug: grouped coupling silently returns zero rows at default thresholds

`codelens coupling` can return `row_count: 0` while there is obvious coupling in
the data, because every candidate pair fell below `--min-coupling`. The empty
result is technically valid (exit 0), but it silently reads as "no coupling" when
it is really a threshold mismatch. Emit a JSON diagnostic (on the shared stderr
warning channel) that explains it and reports the highest coupling actually
observed.

## Reproduction

Rolling the log up to architectural components with `--group` dilutes per-pair
degrees, so nothing clears the default `--min-coupling 30`:

```sh
codelens coupling --group groups.txt --log git.log --format json
# -> {"schema_version":1,"ok":true,"analysis":"coupling","row_count":0,"rows":[]}
```

In the test-drive the highest observed component coupling was 27% (Admin Server
<-> Shared Kernel), so the default 30 threshold filtered everything. Lowering
`--min-coupling 5` produced 19 pairs. Nothing told the operator why the first run
was empty.

Note this is not grouping-specific: any `--min-coupling` set above the highest
actual degree produces the same silent-empty result. The fix is general to
`coupling`.

## Depends on `cod-lrte`

This ticket emits its advisory through the structured JSON warning channel added
in `cod-lrte` (`output.EmitWarning` + the `analysis.Opts` warning sink). Implement
`cod-lrte` first. This ticket is a dep of `cod-lrte`'s facility (wire the
dependency in `tk`).

## Decision

When `coupling` computes at least one candidate pair but **every** pair is
filtered out by the thresholds, emit one JSON warning to stderr naming the
highest observed coupling degree and hinting to lower `--min-coupling`. Keep exit
0 and the empty-but-valid `rows: []` envelope on stdout unchanged (do not
fabricate rows, do not change the contract). Chosen over auto-scaling the
threshold (makes results depend invisibly on `--group`) and docs-only (leaves the
silent trap).

## Implementation

In `runCoupling` (`src/internal/analysis/coupling.go`):

- The function already iterates `couplingalgo.Couplings(mods, ...)` and skips
  pairs failing `couplingalgo.WithinThreshold`. Track two things while iterating:
  the number of candidate pairs seen (`len(pairs)`) and the **maximum `degree`**
  observed across all candidates (before the threshold filter).
- After building `rows`: if `len(rows) == 0 && len(pairs) > 0`, raise a warning via
  the `Opts` sink:

  ```go
  opts.warn(
      "coupling_all_filtered",
      "0 pairs met the coupling thresholds",
      fmt.Sprintf("highest observed coupling was %d%%; lower --min-coupling (currently %d) to see weaker links", maxDegree, opts.MinCoupling),
      map[string]any{"max_degree": maxDegree, "min_coupling": opts.MinCoupling, "candidate_pairs": len(pairs)},
  )
  ```

  Use the nil-safe `Opts.warn` helper from `cod-lrte` so the call is a no-op when
  no sink is set (keeps unit tests and library use clean).
- `maxDegree` is `calc.TruncInt(degree)` maximized over candidates, matching the
  integer degree reported in rows. Only warn when there were candidate pairs
  (`len(pairs) > 0`); an empty log / no pairs is a different (existing) condition
  and should not produce this warning.

Add `coupling_all_filtered` to the coupling descriptor... — note diagnostics are
warnings, not errors, so they do **not** belong in `ErrorCodes`/`ExitCodes` (those
are for exit-affecting outcomes). Do not add it there; document the warning code in
the coupling card instead.

## Guidance (rides with this ticket)

Update `docs/skills/codelens/references/catalog.md` (Change-coupling graph card)
and the `--group` description in
`docs/skills/codelens/references/operating.md`: when grouping to components,
degrees are diluted, so lower `--min-coupling` (around 5) or the result may be
empty; codelens now warns on stderr with the highest observed degree.

## TDD plan (/tdd)

Drive `runCoupling` directly with a captured warning sink (a test `WarnFunc` that
records calls), plus a CLI-level check that the warning reaches stderr as JSON.

1. `TestCoupling_WarnsWhenAllFiltered`: build `mods` with a real but weak coupling
   (a pair whose degree is, say, 20%), run with `MinCoupling: 30` and a recording
   sink -> `rows` empty AND exactly one warning with code
   `coupling_all_filtered`, `details.max_degree == 20`, `details.min_coupling ==
   30`. (Tracer bullet + core behavior.)
2. `TestCoupling_NoWarnWhenRowsPresent`: same data with `MinCoupling: 10` -> rows
   non-empty, zero warnings.
3. `TestCoupling_NoWarnWhenNoPairs`: mods with no co-changing pairs -> rows empty,
   zero warnings (distinguish "nothing coupled" from "everything filtered").
4. `TestCoupling_NilSinkSafe`: `Opts.Warn == nil` and all-filtered -> no panic,
   rows empty (library-use safety).
5. CLI e2e: `coupling --group ... --min-coupling 30` on a fixture log with only
   weak component coupling -> stdout is the empty-valid envelope (exit 0) and
   stderr contains a single JSON line with `level:"warning"`,
   `code:"coupling_all_filtered"`.

One test -> one step; do not pre-write all five.

## Files touched

```text
src/internal/analysis/coupling.go        track max degree; warn when all filtered
src/internal/analysis/coupling_test.go   the four unit cases (recording sink)
src/cmd/codelens/*_test.go               CLI e2e: warning JSON on stderr, exit 0
docs/skills/codelens/references/catalog.md     coupling card: warning + lower --min-coupling when grouping
docs/skills/codelens/references/operating.md   --group note: dilution + threshold
```

## Acceptance criteria

- When `coupling` finds candidate pairs but all are filtered by the thresholds, it
  emits exactly one JSON warning on stderr (via `cod-lrte`'s channel) with a code,
  a message, a hint to lower `--min-coupling`, and `details` carrying the highest
  observed degree and the effective `min-coupling`.
- The stdout envelope is unchanged (`ok:true`, `row_count:0`, `rows:[]`, exit 0);
  no rows are fabricated.
- No warning is emitted when rows are present, or when there were no candidate
  pairs at all.
- A nil warning sink is safe (library use / tests).
- The coupling catalog card and `--group` operating note explain the dilution and
  the warning; Markdown passes markdownlint per project standard.
- `make build` green.

## References

- `src/internal/analysis/coupling.go` (`runCoupling`, `couplingDescriptor`)
- `src/internal/analysis/couplingalgo` (pair degrees, `WithinThreshold`)
- `src/internal/analysis/calc` (`TruncInt`, `Percentage`)
- Depends on: `cod-lrte` (JSON warning channel + `Opts` sink)
- `docs/skills/codelens/references/catalog.md`, `references/operating.md`
- Skills: `/golang`, `/tdd`, `/llm-coding`
