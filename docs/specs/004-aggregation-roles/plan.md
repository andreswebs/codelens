# Implementation plan: aggregation roles for the semantic vocabulary

Status: FINAL (decisions resolved 2026-07-28). Ready to ticket.

Scope: name the package's three enums as Go types, add a pure function over the
semantic enum stating how a value carrying that semantic may be combined, publish it
through `codelens schema`, warn when `--temporal-period` invalidates a count, and
consume the vocabulary in a guard in the skill's Python scripts. Borrowed in concept
from `flint-chart`'s `aggRole`, deliberately not borrowed in shape.

Inputs read: `internal/analysis` (`semantics.go`, `shape.go`, `analysis.go`, and all
18 descriptors), `internal/transform/{group,temporal,teammap}`,
`internal/command/commands.go`, `internal/output/{types.go,meta.go}`, the five renderer
scripts and `digest.py` under `docs/skills/codelens/scripts/`, Flint's
`packages/flint-js/src/core/{type-registry,field-semantics,aggregate}.ts`,
`docs/specs/002-data-output/plan.md` decisions D3/D3a/D3c, ADR 0008's reachable-only
rule, and `docs/specs/003-flint-adapter/plan.md`.

**Read section 1 first.** The defect that motivated this plan does not exist, and
section 1.1 records two sites that DO sum an intensive column and are nonetheless
correct. The case for the work rests on exhaustiveness and checkability, not repair;
section 10 states that at its true strength and no higher.

## 1. What was verified, including the part I got wrong

The idea came from a suspicion that `--group` might sum ownership ratios. It does
not. Verified 2026-07-27:

| Check                            | Result                                                                                                                         |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `group.Apply` signature          | Operates on `[]model.Modification`, i.e. the parsed log, rewriting `m.Entity` to the layer name. It never touches output rows. |
| `temporal.Apply` signature       | Same: `[]model.Modification`, upstream of analysis.                                                                            |
| `teammap.Apply` signature        | Same.                                                                                                                          |
| `main-developer` under `--group` | `ownership` is `1`, with `added == total_added == 11861`. The ratio was RE-DERIVED at layer level, not averaged or summed.      |

The architecture is why. Every transform runs on the modification stream _before_
`Run`, so each analysis re-derives its measures from transformed events. There is
no row-merging step in which a ratio could be summed. `group.go`'s package comment
states the intent outright: transforms run "before analysis when --group is
supplied, so downstream analyses aggregate at the layer level."

**No Go code aggregates output rows.** That is a load-bearing fact, and section 6
turns on it.

### 1.1 Two sites DO sum an intensive column

Revision 1 asserted that every aggregation honoured the additive/intensive
distinction. An exhaustive scan of the skill scripts contradicts it:

| Site                              | Code                                            | Column                        |
| --------------------------------- | ----------------------------------------------- | ----------------------------- |
| `dev_network.py:85-86`            | `strength_total[a] += s`                        | `communication.strength`, declared `percentage` |
| `pair_matrix.py:67-68`            | `involvement[a] += w`                           | `--weight-col`, documented as `degree` or `strength`, both `percentage` |

Neither is a defect, and the reason they are safe is the most important design
input in this plan: **in both cases the sum is used ORDINALLY, never as a value.**

- `pair_matrix.py` uses `involvement` only for `sorted(involvement, ...)[:top]`,
  selecting which entities appear in the matrix. The number is never rendered.
- `dev_network.py` uses `strength_total` as `nodes[].val`, a node radius. It is
  weighted degree centrality: monotonic in both link count and link strength,
  which is what a node size should be. No consumer reads the magnitude.

So the property held, but by instinct rather than by check, and revision 1
concluded it held without verifying. That is the actual argument for this work: not
that a defect exists, but that a correctness property everyone assumes has never
been established, and assuming it is exactly the failure mode.

### 1.2 Sites confirmed correct

For completeness, so a future reader does not re-derive the scan:

| Site                                 | Assessment                                                             |
| ------------------------------------ | ---------------------------------------------------------------------- |
| `digest.py:176-177`                  | Sums `added`/`deleted`, both `loc`, additive. Correct.                 |
| `digest.py:147`                      | Counts `ownership >= 0.999` rather than summing ratios. Correct.       |
| `digest.py:167`                      | `min`/`median`/`max` over ages. Correct: order statistics are legal on an intensive measure. |
| `enclosure.py:61`, `treemap.py:67`   | Sum tokei `size`, additive, and not codelens data at all.              |
| `enclosure.py:339`, `treemap.py:238` | `max`/`min` over weights for normalization. Legal on any measure.      |
| `complexity_trend.py:97`             | Means its own indentation metric, not a codelens column.               |
| `coupling.go:83`, `communication.go:88` | Compute a percentage from two counts. Derivation, not aggregation.  |
| `fragmentation.go:62`                | Computes a fractal value from squares of shares. Derivation.           |

