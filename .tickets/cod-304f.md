---
id: cod-304f
status: open
deps: [cod-fj7t, cod-1d04]
links: [cod-z4wu]
created: 2026-07-27T13:15:19Z
type: epic
priority: 3
assignee: Andre Silva
tags: [codelens, spec-002, output, viz, deferred]
---
# Epic: non-table shapes and downstream viz-spec adapters

Follow-on epic to the canonical-envelope rollout: emit the non-table shapes and build the
downstream viz-spec adapters that consume them. Tracked so it is not forgotten; NOT planned
in detail, and deliberately not decomposed until the envelope work has landed and the skill
has been exercised against it.

The envelope rollout declares the full shape enum (`table`, `tree`, `graph`, `matrix`,
`series`, `text`) but only `table` and `text` are reachable: `payloadKey` panics for the
other four. This epic makes them real.

Framing constraint carried over from decision D12: shape is FIXED PER COMMAND. A non-table
shape therefore arrives as a genuinely new analysis whose natural form is not a row set,
NOT as an alternate view of an existing analysis. `coupling` stays a table of pairs; a
graph of those pairs is a downstream derivation, not a `coupling --shape graph`.

Framing constraint carried over from decision D13: the viz-spec adapters live in skill
scripts that consume the canonical envelope on stdin. codelens stays strictly the data
plane. No `export` verb in the binary, no chart-language knowledge inside it, and no new
distributable artifact.

Reference: docs/specs/002-data-output/plan.md section 9 (the deferred list) and section 2
(D12, D13).

## Design

Open work, roughly in the order it would be picked up. Each bullet needs its own design
before it becomes a ticket.

### Non-table payload schemas

- Per-shape payload contracts for `tree`, `graph`, `matrix`, and `series`, and which new
  analyses introduce them. Candidates suggested by the current catalog: a path-hierarchy
  `tree` (the enclosure family's input, today built by scripts/enclosure.py), a
  nodes-and-edges `graph` (what scripts/coupling_graph.py and scripts/dev_network.py build
  by reshaping pairs), an author-by-entity `matrix`, and a date-bucketed `series`.
- `payloadKey` currently panics for these four with a message naming the shape. Each
  implementation replaces one panic. The `graph` case needs more than one payload key
  (`nodes` plus `edges`), which the marshaler deliberately does not attempt yet: decide
  whether `payloadKey` becomes `payloadKeys(shape) []string` or the marshaler special-cases
  it.
- Whether `row_count` stays named `row_count` for a non-table payload, or each shape gets
  its own count (`node_count` / `edge_count`). THIS IS THE FIRST GENUINE `schema_version`
  BUMP CANDIDATE: decision D1 kept `schema_version` at 1 precisely because the payload key
  was still always `rows`, and a renamed count field is a real envelope break.
- Shape-aware `--rows` truncation. Today `truncate` slices a slice payload and leaves a
  non-slice payload alone (`RowLen` returns 0 for a non-slice), which is correct but inert:
  capping a tree or a graph needs a per-shape rule, and it is not obvious what "the first N"
  means for either.
- Shape-aware `--fields` projection for a payload that is not an array of flat objects.

### Downstream adapters (skill-side, per D13)

- A Flint `ChartAssemblyInput` adapter. Flint's schema is data plus `semantic_types` plus a
  chart spec, which is why the envelope's `semantics` is a flat field-to-string map: the
  adapter should be close to identity. Flint covers roughly 30 tabular chart types and does
  NOT cover the hierarchical (enclosure/treemap) or graph (coupling/communication)
  flagships, which stay custom. Upstream reference: the flint-chart project's
  docs/api-reference.md.
- A Vega-Lite adapter for the tabular family, driven by `shape` plus `semantics` (for
  example: a `loc` field is the size channel, a `count` field the colour channel, a
  `filepath` the nominal axis).
- GraphML/DOT emission for the graph family, and CodeCharta `.cc.json` for the hierarchical
  family.
- Decide per script whether it becomes a thin adapter over one of the above or stays
  bespoke.

### Skill cleanups unlocked by the shapes

- Retire the path-tree building in docs/skills/codelens/scripts/enclosure.py (and the tokei
  join it depends on) once codelens emits a `tree` directly.
- Simplify docs/skills/codelens/scripts/coupling_graph.py and
  docs/skills/codelens/scripts/dev_network.py to consume a `graph` payload instead of
  reshaping pairs.
- Realign docs/skills/codelens/references/catalog.md: the per-card `Command` and `Formats`
  lines and the static-versus-interactive split all change once shapes and specs are
  emitted.
- Revisit docs/skills/codelens/references/embedding.md and
  docs/skills/codelens/references/reporting.md for the same.

### Explicitly a separate epic, not this one

The complexity direction (reading file content, indentation-complexity trend, a `hotspot`
fusion command) from docs/research/complexity-analysis-synthesis.md and its three detailed
reports. It is a larger, independent direction: it breaks the current read-only,
log-only-input posture, since it requires reading working-tree file content.

## Acceptance Criteria

Epic-level, to be refined when the epic is decomposed:

- No shape in the declared enum panics in `payloadKey`; every declared shape is either
  emitted by at least one command or removed from the enum.
- Every non-table shape has a documented payload contract in docs/cli-design.md and is
  discoverable via `codelens schema --command CMD`.
- The `row_count` naming question is resolved for non-table payloads, with any envelope
  break carried by a `schema_version` bump and recorded in CHANGELOG.md.
- The skill's reshaping scripts either consume a native shape or are documented as
  deliberately bespoke.
- codelens still emits no chart language: every viz-spec encoding lives outside the binary.


## Notes

**2026-07-27T13:45:37Z**

Scope refinement from a verification sweep (2026-07-27), before the source scratch list was retired. Two items from that list resolve differently than written:

1. "Revisit references/embedding.md and references/reporting.md for format-era assumptions" is a NO-OP, already verified. embedding.md's "Format to target" section and table describe SVG / PNG / HTML / PDF rendered-artifact formats, not codelens output formats. reporting.md contains no format references at all. Nothing to do in either file, now or when shapes land.

2. "Revisit references/catalog.md cards" is narrower than it sounds. The per-card 'Formats:' lines are also about rendered artifacts (interactive HTML, or SVG/PNG chosen by the -o extension), so they do NOT change with the envelope. What genuinely may change once shapes and specs are emitted is the static-versus-interactive split and any per-card 'Command:' line whose pipeline gets simpler. Only catalog.md:129 carried a codelens --format flag, and that is handled by the skill ticket, not here.
