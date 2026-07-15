---
id: cod-c52u
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
# P4-9 analysis: author-churn

Analysis: author-churn (churn per author). Batch C.
New files: src/internal/analysis/authorchurn.go, authorchurn_test.go.
Docs: plan.md (Phase 4 Batch C), reference docs/research/code-maat.md 6 (churn family) and 7 (rounding). Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: research 6; churn.clj by-author. Depends on P4-7.

## Design

Row: type authorChurnRow struct { Author string; Added,Deleted,Commits int } json author,added,deleted,commits.
Descriptor Name:"author-churn", Summary:"Lines added/deleted per author". ErrorCodes:["empty_log","missing_metrics"].
Algorithm: requireLoc; sumByGroup by Author; sort [author, added] ASC.
TDD:

1. TestAuthorChurn_PerAuthorSums.
2. TestAuthorChurn_SortAuthorAsc.
3. TestAuthorChurn_MissingMetrics -> exit 3.

## Acceptance Criteria

- author-churn matches original; author-asc sort; loc guard. Cases pass; make validate green.

## Notes

**2026-07-14T12:47:51Z**

Implemented author-churn (P4-9, Batch C). New files: src/internal/analysis/authorchurn.go + authorchurn_test.go. Uses existing churn.RequireLoc + churn.SumByGroup(by Author) helpers (from cod-migh); no new helpers needed. Row {author,added,deleted,commits}, sort [author,added] asc per reference doc 6. Self-registers via init(); verified end-to-end through CLI (JSON/CSV kebab headers/schema --command) and full make build green. Mirrors abschurn.go pattern exactly. Sibling entity-churn (cod-sg05) is the same shape but groups by Entity and sorts added DESC (tiebreak entity asc).
