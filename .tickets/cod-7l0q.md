---
id: cod-7l0q
status: closed
deps: []
links: []
created: 2026-07-27T16:22:11Z
type: bug
priority: 2
assignee: Andre Silva
tags: [codelens, diagnostics, agent-dx]
---
# Bug: coupling_all_filtered hint blames --min-coupling when revision thresholds are the blocker

The `coupling_all_filtered` warning blames `--min-coupling` unconditionally, even when that
threshold provably did not exclude anything. An operator or agent following its hint is sent
to the wrong flag and concludes the data is unusable.

Found by running the codelens skill against this repository itself on 2026-07-27 (14 commits,
1 author, 13 days). The warning fired correctly (the empty result IS a threshold mismatch, not
an absence of coupling), but its hint read:

    highest observed coupling was 100%; lower --min-coupling (currently 30) to see weaker links

That advice cannot be right on its own terms: if the highest observed degree is 100 and the
floor is 30, then `--min-coupling` is not the clause that filtered those pairs out. The real
blockers were `--min-revs` and `--min-shared-revs`.

Threshold isolation against that log, 854 candidate pairs:

| Thresholds | Rows |
| --- | --- |
| defaults (min-revs 5, min-shared-revs 5, min-coupling 30) | 0 |
| `--min-coupling 5` | 0 |
| `--min-coupling 1` | 0 |
| `--min-shared-revs 2` | 0 |
| `--min-revs 2` | 0 |
| `--min-revs 2 --min-shared-revs 2` | 126 |

At `--min-coupling 1` the result is still empty AND the hint still says to lower
`--min-coupling`, now to below 1. The advice is not merely imprecise, it is unfollowable.

This matters more than a typical wording bug because the warning exists specifically to stop a
caller misreading an empty result, and because codelens is agent-first: an agent has no
independent judgement to fall back on and will act on `hint` literally. A young or thin-history
repository is exactly where the warning fires, and exactly where revision-count thresholds
rather than the degree floor are the binding constraint.

Skills: /golang, /tdd, /llm-coding.

## Design

### Root cause

`internal/analysis/couplingalgo/couplingalgo.go:189` `WithinThreshold` is a conjunction of
FOUR clauses:

```go
func WithinThreshold(revs, sharedRevs int, coupling float64, o Opts) bool {
    return revs >= o.MinRevs &&
        sharedRevs >= o.MinSharedRevs &&
        coupling >= float64(o.MinCoupling) &&
        math.Floor(coupling) <= float64(o.MaxCoupling)
}
```

`internal/analysis/coupling.go:124-131` raises the warning knowing only one of them:

```go
if len(rows) == 0 && len(pairs) > 0 {
    opts.warn(
        "coupling_all_filtered",
        "0 pairs met the coupling thresholds",
        fmt.Sprintf("highest observed coupling was %d%%; lower --min-coupling (currently %d) to see weaker links", maxDegree, opts.MinCoupling),
        map[string]any{"max_degree": maxDegree, "min_coupling": opts.MinCoupling, "candidate_pairs": len(pairs)},
    )
}
```

The loop at `coupling.go:79-85` already walks every candidate pair to compute `maxDegree`, so
the information needed to attribute the exclusion correctly is in hand at zero extra cost. It
is simply not collected.

### Fix

In the existing candidate-pair loop, tally per-clause failures and the best observed value for
each bounded quantity. Then report the clause(s) that actually bound.

```go
// couplingBlockers records, across every candidate pair, which threshold clause
// excluded it and the best value observed for each bounded quantity. Attributing
// the exclusion is what lets the warning name the threshold a caller must
// actually change: the clauses are a conjunction, so the highest observed degree
// says nothing about whether the degree floor was the binding one.
type couplingBlockers struct {
    MaxDegree      int // best degree seen (already computed today)
    MaxSharedRevs  int // best shared-revision count seen
    MaxAverageRevs int // best average-revision count seen
    FailedMinRevs       int // pairs excluded by min-revs
    FailedMinSharedRevs int // pairs excluded by min-shared-revs
    FailedMinCoupling   int // pairs excluded by min-coupling
    FailedMaxCoupling   int // pairs excluded by max-coupling
}
```

Count a clause as failed whenever that clause alone is false for the pair (evaluate each
independently, NOT short-circuited), so a pair failing two clauses increments both. That is the
honest picture: lowering one threshold in isolation would still not admit it, which is exactly
the trap the current hint sets.

Derive the hint from which thresholds are unmet at the observed best:

- If `MaxAverageRevs < MinRevs`, `--min-revs` is binding; report the best observed.
- If `MaxSharedRevs < MinSharedRevs`, `--min-shared-revs` is binding; same.
- If `MaxDegree < MinCoupling`, `--min-coupling` is binding; same.
- If the floored best degree exceeds `MaxCoupling`, `--max-coupling` is binding.

