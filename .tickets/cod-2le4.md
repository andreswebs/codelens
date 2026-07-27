---
id: cod-2le4
status: closed
deps: []
links: [cod-z4wu]
created: 2026-07-27T14:54:14Z
type: chore
priority: 2
assignee: Andre Silva
tags: [codelens, spec-002, output]
---
# Trim the shape enum to the shapes codelens emits (table, text)

Trim the shape enum to the shapes codelens actually emits: `table` and `text`. Remove
`tree`, `graph`, `matrix`, and `series`, and with them the `payloadKey` panic branch that
exists only to guard them.

THE DOCUMENTATION FOR THIS CHANGE HAS ALREADY LANDED. docs/cli-design.md, README.md,
docs/skills/codelens/references/operating.md, CHANGELOG.md, and
docs/adr/0008-canonical-output-representation.md were all updated on 2026-07-27 and now
describe a two-member set. THIS TICKET IS THE CODE HALF ONLY: it makes the binary match the
docs. Do not re-edit those documents; if the code you write disagrees with them, the code is
wrong.

## Why

The five-member enum was declared speculatively when `shape` was introduced: only `table`
was reachable, and the other four were declared so the vocabulary would "look complete".
That was a mistake in an agent-first tool. `codelens schema` is the runtime source of truth
an agent is instructed not to second-guess, so a declared shape that no command can emit is
an unkeepable promise: an agent can legitimately read `shape` out of the closed set, plan
for a `graph` payload, and never be able to obtain one. The four unreachable members also
forced a `payloadKey` panic branch whose only purpose was to say "declared but not
implemented", which is a code smell standing in for a contract mistake.

Two follow-on decisions, taken 2026-07-27, made the trim concrete:

- `matrix` and `series` were RETRACTED as adding nothing over `table` plus `semantics`. A
  matrix is already derivable from any symmetric pair table (the skill's
  docs/skills/codelens/scripts/pair_matrix.py does exactly that, generically, via
  `--a-col`/`--b-col`/`--weight-col`), and `absolute-churn` is already a date-keyed series
  whose `date` column is labelled `date` by `semantics`. Neither shape earns a payload
  topology of its own.
- `tree` was DEFERRED. A codelens-emitted tree could only ever hold the entities present in
  the log, whereas the enclosure diagram's structure (which files exist, how big) comes from
  a `tokei` sidecar. See docs/skills/codelens/references/enclosure.md: tokei is the skeleton
  and codelens is a colour overlay joined on path. Supplying the real node set would require
  reading the working tree, which contradicts the "no direct VCS invocation" non-goal in
  docs/cli-design.md section 2 and the read-only posture in AGENTS.md. That capability
  belongs to the separate complexity direction.

`graph` remains genuinely wanted and is not retracted as an idea: it is simply not declared
until the analysis that emits it ships. The same applies to a future hierarchy shape.

The governing rule this establishes, now stated in ADR 0008 and worth preserving in the
code comments: A SHAPE IS ADDED TO THE SET BY THE CHANGE THAT MAKES IT EMITTABLE, NEVER
AHEAD OF IT.

## Relationship to other tickets

Standalone, and deliberately not dependent on anything. The parked epic on non-table shapes
and viz-spec adapters (cod-304f) is where `graph` will be reintroduced, and the complexity
direction is where a hierarchy shape would come from. Neither is a prerequisite for this
ticket, and this ticket must not wait on them: it is a correction to what already shipped in
the spec-002 rollout (see docs/specs/002-data-output/plan.md).

Nothing has been released yet. The five-member enum exists only under `[Unreleased]` in
CHANGELOG.md, so no consumer ever saw it as a shipped promise and no deprecation path is
needed.

Skills: /golang, /llm-coding (surgical, verifiable).

## Design

Three files change. Verified against the tree at 2026-07-27; re-locate by grep if lines have
drifted.

### 1. `internal/analysis/shape.go`

Current content in full:

```go
package analysis

// Shape names the topology of a command's output payload, and the payload key
// follows from it: "table" carries rows, "graph" carries nodes and edges, and so
// on. It is declared per command (not per invocation): one command has exactly
// one shape, and alternate views of the same data are downstream derivations, not
// output modes.
const (
    ShapeTable  = "table"
    ShapeTree   = "tree"
    ShapeGraph  = "graph"
    ShapeMatrix = "matrix"
    ShapeSeries = "series"
    // ShapeText is the escape hatch for a helper whose stdout is a bare string
    // meant to be copied and run (print-log-command), not a data payload. It is
    // declared so `schema --command` tells an agent not to pipe that stdout into a
    // JSON parser.
    ShapeText = "text"
)

// Shapes is the closed set of shape names, in declaration order. A conformance
// test pins every descriptor's Shape to a member.
func Shapes() []string {
    return []string{ShapeTable, ShapeTree, ShapeGraph, ShapeMatrix, ShapeSeries, ShapeText}
}
```

