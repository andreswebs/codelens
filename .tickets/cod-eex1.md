---
id: cod-eex1
status: open
deps: [cod-x0fv]
links: []
created: 2026-07-28T16:34:16Z
type: feature
priority: 2
assignee: Andre Silva
tags: [codelens, spec-004, go, semantics]
---
# Aggregation-role vocabulary, exhaustiveness test, and the --temporal-period warning

Add the aggregation-role vocabulary, its exhaustiveness test, `Descriptor.ChangesetBased`,
and the `--temporal-period` warning that consumes them.

Spec: docs/specs/004-aggregation-roles/plan.md section 7.2 (ticket B of five).

## Why the vocabulary

A role states how a value carrying a given semantic may be COMBINED. codelens currently
knows this and says it only in prose: `internal/transform/temporal/temporal.go`'s package
comment admits "correct for logical coupling, wrong for count-based analyses", and
`adjustForTransforms` in `internal/command/commands.go` distinguishes "a transform that
merely aggregates" from one that destroys an affordance. Neither is machine-readable.

IMPORTANT CONTEXT SO NOBODY OVERSELLS THIS: no defect exists. Verified 2026-07-27 that
every aggregation in the repository and the skill is correct, including two that sum an
INTENSIVE column legitimately (see plan section 1.1). The value is exhaustiveness and
checkability, not repair. The one genuine repair is the temporal warning below.

## Why the warning is bundled here

Decision Q2. Without it the vocabulary lands with no Go-side consumer at all, and the
plan's section 11.5 risk (a vocabulary with no runtime enforcement if later tickets
stall) becomes live. Bundling means the vocabulary ships used.

## Why the role alone is not enough for the warning

The naive rule "temporal is set and a column is additive, so warn" is WRONG.
`sum-of-coupling`'s `soc` is an additive count that is SUPPOSED to count windows,
because reinterpreting a revision as a logical change set is the entire purpose of the
flag. Under the naive rule the two analyses the flag exists to serve would be the
loudest warners. Hence `Descriptor.ChangesetBased`.

## Design

## 1. `internal/analysis/aggrole.go` (new file)

Four roles. NOT Flint's five: `signed-additive` is omitted because codelens has no
signed measure (`added` and `deleted` are separate non-negative columns, never a net
delta). Declaring an unreachable member repeats the mistake ADR 0008 corrected for the
shape enum, whose rule is that a member is added by the change that makes it reachable.

```go
const (
    AggAdditive   AggRole = "additive"    // parts sum to a meaningful total
    AggIntensive  AggRole = "intensive"   // a level or proportion; a sum is not a value
    AggDimension  AggRole = "dimension"   // a grouping key, not a measure
    AggIdentifier AggRole = "identifier"  // neither aggregated nor grouped on
)

func AggRoles() []AggRole            // closed set, declaration order
func ValidAggRole(r AggRole) bool    // membership
func AggRoleOf(s Semantic) AggRole   // "" if s is not a member of the vocabulary
```

The full assignment, all 12 semantics:

| semantic          | role         |
| ----------------- | ------------ |
| `count`           | `additive`   |
| `loc`             | `additive`   |
| `percentage`      | `intensive`  |
| `ratio`           | `intensive`  |
| `duration_months` | `intensive`  |
| `filepath`        | `dimension`  |
| `person`          | `dimension`  |
| `date`            | `dimension`  |
| `label`           | `dimension`  |
| `flag`            | `dimension`  |
| `commit_id`       | `identifier` |
| `text`            | `identifier` |

`AggRoleOf` returns `""` for unknown input rather than panicking, because it is
reachable with arbitrary input once a caller reads a semantic from data. Contrast
`payloadKey`, whose panic is correct precisely because it is reachable only through a
descriptor bug.

### TWO DOC COMMENTS ARE MANDATORY, not optional polish

These are the two assignments a future reader will try to "fix". Both objections are
reasonable and were considered and rejected.

**`duration_months` is `intensive`, and Flint registers `Duration` as `additive`.**
Flint is right for its domain and wrong for ours. codelens's only `duration_months`
column is `code-age.age_months`, "months since the entity last changed", which is a
LEVEL, not a quantity. Summing the ages of 500 files is meaningless; the useful
statistics are median and max. Do not align this with Flint.

**`identifier` is defined by BEHAVIOUR, not by uniqueness.** Write it so:

```go
// AggIdentifier marks a value that is neither aggregated nor grouped on. It does NOT
// imply uniqueness: commit_id is unique and text (a commit subject) is not, and both
// are equally unaggregatable. Grouping by free prose yields one group per row.
```

