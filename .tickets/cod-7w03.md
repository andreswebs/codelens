---
id: cod-7w03
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
# P4-13 analysis: refactoring-main-developer

Analysis: refactoring-main-developer by lines removed (alias refactoring-main-dev). Batch C.
New files: src/internal/analysis/refactoringmaindev.go, refactoringmaindev_test.go.
Docs: plan.md (Phase 4 Batch C), reference docs/research/code-maat.md 6 (churn family) and 7 (rounding). Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: research 6; churn.clj by-refactoring-main-developer. Depends on P4-7, P4-0.

## Design

Row: type refMainDevRow struct { Entity,MainDev string; Removed,TotalRemoved int; Ownership float64 } json entity,main_dev,removed,total_removed,ownership.
Descriptor Name:"refactoring-main-developer", Aliases:["refactoring-main-dev"], Summary:"Main developer per entity by lines removed". ErrorCodes:["empty_log","missing_metrics"].
Algorithm: like main-dev but rank by DELETED lines; Ownership=CentiRatio(mainRemoved, TotalRemoved). Sort Entity ASC.
TDD:

1. TestRefMainDev_PicksMaxRemover.
2. TestRefMainDev_Ownership.
3. TestRefMainDev_MissingMetrics -> exit 3.

## Acceptance Criteria

- refactoring-main-developer matches original (by removed); alias; ownership rounding. Cases pass; make validate green.

## Notes

**2026-07-14T13:20:52Z**

Implemented refactoring-main-developer (alias refactoring-main-dev) as the deletion-ranked counterpart to main-developer: reused churn.RequireLoc, churn.ByEntityAuthorContrib, and calc.CentiRatio, ranking by AuthorContrib.Deleted instead of Added. Row {entity, main_dev, removed, total_removed, ownership}, sort entity asc, ties broken by ascending author (contribs arrive sorted). Descriptor auto-registers via init() so the command tree, schema, and help pick it up with no extra wiring. 6 TDD cases (max-remover, tie-break, ownership centi-rounding, sort, missing_metrics->exit 3, descriptor). make build green; verified e2e via CLI alias and schema introspection.
