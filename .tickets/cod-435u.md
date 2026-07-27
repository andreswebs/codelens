---
id: cod-435u
status: open
deps: [cod-4typ]
links: []
created: 2026-07-27T13:10:47Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-z4wu
tags: [codelens, spec-002, output, semantics]
---
# Envelope: semantics map and transforms record

Add `semantics` (a field-to-semantic-type map) and `transforms` (a record of the active
pipeline transforms) to the envelope, and a per-column `semantic` to `schema --command`.
This is the payoff ticket of the ADR 0008 rollout: `semantics` is the asset only codelens
can provide, because it authored the data, and it is what makes a downstream chart spec
derivable without domain knowledge.

Implements decisions D3, D3a, D3b, D3c, D4, D4a, D4b, D6, D9, and D15 from
docs/specs/002-data-output/plan.md section 2. The load-bearing ones:

- D3/D3b: a semantic is a BARE closed enum string, not an object with units and range.
  This is exactly the flat `field -> string` map Flint's `semantic_types` expects, so the
  downstream adapter stays near-identity. The closed vocabulary has 12 members.
- D3c: semantics are assigned per (analysis, column), NEVER per column name globally.
  `added` and `total_added` are line counts (`loc`) in `main-developer` but revision counts
  (`count`) in `main-developer-by-revisions`. This is the single easiest thing to get wrong
  in this ticket; the design section carries the full 71-pair assignment table so it does
  not have to be re-derived.
- D4/D4a: under `--group`, the EMITTED semantics report `entity` as `label`, not
  `filepath`, because a layer name is not a splittable path and a treemap builder that
  splits it on `/` produces garbage. The governing rule is: degrade a semantic only when a
  STRUCTURAL AFFORDANCE is lost. `--team-map` therefore does NOT degrade `author`: a team
  name and a person name are both opaque categorical actor labels, so `person` stays.
- D4b: the envelope gains a `transforms` object recording which pipeline transforms ran,
  omitted entirely when the pipeline was a pass-through. `params` keeps its current meaning
  (per-analysis tuning only), so existing goldens and `--fields params.*` are unaffected.
- D15: the `semantics` map lists every declared column MINUS those excluded by an
  invocation flag. Governing rule: SEMANTICS TRACK FLAGS, NEVER DATA. So `coupling` without
  `--verbose` omits its three verbose columns, while `parse` always lists
  `loc_added`/`loc_deleted`/`binary` (whose absence is per-row, i.e. data). The map is
  therefore deterministic for a given command line, which the goldens depend on.
- D6: under `--fields`, `semantics` is retained FILTERED to the surviving payload fields,
  and `transforms` is retained whenever present, because it is what justifies an adjusted
  semantic.

Note the deliberate asymmetry introduced by D4: `schema --command CMD` declares the
UNTRANSFORMED default (`entity` is `filepath`), while the envelope declares what THIS RUN
emitted (`entity` may be `label`). That is not drift: the schema states the command's
default, the envelope states the invocation. Document it where both are described.

