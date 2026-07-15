---
id: cod-vdoq
status: closed
deps: [cod-xx0y]
links: []
created: 2026-07-14T03:52:51Z
type: chore
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-6]
---
# P6-3 docs: README rewrite

Rewrite README.md for codelens (replace any code-maat-oriented placeholder content).
Edit: README.md.
Docs: plan.md, design cli-design.md, requirements.md. Skills: /golang /tdd. Reference: whole design. Depends on P5-1 (final surface).

## Design

Sections: what/why; install (go install / build via make); quick start (piped workflow + print-log-command); the 20 analyses table (canonical name, alias, purpose); output formats + examples (json/ndjson/csv/table, --fields, --rows); schema introspection; exit codes; link to docs/ (design, requirements, plan, research); GPL-3.0 + attribution to code-maat/Adam Tornhill. markdownlint clean. Use env-var-style placeholders in shell examples per repo conventions.

## Acceptance Criteria

- README documents install, quick start, all analyses, formats, schema, exit codes; GPL-3.0 + attribution; markdownlint-clean.

## Notes

**2026-07-14T14:04:40Z**

Rewrote README.md from placeholder to full docs: what/why, install (go install + make build), quick start (print-log-command + piped workflow), 18-analysis table (canonical/alias/purpose sourced from live 'codelens schema'), output formats with real examples (json/ndjson/csv/table + --fields/--rows), schema introspection, common flags, error envelope + exit codes, docs/ links (all verified present), and GPL-3.0 + code-maat/Adam Tornhill attribution. Examples produced from bin binary against testdata/authors.log so they match real output. markdownlint clean via .markdownlint.yaml. Unblocks cod-3fh4 (P6-5 DoD verification).