## 2. The gap that is genuinely unsaid

`temporal/temporal.go`'s package comment contains this admission:

> Because windows overlap (step 1), a physical commit is intentionally counted in
> several windows: correct for logical coupling, **wrong for count-based
> analyses**.

That is an aggregation-role statement, in prose, in a Go package comment no
consumer reads. Verified behaviour:

```text
codelens revisions                        -> operating.md n_revs = 9
codelens revisions --temporal-period 3    -> operating.md n_revs = 7
```

The number changed, `transforms: {"temporal_period": 3}` was recorded, and **stderr
was zero bytes**. Nothing marks `n_revs` as no longer meaning "revisions"; under
the transform it means "overlapping windows in which the entity appeared".

The direction is not predictable: dedup within a window pushes counts down, overlap
pushes them up. A consumer cannot compensate, only be warned.

### 2.1 The mechanism that already exists

`command/commands.go` already reasons in this shape, and its doc comment already
draws the distinction:

```go
// adjustForTransforms returns semantics describing what THIS RUN emitted. A
// transform that destroys a field's structural affordance degrades its semantic:
// --group replaces entity paths with layer names, so a filepath (splittable on
// "/") becomes an opaque label. A transform that merely aggregates does NOT:
// --team-map replaces an author with a team, and both are opaque categorical
// actor names, so person stands (that is why only filepath is degraded here).
func adjustForTransforms(sem map[string]string, grouped bool) map[string]string {
```

"A transform that merely aggregates does NOT" is the concept, articulated once, for
one transform, on one semantic. Note the signature: `grouped bool`.
`transformsRecord` records `temporal_period`, but `adjustForTransforms` is never
told about it. That asymmetry is the gap.

### 2.2 The constraint that shapes the design

`analysis.Opts`'s doc comment:

> Group, temporal, and team-map options are absent here because those transforms
> are applied to the modification set by the pipeline before Run is called.

An analysis therefore **cannot** raise the temporal warning: it does not know the
transform ran. Enforcement must live in the command layer, beside
`adjustForTransforms`, the only place holding both the descriptor and the parsed
flags. This is a deliberate layering choice and the plan must not undo it.

## 3. What Flint actually does with `aggRole`

Studied from `packages/flint-js/src/core`. A closed string union, 5 members:

```typescript
export type AggRole =
  | "additive"
  | "intensive"
  | "signed-additive"
  | "dimension"
  | "identifier";
```

It is one field of a per-type registry entry, keyed by semantic type name, never a
per-field annotation:

```typescript
Count:      { t0: 'Measure', t1: 'GenericMeasure', aggRole: 'additive',  ... },
Percentage: { t0: 'Measure', t1: 'Proportion',     aggRole: 'intensive', ... },
ID:         { t0: 'Identifier', t1: 'ID',          aggRole: 'identifier', ... },
```

Four consumers, all pure functions of the type name:

| Consumer                    | What the role decides                                                                                    |
| --------------------------- | -------------------------------------------------------------------------------------------------------- |
| `resolveAggregationDefault` | `additive`/`signed-additive` to `sum`, `intensive` to `average`, `dimension`/`identifier` to `undefined` |
| `resolveStackable`          | `additive` to `'sum'`, `intensive` to `false` except `Percentage` to `'normalize'`, others `false`       |
| `resolveBinningSuggested`   | `identifier` and `dimension` are never binned                                                            |
| scale selection             | log scale only for `additive` with an `open` domain                                                      |

Take: **the role is a property of the type, resolved by a function, never stored per
field**, which is what stops it drifting from the vocabulary it describes; and each
consumer is a **total function over the closed set**, which is what makes the set's
exhaustiveness testable.

Leave: the member list, and the location. Flint's registry bundles the role with
`zeroPad`, `formatClass`, and `visEncodings`, rendering concerns with no business in
a git-log miner (F3 of the adapter plan, D13 of spec-002).

### 3.1 The critical difference in how Flint uses it

**Flint never prohibits anything.** Every consumer above picks a DEFAULT:
`resolveAggregationDefault` returns `'average'` for intensive and moves on. Nothing
in Flint says "you may not sum this".

codelens wants the role as a correctness rule, which is a stronger use than the
source it is borrowed from. That difference is why section 5.4 exists: a naive
port would turn a default-picker into a prohibition, and the first thing it would do
is declare the two sites in section 1.1 wrong.

## 4. The design decision that matters most: a function, not a field

The obvious move is to add `AggRole` to `analysis.Column`, beside `Semantic`. **That
would be wrong**, for four reasons.

1. **It would store derivable data redundantly.** A role is a pure function of the
   semantic: every `ratio` column is intensive, in every analysis, always. Storing
   it across 71 (analysis, column) pairs creates 71 chances to disagree with the
   vocabulary.