Reference: docs/specs/002-data-output/plan.md section 6 (this ticket's step list), section
3.2 (the vocabulary), section 3.3 (the conformance obligations).
Skills: /golang, /tdd, /llm-coding.

## Design

Depends on the `shape` ticket: `Result.MarshalJSON`, `output.Meta`, and the JSON-tree
`--fields` path collection must already exist, with the `semantics` and `transforms` slots
reserved in the marshaler's key order.

### 1. The vocabulary: `internal/analysis/semantics.go` (new)

```go
// A semantic names what a payload field MEANS, as distinct from its JSON type: a
// filepath rather than a string, a percentage rather than an int. It is the asset only
// codelens can provide, because it authored the data, and it is what lets a downstream
// renderer derive a chart without domain knowledge.
//
// A semantic is a bare enum string; range and unit are implied by the name and fixed
// here, so the map projects to a chart-spec input (Flint's semantic_types) unchanged:
//   - Percentage is an integer 0-100; Ratio is a float 0-1. A field is one or the
//     other, never both.
//   - Count is a tally of things; Loc is a count of LINES. The split is not
//     cosmetic: lines are the conventional size channel of a treemap while
//     frequencies are the colour channel, and a renderer cannot tell them apart from
//     the type alone.
const (
    SemanticFilepath       = "filepath"        // repository path, splittable on "/"
    SemanticPerson         = "person"          // actor name (an author, or a team under --team-map)
    SemanticDate           = "date"            // calendar date, YYYY-MM-dd
    SemanticCommitID       = "commit_id"       // opaque commit identifier
    SemanticText           = "text"            // free prose; never a plottable category
    SemanticLabel          = "label"           // categorical name
    SemanticFlag           = "flag"            // boolean
    SemanticCount          = "count"           // tally of things
    SemanticLoc            = "loc"             // line count (a size measure)
    SemanticPercentage     = "percentage"      // integer 0-100
    SemanticRatio          = "ratio"           // float 0-1
    SemanticDurationMonths = "duration_months" // whole calendar months
)

// Semantics returns the closed set. A conformance test pins every declared column's
// Semantic to a member, so a new analysis cannot invent one.
func Semantics() []string

// ValidSemantic reports whether s is a member of the closed set.
func ValidSemantic(s string) bool
```

### 2. `internal/analysis/analysis.go`: `Column.Semantic`

Add to the `Column` struct (line 22), between `Type` and `Desc`, matching the
docs/cli-design.md section 8 example's key order (`name`, `type`, `semantic`, `desc`):

```go
// Semantic names what the field means as opposed to how it is encoded (see
// Semantics). It is what makes a downstream chart spec derivable without domain
// knowledge, so it is required on every column.
Semantic string `json:"semantic"`
```

### 3. The assignment table (71 pairs, 18 analyses)

Fill `Semantic` on every column of every descriptor. This table is authoritative and was
derived from the actual `RowSchema` declarations; do not re-derive it.

```text
absolute-churn (abschurn.go)
  date                     date
  added                    loc
  deleted                  loc
  commits                  count

author-churn (authorchurn.go)
  author                   person
  added                    loc
  deleted                  loc
  commits                  count

authors (authors.go)
  entity                   filepath
  n_authors                count
  n_revs                   count

code-age (codeage.go)
  entity                   filepath
  age_months               duration_months

communication (communication.go)
  author                   person
  peer                     person
  shared                   count
  average                  count
  strength                 percentage

coupling (coupling.go)
  entity                   filepath
  coupled                  filepath
  degree                   percentage
  average_revs             count
  first_entity_revisions   count
  second_entity_revisions  count
  shared_revisions         count

entity-churn (entitychurn.go)
  entity                   filepath
  added                    loc
  deleted                  loc
  commits                  count

entity-effort (entityeffort.go)
  entity                   filepath
  author                   person
  author_revs              count
  total_revs               count

fragmentation (fragmentation.go)
  entity                   filepath
  fractal_value            ratio
  total_revs               count

main-developer (maindev.go)
  entity                   filepath
  main_dev                 person
  added                    loc
  total_added              loc
  ownership                ratio

main-developer-by-revisions (maindevbyrevs.go)
  entity                   filepath
  main_dev                 person
  added                    count      <-- REVISIONS, not lines (D3c)
  total_added              count      <-- REVISIONS, not lines (D3c)
  ownership                ratio

messages (messages.go)
  entity                   filepath
  matches                  count

entity-ownership (ownership.go)
  entity                   filepath
  author                   person
  added                    loc
  deleted                  loc

parse (parse.go)
  entity                   filepath
  rev                      commit_id
  date                     date
  author                   person
  message                  text
  loc_added                loc
  loc_deleted              loc
  binary                   flag

refactoring-main-developer (refactoringmaindev.go)
  entity                   filepath
  main_dev                 person
  removed                  loc
  total_removed            loc
  ownership                ratio

revisions (revisions.go)
  entity                   filepath
  n_revs                   count

sum-of-coupling (soc.go)
  entity                   filepath
  soc                      count

summary (summary.go)
  statistic                label
  value                    count
```

Two judgement calls worth preserving as comments where they occur:

- `main-developer-by-revisions`: `added`/`total_added` are `count`, because that analysis
  counts revisions while `main-developer` counts lines. Add a short inline comment on
  those two columns, since the identical column names across the two files invite a
  copy-paste error. Their `Desc` strings already say "revisions by the main developer",
  which is the corroborating evidence.
- `summary`: `statistic` is a `label` (the categorical key of a key-value table) and
  `value` is a `count`. `summary` is the one analysis whose payload is a metric-name and
  metric-value pair rather than an entity row.

### 4. Deriving the map: `SemanticsOf`

```go
// SemanticsOf projects a descriptor's declared columns to the field-to-semantic map an
// envelope carries, omitting any column excluded by the invocation. Semantics track
// FLAGS, never data: a column gated behind a flag that was not supplied is absent (its
// field will never appear in any row), while a column whose presence varies per row
// (parse's loc metrics) is always listed. That keeps the map deterministic for a given
// command line.
func SemanticsOf(d Descriptor, omit map[string]bool) map[string]string
```

Only one analysis needs the `omit` set today: `coupling` without `--verbose` omits
`first_entity_revisions`, `second_entity_revisions`, and `shared_revisions`. Rather than
hardcoding those names in the command layer, declare the gate on the descriptor. Preferred
shape: add an optional field to `Column`:

```go
// FlagGated names the flag a column's presence depends on. An empty value means the
// column is always declared. It is what lets SemanticsOf omit a column that cannot
// appear, without the output layer knowing which analysis it is describing.
FlagGated string `json:"-"`
```

Set `FlagGated: "verbose"` on coupling's three verbose columns. `json:"-"` keeps it out of
the schema, which declares the full untransformed vocabulary. The command layer then builds
`omit` from the descriptor's gated columns whose flag is false, with no analysis-specific
knowledge.

### 5. The runtime adjustment (D4/D4a)

```go
// adjustForTransforms returns semantics describing what THIS RUN emitted. A transform
// that destroys a field's structural affordance degrades its semantic: --group replaces
// entity paths with layer names, so entity stops being a splittable filepath and
// becomes a label. A transform that merely aggregates does NOT: --team-map replaces an
// author with a team, and both are opaque categorical actor names, so person stands.
//
// This is why the schema and the envelope can disagree: the schema declares the
// command's default, the envelope declares the invocation.
```

Rule as implemented: when `--group` is active, any column whose declared semantic is
`filepath` becomes `label`. Apply it generically over the map rather than naming `entity`,
so `coupling`'s `coupled` column is covered too (it is equally a layer name after
grouping). Nothing else adjusts.

