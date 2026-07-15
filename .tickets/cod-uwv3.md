---
id: cod-uwv3
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
# P4-5 analysis: coupling (+ --verbose)

Analysis: coupling (logical/temporal coupling between entity pairs), incl. --verbose. Batch B.

New files: src/internal/analysis/coupling.go, coupling_test.go.

Docs: plan.md (Phase 4), reference docs/research/code-maat.md sections 6 (algorithms) and 7 (rounding). Register descriptor per P2-1; verified by P2-5 schema conformance. Skills: /golang /tdd /llm-coding.
Reference: research 6 (coupling); original logical_coupling.clj + logical_coupling_test.clj. Depends on P4-4.

## Design

Row + descriptor:

- type couplingRow struct { Entity,Coupled string; Degree,AverageRevs int; FirstEntityRevisions,SecondEntityRevisions,SharedRevisions *int (verbose only) } json: entity,coupled,degree,average_revs,first_entity_revisions,second_entity_revisions,shared_revisions.
- Flags: --min-revs(5),--min-shared-revs(5),--min-coupling(30),--max-coupling(100),--max-changeset-size(30),--verbose(false).
- RowSchema documents standard cols always; verbose cols noted.
- ErrorCodes:["empty_log"], ExitCodes:[0,2,3,1].
Algorithm: coChanging = couplingalgo.coChangingByRevision(changeSetsByRevision(mods), MaxChangesetSize); moduleRevs=moduleByRevs; freqs=couplingFrequencies. For each (pair=[e1,e2], shared): r1=moduleRevs[e1], r2=moduleRevs[e2], avg=Average(r1,r2), coupling=Percentage(float64(shared)/avg); if WithinThreshold(avg-as-revs? NOTE original passes average-revs as 'revs' to within-threshold) -> emit. degree=TruncInt(coupling), average_revs=Ceil(avg). Verbose adds r1,r2,shared. Sort [degree, average_revs] desc (tiebreak entity,coupled asc for determinism).
IMPORTANT: within-threshold's 'revs' arg is average-revs (see logical_coupling.clj passing average-revs). Match that.

TDD cases - port logical_coupling_test.clj:

1. TestCoupling_TwoModulesAlwaysTogether: A,B in every rev -> degree 100.
2. TestCoupling_DegreeFormula: known shared/avg -> degree matches (use the InfoUtils/Page 78 style example).
3. TestCoupling_ThresholdFilters: below --min-coupling excluded; above --max-coupling excluded.
4. TestCoupling_MaxChangesetSize: giant commit excluded from pairs.
5. TestCoupling_Verbose: --verbose adds first/second/shared revision columns.
6. TestCoupling_SortDesc: [degree, average_revs] desc ordering.

## Acceptance Criteria

- coupling matches logical_coupling_test.clj values incl. degree/average-revs rounding and thresholds; --verbose columns correct; deterministic sort. All cases pass; make validate green.

## Notes

**2026-07-14T12:07:34Z**

Implemented coupling analysis (analysis/coupling.go + coupling_test.go). Added exported couplingalgo.Couplings + PairRevs entry point (couplingalgo helpers stay unexported per the churn/couplingalgo pattern; the analysis lives in package analysis so it reaches core via this one exported fn, mirroring the effort-batch export rule). Degree=TruncInt(Percentage(shared/avg)), average_revs=Ceil(avg); WithinThreshold receives TruncInt(avg) as its revs arg (floor(avg) is identical to the raw ratio for the inclusive >=min-revs check, so no signature change to the closed WithinThreshold). --verbose adds first/second/shared_revisions as *int with omitempty (absent in the 4 standard columns / csv / table). Sort [degree, average_revs] desc, tiebreak entity,coupled asc. No fixtures shipped in repo, so tests use hand-computed values from the spec formulas (degree 76 from avg 6.5 pins trunc+ceil). Registry/CLI/schema wire automatically. make build green.