2. **It would invite the D3c trap in reverse.** D3c exists because a semantic
   genuinely varies per (analysis, column): `added` is `loc` in `main-developer` and
   `count` in `main-developer-by-revisions`. A role does not vary that way, because
   it varies _with the semantic_, which already captured the difference. Both are
   additive, so the D3c case needs no per-column role.
3. **It would grow the schema surface silently.** A new `Column` field appears in
   `schema --command` output as a side effect rather than as a decision. Section 6.2
   argues that exposure is wanted, but it should be a deliberate shape, not a
   consequence of where the data was stored.
4. **Flint does it as a registry lookup too**, mild evidence the shape is right
   rather than merely convenient.

So: `AggRoleOf(semantic string) string` in `internal/analysis`, beside the enum it
describes.

## 5. Target design

### 5.1 The role set

Four members, not Flint's five.

| Role         | Meaning                                                | Semantics                                     |
| ------------ | ------------------------------------------------------ | --------------------------------------------- |
| `additive`   | Parts sum to a meaningful total                        | `count`, `loc`                                |
| `intensive`  | A level or proportion; a sum is not a meaningful value  | `percentage`, `ratio`, `duration_months`      |
| `dimension`  | A grouping key, not a measure                          | `filepath`, `person`, `date`, `label`, `flag` |
| `identifier` | Neither aggregated nor grouped on                      | `commit_id`, `text`                           |

`signed-additive` is **omitted**, principled rather than lazy: codelens has no
signed measure. `added` and `deleted` are separate non-negative columns, never a net
delta. Declaring a role no semantic can carry repeats exactly the mistake ADR 0008
corrected for the shape enum, whose governing rule is that a member is added by the
change that makes it reachable, never ahead of it. A net-churn column would bring
`signed-additive` with it.

Two assignments were challenged and both were kept, with their reasoning recorded
because the objections were reasonable and will recur.

**`text` is `identifier` (Q4).** The objection was that "identifier" connotes
uniqueness, which free prose lacks. Resolved by defining the role by BEHAVIOUR rather
than by cardinality: `identifier` means "neither aggregated nor grouped on", which is
exactly right for `parse.message`, a commit subject that cannot be summed and whose
grouping would produce one group per row. The doc comment must lead with the
behaviour and explicitly disclaim uniqueness, or the objection returns:

```go
// AggIdentifier marks a value that is neither aggregated nor grouped on. It does
// NOT imply uniqueness: commit_id is unique and text (a commit subject) is not, and
// both are equally unaggregatable. Flint's registry glosses its own identifier role
// as "never aggregate, tooltip only", which is the same idea.
```

**`flag` is `dimension` (Q5).** The objection was that a boolean column is often
usefully summed as a count of true values, which `dimension` forbids. The objection
dissolves on inspection: `sum(flag)` producing "number of binary changes" is really
`count(rows where flag)`. The result is a NEW column whose semantic is `count`,
derived from a filter, not an aggregation of the flag itself. The flag is what you
filter or group BY. Note that `parse.binary` is documented as "omitted when false",
which is filter semantics rather than measure semantics, and Flint independently
registers `Boolean` as `aggRole: 'dimension'`.

### 5.2 The one assignment that must not be copied from Flint

`duration_months` is **intensive**; Flint registers `Duration` as **additive**.

Flint is right for its domain and wrong for ours. A duration meaning "time spent"
is additive. codelens's only `duration_months` column is `code-age.age_months`,
"months since the entity last changed", which is a level, not a quantity. Summing
the ages of 500 files is meaningless; the useful statistics are median and max,
which is how the interpretation guidance already reads it.

This single divergence is the argument for stealing the concept rather than the
table. A borrowed table would have imported a wrong answer silently, on the one
semantic where the two domains disagree.

### 5.3 Shape in Go (Q1 RESOLVED: named types, for all three enums)

All three enums in the package become named string types, not just this one. Q1's
original framing was "named type for `AggRole` only, accepting inconsistency, or
plain strings for consistency". The answer taken was the third option: make the
package consistent at the STRONGER level.

```go
type Semantic string
type Shape string
type AggRole string

const (
    AggAdditive   AggRole = "additive"
    AggIntensive  AggRole = "intensive"
    AggDimension  AggRole = "dimension"
    AggIdentifier AggRole = "identifier"
)

// AggRoles returns the closed set, in declaration order.
func AggRoles() []AggRole

// AggRoleOf returns the aggregation role of a semantic, or "" if s is not a
// member of the semantic vocabulary.
func AggRoleOf(s Semantic) AggRole
```

The specific bug this prevents is why `AggRole` wanted typing most: it is the ONLY
enum in the package that takes a member of ANOTHER enum as input, so it is the only
place two string vocabularies meet in one expression. With plain strings,
`AggRoleOf(c.Semantic) == SemanticCount` compiles and is silently always false.
Typed, it is a compile error.