Home for this: the command layer, next to where the pipeline config is built, since that
is what knows which transforms ran. Keep it a pure function of (map, activeTransforms) so
it is trivially testable.

### 6. `transforms` (D4b)

Built from the same flags `pipelineConfig` reads (`internal/command/commands.go:185`).
Include a key ONLY when its transform actually ran, and omit the whole object when none
did:

```json
"transforms": {
  "include": ["src/**"],
  "exclude": ["**/vendor/**"],
  "group": true,
  "temporal_period": 30,
  "team_map": true
}
```

- `include`/`exclude`: the raw glob slices, present only when non-empty.
- `group`/`team_map`: `true` when the flag supplied a file. Booleans rather than paths: a
  local filesystem path in an envelope is noise for a consumer and leaks the caller's
  layout.
- `temporal_period`: the integer, present only when non-zero.

Keys are snake_case, matching every other envelope key, even though the flags are
kebab-case. That mirrors the existing convention that column keys are snake_case JSON;
note it in the doc comment, since `params` does the opposite (it is keyed by FLAG name, so
`min-coupling`, and that stays as it is for compatibility).

### 7. `internal/output`: envelope and Meta

- `Result` gains `Semantics map[string]string` and `Transforms map[string]any`.
- `MarshalJSON` fills the two reserved slots: `semantics` after `shape` (always present),
  `transforms` after `semantics` (omitted when empty).
