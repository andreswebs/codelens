---
id: cod-z4wu
status: closed
deps: []
links: [cod-304f]
created: 2026-07-27T12:56:46Z
type: epic
priority: 1
assignee: Andre Silva
tags: [codelens, spec-002, output]
---
# codelens: canonical shape-aware data output (spec 002)

Rollout of ADR 0008: codelens emits exactly one thing on stdout, a self-describing,
shape-aware JSON envelope. The `--format` matrix (`json|ndjson|csv|table`) and the
code-maat CSV compatibility are removed; the envelope gains `shape`, `semantics`, and
`transforms`; `schema --command CMD` declares `shape` and a per-column `semantic`.

The point is that codelens becomes a code visualization DATA engine: it hands any
renderer or agent the data plus the meaning of that data, and downstream consumers
produce pixels. `semantics` is the asset only codelens can provide, because it authored
the data, and it is what makes a downstream chart spec derivable without domain
knowledge.

Authoritative documents (read both before starting any child ticket):

- docs/specs/002-data-output/plan.md - the implementation plan. Section 2 is the
  decisions register (D1 to D16); every child ticket cites the decisions it implements.
- docs/adr/0008-canonical-output-representation.md - the decision record. Amends
  docs/adr/0006-output-contract.md, which now carries a back-pointer.
- docs/cli-design.md sections 6 and 8 - already describe the target envelope and the
  extended schema; they are ahead of the code and are the intended end state.

Child tickets, in dependency order: remove `--format` (subtractive), add `shape` plus
the payload marshaler (structural), add `semantics` plus `transforms` (content), then
the repo docs and the codelens skill. A separate follow-on epic tracks non-table shapes
and the downstream viz-spec adapters.

Not in scope: non-table shapes (`tree`, `graph`, `matrix`, `series`) beyond declaring
the enum, any viz-spec export inside the binary, and the complexity epic.

## Acceptance Criteria

- Every child ticket is closed, each having landed with `make build` green.
- `codelens --format json authors` is a usage error (exit 64); no `--format`, `ndjson`,
  `csv`, or `table` remains as a codelens output reference anywhere in the tree except
  in the historical spec-001 documents (annotated) and the ADR/changelog records of the
  removal.
- Every analysis envelope carries `shape` and `semantics`; `transforms` appears when a
  pipeline transform ran and is absent otherwise.
- `codelens schema --command CMD` declares `shape` and a per-column `semantic` for every
  command, meta commands included.
- `schema_version` is still `1`, with the breaking change recorded in CHANGELOG.md.
- markdownlint clean on every touched markdown file.


## Notes

**2026-07-27T14:29:37Z**

Epic closed after verifying all acceptance criteria against the built binary (all 5 children were already closed):
- `codelens --format json authors` -> exit 64 unknown_flag; no `--format`/ndjson/csv/table output references remain in code (remaining "csv" is only --team-map-format). Pinned by golden test removed_format_flag.
- Every analysis envelope carries `shape` (table) and `semantics`; `transforms` appears only when a pipeline transform ran (verified group -> {"group":true} and entity semantic degrades filepath->label; absent otherwise).
- `schema --command CMD` declares `shape` and per-column `semantic` for analyses and meta commands (print-log-command declares shape:text).
- schema_version stays 1 (all three envelope fields additive); breaking --format removal recorded in CHANGELOG.md.
- markdownlint clean (project .markdownlint.yaml) on CHANGELOG.md, docs/cli-design.md, ADR 0008. make build green.
Follow-on work (non-table shapes, viz-spec adapters) is tracked in cod-304f, deliberately deferred and not yet decomposed.
