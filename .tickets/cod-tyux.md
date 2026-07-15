---
id: cod-tyux
status: closed
deps: [cod-xx0y, cod-x9ol]
links: []
created: 2026-07-14T03:52:51Z
type: chore
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-6]
---
# P6-1 docs: AGENTS.md agent guide

Author AGENTS.md at the repo root: the agent-facing operating guide.
New file: AGENTS.md (repo root).
Docs: plan.md, design cli-design.md, requirements.md. Skills: /golang /tdd. Reference: cli-design.md 10, requirements 12. Depends on P5-1 (print-log-command) and P2-5 (schema) so the documented surface is final.

## Design

Contents:

- One-paragraph what codelens is.
- The canonical piped workflow: `git log ... --%s ... | codelens <analysis>` and `codelens print-log-command`.
- How to discover a command at runtime: `codelens schema` and `codelens schema --command CMD` (row_schema, flags, error/exit codes).
- Invariants: always bound output with --fields and/or --rows; JSON is default; stdin is default input; canonical names + terse aliases table.
- Exit-code table (0/2/3/1) and error envelope shape.
- Format guidance: json default, ndjson for streaming large results, csv/table for humans.
Keep it concise and accurate to the final surface. markdownlint clean (~/.markdownlint.yaml).
TDD/verification (no Go tests): a doc-lint check + a manual cross-check that every command named exists in the registry.

## Acceptance Criteria

- AGENTS.md present, accurate to the shipped surface, markdownlint-clean. An agent can operate codelens from AGENTS.md + schema alone.

## Notes

**2026-07-14T14:01:01Z**

Rewrote AGENTS.md (repo root) as the agent operating guide for the codelens CLI, replacing the build-instructions stub that duplicated CLAUDE.md. Contents: one-paragraph what-it-is; canonical stdin pipe workflow + print-log-command; runtime discovery via schema / schema --command; output envelope (schema_version, ok, analysis, row_count, rows; total_count/truncated on --rows cap); invariants (JSON default, stdin default, bound output with --fields/--rows, diagnostics on stderr); canonical+alias table; format guidance (json/ndjson/csv/table); error envelope + exit-code table (0/2/3/1); common-flags table. Verified against the shipped surface (go run ./cmd/codelens ...). NOTE: envelope 'params' field is defined (omitempty in internal/output/types.go) but no analysis populates it, so it is NOT documented as present. markdownlint-clean via /workspace/.markdownlint.yaml. Cross-checked: every command named in the doc exists in 'codelens schema' registry. Unblocks cod-quj5 (skill file) and cod-3fh4 (DoD).
