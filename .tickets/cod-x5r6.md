---
id: cod-x5r6
status: closed
deps: [cod-migh, cod-s8uc]
links: []
created: 2026-07-14T03:49:32Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-12 analysis: main-developer (alias main-dev)

Analysis: main-developer by lines added (alias main-dev). Batch C.
New files: src/internal/analysis/maindev.go, maindev_test.go.
Docs: plan.md (Phase 4 Batch C), reference docs/research/code-maat.md 6 (churn family) and 7 (rounding). Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: research 6; churn.clj by-main-developer + 7 (centi ownership). Depends on P4-7, P4-0.

## Design

Row: type mainDevRow struct { Entity,MainDev string; Added,TotalAdded int; Ownership float64 } json entity,main_dev,added,total_added,ownership.
Descriptor Name:"main-developer", Aliases:["main-dev"], Summary:"Main developer per entity by lines added". ErrorCodes:["empty_log","missing_metrics"].
Algorithm: requireLoc; byEntityAuthorContrib; per entity: TotalAdded=sum(added); MainDev=author with max added (stable tie: first by sort? original uses (first (reverse (sort-by added))) => the max; ties -> last after sort = ? document: pick max added, tiebreak author asc); Ownership=calc.CentiRatio(mainAdded, TotalAdded). Sort Entity ASC.
TDD:

1. TestMainDev_PicksMaxAdder.
2. TestMainDev_OwnershipCentiRatio: e.g. main 164 of total 245 -> 0.67 (verify 2 sig digits).
3. TestMainDev_SortEntityAsc.
4. TestMainDev_MissingMetrics -> exit 3.

## Acceptance Criteria

- main-developer matches original + centi ownership rounding; alias main-dev; entity-asc sort. Cases pass; make validate green.

## Notes

**2026-07-14T13:02:00Z**

Implemented main-developer (alias main-dev) in src/internal/analysis/maindev.go + maindev_test.go. Row: {entity, main_dev, added, total_added, ownership(float)}. Algorithm: churn.RequireLoc guard; per entity, author with max added lines; ties break to ascending author (contributions arrive author-asc, keep-first-on-equal); total_added = sum of all authors' added; ownership = calc.CentiRatio(mainAdded, total) at 2 significant digits; sort entity asc. Refactor: exported churn.ByEntityAuthorContrib + EntityContribs/AuthorContrib (were unexported, test-only) so package analysis can reuse them; entity-ownership (cod-rq5s) and refactoring-main-developer (cod-7w03) can build on the same helper. Command + main-dev alias + schema all generated from the registry (no extra cmd wiring). make build green.
