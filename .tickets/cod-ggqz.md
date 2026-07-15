---
id: cod-ggqz
status: closed
deps: [cod-3ksh]
links: []
created: 2026-07-14T03:40:09Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-1]
---
# P1-5 gitlog: ported git2 fixtures + golden tests

Port code-maat's git2 fixtures as Go testdata golden files and add golden-based parser tests. GPL-3.0 lets us reuse the corpus directly (attribute origin).

New files: src/internal/gitlog/testdata/*.log (+ expected .json goldens), golden test in parse_test.go or parse_golden_test.go.

Docs: plan.md (Phase 1), design cli-design.md section 5, port reference docs/research/code-maat.md sections 2 (data model), 3 (log format incl 3.4 stacked preludes, 3.5 parser notes). Skills: /golang /tdd.
Source fixtures: .local/refs/code-maat/test/code_maat/parsers/git2_test.clj (inline consts), test/code_maat/end_to_end/simple_git2.txt. Depends on P1-3.

## Design

- Copy the four inline consts from git2_test.clj into testdata: entry.log, binary.log, entries.log, pull_requests.log; also copy end_to_end/simple_git2.txt -> testdata/simple_git2.log. Add a header comment noting origin (code-maat, GPL-3.0).
- For each, commit an expected golden ([]Modification serialized as JSON) generated once and reviewed by hand against the original clj expectations.
- Golden test: table over testdata pairs; Parse(file) -> compare to golden. Support -update flag to regenerate goldens (guarded), standard Go golden pattern.

TDD cases:

1. TestParse_Golden_entry / _binary /_entries / _pull_requests / _simple_git2: each parses to its golden.
2. TestGoldens_Reviewed: a sanity assertion that the entries.log golden has exactly 6 records (guards accidental regen drift).

## Acceptance Criteria

- testdata logs + goldens committed with GPL-3.0 attribution.
- Golden tests pass; -update regenerates; drift guard in place.
- make validate green.

## Notes

**2026-07-14T11:41:21Z**

Golden parser tests added in src/internal/gitlog/parse_golden_test.go: table over 5 testdata fixtures (entry, binary, entries, pull_requests, simple_git2) parsed and byte-compared to committed *.golden.json ([]model.Modification, PascalCase since the struct carries no JSON tags). Standard -update flag regenerates; TestGoldens_Reviewed pins entries.log at exactly 6 records as a regen drift guard. NOTE: .local/refs/code-maat is a DANGLING symlink in this env (-> /Users/andre/...), so the code-maat corpus was NOT copyable. Fixtures are reconstructed faithful to the documented git2(+subject) format (reference doc 3.2-3.4) and the existing inline parse_test.go constants (themselves derived from code-maat); GPL-3.0 origin documented in testdata/README.md. simple_git2.log has repeated entities/authors so downstream coupling/authors e2e (cod-i10e) has meaningful signal. make build green (fmt/vet/lint 0 issues/test), -race clean.