- `output.Meta` gains `Semantics map[string]string` and `Transforms map[string]any`.
- `Semantics` is always present, even when empty, so a consumer can read it
  unconditionally; it marshals as `{}` rather than `null` (the ADR 0006 rule that absent
  maps are `{}`, enforced by the envelope's own marshaler rather than call-site
  discipline).

### 8. `internal/output/fields.go` (D6)

- Retain `transforms` whenever present, alongside the existing `schema_version`, `ok`, and
  `shape`.
- Filter `semantics` to the payload fields that survived the projection:

```go
// projectSemantics narrows the semantics map to the payload fields the projection kept,
// so a projected envelope stays self-describing without advertising fields it dropped.
// A projection that keeps no payload field yields an empty map, not a missing key.
```

Implementation: after building the projection tree, determine the surviving payload field
names. If the payload subtree is a leaf (the whole payload was requested, `--fields rows`)
or carries the wildcard, keep every semantic. Otherwise intersect the semantics keys with
the subtree's keys. Reuse `payloadKey(shape)` to find the payload subtree.

### 9. `internal/analysis/schema.go`

No struct change is needed beyond `Column.Semantic` flowing through `Schema(d)`, since
`RowSchema` is carried whole. Verify `TestSchema_JSONKeys`
(`internal/analysis/schema_test.go:64`) and the `schemaCmd` fixture in
`internal/command/schema_test.go:41` pick up the new per-column key.

`schema --command` declares the UNTRANSFORMED semantic. It does not vary with the
invocation, and it declares gated columns too (the full vocabulary).

### 10. Conformance tests (the anti-drift spine)

Follow the style of `internal/command/registry_guard_test.go`, which pins the error
registry, and extend `TestSchema_Conformance`
(`internal/command/schema_test.go:191`), which already sweeps `analysis.All()`:

1. Every declared column on every registered descriptor has a NON-EMPTY `Semantic` that is
   a member of `Semantics()`. This is what stops a new analysis from shipping without
   semantics.
2. The envelope's `semantics` keys equal the descriptor's declared columns after the D15
   flag filter: run `coupling` and assert 4 keys, run `coupling --verbose` and assert 7.
3. `parse` always declares all 8, including on a log with no numstat (where `loc_added`
   and `loc_deleted` are absent from every row). This pins "semantics track flags, not
   data".
4. `--group`: `entity` (and `coupled`) become `label` in the envelope, while
   `schema --command` still reports `filepath`. Assert BOTH halves in one test, since the
   deliberate asymmetry is the thing most likely to be "fixed" by a later reader.
