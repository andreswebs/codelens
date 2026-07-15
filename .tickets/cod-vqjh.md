---
id: cod-vqjh
status: closed
deps: [cod-g7yh]
links: []
created: 2026-07-14T03:37:08Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-0]
---
# P0-4 output: --fields projection

Field projection for --fields: validate comma-separated JSON paths against an envelope and project marshaled JSON down to them, always retaining schema_version and ok.

New files: src/internal/output/fields.go, fields_test.go.

Docs (repo-root relative):

- Plan task: docs/specs/001-initial-implementation/plan.md (Phase 0)
- Design: docs/cli-design.md
- Requirements: docs/specs/001-initial-implementation/requirements.md
- Go style: /golang skill. TDD: /tdd skill (vertical slices, one test -> one impl).
Design ref: cli-design.md section 6.5. Mirrors terminology ValidateFields/ProjectFields.
Depends on P0-3 (same package, Result type).

## Design

Public surface:

- func ValidateFields(paths string, envelope any) ([]string, error)  // "" -> nil,nil; invalid path -> wraps output.ErrInvalidField (terr coded, exit 2) listing valid paths
- func ProjectFields(data []byte, fields []string) ([]byte, error)   // re-marshal keeping only requested paths + always schema_version, ok
- var ErrInvalidField = terr.New("invalid_field", 2, "see `codelens schema --command CMD` for valid paths", "unknown field path")
- func EmitProjected(w io.Writer, envelope any, fieldsStr string) error  // "" -> EmitJSON; else validate+project+write

Path model: dotted paths over JSON tags; nested structs and slices of structs supported; a "*" wildcard segment matches all map keys (reflect over json tags). Validation uses reflection to collect valid paths from the envelope type.

TDD cases:

1. TestValidate_Empty: "" -> (nil,nil).
2. TestValidate_TopLevel: "rows" on Result -> ok.
3. TestValidate_Nested: "rows.entity" valid when Rows elem has json:"entity".
4. TestValidate_Invalid: "rows.bogus" -> ErrInvalidField; message lists the valid paths; errors.As Coded exit 2.
5. TestProject_KeepsSchemaAndOK: projecting to ["rows.entity"] still includes schema_version and ok.
6. TestProject_NestedSliceRows: rows projected to only the entity field per row object.
7. TestEmitProjected_EmptyEqualsEmitJSON: EmitProjected(...,"") byte-equals EmitJSON.

## Acceptance Criteria

- Invalid field path returns a coded error (exit 2) that names the offending path and lists valid ones.
- schema_version and ok are always retained after projection.
- All 7 cases pass; make validate green.

## Notes

**2026-07-14T11:17:25Z**

Implemented --fields projection in src/internal/output/fields.go (+fields_test.go). Public surface per spec: ValidateFields(paths, envelope) (""->nil,nil; unknown path -> ErrInvalidField wrapped w/ offending path + sorted valid set, exit 2), ProjectFields(data, fields) (JSON-tree projection, always keeps schema_version+ok), EmitProjected(w, env, "") byte-equals EmitJSON, and var ErrInvalidField (code invalid_field, exit 2). Path model: dotted json-tag paths via reflection; struct fields, slice/array elems (zero elem when empty so empty slices still expose nested paths), and maps contribute a '*' wildcard + present keys. Projection operates on the decoded JSON tree, not Go types, so it is independent of the concrete row type held in Rows(any). All 7 TDD cases pass; make build green. Unblocks cod-9eay (P2-4 formats+fields/rows).