Changes:

- Delete the `ShapeTree`, `ShapeGraph`, `ShapeMatrix`, and `ShapeSeries` constants.
- `Shapes()` returns `[]string{ShapeTable, ShapeText}`.
- REWRITE THE PACKAGE-LEVEL DOC COMMENT. It currently uses `"graph" carries nodes and edges`
  as its illustrative example, which stops making sense once `graph` is undeclared. It must
  also carry the rule that justifies the trim, because that rule is the whole point and the
  speculative-enum instinct will recur. Something along these lines:

```go
// Shape names the topology of a command's output payload, and the payload key
// follows from it: "table" carries rows. It is declared per command (not per
// invocation): one command has exactly one shape, and alternate views of the same
// data are downstream derivations, not output modes.
//
// The set holds only the shapes codelens actually emits. A shape is added by the
// change that makes it emittable, never ahead of it: `schema` is the runtime
// contract an agent relies on, so a declared shape no command can produce would
// be an unkeepable promise. A hierarchy shape and a graph shape are both
// anticipated and will arrive with the analyses that need them (see ADR 0008).
```

- `ValidShape` needs no change: it iterates `Shapes()`.

### 2. `internal/analysis/shape_test.go`

`TestShapes_Closed` (line 18) pins the exact membership and order:

```go
want := []string{"table", "tree", "graph", "matrix", "series", "text"}
```

becomes

```go
want := []string{"table", "text"}
```

`TestValidShape` (line 5) needs no change: it loops over `Shapes()` for the positive cases
and over a hardcoded `{"", "rows", "TABLE", "unknown"}` for the negative ones. CONSIDER
ADDING the four retracted names to that negative list, which turns this test into the guard
that stops them being silently reintroduced:

```go
for _, s := range []string{"", "rows", "TABLE", "unknown", "tree", "graph", "matrix", "series"} {
```

If you add them, comment why, so a future author implementing the graph shape understands
they must remove `"graph"` from the negative list in the same change that declares it,
rather than being confused by a test that appears to forbid their new shape.

### 3. `internal/output/types.go`

`payloadKey` (line 162) currently reads:

```go
// payloadKey returns the JSON key the shape's payload is written under. Only the
// table shape is reachable today; the other declared shapes panic, matching
// toCLIFlag's treatment of an unsupported flag type, so a descriptor that
// declares a not-yet-emitted shape surfaces at first run rather than as a
// silently misnamed key. The graph payload (which needs both "nodes" and
// "edges") is deferred with the rest: guessing its Go shape now, without an
// analysis that needs it, would be speculative.
func payloadKey(shape string) string {
    switch shape {
    case "table":
        return "rows"
    case "tree", "graph", "matrix", "series":
        panic(fmt.Sprintf("output: shape %q is declared but not yet emitted (deferred epic cod-304f)", shape))
    }
    panic(fmt.Sprintf("output: unknown payload shape %q", shape))
}
```

Changes:

- DELETE the `case "tree", "graph", "matrix", "series":` branch and its panic entirely. With
  those shapes undeclared, an unknown-shape panic is the correct and only failure mode, and
  the "declared but not yet emitted" state no longer exists.
- Keep the final unknown-shape panic. It is the backstop for a descriptor typo and is
  reachable only via a programmer error, matching `toCLIFlag`'s treatment of an unsupported
  flag type.
- Rewrite the doc comment: drop the "other declared shapes" and deferred-graph paragraphs;
  state that `table` is the only data shape and that `text` never reaches this function.

IMPORTANT, verify rather than assume: `text` must NOT be added as a case here.
`print-log-command` writes a bare command line directly to the writer and never builds a
`Result`, so `payloadKey` is never called with `"text"`. Confirm that by reading
`internal/command/printlogcommand.go` (its action calls `emitLogCommand`, which does
`fmt.Fprintln` on the raw command string). If that has changed and `text` does reach the
marshaler, stop and reconsider: a `text` payload key would be a new contract decision, not
part of this cleanup.

