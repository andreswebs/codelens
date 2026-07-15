---
id: cod-0xx4
status: closed
deps: [cod-f0zl, cod-kyfe, cod-en35, cod-2boa]
links: []
created: 2026-07-14T03:45:34Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-3]
---
# P3-1 pipeline: compose transforms + wire into Action

Compose the transforms into the run pipeline in the correct order (group -> temporal -> teammap) and wire them into the command Action so every analysis honors --group/--temporal-period/--team-map.

New files: src/internal/pipeline/pipeline.go, pipeline_test.go. Edit the Action (P2-3) to call pipeline.

Docs: plan.md (Phase 3), design cli-design.md 4.2; reference docs/research/code-maat.md sections 4 (pipeline order) and 5 (transforms). Skills: /golang /tdd.
Reference: research 4 (order). Depends on P2-3 (Action) and the three transforms (P3-2,P3-3,P3-4).

## Design

Surface:

- func Apply(mods []model.Modification, cfg Config) ([]model.Modification, error)
  Config carries: GroupSpecs []group.Spec (nil=skip), TemporalPeriod int (0=skip), TeamMap map[string]string (nil=skip).
- Order (matches original parse-commits-to-dataset): group -> temporal -> teammap. Each stage a no-op when its config is absent.
- The Action parses the group/team files (using the *-format flags) and passes specs/map + temporal period into pipeline.Apply, then hands the result to descriptor.Run.

TDD cases (pipeline_test.go):

1. TestPipeline_NoOps: empty Config returns input unchanged.
2. TestPipeline_Order: with group+teammap set, grouping happens before team substitution (author remap sees grouped entities; assert both applied).
3. TestPipeline_GroupThenTemporal: grouping then temporal windowing composes.
4. TestPipeline_Integration_ViaCommand: run() `authors --group testdata/layers.txt` end-to-end -> grouped entities in output; `authors --temporal-period 1` and `--team-map` similarly (small e2e).

## Acceptance Criteria

- pipeline applies group->temporal->teammap in order, each skippable; Action wires flags to pipeline.
- authors honors --group/--temporal-period/--team-map end-to-end.
- All cases pass; make validate green.

## Notes

**2026-07-14T12:32:16Z**

P3-1 pipeline done. New package internal/pipeline: Config{GroupSpecs, TemporalPeriod, TeamMap} + Apply() running group->temporal->teammap in code-maat order, each stage skipped when its config is zero (len>0 for slices/maps, >0 for period). Wired into the analysis Action in cmd/codelens/commands.go via new pipelineConfig(cmd) helper (opens --group/--team-map read-only, parses with their *-format flags, reads --temporal-period) and a generic parseDefinition[T] helper. Unreadable --group/--team-map path -> new errFileOpen (input_error, exit 3, details {flag,path}); malformed definitions surface each transform's own coded error. Tests: pipeline_test.go (NoOps, Order, GroupThenTemporal, TemporalError) + cmd/codelens/pipeline_e2e_test.go (group drop+remap, temporal same-day collapse via period 1, team-map author merge, bad-file exit 3). make build green.