**Blast radius, measured at 2026-07-27 so nobody re-derives it.** 27 non-test files
reference the two existing enums, but **18 of those are descriptor files containing
only literals**, and a typed string constant satisfies a typed field unchanged. Zero
descriptors use a raw string (`grep -c 'Semantic: "'` returns 0), so no literal needs
touching. The real signature surface is six files:

```text
internal/analysis/analysis.go       Column.Semantic, Descriptor.Shape
internal/analysis/semantics.go      Semantics(), ValidSemantic, SemanticsOf
internal/analysis/shape.go          Shapes(), ValidShape
internal/analysis/schema.go         schema projection
internal/command/commands.go        semanticsFor, adjustForTransforms
internal/command/metacommands.go    schema output
```

plus roughly ten test files. **No golden file changes**, because a named string type
marshals to identical JSON, so there is no `schema_version` concern.

#### 5.3.1 The typing stops at the output boundary

`internal/output/meta.go` states the constraint in its own comment: output does not
import `internal/analysis`, "the dependency runs the other way". Verified still true.

So `output.Result.Shape` stays `string` and `output.Result.Semantics` stays
`map[string]string`, and the COMMAND LAYER converts when it builds the result. The
alternative, giving `output` its own mirrored named types, would duplicate the
vocabulary in two packages, which is exactly the drift section 4 argues against.

This is a decision, not an oversight: the named types buy compile-time safety inside
the package that owns the vocabulary, and the boundary conversion is one or two lines
where `semanticsFor` already runs.

`AggRoleOf` returns `""` for unknown input rather than panicking, because it is
reachable with arbitrary input once a caller reads a semantic from data, unlike
`payloadKey`, whose panic is correct precisely because it is reachable only through
a descriptor bug.

### 5.4 The ordinal problem, and why it is NOT a fifth role

Section 1.1 found two legitimate sums of an intensive column. A rule saying "never
sum an intensive measure" would flag both. The vocabulary needs to accommodate them
without weakening to uselessness.

The tempting fix is a fifth role, `ordinal-only`. **That is wrong**, because it
misplaces the distinction. Ordinal summation is legal for _every_ numeric measure,
additive and intensive alike, so it is not a property that distinguishes one
semantic from another. `percentage` is intensive AND ordinally summable, and a
per-semantic enum cannot hold both.

The distinction is a property of the **call site's intent**, not of the data. So the
role stays a pure function of the semantic, and the _guard API_ expresses intent:

```python
combine_for_value(rows, col, semantics)  # refuses to sum an intensive column
combine_for_rank(rows, col, semantics)   # permits any measure; result is ordinal
```

`pair_matrix.py` and `dev_network.py` call `combine_for_rank` and become
self-documenting: the code states that the number is a ranking key, which is
currently only inferable by reading how the result is used. `digest.py` calls
`combine_for_value` and is checked.

This is the design's best idea, and it comes directly from Flint's difference in
3.1: Flint picks defaults, codelens needs prohibitions, and a prohibition has to
know what the caller is trying to do.

## 6. Consumers

### 6.1 The temporal warning, and why the role alone is not enough

**The role does not deliver this by itself.** The naive rule, "if
`--temporal-period` is set and any column is additive, warn", is wrong:
`sum-of-coupling`'s `soc` is an additive count that is _supposed_ to count windows,
because reinterpreting a revision as a logical change set is the entire purpose of
the flag. Under the naive rule the two analyses the flag exists to serve would be
the loudest warners.

The distinguishing property is whether an analysis treats a revision as a physical
commit or as a logical change set. That is **not derivable from column semantics**.
So the warning needs two inputs: `AggRoleOf` to find affected columns, and a new
per-descriptor fact, provisionally `ChangesetBased bool`, true for `coupling` and
`sum-of-coupling`.

That second part is a `Descriptor` field, which section 4 argued against for roles.
The difference is that this one genuinely varies per analysis and is derivable from
nothing else, the same test D3c passes. Recorded as Q2.

Emitted from the command layer beside `adjustForTransforms`, through the existing
`WarnFunc` sink, naming affected columns in `details` so a consumer branches on data
rather than parsing prose.

### 6.2 Exposure in `schema`, in both forms (BLOCKER RESOLVED)

Revision 1 had this as an open question. It is not one, and the reason is section
1's finding: **no Go code aggregates output rows.** A Go-only `AggRoleOf` has zero
consumers that aggregate anything. Every aggregation in the system is in Python,
and Python cannot call a Go function.

**Where the roles go, resolved 2026-07-28: BOTH schema forms, mirroring the existing
treatment of error codes.** That precedent is exact, and it is the reason to prefer it
over the alternatives: error codes are published as a full global catalog in the list
form (`errors`) AND as a per-command subset (`error_codes`) plus the shared remainder
(`common_error_codes`). Aggregation roles are the same kind of thing, a closed global
vocabulary with per-command relevance.