- Check whether `fmt` is still used elsewhere in the file after removing one panic. It is
  used by the remaining panic, so the import stays, but `go build` will tell you if not.

### Verification

```sh
grep -rn 'ShapeTree\|ShapeGraph\|ShapeMatrix\|ShapeSeries' --include='*.go' .
grep -rn '"tree"\|"graph"\|"matrix"\|"series"' --include='*.go' internal/
make build
```

The first grep must return nothing. The second should return only the shape_test.go negative
list, if you chose to add it.

Expected test outcomes:

- `TestShapes_Closed` passes with the two-member list.
- `TestSchema_Conformance` (internal/command/schema_test.go:219) is unaffected: it asserts
  each descriptor's shape is a member of `Shapes()`, and every analysis declares `table`
  while `print-log-command` declares `text`. Both remain members.
- NO GOLDEN FILE SHOULD CHANGE. The enum is not serialized anywhere: `schema --command`
  emits a single `shape` STRING per command, never the set. If `go test ./internal/command/
  -run TestGolden` reports a diff, something else is wrong; investigate rather than running
  `-update`.

### Out of scope

- Implementing any shape. `graph` is reintroduced by the ticket that emits it (see the parked
  epic cod-304f), and a hierarchy shape by the complexity direction.
- Any documentation change. All five affected documents already landed on 2026-07-27:
  docs/cli-design.md section 6.2, README.md's envelope field list, the skill's
  references/operating.md output section, the CHANGELOG `[Unreleased]` entry, and ADR 0008
  (amended in place, since nothing has been released).
- The `semantics` map, the `transforms` record, and the marshaler's key order: untouched.

### Files touched

```text
internal/analysis/shape.go        remove 4 consts, Shapes() -> {table, text}, rewrite doc comment
internal/analysis/shape_test.go   TestShapes_Closed want list; optional negative-case guard
internal/output/types.go          payloadKey: delete the deferred-shape panic branch, rewrite doc comment
```

## Acceptance Criteria

- `analysis.Shapes()` returns exactly `["table", "text"]`, in that order, and the
  `ShapeTree`, `ShapeGraph`, `ShapeMatrix`, and `ShapeSeries` constants no longer exist:
  `grep -rn 'ShapeTree\|ShapeGraph\|ShapeMatrix\|ShapeSeries' --include='*.go' .` returns
  nothing.
- `output.payloadKey` has no "declared but not yet emitted" branch; an unrecognized shape
  hits the single unknown-shape panic, which remains as the programmer-error backstop.
- The doc comment on the shape enum states the governing rule (a shape is added by the change
  that makes it emittable, never ahead of it) and no longer uses `graph` as its illustrative
  example.
- `codelens schema --command CMD` still declares `shape` for every command: `table` for all
  18 analyses, `text` for `print-log-command`, absent for `schema`.
- `TestSchema_Conformance` and `TestValidShape` pass unchanged in intent; `TestShapes_Closed`
  pins the two-member set.
- NO golden file changes. The shape SET is not serialized anywhere, only a per-command shape
  string, so `go test ./internal/command/ -run TestGolden` passes without `-update`.
- The binary now matches the documentation that landed on 2026-07-27 (docs/cli-design.md
  section 6.2, README.md, docs/skills/codelens/references/operating.md, CHANGELOG.md, and ADR
  0008): every one of them describes the set as `table` and `text`, and none of them needed a
  further edit for this ticket.
- `make build` green.

## Notes

**2026-07-27T15:14:44Z**

Trimmed the shape enum to the two shapes codelens actually emits (table, text). Changes: internal/analysis/shape.go removed ShapeTree/ShapeGraph/ShapeMatrix/ShapeSeries constants, Shapes() now returns {table, text}, and the package doc comment was rewritten to carry the governing rule (a shape is added by the change that makes it emittable, never ahead of it) and drop the stale 'graph carries nodes and edges' example. internal/output/types.go payloadKey dropped the 'declared but not yet emitted' panic branch; only table->rows plus the unknown-shape backstop remain, and the doc comment notes text never reaches payloadKey (print-log-command Fprintln's a bare line, never builds a Result). shape_test.go pins the two-member set and adds tree/graph/matrix/series to the ValidShape negative list as a reintroduction guard, with a comment telling a future graph author to remove 'graph' from it in the same change that declares the shape. No golden files changed (the set is not serialized, only a per-command shape string). make build green. Docs were already landed on 2026-07-27; no doc edits needed.
