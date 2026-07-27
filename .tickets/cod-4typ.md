---
id: cod-4typ
status: closed
deps: [cod-fb72]
links: []
created: 2026-07-27T13:05:46Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-z4wu
tags: [codelens, spec-002, output]
---
# Envelope: shape field and shape-derived payload marshaling

Add `shape` to the envelope and to `schema --command`, and restructure the result payload
so it is written under a shape-derived key by a custom `MarshalJSON`. The only new
envelope key here is `shape`; `semantics` and `transforms` follow in the next ticket. This
split keeps the golden diff of each ticket legible: this one is structural, the next is
content.

Implements decisions D5, D7, D7a, D8, and D12 from docs/specs/002-data-output/plan.md
section 2:

- D12: shape is FIXED PER COMMAND, so `CommandSchema.Shape` is a string, not a list. There
  is no `--shape` flag and no per-shape sibling commands. Alternate views (a graph of the
  coupling pairs, a tree of the code-age paths) are downstream derivations from the table
  plus its semantics.
- D7: the payload is one `Payload any` field written under the key the shape dictates, so
  a `table` result structurally cannot emit a `nodes` key and vice versa.
- D7a: because the payload key no longer exists as a Go struct field, `--fields` path
  validation moves off struct reflection onto the marshaled JSON, unioned with the
  declared column paths. This also FIXES A LATENT BUG (see the design note): today
  `--fields rows.entity` on an empty result works only because the reflection walker
  synthesizes a zero row element.
- D8: the envelope is built from an explicit `output.Meta` assembled in
  `internal/command` from the descriptor, so `internal/output` never imports
  `internal/analysis` (the dependency runs the other way today and must keep doing so).
- D5: `print-log-command` declares `shape: "text"`, so an agent learns from
  `schema --command print-log-command` that its stdout is a bare command line and not
  JSON. Its output does NOT change: it stays copy-pasteable, which is the reason the
  helper exists.

