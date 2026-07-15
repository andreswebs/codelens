---
id: cod-90c9
status: closed
deps: [cod-7s7l, cod-s8uc]
links: []
created: 2026-07-14T03:50:43Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-4]
---
# P4-18 analysis: communication

Analysis: communication (shared-work strength between author pairs). Batch D.
New files: src/internal/analysis/communication.go, communication_test.go.
Docs: plan.md (Phase 4 Batches D/E), reference docs/research/code-maat.md 6 and 7. Register descriptor (P2-1); schema conformance (P2-5). Skills: /golang /tdd. Reference: research 6; communication.clj + communication_test.clj. Depends on P4-14, P4-0.

## Design

Row: type communicationRow struct { Author,Peer string; Shared,Average,Strength int } json author,peer,shared,average,strength.
Descriptor Name:"communication", Summary:"Heuristic communication strength between author pairs". ErrorCodes:["empty_log"].
Algorithm (port communication.clj): from entity-effort rows grouped by entity, build author pairs via selections-with-replacement (includes self-pairs); frequency of each pair across entities. Self-pair [a,a] frequency = total commits for a. For each distinct pair (me!=peer): myC=freq[me,me], peerC=freq[peer,peer], average=Ceil(Average(myC,peerC)), strength=TruncInt(Percentage(shared/average)). Sort [strength, author] DESC.
TDD - port communication_test.clj:

1. TestComm_PairStrength: two authors sharing work -> expected strength.
2. TestComm_SelfPairsExcludedFromOutput: no [a,a] rows.
3. TestComm_SortStrengthDesc.

## Acceptance Criteria

- communication matches communication_test.clj incl. strength formula and self-pair handling. Cases pass; make validate green.

## Notes

**2026-07-14T13:37:34Z**

Implemented communication analysis (P4-18). New: src/internal/analysis/communication.go + _test.go. Algorithm ports code-maat communication.clj: for each entity take distinct authors (via effort.ByEntity), form all ordered pairs-with-replacement (selections), count entity co-occurrences into a freq map. Self-pair freq[a,a] = distinct entities author a touched (this is the 'total' the strength divides against, not literal commit count). For each directed pair me!=peer: average=Ceil(Average(freq[me,me],freq[peer,peer])), strength=TruncInt(Percentage(shared/average)). Both directions emitted (symmetric), self-pairs excluded from output. Sort strength/author/peer all desc (peer is a deterministic tie-break beyond the original's reverse-sort on [strength author], since code-maat leaves full ties to nondeterministic map order). No per-analysis flags; ErrorCodes [empty_log]; empty log handled upstream in pipeline, Run returns empty rows for nil mods. Descriptor auto-registers; command tree + schema are registry-driven so no extra wiring. make build green. No .clj fixtures in repo, so tests are hand-derived from the algorithm; recommend a golden e2e in the authors-style e2e suite if the code-maat corpus is later imported.
