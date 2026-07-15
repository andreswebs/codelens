---
id: cod-i10e
status: closed
deps: [cod-sskw, cod-9eay, cod-x9ol, cod-ggqz]
links: []
created: 2026-07-14T03:42:54Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-2]
---
# P2-6 e2e: authors golden tests (all formats + schema)

End-to-end golden tests for the authors slice across all formats + fields/rows + schema snapshot. Freezes the spine before fanning out to 20 analyses.

New files: src/cmd/codelens/testdata/authors.* goldens, e2e test.

Docs: plan.md (Phase 2), design cli-design.md sections 4 (surface), 6 (output), 8 (schema); reference docs/research/code-maat.md section 6 (authors). Skills: /golang /tdd /llm-coding.
Reference: original authors expected output. Depends on P2-2, P2-4, P2-5, P1-5.

## Design

- Use a ported git2 log fixture (from P1-5 or a small purpose-built one) with a known authors result.
- Golden files: authors.json, authors.ndjson, authors.csv, authors.table, authors.fields.json (--fields rows.entity), authors.rows2.json (--rows 2), authors.schema.json (schema --command authors). -update flag regenerates.
- e2e test drives run() with the fixture on stdin for each format and compares stdout to the golden.

TDD cases:
1-6. TestE2E_Authors_<format/variant>: json, ndjson, csv, table, fields, rows2 match goldens.
7. TestE2E_Authors_Schema: schema --command authors matches golden.

## Acceptance Criteria

- authors verified across json/ndjson/csv/table + --fields + --rows + schema against committed goldens.
- The output/CLI spine is frozen; Phase 4 analyses are additive.
- All cases pass; make validate green.

## Notes

**2026-07-14T11:46:06Z**

P2-6 done. Added end-to-end golden suite in cmd/codelens (package main) driving run() against testdata/authors.log for all 4 formats + --fields rows.entity + --rows 2 + schema --command authors (7 goldens), plus TestE2E_Authors_JSONReviewed drift guard (decodes envelope: 4 rows, git2.clj ranked first). Input fixture is ported code-maat simple_git2 content (GPL-3.0 provenance in testdata/README.md). -update regenerates. The output/CLI spine is now frozen; the 8 unblocked P4 analyses (revisions, summary, parse, code-age, messages, coupling/churn/effort cores) are additive and should mirror this test shape. make build green. Learnings appended.
