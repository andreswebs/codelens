---
id: cod-0sst
status: closed
deps: [cod-s8uc, cod-i10e]
links: []
created: 2026-07-14T03:48:27Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-1 analysis: revisions

Analysis: revisions (change frequency per entity). Batch A.

New files: src/internal/analysis/revisions.go, revisions_test.go.

Docs: plan.md (Phase 4), reference docs/research/code-maat.md sections 6 (algorithms) and 7 (rounding). Register descriptor per P2-1; verified by P2-5 schema conformance. Skills: /golang /tdd /llm-coding.
Reference: research 6 (revisions); original analysis/entities.clj + entities_test.clj. Depends on P4-0 (helpers), P2-6 (frozen spine).

## Design

Row + descriptor:

- type revisionsRow struct { Entity string `json:"entity"`; NRevs int `json:"n_revs"` }
- Descriptor{ Name:"revisions", Summary:"Change frequency per entity",
    RowSchema:[{entity,string,"module path"},{n_revs,int,"number of distinct revisions"}],
    ErrorCodes:["empty_log"], ExitCodes:[0,2,3,1], Run:runRevisions }
Algorithm: group by Entity; NRevs = count(distinct Rev) in group; sort NRevs desc, tiebreak Entity asc (determinism).

TDD cases:

1. TestRevisions_CountsDistinctRevs: entity with 3 rows across 2 revs -> NRevs 2.
2. TestRevisions_SortDesc: highest n-revs first; entity tiebreak.
3. TestRevisions_Empty: no mods -> empty ok result.

## Acceptance Criteria

- revisions registered; distinct-rev count; deterministic desc sort; matches entities_test.clj. Cases pass; make validate green.

## Notes

**2026-07-14T12:35:05Z**

Implemented revisions analysis (src/internal/analysis/revisions.go + _test.go) via TDD. Group by entity, NRevs = count(distinct Rev) per group (distinct matters: a single rev touching an entity twice counts once, unlike authors' NRevs which is row count). Sort n_revs desc, tiebreak entity asc for deterministic --rows truncation. Descriptor registered via init(); RowSchema [entity, n_revs]; ErrorCodes [empty_log]; ExitCodes [0,2,3,1]. Verified via schema --command revisions and stdin e2e. make build green (fmt/vet/lint/test).
