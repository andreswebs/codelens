---
id: cod-joym
status: closed
deps: [cod-g7yh, cod-8h5b]
links: []
created: 2026-07-14T03:42:53Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-2]
---
# P2-1 analysis: descriptor + registry

Define the analysis descriptor and the in-process registry that the command tree, help, and schema are all generated from. This is the extensibility spine: adding an analysis later = registering a descriptor.

New files: src/internal/analysis/analysis.go (types), registry.go, registry_test.go.

Docs: plan.md (Phase 2), design cli-design.md sections 4 (surface), 6 (output), 8 (schema); reference docs/research/code-maat.md section 6 (authors). Skills: /golang /tdd /llm-coding.
Design ref: cli-design.md 8, 9 (registry-driven surface). Depends on output (P0-3) and model (P1-1).

## Design

analysis.go:

- type Column struct { Name, Type, Desc string }
- type Flag struct { Name, Type string; Default any; Required bool; Desc string }
- type Opts struct {  // parsed, effective run options (superset; each analysis reads what it needs)
    MinRevs, MinSharedRevs, MinCoupling, MaxCoupling, MaxChangesetSize int
    Verbose bool
    TimeNow string      // YYYY-MM-dd, "" => today (UTC)
    Expression string   // messages regex
    // group/temporal/teammap are applied in the pipeline BEFORE Run, not here
  }
- type Descriptor struct {
    Name string; Aliases []string; Summary string
    Flags []Flag; RowSchema []Column
    ErrorCodes []string; ExitCodes []int
    Run func(mods []model.Modification, opts Opts) (output.Result, error)
  }

registry.go:

- func Register(d Descriptor)   // panics on duplicate name/alias (programmer error, caught at init)
- func All() []Descriptor       // sorted by Name, copy
- func Lookup(nameOrAlias string) (Descriptor, bool)

Run returns a fully-built output.Result (Analysis set, Params echoing effective opts, Rows a typed slice with snake_case json tags, RowCount set). Truncation (total_count/truncated) and field projection are applied by the output/CLI layer, not Run.

TDD cases (registry_test.go):

1. TestRegister_LookupByName: register a stub -> Lookup("x") ok.
2. TestRegister_LookupByAlias: alias resolves to the same descriptor.
3. TestRegister_DuplicatePanics: registering a duplicate name or alias panics.
4. TestAll_SortedCopy: All() sorted by Name; mutating the returned slice doesn't affect registry.

## Acceptance Criteria

- Descriptor/Column/Flag/Opts types defined and documented.
- Register/Lookup/All behave per spec incl. alias resolution and duplicate panic.
- All 4 cases pass; make validate green.

## Notes

**2026-07-14T10:43:15Z**

Implemented analysis.Descriptor/Column/Flag/Opts (internal/analysis/analysis.go) and the process-global registry (registry.go): Register (panics on duplicate name/alias, incl. within-descriptor dup), Lookup (name or alias -> descriptor), All (sorted-by-Name fresh copy). Registry is a name->Descriptor map plus a byKey alias index, guarded by sync.RWMutex. Run signature is func([]model.Modification, Opts) (output.Result, error); group/temporal/team-map transforms are applied by the pipeline BEFORE Run, truncation/field-projection by the output layer AFTER Run (kept out of Run per design). TDD: 4 cases (name/alias lookup, duplicate panics x3 subtests, sorted-copy). make build + test-race green. Unblocks cod-sskw (authors), cod-x9ol (schema), cod-f0zl (command tree).
