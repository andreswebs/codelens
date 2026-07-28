---
id: cod-x0fv
status: open
deps: []
links: []
created: 2026-07-28T16:24:56Z
type: chore
priority: 2
assignee: Andre Silva
tags: [codelens, spec-004, go, refactor]
---
# Name the three analysis enums as Go types (Semantic, Shape, AggRole)

Convert the three enums in `internal/analysis` to named string types:
`type Semantic string`, `type Shape string`, `type AggRole string`.

PURE REFACTOR. No behaviour change, no output change, NO GOLDEN FILE DIFFS. If any
`.out` golden changes, something is wrong: investigate rather than running `-update`.

Spec: docs/specs/004-aggregation-roles/plan.md sections 5.3 and 5.3.1. This is ticket A
of five in that plan (section 7.1).

## Why

`AggRole` (added by the next ticket) is the only enum in the package that takes a
member of ANOTHER enum as its input, so it is the only place two string vocabularies
meet in one expression. With plain strings this compiles and is silently always
false:

```go
if AggRoleOf(c.Semantic) == SemanticCount { ... }
```

Typed, it is a compile error. Decision Q1 chose to make the whole package consistent
at the stronger level rather than type only the new enum and leave two conventions.

It lands separately from the vocabulary because a wide mechanical diff reviewed on its
own is trivially checkable, and if the conversion has surprising blast radius that
surfaces before anything semantic is committed to.

## Design

## Blast radius, measured 2026-07-27

27 non-test files reference the two existing enums, but **18 are descriptor files
containing only literals**, and a typed string constant satisfies a typed field
unchanged. Verify before starting that no descriptor uses a raw string:

```sh
grep -rn 'Semantic: "' internal/analysis/*.go | grep -v _test    # must be empty
grep -rn 'Shape: "' internal/analysis/*.go | grep -v _test       # must be empty
```

Both returned zero at 2026-07-27. If either now returns a hit, that literal needs a
typed constant first.

## The six files that actually change

```text
internal/analysis/analysis.go       Column.Semantic -> Semantic; Descriptor.Shape -> Shape
internal/analysis/semantics.go      type Semantic; typed constants; Semantics() []Semantic;
                                    ValidSemantic(Semantic) bool; SemanticsOf -> map[string]Semantic
internal/analysis/shape.go          type Shape; typed constants; Shapes() []Shape;
                                    ValidShape(Shape) bool
internal/analysis/schema.go         schema projection of Semantic/Shape
internal/command/commands.go        semanticsFor, adjustForTransforms
internal/command/metacommands.go    schema output
```

Plus roughly ten `_test.go` files, notably:

- `internal/analysis/shape_test.go` — `TestShapes_Closed`'s `want := []string{"table","text"}`
  becomes `[]Shape{ShapeTable, ShapeText}`. Its negative-case list (`"", "rows", "TABLE",
  "unknown", "tree", "graph", "matrix", "series"`) needs `Shape(...)` conversions.
- `internal/analysis/semantics_test.go`, `internal/analysis/schema_test.go`,
  `internal/command/{schema,semantics}_test.go`, `internal/output/*_test.go`.

## CRITICAL: the typing stops at the output boundary

`internal/output/meta.go` line 7 states the constraint in its own comment: output does
NOT import `internal/analysis`, "the dependency runs the other way". Verified still
true at 2026-07-28.

So DO NOT change these:

```go
// internal/output/types.go — both stay as-is
Shape     string            `json:"-"`
Semantics map[string]string `json:"-"`
```

The COMMAND LAYER converts when it builds the result. `semanticsFor` in
`internal/command/commands.go` is where the conversion goes; it already returns the map
that populates `Result.Semantics`, so this is one or two lines:

```go
func semanticsFor(cmd *cli.Command, d analysis.Descriptor) map[string]string {
    // ... unchanged logic producing map[string]analysis.Semantic ...
    out := make(map[string]string, len(sem))
    for k, v := range sem { out[k] = string(v) }
    return out
}
```

Giving `output` its own mirrored named types is explicitly REJECTED: it would duplicate
the vocabulary in two packages, which is the drift the plan's section 4 argues against.

`payloadKey(shape string)` in `internal/output/types.go` also stays string-typed, for
the same reason.

## Serialization is unaffected

A named string type marshals to identical JSON. No `schema_version` bump, no envelope
change, no golden change. This is the property that makes the whole ticket safe.

## Acceptance Criteria

- `type Semantic string`, `type Shape string`, and `type AggRole string` declared in
  `internal/analysis`, with existing constants typed.
- The six signature files converted; all 18 descriptor literals UNTOUCHED.
- `internal/output` still does not import `internal/analysis`; `Result.Shape` is still
  `string` and `Result.Semantics` still `map[string]string`; conversion happens in
  `internal/command`.
- ZERO golden-file diffs. `go test ./internal/command/ -run TestGolden` passes without
  `-update`.
- `make build` green.

