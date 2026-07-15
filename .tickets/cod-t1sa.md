---
id: cod-t1sa
status: closed
deps: [cod-7s7l]
links: []
created: 2026-07-14T03:50:43Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-15 analysis: entity-effort

Analysis: entity-effort (per-author revision share of each entity). Batch D.
New files: src/internal/analysis/entityeffort.go, entityeffort_test.go.
Docs: plan.md (Phase 4 Batches D/E), reference docs/research/code-maat.md 6 and 7. Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: research 6; effort.clj as-revisions-per-author. Depends on P4-14.

## Design

Row: type entityEffortRow struct { Entity,Author string; AuthorRevs,TotalRevs int } json entity,author,author_revs,total_revs.
Descriptor Name:"entity-effort", Summary:"Each author revision share per entity". ErrorCodes:["empty_log"].
Algorithm: effort.byEntity; flatten to rows; sort: primary Entity ASC, within entity AuthorRevs DESC (original: stable sort-by revs desc then sort-by entity). Tiebreak author asc.
TDD:

1. TestEntityEffort_Rows: entity with two authors -> two rows with author_revs and total_revs.
2. TestEntityEffort_SortEntityThenRevsDesc.
3. TestEntityEffort_Empty.

## Acceptance Criteria

- entity-effort matches effort_test.clj incl. sort. Cases pass; make validate green.

## Notes

**2026-07-14T13:09:55Z**

Implemented entity-effort analysis (P4-15) via TDD. New files: src/internal/analysis/entityeffort.go + entityeffort_test.go. Row {entity,author,author_revs,total_revs}. Reused effort.ByEntity (already returns entities+authors in ascending key order) and flattened to one row per (entity,author). Sort: SliceStable primary entity ASC, secondary author_revs DESC; author-asc tiebreak preserved for free by the stable sort over ByEntity's author-ascending output (matches original stable sort-by revs desc then sort-by entity). No new flags, so registry auto-wires the command into the tree and schema. ErrorCodes [empty_log] (empty input is caught upstream at parse time via gitlog.ErrEmptyLog; Run returns empty rows harmlessly). Verified: make build green, e2e run + schema --command entity-effort conform.