Name every binding threshold, not just the first, since in the observed case TWO were.
Suggested shape (exact wording is the implementer's, but it must name the binding flags and
give the observed best for each):

    no pair cleared every threshold; best observed: average revs 3 (--min-revs 5),
    shared revs 3 (--min-shared-revs 5), degree 100% (--min-coupling 30 met);
    lower --min-revs and --min-shared-revs

`details` should carry the machine-readable form, since an agent will branch on it rather than
parse the hint:

```json
{
  "candidate_pairs": 854,
  "blocking": ["min-revs", "min-shared-revs"],
  "observed": {"max_degree": 100, "max_shared_revs": 3, "max_average_revs": 3},
  "thresholds": {"min-revs": 5, "min-shared-revs": 5, "min-coupling": 30, "max-coupling": 100}
}
```

`blocking` is the important addition: it is the field an agent can act on without natural
language. Keep `max_degree` and `min_coupling` as top-level keys too if you want to avoid
breaking any consumer reading today's shape, but note nothing in this repo or the skill reads
them (`grep -rn 'max_degree\|candidate_pairs' docs/ internal/`), and the warning is unreleased,
so restructuring `details` freely is acceptable.

### Simpler alternative, if the above is judged too much

Report all four thresholds and all observed maxima with NO attribution, and drop the
prescriptive "lower X" clause entirely. Strictly less useful but strictly honest, and it cannot
mislead. Prefer the attributing version: an agent-first tool should say which flag to change,
and the data is already in the loop.

### Tests

- A fixture where ONLY `--min-coupling` binds (pairs with plenty of shared revisions but low
  degree): hint names `min-coupling`, `blocking` is `["min-coupling"]`.
- A fixture where ONLY `--min-shared-revs` binds (a 100% pair with 2 shared revisions, high
  average revs): hint does NOT mention `min-coupling`; `blocking` is `["min-shared-revs"]`.
- A fixture where TWO clauses bind: `blocking` carries both, in a stable order.
- The regression from this bug report: a 100% max degree with `min-coupling` satisfied must
  never produce a hint telling the caller to lower `--min-coupling`. Assert the negative.
- `internal/command/golden_test.go` has a `coupling_warning` scenario built by
  `weakCouplingLog(5, 12)` (`golden_test.go:141`), which is a genuine min-coupling case (degree
  29% against a floor of 30). Its `.err` golden WILL change with the new hint and details.
  Regenerate and hand-review; consider adding a second golden scenario for the
  revision-threshold case so both attributions are frozen.

### Files touched

```text
internal/analysis/coupling.go             collect per-clause blockers in the existing loop; build hint + details
internal/analysis/coupling_test.go        the four attribution cases above
internal/command/testdata/coupling_warning.err   regenerate (hint and details change)
internal/command/golden_test.go           optionally add a revision-threshold warning scenario
```

Out of scope: the threshold DEFAULTS themselves (`min-revs 5`, `min-shared-revs 5`) are
code-maat parity values and are not being changed here. Only the diagnostic is wrong.

## Acceptance Criteria

- The `coupling_all_filtered` hint names only thresholds that actually bound. With a candidate
  pair at 100% degree and `--min-coupling 30`, the hint never tells the caller to lower
  `--min-coupling`.
- The hint names EVERY binding threshold when more than one binds, and reports the best observed
  value for each bounded quantity so a caller can compute a working value.
- `details` carries a machine-readable `blocking` list of threshold flag names, so an agent can
  branch without parsing prose.
- Reproduction from the bug report is fixed: against a 14-commit single-author log where
  `--min-revs 2 --min-shared-revs 2` yields 126 rows, the default-threshold run's hint names
  `--min-revs` and `--min-shared-revs` and not `--min-coupling`.
- A pair failing two clauses is counted against both, so no hint implies that changing one
  threshold alone would admit it when it would not.
- The existing `coupling_warning` golden still covers a genuine min-coupling case and its
  regenerated `.err` was reviewed by hand.
- `make build` green.


## Notes

**2026-07-27T16:29:16Z**

Fixed coupling_all_filtered attribution. Root cause: the hint hardcoded 'lower --min-coupling' though min-coupling is one of four AND'd clauses. Fix: in runCoupling's existing candidate-pair loop, accumulate a couplingBlockers struct (max observed degree, shared-revs, average-revs); emitCouplingFilteredWarning now names only thresholds whose BEST observed value still fails (maxAverageRevs<MinRevs, maxSharedRevs<MinSharedRevs, maxDegree<MinCoupling, maxDegree>MaxCoupling), lists all binding ones, and reports each observed best so a caller can compute a working value. details restructured: {candidate_pairs, blocking:[flag names], observed:{max_*}, thresholds:{min-*,max-*}} -- old flat max_degree/min_coupling keys removed (unreleased, nothing read them). New golden coupling_warning_revs (weakCouplingLog(3,0)) freezes the revision-threshold attribution; coupling_warning still covers the genuine min-coupling case. Verified against this repo: default run now says 'lower --min-revs and lower --min-shared-revs', blocking=[min-revs,min-shared-revs]. Files: internal/analysis/coupling.go, coupling_test.go, command/golden_test.go, testdata/coupling_warning{,_revs}.err.
