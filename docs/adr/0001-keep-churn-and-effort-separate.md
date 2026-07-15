---
status: accepted
---

# Keep churn and effort as separate aggregation packages

## Context

`internal/analysis/churn` and `internal/analysis/effort` look like duplicate
roll-ups: both group modifications by entity and then by author on top of
`internal/analysis/calc`'s `GroupBy`. An architecture review flagged merging them
into one aggregation package.

## Decision

Keep them separate. `churn` sums lines of code (added and deleted), counts distinct
commits, and owns the loc-metrics guard (`RequireLoc` / `ErrMissingMetrics`); its
package invariant is that every consumer needs loc data. `effort` counts revisions as
modification rows (`nrows`, not distinct commits) and deliberately needs no loc data
or guard. These are two different, load-bearing counting rules, each pinned in one
place with a package doc that states it.

## Consequences

The shared structural primitive (`calc.GroupBy`) is already extracted, so a merge
would concentrate no duplicated logic; it would only relocate two distinct functions
into one package and dilute churn's "needs loc" invariant, since half the merged
package (the effort roll-up) would not honor it. Two packages with crisp, separate
invariants are preferred over one package with a qualified one. Future architecture
reviews should not re-propose merging `churn` and `effort` unless the counting rules
themselves converge.