```jsonc
// codelens schema                      -- the full 12-entry catalog
{ "schema_version": 1, "ok": true, "commands": [...], "errors": [...],
  "aggregation_roles": {
    "count": "additive",    "loc": "additive",
    "percentage": "intensive", "ratio": "intensive",
    "duration_months": "intensive",
    "filepath": "dimension", "person": "dimension", "date": "dimension",
    "label": "dimension",    "flag": "dimension",
    "commit_id": "identifier", "text": "identifier" } }

// codelens schema --command revisions  -- only what this command's columns use
{ "command": "revisions",
  "row_schema": [ {"name": "entity", "semantic": "filepath"},
                  {"name": "n_revs", "semantic": "count"} ],
  "aggregation_roles": { "filepath": "dimension", "count": "additive" } }
```

Three consequences, each of which was a reason to choose this shape:

1. **One `--schema` file still suffices** for the Python guard, because the
   per-command form carries the roles its own columns need. Had the catalog gone only
   in the list form, the guard would need a SECOND input and would reopen the settled
   Q3/Q5 decision.
2. **It fixes an existing hole.** Today `codelens schema` publishes
   `schema_version, ok, commands, errors` and enumerates the 12 semantics NOWHERE:
   they appear only per-column inside `row_schema[].semantic`. A semantic-to-role map
   in the list form implicitly publishes the closed set for the first time.
3. **It is a map, not a per-column field.** Same rationale as section 4: repeating a
   per-semantic fact on each column would restate it up to seven times for one command
   and re-create the drift risk in the wire format. A consumer joins on
   `row_schema[].semantic`, which is a one-line lookup.

Two details settled by existing behaviour rather than by new decision:

- **Flag-gated columns are included** in the per-command subset. `Column.FlagGated` is
  `json:"-"` precisely because "the schema declares the full untransformed vocabulary,
  gated columns included", so the roles must cover every DECLARED column, not only the
  ones a hypothetical invocation would emit. `schema` describes the command;
  `semantics` in the envelope describes the run.
- **The key name is `aggregation_roles` in both forms.** Error codes use different
  names (`errors` versus `error_codes`) because their CONTENT differs: full objects
  with hints versus bare codes. Roles have identical content in both forms, differing
  only in scope, so one name is correct.

Golden-file impact: `internal/command/testdata/schema_list.out` and every
`*_schema.out` (currently `authors_schema.out`) change. Additive only; no
`schema_version` bump, because adding a key breaks no documented field.

The result envelope is **not** touched. `semantics` stays a flat `field -> string`
map, so the Flint adapter's near-identity property is intact.

Read the heading with one qualification, since Q3 resolved as an OPTIONAL
`--schema FILE`. Exposure in `schema` is required in the sense that it is the only
channel by which roles reach the code that aggregates. But because the flag degrades
gracefully, enforcement is available rather than guaranteed: an invocation without
`--schema` has no roles to check against and must pass through. See the Q3 entry in
section 9 for what that costs the correctness claim.

### 6.3 The Python guard

The concrete deliverable of 6.2. A shared helper in the skill scripts providing
`combine_for_value` and `combine_for_rank` per 5.4, plus retrofitting the four
existing aggregation sites (two value, two rank) to use it.

This is the step that converts "roles exist" into "aggregation is checked", and
without it the correctness argument is unfulfilled.

### 6.4 What is NOT a consumer

- **The Flint adapter's `semantic_types` map.** Flint derives aggregation from its
  own registry once the right type is supplied, so the type map needs no roles.
  Worth stating because it is the natural assumption given where this idea came
  from. Its _channel policy_ is a different matter; see section 11.
- **Any `--aggregate` flag on codelens.** Speculative, and it would breach the
  data-plane boundary. Out of scope.

## 7. Tickets

Five tickets. A gates B; B gates C; C gates D. Ticket A is a pure refactor and ticket
B is where the vocabulary and its first consumer land together.

### 7.1 Ticket A: name the three enums

`type Semantic string`, `type Shape string`, `type AggRole string` (5.3), with the
boundary conversion in 5.3.1. **Pure refactor: no behaviour change, no golden diffs.**

Landed first, separately, because a wide mechanical diff reviewed on its own is
trivially checkable, and if the conversion has surprising blast radius that surfaces
before anything semantic is committed to. `make build` is the whole proof.

Acceptance criteria:

- The six signature files in 5.3 converted; all 18 descriptor literals untouched.
- `output` still does not import `internal/analysis`; the conversion happens in the
  command layer (5.3.1).
- **Zero golden-file diffs.** If any `.out` changes, something is wrong; investigate
  rather than running `-update`.

