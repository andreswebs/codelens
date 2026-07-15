---
id: cod-rbbk
status: closed
deps: [cod-1sde]
links: []
created: 2026-07-14T03:48:27Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-6 analysis: sum-of-coupling (alias soc)

Analysis: sum-of-coupling (soc) per entity. Batch B.

New files: src/internal/analysis/soc.go, soc_test.go.

Docs: plan.md (Phase 4), reference docs/research/code-maat.md sections 6 (algorithms) and 7 (rounding). Register descriptor per P2-1; verified by P2-5 schema conformance. Skills: /golang /tdd /llm-coding.
Reference: research 6 (soc); original sum_of_coupling.clj + sum_of_coupling_test.clj. Depends on P4-4.

## Design

Row + descriptor:

- type socRow struct { Entity string `json:"entity"`; Soc int `json:"soc"` }
- Aliases:["soc"], Name:"sum-of-coupling", Summary:"Sum of coupling per entity".
- Flags: --min-revs (5).
- RowSchema:[{entity,...},{soc,int,"number of shared transactions"}]. ErrorCodes:["empty_log"], ExitCodes:[0,2,3,1].
Algorithm (port sum_of_coupling.clj): for each rev's entity set (NO max-changeset filter), n = len-1; each entity += n. Sum across revs. Keep entities with soc > MinRevs (STRICT >, unlike coupling's >=). Sort [soc, entity] desc.

TDD cases - port sum_of_coupling_test.clj:

1. TestSoc_AccumulatesPerRev: entity in a 3-file commit gains 2.
2. TestSoc_StrictMinRevs: soc == MinRevs excluded; soc > MinRevs kept.
3. TestSoc_SortDesc: [soc, entity] desc.

## Acceptance Criteria

- soc matches sum_of_coupling_test.clj; strict > MinRevs filter; deterministic desc sort; alias soc. Cases pass; make validate green.

## Notes

**2026-07-14T12:11:50Z**

Implemented sum-of-coupling (alias soc) in src/internal/analysis/soc.go + soc_test.go. Algorithm per reference doc §6: for each revision's distinct-entity change set of size k, every member gains k-1; summed across all revisions; NO max-changeset-size filter (unlike coupling). Filter is STRICT soc > min-revs (contrast coupling's inclusive >=). Sort [soc, entity] descending for determinism. Descriptor: name sum-of-coupling, alias soc, single --min-revs(5) flag, row_schema {entity, soc}, error_codes [empty_log], exit_codes [0,2,3,1]. Uses calc.GroupBy/Distinct; no couplingalgo dependency needed since soc is a simple per-rev accumulation without pair frequencies. TDD: 3 spec cases (AccumulatesPerRev, StrictMinRevs, SortDesc) + descriptor test. make build green; verified e2e against gitlog/testdata/entries.log.
