---
id: cod-7s7l
status: closed
deps: [cod-s8uc, cod-i10e]
links: []
created: 2026-07-14T03:50:43Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-14 analysis: effort core helper

Effort core helper shared by entity-effort, main-developer-by-revisions, fragmentation, communication. Batch D.
New files: src/internal/analysis/effort/effort.go, effort_test.go.
Docs: plan.md (Phase 4 Batches D/E), reference docs/research/code-maat.md 6 and 7. Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: original analysis/effort.clj + effort_test.clj. Depends on P4-0, P2-6.

## Design

Helper (port sum-effort-by-author):

- type authorRevs struct { Author string; Revs, TotalRevs int }
- type entityEffort struct { Entity string; Authors []authorRevs }
- func byEntity(mods) []entityEffort : group by Entity; TotalRevs = number of rows in the entity group (matches original nrows); per author Revs = rows for that author in the entity; deterministic order.
TDD cases:

1. TestEffort_TotalRevsIsEntityRows: entity with 3 rows -> each authorRevs.TotalRevs=3.
2. TestEffort_PerAuthorRevs: author counts per entity correct.

## Acceptance Criteria

- effort core matches effort.clj (TotalRevs=entity row count). Cases pass; make validate green.

## Notes

**2026-07-14T11:59:27Z**

Implemented effort core helper in src/internal/analysis/effort/ (package effort). Exports ByEntity(mods) []EntityEffort, plus EntityEffort{Entity, Authors []AuthorRevs} and AuthorRevs{Author, Revs, TotalRevs}. Ports code-maat effort.clj sum-effort-by-author: TotalRevs = entity ROW count (nrows), per-author Revs = that author's ROW count within the entity (frequencies) -- counts ROWS, not distinct revs (a file listed twice in one change set counts twice). Entities and authors returned in ascending key order (via calc.GroupBy) for deterministic downstream sort/truncation. NOTE: ticket design wrote 'byEntity' lowercase but referenced it as 'effort.byEntity' cross-package; since the downstream analyses live in package 'analysis' (e.g. entityeffort.go), the API is EXPORTED (ByEntity/EntityEffort/AuthorRevs). TotalRevs is repeated on every AuthorRevs row so consumers read the share without re-deriving. Unblocks cod-t1sa (entity-effort), cod-ew96 (main-dev-by-revs), cod-6xou (fragmentation), cod-90c9 (communication). make build green.