### 7.2 Ticket B: `AggRole`, the exhaustiveness test, and the temporal warning

The vocabulary plus its first consumer, bundled (Q2). Spans the analysis and command
layers.

Acceptance criteria:

- `aggrole.go`: four constants, `AggRoles()`, `ValidAggRole`, `AggRoleOf`, with the
  doc comment carrying BOTH the `duration_months` divergence from Flint (5.2) and the
  behaviour-first definition of `identifier` (5.1). Those are the two assignments a
  future reader will try to "fix".
- **The exhaustiveness test**: every member of `Semantics()` maps to a member of
  `AggRoles()`. This is the invariant the whole plan rests on, and it must land in the
  same change as the function.
- `Descriptor.ChangesetBased bool`, true for `coupling` and `sum-of-coupling`, false
  for the other 16. Required because the role alone cannot distinguish them: `soc` is
  an additive count that is SUPPOSED to count windows (6.1).
- The `--temporal-period` warning, emitted from the command layer beside
  `adjustForTransforms` through the existing `WarnFunc` sink. It must NOT live in an
  analysis: `Opts` deliberately excludes transform state (2.2).
- The warning names the affected columns in `details` so a consumer branches on data
  rather than parsing prose, and uses a snake_case code matching the existing
  convention (`low_signal`, `coupling_all_filtered`).
- **A NEW golden triple for a temporal run.** Verified 2026-07-28: no golden exercises
  `--temporal-period` at all today, so this ticket ADDS coverage rather than updating
  it. That gap is why the warning's absence went unnoticed.

### 7.3 Ticket C: publish roles in `schema`

Section 6.2: `aggregation_roles` in both the list form and the per-command form.

Acceptance criteria:

- Full 12-entry map in `codelens schema`; per-command subset in
  `schema --command CMD`, covering every DECLARED column including flag-gated ones.
- Golden updates to `schema_list.out` and every `*_schema.out`. Additive only, no
  `schema_version` bump.
- The list-form map is itself the wire-level exhaustiveness check: its golden pins all
  twelve semantics, so a thirteenth cannot be added without updating it.

### 7.4 Ticket D: the Python guard and four retrofits

Section 6.3, the step that turns "roles exist" into "aggregation is checked".

Acceptance criteria:

- A shared helper providing `combine_for_value` and `combine_for_rank` per 5.4.
- `digest.py`'s two value-aggregations use `combine_for_value`;
  `dev_network.py` and `pair_matrix.py` use `combine_for_rank` (1.1). Their computed
  output must NOT change: the retrofit makes intent explicit, not different.
- Absent `--schema`, both helpers pass through without error (Q3).
- A test asserting `combine_for_value` REFUSES an intensive column when roles are
  available. That negative assertion is the guard's entire point.

### 7.5 Order

A, then B, then C, then D. **003's ticket 1 can start after B**, which is when the role
vocabulary exists as a settled classification; it does not need C or D. See section 11.

## 8. Non-goals

- No change to the result envelope. `semantics` stays a flat `field -> string` map.
- No `schema_version` bump. The `schema` addition is additive; nothing breaks.
- No change to any transform's behaviour. `--temporal-period` keeps overlapping
  windows; the plan adds a warning, never a different number.
- No per-column role annotation, per section 4.
- The named types do NOT spread past `internal/analysis`. `output.Result.Shape` stays
  `string` and `Semantics` stays `map[string]string`, because `output` deliberately
  does not import `analysis` (5.3.1). Mirroring the types there would duplicate the
  vocabulary in two packages.
- No import of any other Flint registry field. `formatClass`, `zeroBaseline`,
  `zeroPad`, `visEncodings` are chart-compiler concerns.
- No change to what `dev_network.py` or `pair_matrix.py` compute. They are correct;
  the retrofit makes their intent explicit, not different.

## 9. Resolved questions

All resolved. Q3 on 2026-07-27 jointly with 003's Q5; the rest on 2026-07-28. Kept in
full with their answers, because two of them were kept AGAINST a reasonable objection
and the objection will recur.

- **Q1. Named type or plain string constants? RESOLVED: named types, for all three
  enums.** Not just `AggRole`. See 5.3 for the reasoning, the specific bug it prevents,
  and the measured blast radius; 5.3.1 for why the typing stops at the `output`
  boundary.
- **Q2. Is the temporal warning part of this plan or its own ticket? RESOLVED:
  BUNDLED into ticket B.** The alternative was a separate ticket after everything,
  which is what an earlier revision of section 11 assumed. Bundling wins because it
  gives the vocabulary a Go-side consumer ON LANDING. That matters more than
  single-purpose tickets here: without it, tickets A and B ship a vocabulary that
  nothing in Go uses, and section 11.5's residual risk (a vocabulary with no runtime
  enforcement if C and D never land) becomes live. Cost: ticket B carries a
  vocabulary, a descriptor flag, a warning, and a new golden triple.
