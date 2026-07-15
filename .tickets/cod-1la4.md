---
id: cod-1la4
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
# P4-2 analysis: summary

Analysis: summary (overview counts). Batch A.

New files: src/internal/analysis/summary.go, summary_test.go.

Docs: plan.md (Phase 4), reference docs/research/code-maat.md sections 6 (algorithms) and 7 (rounding). Register descriptor per P2-1; verified by P2-5 schema conformance. Skills: /golang /tdd /llm-coding.
Reference: research 6 (summary); original analysis/summary.clj. Depends on P4-0, P2-6.

## Design

Row + descriptor:

- type summaryRow struct { Statistic string `json:"statistic"`; Value int `json:"value"` }
- Descriptor{ Name:"summary", Summary:"Overview counts for the mined data",
    RowSchema:[{statistic,string,"metric name"},{value,int,"metric value"}], ErrorCodes:["empty_log"], ExitCodes:[0,2,3,1], Run:runSummary }
Rows (fixed order): number-of-commits = count(distinct Rev); number-of-entities = count(distinct Entity); number-of-entities-changed = len(mods) (total rows); number-of-authors = count(distinct Author).
NOTE statistic labels use kebab-case values (they are data, not json keys): "number-of-commits" etc.

TDD cases:

1. TestSummary_Counts: a known small set -> the four values correct.
2. TestSummary_FixedOrderAndLabels: rows in the documented order with exact labels.
3. TestSummary_NDJSON_Uniform (light): summary emits one {statistic,value} per line under ndjson (covered fully by format tests, assert shape here).

## Acceptance Criteria

- summary registered; four counts correct with exact kebab labels and order; matches original. Cases pass; make validate green.

## Notes

**2026-07-14T12:38:22Z**

Implemented summary analysis (P4-2). Added src/internal/analysis/summary.go + summary_test.go, TDD red->green. Emits 4 fixed-order rows with kebab-case statistic labels: number-of-commits (distinct Rev), number-of-entities (distinct Entity), number-of-entities-changed (len(mods)), number-of-authors (distinct Author). Uses calc.Distinct; no per-analysis flags. Empty input yields all-zero counts (empty_log guarded upstream in gitlog parser). Registry auto-wires subcommand/schema/help. Verified e2e across json/csv/ndjson/table via bin. make build green.
