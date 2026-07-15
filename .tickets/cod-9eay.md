---
id: cod-9eay
status: closed
deps: [cod-f0zl, cod-vqjh]
links: []
created: 2026-07-14T03:42:54Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-2]
---
# P2-4 output: json/ndjson/csv/table formats + fields/rows

Implement the four output formats generically over any analysis result: json (default), ndjson, csv, table; plus --fields projection (json only) and --rows truncation metadata.

New files: src/internal/output/format.go (csv/table/ndjson writers), format_test.go. Wire into the Action (P2-3).

Docs: plan.md (Phase 2), design cli-design.md sections 4 (surface), 6 (output), 8 (schema); reference docs/research/code-maat.md section 6 (authors). Skills: /golang /tdd /llm-coding.
Design ref: cli-design.md 6.1-6.5. Depends on P2-3, P0-4 (fields).

## Design

- func Emit(w io.Writer, format string, res Result, schema []analysis.Column, fields string) error : dispatch on format.
  - json: EmitProjected(w, res, fields) (fields "" => full envelope).
  - ndjson: marshal each row object on its own line (no wrapper), uniformly for all analyses incl. summary. Ignore --fields.
  - csv: header from schema column Names mapped snake_case->kebab-case; rows in schema column order; original ordering preserved (rows already sorted by Run). Ignore --fields.
  - table: aligned columns (text/tabwriter), header = column Names.
- Extract row values by marshaling res.Rows to JSON then unmarshaling to []map[string]any, reading columns by their json key (schema Name is the json key). This keeps one source of truth (json tags) and column order (schema).
- snake->kebab helper for csv header parity with code-maat.

TDD cases (format_test.go with a fixed 2-row Result + schema):

1. TestFormat_JSON_Default: full envelope with schema_version/ok/rows.
2. TestFormat_NDJSON: two lines, each a row object; no envelope keys; valid JSON per line.
3. TestFormat_CSV_Header_Kebab: header "entity,n-authors,n-revs"; rows in order.
4. TestFormat_Table_Aligned: contains aligned column headers and row values.
5. TestFormat_Fields_JSONOnly: --fields rows.entity -> json projects; same --fields with csv -> ignored (full csv emitted).
6. TestFormat_Rows_AllFormats: --rows applied before formatting for csv/table/ndjson too (assert row counts).

## Acceptance Criteria

- All four formats correct; ndjson uniform (incl. scalar shapes); csv headers kebab-case and parity-ordered.
- --fields applies to json only, ignored elsewhere; --rows applies to every format.
- Cases 1-6 pass; make validate green.

## Notes

**2026-07-14T11:25:14Z**

Implemented output.Emit(w, format, res, columns []string, fields) dispatching json/ndjson/csv/table in src/internal/output/format.go, wired into the analysis Action (commands.go) replacing the JSON-only emitResult. Design divergence: Emit takes columns as []string (ordered snake_case row-schema Names) not []analysis.Column, because output cannot import analysis (analysis->output cycle); cmd maps d.RowSchema names via columnNames(). Number cells decoded with json.Number (UseNumber) so ints/centi-floats render exact text (no float widening / sci-notation). csv headers snake->kebab for code-maat parity; ndjson via json.RawMessage preserves row field order; unknown --format is usage_error exit 2. --fields honored by json only; --rows truncation (in cmd) happens before Emit so all formats respect it. 6 TDD cases in format_test.go; make build green; verified end-to-end via binary across all formats + fields + rows + bad-format.