- **Q3. Does the Python guard read `schema` at runtime, or carry its own table?
  RESOLVED: reads it, via an optional `--schema FILE`.** Decided jointly with 003's Q5,
  which is the same question, and recorded in
  `docs/specs/003-flint-adapter/plan.md` section 3.5. A skill script may take a second
  input besides the stdin envelope, provided it DEGRADES GRACEFULLY when the flag is
  absent: no `--schema` means raw column names for 003 and no role checking here, never
  an error. `run.bash` captures `codelens schema --command CMD` once per analysis.

  Two consequences. First, section 6.2's requirement is transportable, so ticket C has
  an agreed delivery mechanism. Second, graceful degradation means the guard cannot be
  a hard invariant at runtime: with no `--schema`, `combine_for_value` has no roles to
  check against and must pass through. The exhaustiveness guarantee therefore lives in
  Go and in the `--schema`-supplied path, not in every possible invocation. That is a
  real limit on the correctness claim in section 10 and should not be overstated.
- **Q4. Is `text` really `identifier`? RESOLVED: yes, with the role defined by
  behaviour.** The uniqueness connotation is disclaimed in the doc comment rather than
  worked around by renaming the role or reclassifying the semantic. See 5.1.
- **Q5. Is `flag` a dimension? RESOLVED: yes.** `sum(flag)` is really
  `count(rows where flag)`, which produces a `count` column rather than aggregating the
  flag. See 5.1.
- **Q6. Should anything state how an intensive column SHOULD be combined? RESOLVED:
  no.** The role stays a prohibition and does not grow into a recipe. Knowing `ratio`
  must not be summed does not say whether to take the median, the max, or to re-derive
  from underlying totals, and re-derivation (what codelens does internally, per section
  1) is the only generally correct answer. A recipe would have to be wrong somewhere.
- **Q7. Should `--group` also warn? RESOLVED: no.** Grouping re-derives correctly, as
  section 1 verified by running it: `main-developer --group` returns `ownership: 1`
  with `added == total_added`, a genuine re-derivation at layer level. There is nothing
  to warn about. `--group` already degrades `filepath` to `label` in `semantics`, which
  is the one thing about it a consumer needs to know, and that already works.

### Where the blocker went

A blocker that was NOT in this list is now resolved in 6.2: **where
`aggregation_roles` lives in `schema` output.** Revision 3 said "a top-level
`aggregation_roles` map" without saying which of the two schema forms, and the two
have different shapes and different golden files. Resolved as BOTH, mirroring error
codes. It is recorded in 6.2 rather than here because it is a contract-shape decision
that belongs with the contract.

## 10. Verdict

Revision 1 argued this might be skippable in favour of a two-line temporal warning.
**Withdrawn.** That warning serves one gap and delivers neither exhaustiveness nor
checkability.

The case now rests on two things, stated at their true strength:

- **Exhaustiveness: strong, and sufficient alone.** The test "every semantic maps to
  a declared role" is a real invariant, and it composes with the conformance test
  already pinning every column's semantic to a member. Chained, they mean every
  column in every analysis has a machine-known aggregation rule. More to the point,
  it makes forgetting impossible: today, adding a 13th semantic requires remembering
  to think about aggregation.
- **Correctness: real, but it is checkability, not repair.** No defect exists. What
  section 1.1 demonstrates is that the property was never established, and that a
  careful reader (revision 1) concluded it held without checking. The two ordinal
  sums stay exactly as they are; the value is that the next one is caught.

What this does **not** buy: any change in current output for the four retrofitted
aggregation sites, and any guarantee for consumers outside this repository, who will
hold an envelope with no roles in it.

One thing Q2's resolution did change in this accounting. Bundling the temporal warning
into ticket B means the plan DOES produce a user-visible behaviour change after all:
a run combining `--temporal-period` with a non-changeset analysis starts emitting a
warning where it previously emitted zero bytes on stderr. That is the one place this
work repairs something rather than merely making it checkable, and it is the reason
the vocabulary never lands as dead code.

## 11. Sequencing against 003-flint-adapter

REVISED 2026-07-27, after all ten of 003's open questions were resolved. **The
recommendation has changed shape**: the answer is no longer "this whole plan before
003" but "this plan's first three steps before 003, and the rest after".

### 11.1 What 003's resolutions changed

Four things, two of which cut against the original ordering.

| 003 change | Effect here |
| --- | --- |
| **Q1 resolved** (Network Graph is an edge table) | The prerequisite spike this plan waited on is DONE. Step 1 of the old order is deleted. |
| **Q5 resolved** (optional `--schema`) | This plan's Q3 is answered, and its step 4 gains an agreed transport. The shared decision in old 11.3 is settled, so that subsection is spent. |
| **003 is now BUILDABLE**, zero open questions | Reverses the readiness balance. This plan still has six open items. Blocking a ready plan behind an unready one is a worse trade than when both were unready. |
| **F5 cut from four object-form semantics to two** | Weakens one of the three couplings below. See 11.2. |