5. `--team-map`: `author` stays `person` (the D4a rule's negative case).
6. `transforms` is absent for a plain run and carries exactly the expected keys for a run
   with `--group`, `--exclude`, and `--temporal-period`.
7. `--fields rows.entity` yields a `semantics` map of exactly one entry; `--fields rows`
   yields the full map.

### 11. Goldens

Regenerate and hand-review. Expected diff: every successful `*.out` gains `"semantics":
{...}` after `"shape"`. `authors_schema.out` gains a `"semantic"` key inside each
`row_schema` entry. `authors_fields.out` gains a one-entry `semantics` (the D6 filter).
`schema_list.out` unchanged. All `.err` and `.exit` unchanged.

Add two scenarios so the new behaviour is frozen:

```go
// grouped: pins the D4 semantic degradation and the transforms record together.
{"authors_grouped", []string{"--group", <group file>, "authors"}, authorsLog},
// verbose coupling: pins the D15 flag-gated semantics (7 entries, not 4).
{"coupling_verbose", []string{"--verbose", "coupling"}, <a log with a coupled pair>},
```

The grouped scenario needs a group-spec file. The golden harness supports `<TMPDIR>`
substitution in args (see `tmpToken` in `internal/command/golden_test.go:31`) but only for
paths it does not have to CREATE. Check how `input_file_open_failed` does it: it names a
missing file deliberately. For a group file that must exist, either add a small fixture
under `testdata/` and reference it by relative path, or extend the harness. A fixture file
is simpler and matches `authors.log`.

### 12. CHANGELOG.md

Add to `[Unreleased]`. `Removed` for the format matrix (breaking, name `ndjson`, `csv`,
`table`, and the kebab-case CSV headers, and say consumers must migrate to the JSON
envelope). `Added` for `shape`, `semantics`, and `transforms`, plus the per-column
`semantic` in `schema --command`. Include the D1 note explicitly: `schema_version` stays
`1` because the envelope stays backward compatible (new keys only; a consumer that ignores
unknown keys is unaffected), and the removal is a CLI-surface break rather than an envelope
break. Follow the existing entry style: bold lead-in, prose, and a table where a mapping
helps.

### Out of scope

- Emitting a non-table shape.
- Any doc outside CHANGELOG.md and code comments: the docs ticket covers cli-design.md,
  README.md, and the specs annotations, and the skill ticket covers the skill.
- Adding a `team` semantic: explicitly rejected (D4a), because it could never appear in a
  descriptor and would leave the closed set with an unreachable member.

### Files touched

```text
internal/analysis/semantics.go             NEW  vocabulary, Semantics(), ValidSemantic(), SemanticsOf()
internal/analysis/semantics_test.go        NEW
internal/analysis/analysis.go              Column.Semantic, Column.FlagGated
internal/analysis/*.go (18 descriptors)    71 Semantic assignments; FlagGated on coupling's 3
internal/analysis/schema_test.go           per-column semantic in key assertions
internal/output/types.go                   Result.Semantics, Result.Transforms, marshaler slots
internal/output/meta.go                    Meta.Semantics, Meta.Transforms
internal/output/fields.go                  retain transforms, filter semantics
internal/output/fields_test.go             projection filtering tests
internal/command/commands.go               build semantics + transforms, apply the group adjustment
internal/command/schema_test.go            semantics conformance, group asymmetry, verbose gating
internal/command/golden_test.go            authors_grouped, coupling_verbose scenarios
internal/command/testdata/*                regenerate; new fixtures for the two scenarios
CHANGELOG.md                               Unreleased: Removed + Added entries
```

## Acceptance Criteria

- Every one of the 71 declared (analysis, column) pairs carries a `Semantic` from the
  12-member closed set, matching the assignment table in this ticket's design; a
  conformance test fails on an empty or unknown value, so a new analysis cannot ship
  without semantics.
- `main-developer` reports `added`/`total_added` as `loc` while
  `main-developer-by-revisions` reports the same column names as `count` (D3c).
- Every analysis envelope carries `semantics` immediately after `shape`, always present,
  marshaling as `{}` rather than `null` when empty.
- `coupling` emits a 4-entry `semantics` map; `coupling --verbose` emits 7. `parse` emits
  all 8 entries even on a log whose rows carry no loc metrics (semantics track flags, not
  data).
- With `--group`, the envelope reports `entity` (and `coupled`) as `label` while
  `codelens schema --command CMD` still reports `filepath`; both halves are asserted in one
  test.
- With `--team-map`, `author` remains `person`.
- `transforms` is absent from a plain run's envelope and, for a run with `--group`,
  `--exclude`, and `--temporal-period`, carries exactly `group`, `exclude`, and
  `temporal_period` with snake_case keys.
- `params` is unchanged in meaning and content: per-analysis flags only, keyed by flag
  name, omitted for flagless analyses.
- `codelens schema --command CMD` carries `semantic` on every `row_schema` entry, including
  flag-gated columns, with key order `name, type, semantic, desc`.
- Under `--fields rows.entity`: `semantics` carries exactly one entry, `shape` is retained,
  and `transforms` is retained when present.
- Goldens include a grouped scenario and a verbose-coupling scenario freezing the semantic
  degradation, the transforms record, and the flag gating. All `.err` and `.exit` files are
  unchanged from the previous ticket.
- CHANGELOG.md `[Unreleased]` records the format-matrix removal as breaking, the three new
  envelope fields, the new per-column `semantic`, and states why `schema_version` stays 1.
- `make build` green.

