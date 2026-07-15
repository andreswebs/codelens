---
id: cod-x9ol
status: closed
deps: [cod-joym, cod-ll9s]
links: []
created: 2026-07-14T03:42:54Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-2]
---
# P2-5 cmd: schema introspection (list + --command)

Implement the schema introspection command: list all commands, and fully describe one command (flags, row schema with descriptions, error codes, exit codes).

New files: src/cmd/codelens/schema.go, src/internal/analysis/schema.go (schema builder), schema_test.go.

Docs: plan.md (Phase 2), design cli-design.md sections 4 (surface), 6 (output), 8 (schema); reference docs/research/code-maat.md section 6 (authors). Skills: /golang /tdd /llm-coding.
Design ref: cli-design.md 8. Depends on P2-1 (registry/descriptors) and P0-5 (output registry).

## Design

- `codelens schema` (no args): emit a Result-like JSON listing every command: name, aliases, summary, exit_codes. Include the meta commands (schema, print-log-command, version) too.
- `codelens schema --command CMD`: resolve via analysis.Lookup; emit { schema_version, ok, command, summary, aliases, flags:[{name,type,default,required,desc}], row_schema:[{name,type,desc}], error_codes, exit_codes }.
- Build the schema object purely from the Descriptor (flags, RowSchema) so it cannot drift from behavior.
- IF --command names an unknown command THEN usage error exit 2 listing known commands.

TDD cases (schema_test.go via run()):

1. TestSchema_List: `schema` -> JSON includes "authors" with its aliases and exit_codes.
2. TestSchema_Command_Authors: `schema --command authors` -> flags [] (authors has none), row_schema has entity/n_authors/n_revs with non-empty desc, error_codes includes empty_log, exit_codes [0,2,3,1].
3. TestSchema_Command_Alias: `schema --command <alias>` resolves (use once an aliased analysis exists; for now unknown alias).
4. TestSchema_UnknownCommand: `schema --command nope` -> exit 2, lists known commands.
5. TestSchema_Conformance: for EVERY registered analysis, row_schema is non-empty and each column has a description (guards Phase 4 additions).

## Acceptance Criteria

- schema list + per-command schema emitted from descriptors; cannot drift from actual flags.
- Unknown command -> exit 2. Conformance test enforces non-empty documented row_schema for all analyses.
- Cases 1-5 pass; make validate green.

## Notes

**2026-07-14T11:36:07Z**

Implemented schema introspection. New: internal/analysis/schema.go (Schema(Descriptor)->CommandSchema and List(analyses,extra)->CommandList builders, built purely from descriptors so they can't drift; slices normalized to [] not null) and cmd/codelens/schema.go (the 'schema' cli.Command). Added json tags to analysis.Flag/Column (were untagged) so they marshal snake_case. Wired schemaCommand() into main.go root Commands. 'schema' lists all commands incl meta (schema/print-log-command/version) with name/aliases/summary/exit_codes; 'schema --command CMD' resolves via analysis.Lookup (aliases too) and emits full contract; unknown CMD -> usage_error exit 2 with details.known_commands. Meta usages extracted to consts (printLogCommandUsage/schemaUsage/versionUsage) and reused for both cli Usage and list entries. version listed though its subcommand is P5-2. Tests: internal/analysis/schema_test.go (builder unit) + cmd/codelens/schema_test.go (ticket cases 1-5 via run(), incl conformance guard that every analysis has a fully-documented non-empty row_schema). make build green.