## 2. The exhaustiveness test (same change, non-negotiable)

The function's entire safety property. Landing it separately leaves a window in which
a new semantic silently has no role.

```go
func TestAggRoleOf_Exhaustive(t *testing.T) {
    for _, s := range Semantics() {
        if r := AggRoleOf(s); !ValidAggRole(r) {
            t.Errorf("semantic %q has no declared aggregation role", s)
        }
    }
}
```

Implement `AggRoleOf` as a `switch` with NO `default`, so an unhandled semantic returns
the zero value and this test fires. Also assert `AggRoleOf("nonsense") == ""`.

## 3. `Descriptor.ChangesetBased bool`

`internal/analysis/analysis.go`. True for `coupling` and `sum-of-coupling` ONLY; false
for the other 16 (the zero value, so only two descriptors change).

Doc comment must say WHY it exists: these two analyses interpret a revision as a
logical change set rather than a physical commit, which is exactly what
`--temporal-period` produces, so the transform is correct for them and distorting for
everyone else. That distinction is not derivable from column semantics.

Add a conformance assertion that exactly these two are true, so a new coupling-family
analysis has to think about it.

## 4. The temporal warning

MUST be emitted from the COMMAND LAYER, not from an analysis. `analysis.Opts`'s doc
comment states the constraint: "Group, temporal, and team-map options are absent here
because those transforms are applied to the modification set by the pipeline before Run
is called." An analysis cannot know the transform ran.

The right home is `internal/command/commands.go`, beside `transformsRecord` and
`adjustForTransforms`, which already hold both the descriptor and the parsed flags.
Warnings already emit from there at line ~155 via
`output.EmitWarning(cmd.Root().ErrWriter, code, message, hint, details)`.

Condition: `cmd.Int("temporal-period") > 0 && !d.ChangesetBased` AND the descriptor has
at least one column whose `AggRoleOf(c.Semantic) == AggAdditive`.

Payload, following the existing convention (`low_signal`, `coupling_all_filtered` are
snake_case, subject-first):

```text
code:    temporal_period_recounts
message: counts are per sliding window, not per commit
hint:    --temporal-period reinterprets a revision as a logical change set, so the
         named columns count windows rather than commits; drop it for commit-accurate
         counts, or use it with coupling / sum-of-coupling where that is the intent
details: {"period_days": 3,
          "affected_columns": ["n_revs"],
          "analysis": "revisions"}
```

Name the affected columns in `details` so a consumer branches on data rather than
parsing prose. A warning never alters the exit code (still 0).

Do NOT change what the transform computes. Windows still overlap; the numbers are
unchanged. Verified behaviour to preserve: `revisions` on this repo's log gives
`operating.md n_revs = 9`, and with `--temporal-period 3` gives `7`. The direction is
not predictable (dedup within a window pushes counts down, overlap pushes them up),
which is why a consumer can only be warned, not helped to compensate.

## 5. A NEW golden triple

Verified 2026-07-28: NO golden exercises `--temporal-period` at all. The only matches
for "temporal" in `internal/command/testdata/` are flag descriptions inside
`schema_list.out` and `authors_schema.out`. That missing coverage is why the warning's
absence went unnoticed.

So ADD a triple (`.out`/`.err`/`.exit`) for a temporal run on a non-changeset analysis,
per ADR 0007. The `.err` file is the point of the test. Consider a second triple for
`sum-of-coupling --temporal-period` asserting an EMPTY `.err`, which is the negative
assertion that proves `ChangesetBased` works.

## Acceptance Criteria

- `internal/analysis/aggrole.go` with the four constants, `AggRoles()`,
  `ValidAggRole`, `AggRoleOf`, and the two mandatory doc comments
  (`duration_months` divergence from Flint; `identifier` defined by behaviour).
- All 12 semantics assigned per the table in the design notes.
- Exhaustiveness test passing: every member of `Semantics()` maps to a member of
  `AggRoles()`. `AggRoleOf` has no `default` branch.
- `AggRoleOf` of a non-member returns `""`.
- `Descriptor.ChangesetBased` true for exactly `coupling` and `sum-of-coupling`, with a
  conformance test pinning that.
- `--temporal-period` on a non-changeset analysis with an additive column emits
  `temporal_period_recounts` on stderr, naming the affected columns in `details`.
  Exit code unchanged (0).
- `--temporal-period` on `coupling` or `sum-of-coupling` emits NOTHING.
- The warning is emitted from `internal/command`, not from any analysis;
  `analysis.Opts` gains no transform fields.
- A new golden triple covers a temporal run; ideally a second covers the negative case.
- Transform output numbers are unchanged.
- `make build` green.

