---
id: cod-g7yh
status: closed
deps: [cod-jw0u]
links: []
created: 2026-07-14T03:37:08Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-0]
---
# P0-3 output: envelope, emit, error envelope, exit codes

Build the output spine: the result envelope, JSON emission, the structured error envelope, and exit-code resolution. All commands emit through this package.

New files: src/internal/output/types.go (envelope), emit.go, errors.go + _test.go siblings.

Docs (repo-root relative):

- Plan task: docs/specs/001-initial-implementation/plan.md (Phase 0)
- Design: docs/cli-design.md
- Requirements: docs/specs/001-initial-implementation/requirements.md
- CLI framework: docs/research/urfave-cli.reference.md (urfave/cli v3, for classifyUsageError)
- Go style: /golang skill. TDD: /tdd skill (vertical slices, one test -> one impl).
Design ref: cli-design.md sections 6.1 (envelope) and 7 (errors, exit codes 0/1/2/3).
Depends on terr (P0-2).

## Design

types.go:

- const SchemaVersion = 1
- type Result struct {
    SchemaVersion int            `json:"schema_version"`
    OK            bool           `json:"ok"`
    Analysis      string         `json:"analysis"`
    Params        map[string]any `json:"params,omitempty"`
    RowCount      int            `json:"row_count"`
    TotalCount    int            `json:"total_count,omitempty"`
    Truncated     bool           `json:"truncated,omitempty"`
    Rows          any            `json:"rows"`
  }

emit.go:

- func EmitJSON(w io.Writer, v any) error  // json.Marshal + "\n"

errors.go:

- type errorEnvelope { SchemaVersion int; OK bool; Error *errorDetail }
- type errorDetail { Code, Message string; Hint string `json:",omitempty"`; Details any `json:",omitempty"` }
- func EmitError(w io.Writer, format string, err error)  // format "text" -> `"✗ <msg>\n"` + `"  hint: <h>\n"`; else JSON envelope
- func ExitCodeFor(err error) int
- func classifyUsageError(err error) (code, hint string)  // maps urfave/cli v3 arg/flag errors

ExitCodeFor logic: nil -> 0; errors.As Coded -> its ExitCode(); else if classifyUsageError matches -> 2; else -> 1. EmitError resolves code/message/hint from Coded, details from Detailed, and classifyUsageError for uncoded usage errors.

TDD cases:

1. TestEmitJSON_WritesCompactWithNewline: EmitJSON of a small struct -> exact bytes + trailing "\n".
2. TestResult_JSONShape: marshal a Result{RowCount:0,Rows:[]} -> contains "schema_version":1,"ok":true,"rows":[]; total_count and truncated OMITTED when zero/false.
3. TestResult_TruncatedShape: Result{RowCount:10,TotalCount:812,Truncated:true} -> both fields present.
4. TestEmitError_JSON_Coded: a terr coded error -> envelope {ok:false,error:{code,message,hint}}.
5. TestEmitError_Text_Coded: format "text" -> `"✗ <message>\n  hint: <hint>\n"`.
6. TestExitCodeFor: nil->0; coded(exit 3)->3; a synthesized urfave usage error->2; generic errors.New->1.
7. TestEmitError_Details: a Detailed error includes its details object under error.details.

## Acceptance Criteria

- Result marshals with the exact snake_case keys and omitempty behavior in the design.
- EmitError writes to the given writer (tests use a bytes.Buffer) in both text and JSON.
- ExitCodeFor returns 0/1/2/3 per taxonomy.
- All 7 cases pass; exported symbols documented; make validate green.

## Notes

**2026-07-14T10:21:58Z**

Implemented internal/output: types.go (Result envelope, SchemaVersion=1), emit.go (EmitJSON = compact json + trailing newline), errors.go (errorEnvelope/errorDetail, EmitError, ExitCodeFor, classifyUsageError). All 7 TDD cases pass + a bonus TestEmitError_UsageErrorClassified. Design notes for downstream: (1) EmitError kept its void signature; its single sink write uses an explicit '_, _=' acknowledged best-effort write (stderr sink, unrecoverable), which the .golangci.yml text sanctions as distinct from silencing a handleable error. Internal marshaling errors fall back to text rendering, never swallowed. (2) ExitCodeFor: nil->0, Coded->ExitCode(), classifyUsageError match->2, else->1. (3) classifyUsageError matches urfave/cli v3 substrings: 'flag provided but not defined', 'no such flag', 'not set' (required flags), 'invalid value'; returns code 'usage_error' + a --help hint. Uncoded non-usage errors render code 'internal_error'. Detailed errors surface .details. Blocks: cod-joym, cod-ll9s, cod-ic8y, cod-vdx3, cod-vqjh now unblocked.
