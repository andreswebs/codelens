---
id: cod-1sde
status: closed
deps: [cod-s8uc, cod-i10e]
links: []
created: 2026-07-14T03:48:27Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-4 analysis: coupling core algorithms

Coupling core algorithms shared by coupling and sum-of-coupling. Batch B.

New files: src/internal/analysis/couplingalgo/couplingalgo.go, couplingalgo_test.go.

Docs: plan.md (Phase 4), reference docs/research/code-maat.md sections 6 (algorithms) and 7 (rounding). Register descriptor per P2-1; verified by P2-5 schema conformance. Skills: /golang /tdd /llm-coding.
Reference: research 6 (coupling); original analysis/coupling_algos.clj + coupling_algos_test.clj. Depends on P4-0, P2-6.

## Design

Port coupling_algos.clj faithfully:

- func changeSetsByRevision(mods) [][]string : group by Rev, each -> distinct entity list (order stable).
- func coChangingByRevision(sets [][]string, maxChangesetSize int) [][]pair : drop sets with len > maxChangesetSize; for each remaining set produce unordered pairs INCLUDING self-pairs [A,A] (from selections-with-replacement then sort+distinct). A pair is a sorted 2-tuple.
- func moduleByRevs(coChanging) map[string]int : for each rev, distinct modules present (flatten pairs, distinct); frequency across revs = revisions each module participates in.
- func couplingFrequencies(coChanging) map[pair]int : flatten all pairs across revs, DROP self-pairs [A,A], frequency per unordered pair = shared revisions.
- func WithinThreshold(revs, sharedRevs int, coupling float64, o Opts) bool : revs>=MinRevs && sharedRevs>=MinSharedRevs && coupling>=MinCoupling && floor(coupling)<=MaxCoupling.

TDD cases (couplingalgo_test.go) - port coupling_algos_test.clj:

1. TestChangeSets_GroupByRev: two revs -> two entity lists.
2. TestCoChanging_PairsIncludeSelf: set [A,B] -> pairs {[A,A],[A,B],[B,B]}.
3. TestCoChanging_DropsOversized: set larger than maxChangesetSize excluded.
4. TestModuleByRevs: module rev counts correct (each module counted once per rev it appears in).
5. TestCouplingFrequencies_DropsSelfPairs: shared-rev counts per real pair; self-pairs absent.
6. TestWithinThreshold_Bounds: boundary checks for each threshold.

## Acceptance Criteria

- Core coupling algos match coupling_algos.clj (self-pairs kept for totals, dropped for shared counts). All cases pass; make validate green.

## Notes

**2026-07-14T11:55:59Z**

Implemented src/internal/analysis/couplingalgo/{couplingalgo.go,couplingalgo_test.go}, porting coupling_algos.clj. TDD, 6 cases (all green, incl -race): changeSetsByRevision (group-by-rev -> distinct entities, ascending rev order), coChangingByRevision (selections-with-replacement -> sorted+distinct pairs INCLUDING self-pairs; drops sets > maxChangesetSize), moduleByRevs (per-module rev counts; self-pairs keep singleton change sets countable), couplingFrequencies (drops self-pairs; shared-rev count per real pair), and exported WithinThreshold(revs, sharedRevs, coupling, Opts): revs>=MinRevs && shared>=MinSharedRevs && coupling>=MinCoupling && floor(coupling)<=MaxCoupling.

KEY DESIGN NOTE for consumers (cod-uwv3 coupling, cod-rbbk soc): couplingalgo is a SUBPACKAGE of analysis, so it CANNOT import analysis.Opts (import cycle). It defines its own local couplingalgo.Opts (MinRevs/MinSharedRevs/MinCoupling/MaxCoupling); the coupling analysis must populate it from analysis.Opts. Core funcs (changeSetsByRevision/coChangingByRevision/moduleByRevs/couplingFrequencies) and the pair type are UNEXPORTED (matching the churn helper precedent, exercised by same-package tests). The consuming analyses will need exported wrappers added here, OR live in this package. pair is a canonical sorted struct {A,B} usable as a map key. make build green.
