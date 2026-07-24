---
status: accepted
---

# One canonical, shape-aware JSON representation (drop the format matrix)

## Context

codelens currently exposes a global `--format` flag with four values: `json`
(the self-describing envelope), `ndjson`, `csv` (kebab-case headers targeting a
legacy external format), and `table` (human terminal). All four are
serializations of a single flat, tabular row set; the flag conflates three
unrelated concerns (wire encoding, human presentation, and, prospectively,
semantic shape).

The direction for codelens makes this the wrong surface. codelens is a code
visualization _data_ engine: it should hand any renderer or agent the data plus
the meaning of that data, and let downstream tools (Flint, Vega-Lite, GraphML,
CodeCharta `.cc.json`) produce pixels. That requires output shapes beyond flat
rows (hierarchy, graph, matrix, series) and requires carrying the semantic
meaning of each field, which only codelens knows because it authored the data. A
`--format` matrix of serializations cannot express either need, and a
`--format flint` would wrongly frame a lossy downstream projection as a
serialization of the data.

## Decision

**codelens emits exactly one thing on stdout: a self-describing, shape-aware JSON
envelope. The `--format` flag is removed. Output is JSON only, agent-first.**

Specifics:

- **Remove `--format` entirely**, and with it the `ndjson`, `csv`, and `table`
  outputs and the legacy CSV compatibility (kebab-case headers included). There
  is no human-presentation mode; humans pipe to `jq` or downstream tooling.
- **The canonical envelope is shape-aware and self-describing.** In addition to
  the existing `schema_version`, `ok`, `analysis`, and optional `params`, it
  carries:
  - `shape`: one of `table` | `tree` | `graph` | `matrix` | `series`. The payload
    key and structure follow from `shape` (for example `rows` for `table`;
    `nodes` and `edges` for `graph`). Today every analysis is `shape: "table"`;
    the other shapes are introduced with the analyses that need them.
  - `semantics`: a map from field name to a semantic type (for example `entity`
    is a filepath, `degree` a percentage, `age_months` a duration, `main_dev` a
    person). This is the asset only codelens can provide, and it is what makes
    downstream chart specs derivable without domain knowledge.
  - the shape's payload, plus the existing `row_count` and, when truncated,
    `total_count` and `truncated`.
- **The tabular payload is shaped to sit close to a chart-spec input** (a
  `values` array plus `semantics`), so a downstream adapter to Flint's
  `ChartAssemblyInput` (or Vega-Lite) is near-identity. codelens does not adopt
  any chart language as its format; it only makes the canonical shape cheap to
  project from.
- **`schema` remains the runtime source of truth** and is extended:
  `schema --command CMD` declares that command's `shape`, its `semantics`, and
  its payload contract, so the full output can be discovered without running the
  analysis.
- **Keep `--fields`** (JSON path projection) as the output-bounding lever, and
  keep `--rows`. These operate on the one representation; there is no format to
  qualify them.
- **Viz-spec encodings are downstream transforms, not codelens output modes.**
  Flint `ChartAssemblyInput`, Vega-Lite, GraphML/DOT, and CodeCharta `.cc.json`
  are produced by consumers of the canonical envelope (the visualization skill,
  the Flint MCP server, or a future dedicated export step), never by a `--format`
  value in the core binary. The `semantics` and `shape` fields exist so those
  transforms are mechanical.

Rules that carry over unchanged:

- Errors are still a JSON error envelope on stderr
  (`{ok: false, error: {code, message, hint}}`), independent of stdout, per the
  exit-code taxonomy ADR.
- stdout carries only the canonical envelope, so piping into a JSON parser is
  always safe.

## Consequences

- **Simpler, more predictable surface.** One representation, one parser, one
  contract. Agents never choose a format, and the `--format` decision tree
  disappears.
- **Breaking change.** Any consumer of `ndjson`, `csv`, or `table` breaks. This is
  acceptable pre-1.0 and is the point: the legacy CSV compatibility is cut
  deliberately, freeing column naming and structure from an external contract.
- **Human terminal ergonomics are traded away** for agent-first cleanliness; `jq`
  covers the gap.
- **Shape-aware output is unlocked** without a format zoo: hierarchy, graph,
  matrix, and series payloads become first-class, which is what lets the
  hierarchical and graph visualizations drop bespoke reshaping glue.
- **Downstream viz transforms are decoupled** and can evolve independently of the
  core, each consuming `shape` + `semantics` + payload.
- **New required envelope fields** (`shape`, `semantics`) must be added to every
  analysis; existing analyses become `shape: "table"` with a populated
  `semantics` map, and `schema --command` must be extended to declare both.
- **Follow-up documentation updates** are required: the CLI design document's
  output-format section, the skill's operating and catalog references, and any
  guidance that mentions `--format`, `ndjson`, `csv`, or `table`.
