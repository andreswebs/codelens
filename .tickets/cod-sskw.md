---
id: cod-sskw
status: closed
deps: [cod-joym]
links: []
created: 2026-07-14T03:42:53Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-2]
---
# P2-2 analysis: authors (default)

Implement the authors analysis (the default) and register its descriptor. First real analysis; proves the descriptor shape end to end.

New files: src/internal/analysis/authors.go, authors_test.go.

Docs: plan.md (Phase 2), design cli-design.md sections 4 (surface), 6 (output), 8 (schema); reference docs/research/code-maat.md section 6 (authors). Skills: /golang /tdd /llm-coding.
Reference: research 6 (authors): group by entity; distinct author count; rev count; sort [n-authors,n-revs] desc. Original test/code_maat/analysis/authors_test.clj. Depends on P2-1.

## Design

Row type + descriptor:

- type authorsRow struct { Entity string `json:"entity"`; NAuthors int `json:"n_authors"`; NRevs int `json:"n_revs"` }
- Descriptor{ Name:"authors", Aliases:nil, Summary:"Number of distinct authors per entity",
    RowSchema: [ {entity,string,"module path"}, {n_authors,int,"distinct authors that touched it"}, {n_revs,int,"revisions of the entity"} ],
    ErrorCodes: ["empty_log"], ExitCodes: [0,2,3,1], Run: runAuthors }

Algorithm (runAuthors):

- group mods by Entity; per group NAuthors = count(distinct Author), NRevs = count(rows in group) (matches original revisions-in = nrows).
- sort desc by NAuthors, then NRevs; final tiebreak Entity ascending for DETERMINISM (documented divergence: original leaves equal-key order to dataset insertion; we make it deterministic).
- build output.Result{Analysis:"authors", RowCount:len, Rows:[]authorsRow}. Params may be empty for authors.

TDD cases (authors_test.go, build []model.Modification inline):

1. TestAuthors_CountsDistinctAuthors: entity A touched by author x twice and y once -> NAuthors 2, NRevs 3.
2. TestAuthors_MultipleEntities_SortDesc: two entities, one with more authors -> ordered most-authors-first.
3. TestAuthors_TieBrokenByRevsThenEntity: equal n-authors -> higher n-revs first; equal both -> entity asc.
4. TestAuthors_Empty: no mods -> RowCount 0, Rows empty (ok result).
5. TestAuthors_PortedFixture (optional here, full golden in P2-6).

## Acceptance Criteria

- authors registered; Run returns correct rows, counts, and deterministic sort.
- Matches ported authors_test.clj expectations (modulo the documented tiebreak).
- Cases 1-4 pass; make validate green.

## Notes

**2026-07-14T10:49:53Z**

Implemented authors analysis (default) in src/internal/analysis/authors.go with authors_test.go. authorsRow{entity,n_authors,n_revs}; runAuthors groups by entity via calc.GroupBy, NAuthors=len(distinct authors), NRevs=len(rows in group) matching original revisions-in=nrows. Sort desc by [n_authors,n_revs], final tiebreak entity asc (documented divergence from code-maat's insertion-order for deterministic --rows). Descriptor exposed via authorsDescriptor() (not a package var) so tests inspect it without touching global registry state; init() registers it. Run builds full envelope (schema_version, ok, analysis, row_count, rows); empty mods -> ok result with rows=[]authorsRow{} (empty_log is enforced at parse layer per cod-rf77, not here). 5 tests pass; make build green.