Reference: docs/specs/002-data-output/plan.md section 5 (this ticket's step list) and
section 3.1 (the target envelope and its key order), docs/cli-design.md sections 6.1, 6.2,
and 8. Skills: /golang, /tdd, /llm-coding.

## Design

Depends on the `--format` removal ticket: `output.Emit` must already be gone and
`EmitProjected` must be the single emit path.

### 1. Shape enum: `internal/analysis/shape.go` (new)

```go
// Shape names the topology of a command's output payload, and the payload key
// follows from it: "table" carries rows, "graph" carries nodes and edges, and so
// on. It is declared per command (not per invocation): one command has exactly one
// shape, and alternate views of the same data are downstream derivations, not
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

// Shapes is the closed set, in declaration order. A conformance test pins every
// descriptor's Shape to a member.
func Shapes() []string

// ValidShape reports whether s is a member of the closed set.
func ValidShape(s string) bool
```

Only `table` and `text` are reachable after this ticket. The other four are declared now
so the enum is complete and `schema` consumers can see the full vocabulary; the analyses
that emit them are a separate epic.

### 2. `internal/analysis/analysis.go`: `Descriptor.Shape`

Add to the `Descriptor` struct (after `Summary`, before `Flags`):

```go
// Shape names the topology of this analysis's payload (see Shapes). It is fixed
// per analysis: the payload key follows from it, and alternate views are
// downstream derivations rather than output modes.
Shape string
```

Set `Shape: analysis.ShapeTable` on all 18 analysis descriptors. They are declared in
`<name>Descriptor()` functions, one per file:

```text
abschurn.go authorchurn.go authors.go codeage.go communication.go coupling.go
entitychurn.go entityeffort.go fragmentation.go maindev.go maindevbyrevs.go
messages.go ownership.go parse.go refactoringmaindev.go revisions.go soc.go summary.go
```

Do not add a default: a descriptor with an empty `Shape` must fail the conformance test,
so a new analysis cannot silently omit it.

### 3. `internal/analysis/schema.go`: `CommandSchema.Shape`

- Add ``Shape string `json:"shape,omitempty"` `` to `CommandSchema` (struct at line 66),
  placed after `Aliases` and before `Flags` so the JSON key order matches the
  docs/cli-design.md section 8 example (`command`, `summary`, `shape`, `flags`,
  `row_schema`, ...). Verify against that example; `TestSchema_JSONKeys` in
  `internal/analysis/schema_test.go:64` asserts key presence and will need the new key.
- `Schema(d Descriptor)` (line 119) passes `Shape: d.Shape`.
- `MetaSchema` (line 140) gains a `shape` parameter. Current signature:

```go
func MetaSchema(command, summary string, flags []Flag, errorCodes []string, exitCodes []int) CommandSchema
```

becomes

```go
func MetaSchema(command, summary, shape string, flags []Flag, errorCodes []string, exitCodes []int) CommandSchema
```

`omitempty` matters here: `schema` (the introspection command itself) declares NO shape,
because it is not an analysis result, so it passes `""` and the key is absent from its
schema. `print-log-command` passes `ShapeText`.

Call sites to update: `internal/command/metacommands.go:87` (the `schema()` projection),
plus `internal/analysis/schema_test.go:142` and `:188`.

### 4. `internal/command/metacommands.go`: declare the meta shapes

Add a `Shape` field to the `metaCommand` struct (line 16), set it in the `metaCommands()`
table (line 34): `print-log-command` gets `analysis.ShapeText`, `schema` gets `""`. Then
pass it through the `schema()` projection at line 87.

Document the `""` on the `schema` entry inline, because an empty value in a table
otherwise reads as an oversight:

```go
// schema emits its own introspection envelope, which is not an analysis result, so
// it declares no shape.
Shape: "",
```

### 5. `internal/output/types.go`: `Result` and the marshaler

`Result` becomes:

```go
type Result struct {
    SchemaVersion int            `json:"-"`
    OK            bool           `json:"-"`
    Analysis      string         `json:"-"`
    Shape         string         `json:"-"`
    Params        map[string]any `json:"-"`
    RowCount      int            `json:"-"`
    TotalCount    int            `json:"-"`
    Truncated     bool           `json:"-"`
    // Payload is the shape's data, written under the key Shape dictates (see
    // payloadKey): rows for a table, nodes and edges for a graph. Keeping one
    // payload field rather than one per shape makes a mismatched key
    // unrepresentable.
    Payload any `json:"-"`
}
```

and gains a custom marshaler. The `json:"-"` tags are deliberate: with a custom
`MarshalJSON`, struct tags are dead weight that would mislead a reader into thinking
`encoding/json` drives the output. An alternative is to keep the tags and use an alias
struct inside `MarshalJSON`; either is fine, but pick one and say why in the doc comment.

```go
// MarshalJSON writes the envelope with a stable key order and the payload under the
// key its shape dictates. The order is metadata first, payload last, so `head` on a
// large result shows the descriptive fields. Key order is part of the golden
// contract (ADR 0007), so it is fixed here rather than left to struct field order.
func (r Result) MarshalJSON() ([]byte, error)
```

Required key order (docs/specs/002-data-output/plan.md section 3.1):

```text
schema_version, ok, analysis, shape, semantics, transforms, params, row_count,
total_count, truncated, <payload_key>
```

`semantics` and `transforms` land in the next ticket; leave their slots in the order and a
comment marking them, so the follow-on ticket is an insertion rather than a reshuffle.
Omission rules, preserving today's behaviour exactly: `params` omitted when nil,
`total_count` omitted when zero, `truncated` omitted when false. `row_count` is always
present, including when zero.

Implementation note: build an ordered `bytes.Buffer` or use a small ordered writer.
`map[string]any` will NOT work, since `encoding/json` sorts map keys alphabetically and
would scramble the contract order. Prefer explicit `json.Marshal` per value with manual
key writing, or a slice of key/value pairs.

```go
// payloadKey returns the JSON key the shape's payload is written under. An unknown
// shape is a programmer error in a descriptor: it panics, matching toCLIFlag's
// treatment of an unsupported flag type, so the mistake surfaces at first run
// rather than as a silently misnamed key.
func payloadKey(shape string) string {
    switch shape {
    case "table":  return "rows"
    case "tree":   return "tree"
    case "graph":  return "nodes"   // graph also writes "edges"; see MarshalJSON
    case "matrix": return "matrix"
    case "series": return "series"
    }
    panic(...)
}
```

Since `graph` needs two keys, consider `payloadKeys(shape) []string` returning one or more
keys, or defer the graph case entirely with a `panic("shape not yet emitted")` for the four
unreachable shapes. DEFERRING IS PREFERRED: only `table` is reachable, and guessing the
graph payload's Go shape now, without an analysis that needs it, is speculative. Panic with
a clear message naming the shape and the ticket that will implement it.

### 6. `internal/output/meta.go` (new): the constructor

```go
// Meta is the descriptive half of a result envelope: everything the output layer
// needs to describe a payload but cannot derive from it. The command layer builds it
// from the analysis descriptor, which is why it is a plain struct of values rather
// than a descriptor reference: internal/output must not import internal/analysis
// (the dependency runs the other way).
type Meta struct {
    Analysis string
    Shape    string
    // Columns are the ordered snake_case payload field names the command declares.
    // They seed the --fields valid-path set so a projection stays valid on an
    // empty payload, where the data alone would expose no field paths.
    Columns []string
}

// NewResult wraps a payload in a success envelope, setting the invariants every
// result shares: schema version, ok, the analysis identity and shape from meta, and
// the payload count. Params and the truncation metadata are populated by the caller
// after construction.
func NewResult(meta Meta, payload any) Result
```

`Meta` grows `Semantics` and `Transforms` in the next ticket. Delete
`internal/output/newresult.go`'s old two-argument `NewResult` and move the new one here
(or keep the file and rewrite it; the file name `newresult.go` still fits). `RowLen` stays
where it is: it is the single reflection site for payload counting, shared by `NewResult`
and the `--rows` truncation.

Update `internal/output/newresult_test.go`, which calls `NewResult("coupling", rows)` at
lines 11, 32, and 42.

### 7. `internal/output/fields.go`: path validation off reflection (D7a)

This is the subtlest part of the ticket. Today `ValidateFields` (line 27) calls
`collectValidPaths(envelope)` (line 133), which reflects over the Go struct and, at line
192-197 (`elemValue`), synthesizes a zero element for an empty slice so nested row paths
are still discoverable. With the payload behind `json:"-"` and a custom marshaler, that
walk no longer sees `rows` at all.

Replacement: valid paths are those of the MARSHALED JSON, unioned with
`<payload_key>.<column>` for every declared column.

```go
// ValidateFields parses a comma-separated projection spec and validates each dotted
// path against the envelope's ACTUAL emitted shape, unioned with the payload field
// paths the command declares. Taking the declared columns from the schema rather than
// from the data is what keeps a projection valid on an empty payload: the data alone
// exposes no field paths when the payload is [].
func ValidateFields(paths string, envelope any, declared []string) ([]string, error)
```

- Marshal the envelope once (the emit path marshals it anyway; reuse the bytes rather than
  marshaling twice), decode into `any`, and walk the JSON tree collecting dotted paths.
  For arrays, walk the first element if present. For objects, recurse. Keep the existing
  `wildcard` (`*`) behaviour for map-typed fields, which is what makes `semantics.*` and
  `params.*` work in the next ticket.
- Union in the declared paths: for each column `c`, add `payloadKey(shape) + "." + c`.
- The retention list in `ProjectFields` (lines 54-56) gains `shape`:

```go
tree["schema_version"] = nil
tree["ok"] = nil
tree["shape"] = nil   // always self-describing, even when projected (D6)
```

`semantics` filtering and `transforms` retention are the next ticket's business.

- `EmitProjected` (line 65) gains the declared columns:
  `EmitProjected(w io.Writer, envelope any, fieldsStr string, declared []string) error`.
  Update the caller at `internal/command/commands.go`.

Delete the now-unused reflection helpers if nothing else uses them: `collectValidPaths`,
`collectPaths`, `deref`, `elemValue`, `jsonFieldName`. Check first with
`grep -rn 'collectValidPaths\|collectPaths\|elemValue\|jsonFieldName' internal/`, since
`fields_test.go` exercises some of them directly and those tests move to the JSON-tree
equivalent.

### 8. `internal/command/commands.go`

- Revive `columnNames(d analysis.Descriptor) []string` (deleted in the previous ticket)
  with its new purpose, and say so in the doc comment: it is no longer a csv/table header
  source but the declared payload field set for `Meta.Columns` and the `--fields` seeding.
- In `actionFor` (line 139), replace the envelope construction:

```go
res := output.NewResult(output.Meta{
    Analysis: d.Name,
    Shape:    d.Shape,
    Columns:  columnNames(d),
}, rows)
res.Params = effectiveParams(cmd, d)
truncate(&res, cmd.Int("rows"))

return output.EmitProjected(cmd.Root().Writer, res, cmd.String("fields"), columnNames(d))
```

- `truncate` (line 326) operates on `res.Rows` today; retarget it to `res.Payload`. Its
  doc comment says it "caps res to its first n rows after the analysis's own sort"; keep
  that meaning. It uses `reflect.ValueOf(res.Rows).Slice(0, n)`, which works unchanged on
  `Payload` for any slice payload. Note in the comment that a non-slice payload (a future
  tree or graph) is not truncatable and is left alone, which `RowLen` already handles by
  returning 0 for a non-slice.

### 9. Tests

New or updated, all in the TDD order (red first):

- `internal/analysis/shape_test.go` (new): `ValidShape` accepts every member of
  `Shapes()` and rejects `""` and an unknown string.
- `internal/command/schema_test.go:191` (`TestSchema_Conformance`): it already sweeps
  `analysis.All()` and asserts every column is fully documented. Add: every command
  declares a `shape` that is a member of the closed set. Extend the local `schemaCmd`
  struct (line ~41) with `Shape string`.
- Meta-command shape: assert `schema --command print-log-command` reports
  `shape: "text"`, and that `schema --command schema` omits `shape` entirely.
- `internal/output`: a marshaler test pinning the exact key ORDER (not just presence),
  since order is the golden contract. Assert on the raw bytes, for example that the output
  starts with `{"schema_version":1,"ok":true,"analysis":"authors","shape":"table"` and
  that `"rows"` is the last key. Also assert `params`/`total_count`/`truncated` omission
  is unchanged.
- `internal/output/fields_test.go`: the critical regression test for D7a, which the old
  reflection walk provided only incidentally:

```go
// An empty payload still accepts a declared field path: the valid-path set comes
// from the schema, not from the data.
// codelens authors --fields rows.entity   on a log that yields zero rows -> valid
```

  Plus: an undeclared path is still `invalid_field` (exit 64) with the sorted valid set in
  `details`, and `--fields shape` works while `shape` is retained regardless.

### 10. Goldens

Regenerate and hand-review:

```sh
go test ./internal/command/ -run TestGolden -update
```

Expected diff, and NOTHING else: every `*.out` for a successful analysis gains exactly one
key, `"shape":"table"`, positioned after `"analysis"`. `authors_schema.out` gains
`"shape":"table"` after `"aliases"`. `schema_list.out` is unchanged (the command list
carries summaries, not shapes). The error goldens and all `.err`/`.exit` files are
unchanged. If `authors_fields.out` gains `"shape":"table"`, that is the D6 retention
working as intended.

### Out of scope

- `semantics`, `transforms`, and `Column.Semantic`: the next ticket.
- Emitting any non-table shape: the deferred epic. The four unreachable shapes are
  declared in the enum and panic in `payloadKey`.
- Renaming `row_count` for non-table payloads: deferred (it is the first real
  `schema_version` bump candidate).
- Any doc outside code comments.

### Files touched

```text
internal/analysis/shape.go                NEW  shape enum, Shapes(), ValidShape()
internal/analysis/shape_test.go            NEW
internal/analysis/analysis.go              Descriptor.Shape
internal/analysis/*.go (18 descriptors)    Shape: ShapeTable
internal/analysis/schema.go                CommandSchema.Shape; Schema(); MetaSchema() +shape param
internal/analysis/schema_test.go           MetaSchema call sites (:142, :188); JSON key test
internal/output/types.go                   Result reshaped, MarshalJSON, payloadKey
internal/output/types_test.go              key-order assertions
internal/output/meta.go                    NEW  Meta, NewResult(meta, payload)
internal/output/newresult.go               old NewResult removed or rewritten; RowLen stays
internal/output/newresult_test.go          call sites (:11, :32, :42)
internal/output/fields.go                  JSON-tree path collection, declared seeding, retain shape
internal/output/fields_test.go             empty-payload projection regression test
internal/command/commands.go               columnNames revived, Meta construction, truncate on Payload, EmitProjected args
internal/command/metacommands.go           metaCommand.Shape, table entries, schema() projection
internal/command/schema_test.go            shape conformance, schemaCmd.Shape
internal/command/testdata/*.out            regenerate (one new key)
```

## Acceptance Criteria

- Every analysis envelope carries `"shape": "table"`, positioned immediately after
  `analysis`, and the payload is still the `rows` key.
- Envelope key order is exactly `schema_version, ok, analysis, shape, params, row_count,
  total_count, truncated, rows`, asserted on raw bytes by a marshaler test (not just key
  presence), with the `semantics` and `transforms` slots reserved in the marshaler for the
  next ticket.
- `params`, `total_count`, and `truncated` omission behaviour is unchanged from before the
  ticket.
- A mismatched payload key is unrepresentable: `Result` holds one `Payload`, written under
  the key `payloadKey(shape)` returns, and the four not-yet-emitted shapes panic with a
  message naming the shape.
- `codelens schema --command CMD` declares `shape` for every analysis and for
  `print-log-command` (`"text"`); `schema --command schema` omits the key.
- `TestSchema_Conformance` fails if any descriptor declares an empty or unknown shape.
- `internal/output` does not import `internal/analysis`: the envelope is built from
  `output.Meta` assembled in `internal/command`.
- `--fields rows.entity` is valid on an EMPTY result (zero rows), covered by a regression
  test; an undeclared path is still `invalid_field` (exit 64) listing the sorted valid set.
- `--fields rows.entity` output retains `schema_version`, `ok`, and `shape`.
- `--rows N` truncation still sets `row_count`, `total_count`, and `truncated` correctly,
  now operating on the payload.
- The regenerated goldens differ from the previous ticket's state ONLY by the added
  `shape` key; all `.err` and `.exit` files are unchanged.
- `make build` green.


## Notes

**2026-07-27T14:03:16Z**

Added the shape field and shape-derived payload marshaling (D5,D7,D7a,D8,D12).

Code:
- internal/analysis/shape.go (new): closed shape enum (table/tree/graph/matrix/series/text), Shapes(), ValidShape().
- Descriptor.Shape added; all 18 analysis descriptors set Shape: ShapeTable.
- CommandSchema.Shape (json:"shape,omitempty"), placed after aliases; Schema() passes d.Shape; MetaSchema() gained a shape param. metaCommand.Shape: print-log-command=text, schema="" (omitted).
- output.Result reshaped: metadata fields are json:"-" plus a single Payload any; a custom MarshalJSON (via an ordered fieldWriter) fixes the golden key order schema_version,ok,analysis,shape,[semantics/transforms slots reserved],params,row_count,total_count,truncated,<payload_key>. payloadKey() returns rows for table and panics for the four not-yet-emitted shapes.
- output/meta.go (new): Meta{Analysis,Shape,Columns} + NewResult(meta,payload); RowLen stays in newresult.go (old 2-arg NewResult removed).
- output/fields.go: --fields validation moved OFF struct reflection onto the marshaled JSON tree, unioned with payloadKey(shape).<column> for declared columns (fixes latent D7a bug: rows.entity now valid on an empty payload). shape force-retained under projection (D6). params.* wildcard preserved via Result.wildcardPaths(). EmitProjected/ValidateFields gained a declared []string param; EmitProjected reuses the single marshal.
- commands.go: columnNames revived (now feeds Meta.Columns + --fields seeding, not csv headers); truncate() retargeted to res.Payload.

Goldens: only shape:"table" added (authors_json/rows2/fields/coupling_warning after analysis; authors_schema after aliases). schema_list unchanged by this ticket (the unknown_format removal there is cod-fb72's). All .err/.exit unchanged.

Next: cod-435u adds semantics + transforms into the reserved marshaler slots and Meta.
