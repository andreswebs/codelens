---
id: cod-qoff
status: closed
deps: [cod-x9ol, cod-yo7b]
links: []
created: 2026-07-14T03:52:50Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-5]
---
# P5-3 cmd: error/exit code registration + conformance

Register each command's error codes and exit codes in the output registry so the "schema" command reports them, and add a conformance test.
Edit: per-analysis descriptors already carry ErrorCodes/ExitCodes; wire registration at init. New: registry conformance test.
Docs: plan.md, design cli-design.md, requirements.md. Skills: /golang /tdd. Reference: cli-design.md 8, 7. Depends on P2-5 (schema) and all Phase 4 analyses registered.

## Design

- At init (or in the command builder), call output.RegisterExitCodes(name, descriptor.ExitCodes) and expose ErrorCodes via the descriptor for schema.
- Conformance test: for every analysis.All(), ExitCodes is non-empty and includes 0; ErrorCodes non-empty; schema --command CMD surfaces exactly those.
TDD:

1. TestExitCodesRegistered_AllCommands.
2. TestSchema_ReportsDeclaredErrorCodes: spot-check coupling (empty_log) and messages (missing_messages, invalid_expression).

## Acceptance Criteria

- All commands register exit/error codes; schema reflects them; conformance test guards future additions. Cases pass; make validate green.

## Notes

**2026-07-14T13:57:10Z**

Wired the previously-dead output command registry: cmd/codelens/exitcodes.go registers every command's exit-code set at package init - analyses from their descriptor's ExitCodes, meta commands (schema/print-log-command/version) from metaSummaries(). Error codes intentionally NOT registered: they live on the descriptor and reach schema via analysis.Schema, keeping a single source per analysis. Conformance in cmd/codelens/exitcodes_test.go: TestExitCodesRegistered_AllCommands ties descriptor <-> output.ExitCodesFor <-> 'schema --command' output for every analysis (asserts ExitCodes non-empty & includes 0, ErrorCodes non-empty), plus meta commands are registered; TestSchema_ReportsDeclaredErrorCodes spot-checks coupling(empty_log) and messages(missing_messages, invalid_expression). Note: the analysis pkg's own tests call resetRegistry() which wipes analysis.All(), so the conformance test lives in cmd/codelens where the registry is fully init'd and never reset. Schema still reads codes from the descriptor (registry is a name-keyed cache for consumers lacking the descriptor); the test guarantees the two agree. make build green; go test -race green.
