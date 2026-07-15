---
id: cod-rfa6
status: closed
deps: []
links: []
created: 2026-07-15T00:07:44Z
type: chore
priority: 2
assignee: Andre Silva
tags: [codelens, architecture, deepening]
---
# Delete the test-only output registry

Architecture-review candidate 2. `internal/output/registry.go` is a shallow module
whose interface (8 exported funcs + a documented concurrency contract) exceeds its
map-get/set implementation, and whose stated purpose — letting `schema` reflect a
command's envelope shape and exit-code set without importing the command packages — is
never realized. Every reader is a test. Delete it; `schema` already surfaces
`exit_codes` from the descriptors.

Skills: /tdd (retarget the conformance guard test-first), /golang (idioms),
/llm-coding (surgical deletion, no behavior change, verifiable success).

## Design

### Evidence (verified)

- **Envelope half** (`RegisterEnvelope`/`EnvelopeFor`/`AllEnvelopes`): zero production
  callers — not even a writer. Fully dead.
- **Exit-code half** (`RegisterExitCodes`/`ExitCodesFor`/`AllExitCodes`): the only
  production writer is `cmd/codelens/exitcodes.go` (`init` -> `registerExitCodes`). No
  production reader — `ExitCodesFor`/`AllExitCodes` are read only in tests.
- `schema` emits `exit_codes` straight from the descriptors: `analysis.Schema(d)` and
  `analysis.List(analysis.All(), metaSummaries())` (`internal/analysis/schema.go`), not
  from this registry. The registry is redundant.
- Deletion test: removing the registry concentrates no complexity — nothing crosses
  the seam in production. One adapter (the test), so no real seam.

### Delete

```text
internal/output/registry.go          (both maps, mutex, deep-copy helpers)
internal/output/registry_test.go     (Roundtrip / IsCopy / DeepCopy — tests the
                                       registry's own mechanics only)
cmd/codelens/exitcodes.go            (init + registerExitCodes; sole registry writer)
```

Keep `metaSummaries()` (it lives in `schema.go` and feeds the schema command list;
unrelated to the registry).

### Retarget the conformance guard

`cmd/codelens/exitcodes_test.go` holds two tests:

- `TestExitCodesRegistered_AllCommands` ties three surfaces together: (1) each
  descriptor declares a non-empty exit-code set including `0` and a non-empty
  error-code set; (2) the output registry echoes the exit-code set; (3) `schema`
  surfaces exactly those codes. **Only (2) depends on the registry, and it exists only
  to prove the init wiring ran — the wiring we are deleting.** Drop (2); keep (1) and
  (3), which are the load-bearing invariants and are registry-independent.
- `TestSchema_ReportsDeclaredErrorCodes` reads `schema` output only — already
  registry-independent. Keep unchanged.

Retargeted `TestExitCodesRegistered_AllCommands`:

- Per analysis (`analysis.All()`): assert `d.ExitCodes` non-empty and includes `0`;
  `d.ErrorCodes` non-empty; `schemaOf(d.Name).ExitCodes == d.ExitCodes` and
  `.ErrorCodes == d.ErrorCodes`. (Remove the `output.ExitCodesFor` block.)
- Per meta command (`metaSummaries()`): they do not resolve through
  `analysis.Lookup`, so `schema --command <meta>` is a usage error — verify via the
  full command list instead. Decode `codelens schema` (no `--command`) into the
  existing `schemaList` type (`schema_test.go`), find each meta command by `Name`, and
  assert its `ExitCodes` equal the `metaSummaries()` entry. This keeps the guarantee
  that `metaSummaries` flows into the schema list.
- The `output` import in the test file becomes unused — remove it.

Rename the file to `schemacodes_test.go` (its subject is now the schema-surfaced
codes, and `exitcodes.go` no longer exists). `schemaOf`/`schemaCmd`/`schemaList` and
`run(...)` are shared in package `main`, so the move is import-free.

### TDD plan (/tdd)

1. Red: retarget `TestExitCodesRegistered_AllCommands` to descriptors + schema
   (per-analysis and meta-via-list) before removing the registry; it must still pass
   against the current tree except the removed `output.ExitCodesFor` assertions.
2. Delete `output/registry.go`, `output/registry_test.go`, `cmd/codelens/exitcodes.go`.
3. Green: `make build` — the compiler flags any leftover reference (there are none in
   production; only the test import to remove).

### Parity / behavior

- No runtime behavior change: `schema` output is byte-identical (it never read the
  registry). No CLI, envelope, or exit-code behavior changes.
- Net: -3 files, -1 `init`, one mutex removed, conformance guard preserved with a
  tighter (registry-free) surface.

### Files touched

```text
DELETE internal/output/registry.go
DELETE internal/output/registry_test.go
DELETE cmd/codelens/exitcodes.go
RENAME cmd/codelens/exitcodes_test.go -> cmd/codelens/schemacodes_test.go
       (retarget TestExitCodesRegistered_AllCommands; drop output import)
```

## Acceptance Criteria

- `internal/output/registry.go`, `internal/output/registry_test.go`, and
  `cmd/codelens/exitcodes.go` are deleted; no production or test code references
  `RegisterEnvelope`/`EnvelopeFor`/`AllEnvelopes`/`RegisterExitCodes`/`ExitCodesFor`/
  `AllExitCodes`.
- The conformance guard survives as a registry-free test: every analysis declares a
  non-empty exit-code set including `0` and a non-empty error-code set, and `schema`
  surfaces exactly those; every meta command's exit codes appear in the `schema`
  command list. A new analysis added without codes still trips the test.
- `schema` output is unchanged (byte-identical); no runtime behavior change.
- `make build` green (validate + compile).

## Notes

**2026-07-15T00:15:55Z**

Deleted the test-only output registry (internal/output/registry.go + registry_test.go) and its sole writer (cmd/codelens/exitcodes.go with its init). No production code read the registry: schema surfaces exit_codes/error_codes straight from analysis descriptors via analysis.Schema/analysis.List. Retargeted the conformance guard into cmd/codelens/schemacodes_test.go (renamed from exitcodes_test.go): per-analysis it now asserts descriptor ExitCodes non-empty incl 0, ErrorCodes non-empty, and schema --command surfaces exactly those; meta commands are verified by decoding 'codelens schema' (no --command) into schemaList and matching each metaSummaries() entry by name (added schemaListOf helper). Dropped the output import from the test. Verified: no references to the 6 removed funcs anywhere, schema output unchanged, make build green.
