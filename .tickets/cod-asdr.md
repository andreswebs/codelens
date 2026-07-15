---
id: cod-asdr
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

# P4-8 analysis: absolute-churn (alias abs-churn)

Analysis: absolute-churn (churn per date; alias abs-churn). Batch C.

New files: src/internal/analysis/abschurn.go, abschurn_test.go.

Docs: plan.md (Phase 4 Batch C), reference docs/research/code-maat.md 6 (churn family) and 7 (rounding). Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: research 6 (abs-churn); churn.clj absolutes-trend. Depends on P4-7.

## Design

Row: type absChurnRow struct { Date string `json:"date"`; Added,Deleted,Commits int } json date,added,deleted,commits.
Descriptor Name:"absolute-churn", Aliases:["abs-churn"], Summary:"Lines added/deleted per date". ErrorCodes:["empty_log","missing_metrics"], ExitCodes:[0,2,3,1].
Algorithm: requireLoc; sumByGroup by Date; sort [date, added, deleted] ASC.
TDD:

1. TestAbsChurn_PerDateSums: two dates -> summed added/deleted + commit counts.
2. TestAbsChurn_SortDateAsc: rows ordered by date asc.
3. TestAbsChurn_MissingMetrics: message-only log -> missing_metrics (exit 3).

## Acceptance Criteria

- absolute-churn matches churn_test.clj; date-asc sort; loc guard. Cases pass; make validate green.

## Notes

**2026-07-14T12:44:48Z**

Implemented absolute-churn (alias abs-churn) in internal/analysis/abschurn.go. Group by Date; sum added/deleted; count distinct revs; sort [date,added,deleted] asc. Reuses churn helpers, which I EXPORTED (requireLoc/sumByGroup/groupChurn -> RequireLoc/SumByGroup/GroupChurn) to consume them from package analysis, matching effort/couplingalgo house style (P4-7 had flagged export-vs-colocate; chose export). Updated churn_test.go to exported names. Tests: PerDateSums, SortDateAsc, MissingMetrics(exit 3), DescriptorRegistered - all pass. Descriptor error_codes [empty_log,missing_metrics], exit_codes [0,2,3,1]. Verified e2e via CLI (canonical name + alias + schema --command). make build green; learnings.md P4-8 added. Downstream author-churn(cod-c52u)/entity-churn(cod-sg05) can now reuse RequireLoc+SumByGroup with different key/sort.
