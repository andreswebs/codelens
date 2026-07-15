---
id: cod-ew96
status: closed
deps: [cod-7s7l, cod-s8uc]
links: []
created: 2026-07-14T03:50:43Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-16 analysis: main-developer-by-revisions

Analysis: main-developer-by-revisions (alias main-dev-by-revs). Batch D.
New files: src/internal/analysis/maindevbyrevs.go, maindevbyrevs_test.go.
Docs: plan.md (Phase 4 Batches D/E), reference docs/research/code-maat.md 6 and 7. Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: research 6; effort.clj as-main-developer-by-revisions + 7. Depends on P4-14, P4-0.

## Design

Row: type mainDevByRevsRow struct { Entity,MainDev string; Added,TotalAdded int; Ownership float64 } json entity,main_dev,added,total_added,ownership (added=author revs, total_added=entity total revs - kept for column parity with original).
Descriptor Name:"main-developer-by-revisions", Aliases:["main-dev-by-revs"], Summary:"Main developer per entity by revision count". ErrorCodes:["empty_log"].
Algorithm: effort.byEntity; per entity pick author with max Revs (tiebreak author asc); Added=maxRevs, TotalAdded=TotalRevs; Ownership=calc.CentiRatio(maxRevs, TotalRevs). Sort Entity ASC.
TDD:

1. TestMainDevByRevs_PicksMaxReviser.
2. TestMainDevByRevs_Ownership: e.g. 5 of 10 -> 0.5.
3. TestMainDevByRevs_SortEntityAsc.

## Acceptance Criteria

- main-developer-by-revisions matches original + ownership rounding; alias. Cases pass; make validate green.

## Notes

**2026-07-14T13:17:26Z**

Implemented main-developer-by-revisions (alias main-dev-by-revs) in src/internal/analysis/maindevbyrevs.go with TDD tests in maindevbyrevs_test.go. Built on effort.ByEntity (row-based rev counting, no loc metrics needed, so no missing_metrics error - only empty_log). Per entity, picks author with max Revs; effort.ByEntity returns authors ascending so first-on-tie == ascending-author tiebreak. Row json: entity,main_dev,added,total_added,ownership; added/total_added hold rev counts (naming mirrors original main-dev column parity). Ownership via calc.CentiRatio (2 sig digits). Sort entity asc. Registry-driven wiring picks up command + schema automatically; verified e2e via CLI (alias + csv kebab headers) and schema --command. make build green.
