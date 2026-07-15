---
id: cod-sg05
status: closed
deps: [cod-migh]
links: []
created: 2026-07-14T03:49:32Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-10 analysis: entity-churn

Analysis: entity-churn (churn per entity). Batch C.
New files: src/internal/analysis/entitychurn.go, entitychurn_test.go.
Docs: plan.md (Phase 4 Batch C), reference docs/research/code-maat.md 6 (churn family) and 7 (rounding). Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: research 6; churn.clj by-entity. Depends on P4-7.

## Design

Row: type entityChurnRow struct { Entity string; Added,Deleted,Commits int } json entity,added,deleted,commits.
Descriptor Name:"entity-churn", Summary:"Lines added/deleted per entity". ErrorCodes:["empty_log","missing_metrics"].
Algorithm: requireLoc; sumByGroup by Entity; sort Added DESC (tiebreak entity asc).
TDD:

1. TestEntityChurn_PerEntitySums.
2. TestEntityChurn_SortAddedDesc.
3. TestEntityChurn_MissingMetrics -> exit 3.

## Acceptance Criteria

- entity-churn matches original; added-desc sort; loc guard. Cases pass; make validate green.

## Notes

**2026-07-14T12:57:18Z**

Implemented entity-churn (P4-10). New files src/internal/analysis/entitychurn.go + _test.go. Reuses churn.RequireLoc + churn.SumByGroup(by Entity). Row {entity,added,deleted,commits}; sort added DESC (stable over ascending-key grouping gives entity-asc tiebreak). ErrorCodes [empty_log, missing_metrics], exit 3 on message-only log. Descriptor auto-registers via init(); no CLI/registry sync points needed. TDD: per-entity sums, added-desc sort, missing-metrics guard, descriptor. make build green; verified via CLI (entity-churn + schema --command). Churn batch C remaining: entity-ownership (cod-rq5s), main-developer (cod-x5r6), refactoring-main-developer (cod-7w03).
