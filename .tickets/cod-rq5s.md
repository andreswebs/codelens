---
id: cod-rq5s
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
# P4-11 analysis: entity-ownership

Analysis: entity-ownership (churn per entity+author). Batch C.
New files: src/internal/analysis/ownership.go, ownership_test.go.
Docs: plan.md (Phase 4 Batch C), reference docs/research/code-maat.md 6 (churn family) and 7 (rounding). Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: research 6; churn.clj as-ownership. Depends on P4-7.

## Design

Row: type ownershipRow struct { Entity,Author string; Added,Deleted int } json entity,author,added,deleted.
Descriptor Name:"entity-ownership", Summary:"Per-author churn contribution to each entity". ErrorCodes:["empty_log","missing_metrics"].
Algorithm: requireLoc; byEntityAuthorContrib; splice one row per (entity,author) with added/deleted; sort Entity ASC (tiebreak author asc for determinism).
TDD:

1. TestOwnership_PerEntityPerAuthorRows: entity with two authors -> two rows.
2. TestOwnership_SortEntityAsc.
3. TestOwnership_MissingMetrics -> exit 3.

## Acceptance Criteria

- entity-ownership matches original; per (entity,author) rows; entity-asc sort; loc guard. Cases pass; make validate green.

## Notes

**2026-07-14T13:13:29Z**

Implemented entity-ownership analysis. New files src/internal/analysis/ownership.go + ownership_test.go. Reused churn.ByEntityAuthorContrib (already built for this) and churn.RequireLoc for the loc guard (missing_metrics, exit 3). One row per (entity,author): {entity,author,added,deleted}; sort entity asc via SliceStable, which preserves the helper's ascending author order within an entity for deterministic tiebreak. Registered via init(); command tree/schema are fully registry-driven so no cmd wiring needed. TDD red->green: per-entity-per-author rows, sort-entity-asc, missing-metrics, descriptor-registered. Verified e2e (json/csv/schema). make build green.
