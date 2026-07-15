---
id: cod-ll9s
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
# P0-5 output: command registry

Command registry so schema introspection can reflect each command's envelope and exit-code set.

New files: src/internal/output/registry.go, registry_test.go.

Docs (repo-root relative):

- Plan task: docs/specs/001-initial-implementation/plan.md (Phase 0)
- Design: docs/cli-design.md
- Requirements: docs/specs/001-initial-implementation/requirements.md
- Go style: /golang skill. TDD: /tdd skill (vertical slices, one test -> one impl).
Design ref: cli-design.md section 8 (schema). Mirrors terminology registry.
Depends on P0-3.

## Design

Public surface (package-level maps guarded by simple funcs; populated at init by each command):

- func RegisterEnvelope(command string, zero any)
- func EnvelopeFor(command string) (any, bool)
- func AllEnvelopes() map[string]any            // returns a copy
- func RegisterExitCodes(command string, codes []int)
- func ExitCodesFor(command string) ([]int, bool)  // returns a copy
- func AllExitCodes() `map[string][]int`             // deep copy

TDD cases:

1. TestRegisterEnvelope_Roundtrip: register "authors" zero -> EnvelopeFor returns it, ok true; unknown -> ok false.
2. TestAllEnvelopes_IsCopy: mutating the returned map does not affect subsequent AllEnvelopes.
3. TestRegisterExitCodes_Roundtrip + copy: ExitCodesFor returns a copy; mutating it doesn't change stored.
4. TestAllExitCodes_DeepCopy: mutating inner slice of returned map doesn't affect stored.

## Acceptance Criteria

- Register/lookup works; returned maps/slices are copies (mutation-safe).
- All 4 cases pass; make validate green.

## Notes

**2026-07-14T11:29:48Z**

Implemented src/internal/output/registry.go + registry_test.go. Public surface: RegisterEnvelope/EnvelopeFor/AllEnvelopes and RegisterExitCodes/ExitCodesFor/AllExitCodes. All returned maps/slices are copies (AllEnvelopes shallow-copies the map but shares zero-struct values; AllExitCodes deep-copies map+slices; ExitCodesFor returns a slice copy). Guarded by a package-level sync.RWMutex so concurrent readers are safe under -race (registration is expected at init, reads afterward). All 4 specified TDD cases pass; make build + make test-race green. Unblocks cod-x9ol (schema introspection).