### 11.2 The three couplings, re-checked

The original argument rested on three places where 003 encodes aggregation reasoning
implicitly. Re-examined against revision 2 of that plan:

1. **Measure precedence is a hardcoded semantic list.** 003 section 3.3 still reads
   "`percentage` and `ratio` first (they are the analysis's headline proportion), then
   `loc`, then `duration_months`, then `count`". Unchanged, and still an
   aggregation-role judgment written as an enumeration because no vocabulary existed.
   **HOLDS, and this is now the only strong coupling.**
2. **`ratio -> Percentage` chosen over `Number` for role reasons.** HOLDS, and
   003 revision 2 strengthened it: the plan now states in as many words that "`Number`
   is `additive`, and summing ownership ratios is meaningless, whereas `Percentage` is
   `intensive`". That is role vocabulary already, just not named as such.

   **But the scope of this coupling is narrower than I claimed.** The original text
   said roles would make the semantic map "derivable". They would not. 003's
   `percentage` maps to a BARE string while `ratio` maps to an ANNOTATED one, despite
   the two sharing a role, because the difference is about number FORMATTING and was
   discovered by compiling specs (003 section 3.2.1). So roles explain the type
   choice and say nothing about the annotation choice. Roles make the CHANNEL POLICY
   derivable, not the type map.
3. **The total-mixed-with-components hazard** for `main-developer`,
   `refactoring-main-developer`, `main-developer-by-revisions`, and `entity-effort`.
   Unchanged in 003 section 3.4. HOLDS.

### 11.3 One new coupling, in the other direction

003's spike produced evidence FOR this plan's sharpest design decision, which is
worth recording because it arrived from outside.

Flint's ECharts graph template sizes each node by **summing the edge weight**, and
in the `coupling` case that weight is `degree`, a `Percentage`. So Flint sums an
intensive measure to compute a radius, exactly as `dev_network.py` does by hand
(section 1.1).

That is independent corroboration of 5.4: summing an intensive measure for an
ordinal purpose is idiomatic rather than sloppy, and a role used as a naive
prohibition would flag Flint's own template. It also means the
`combine_for_value` / `combine_for_rank` split is not a local invention to excuse two
scripts; it is the distinction a mature charting library also makes, silently.

### 11.4 Order

The insight that resolves the readiness conflict: 003's channel policy needs the role
VOCABULARY at authoring time, not the role LOOKUP at runtime. Writing "bind the
highest-precedence intensive measure to the value axis" only requires that
additive-versus-intensive be a settled, documented classification of the five measure
semantics. It does not require `schema` to carry them yet.

So:

1. **Ticket A** (name the three enums). Pure refactor, no golden diffs.
2. **Ticket B** (`AggRole`, exhaustiveness test, `ChangesetBased`, temporal warning).
   **003's ticket 1 can start after this.**
3. **003 in full**, with its channel policy written over the role vocabulary rather
   than the hardcoded list of semantic names in its section 3.3.
4. **Tickets C and D** (schema exposure, then the Python guard and retrofits). These
   need the `--schema` plumbing that 003's ticket 3 builds anyway, so they are cheaper
   after it than before it.

Two changes from the earlier ordering, both from decisions taken 2026-07-28:

- Ticket A did not exist. Q1's answer (named types for all three enums) added a
  prerequisite that has nothing to do with 003, so 003 now waits behind two tickets
  rather than one. Ticket A is mechanical and golden-neutral, so the added delay is
  small, but it is real and should not be glossed.
- The temporal warning moved EARLIER, from last to inside ticket B (Q2). That is what
  removes the dead-code window described in 11.5.

### 11.5 The cost, restated

The original cost was "this delays 003, the plan with visible output". Two tickets now
precede it rather than one. Both are small: A is a signature refactor that changes no
golden file, and B is one new file plus a descriptor flag, a warning, and one new
golden triple.

**The residual risk that earlier revisions carried is now closed.** It used to read:
if the vocabulary lands and tickets C and D never do, the project owns a vocabulary
with one authoring-time consumer and no runtime enforcement. Q2's bundling removes
that: ticket B ships the temporal warning, so the vocabulary has a working Go consumer
from the moment it exists. The worst case is now a vocabulary that is used and
enforced in Go but not yet reachable from Python, which is a materially better place
to stall.

The alternative I would still accept: build 003 first in full and retrofit its channel
policy afterward. The retrofit is one function in one new file. What argues against it
is unchanged and is about precedent rather than cost: 003's precedence list would ship
as the authoritative statement of which measures matter most, encoded as a list of
semantic names, and lists like that get copied rather than refactored.
