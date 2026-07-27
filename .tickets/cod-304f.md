---
id: cod-304f
status: closed
deps: [cod-fj7t, cod-1d04]
links: [cod-z4wu]
created: 2026-07-27T13:15:19Z
type: epic
priority: 3
assignee: Andre Silva
tags: [codelens, spec-002, output, viz, deferred]
---
# Epic: non-table shapes and downstream viz-spec adapters

PARKED 2026-07-27. Nothing here is scheduled, and every item in "Open decisions" is
deferred. Read "Two findings" first: they materially shrink what this epic is worth, and
they are the reason two of its original shapes were dropped rather than built.

Follow-on to the spec-002 canonical-envelope rollout
(docs/specs/002-data-output/plan.md), which shipped `shape`, `semantics`, and `transforms`.
That rollout made the envelope able to EXPRESS a payload topology beyond flat rows; this
epic is about whether and how to actually emit one, and about the downstream adapters that
would consume it.

Scope after the 2026-07-27 decisions: ONE candidate shape (`graph`), the unresolved
question of how `semantics` describes a non-flat payload, and the viz-spec adapters. That is
a much smaller epic than the title suggests.

## Settled decisions (do not relitigate)

| Decision | Outcome |
| --- | --- |
| `matrix` and `series` shapes | RETRACTED. They add nothing over `table` plus `semantics`. A matrix is already derivable from any symmetric pair table, which is exactly what docs/skills/codelens/scripts/pair_matrix.py does generically via `--a-col`/`--b-col`/`--weight-col`; `absolute-churn` is already a date-keyed series whose `date` column `semantics` labels `date`. Neither earns a payload topology. |
| `tree` shape | DEFERRED to the complexity direction, for the reason in finding 1 below. Not merely unscheduled: a log-only `tree` cannot deliver the thing that would make it valuable. |
| The shape enum | Trimmed to the reachable set, `table` and `text`. Governing rule, now in ADR 0008: A SHAPE IS ADDED BY THE CHANGE THAT MAKES IT EMITTABLE, NEVER AHEAD OF IT, because `schema` is the runtime contract an agent relies on and a declared-but-unemittable shape is an unkeepable promise. Docs landed 2026-07-27; the code half is ticket cod-2le4. |
| `graph` shape | Still wanted, simply not declared until the analysis that emits it ships. It is the one shape with a payoff that is not free: unlike a matrix, nodes-and-edges is not a trivial reshape of what a pair table already gives you. |
| Shape per command (D12) | FIXED PER COMMAND. A graph therefore arrives as a genuinely NEW analysis, not as `coupling --shape graph`. `coupling` stays a table of pairs. |
| Adapter home (D13) | Skill scripts consuming the envelope on stdin. codelens stays strictly the data plane: no `export` verb, no chart-language knowledge in the binary, no new distributable artifact. |

## Two findings that reframe the epic

### 1. A codelens `tree` cannot retire the tokei join

The original scratch list claimed that once codelens emits a `tree`, the path-tree building
in docs/skills/codelens/scripts/enclosure.py AND the tokei join it depends on could both go.
The second half is wrong.

Per docs/skills/codelens/references/enclosure.md, the two inputs are not symmetric: tokei is
the SKELETON (which files exist, how big each is) and codelens is a COLOUR OVERLAY joined on
path, defaulting to 0 for files with no recorded change. A git log can only ever tell you
which files CHANGED IN THE WINDOW, never which files exist or their size. So a
codelens-emitted tree reproduces only the documented DEGRADED mode (changed files only,
circles sized by the weight itself), and the tokei sidecar remains load-bearing for the full
mode.

Supplying a real node set would require reading the working tree, which contradicts the "no
direct VCS invocation" non-goal in docs/cli-design.md section 2 and the read-only posture in
AGENTS.md. That capability, if it is ever wanted, arrives with the complexity direction,
which already has to read file content. Hence the deferral rather than a scheduled ticket.

### 2. The reshaping payoff is small, and was measured

Line counts at 2026-07-27, so a future reader does not have to re-derive them:

```text
docs/skills/codelens/scripts/dev_network.py      118
docs/skills/codelens/scripts/coupling_graph.py   131
docs/skills/codelens/scripts/pair_matrix.py      176
docs/skills/codelens/scripts/enclosure.py        393
docs/skills/codelens/scripts/treemap.py          413
                                          total 1231
```

Inside enclosure.py, the tree building is `build_tree`, roughly 26 lines of 393. The rest is
glob filtering, tokei structure parsing, the area-domination warning, and HTML rendering,
none of which a native shape removes. Across all five scripts the genuinely retireable
reshaping is on the order of 60 to 80 lines. The epic should be justified by contract quality
(a renderer receiving nodes and edges directly, with semantics), NOT by a claim of deleting
significant code.

## What remains in scope

1. A `graph` analysis: nodes and edges emitted natively, declared as a new command.
2. How `semantics` describes a non-flat payload (see decision E3, the sharpest open item).
3. The viz-spec adapters, and which existing scripts become thin adapters over them.

## Open decisions (all deferred)

None of these is settled. They are grouped by area; E for envelope, A for adapters, P for
process.

### Envelope and contract

- **E1. What is the graph command called, and does it share coupling's flags?** Under D12 it
  is a new analysis, so it needs a name and a decision on whether it re-declares coupling's
  six threshold flags (`min-revs`, `min-shared-revs`, `min-coupling`, `max-coupling`,
  `max-changeset-size`, `verbose`) or shares them some other way.
- **E2. Are fusion commands a concept?** docs/skills/codelens/scripts/coupling_graph.py
  consumes `coupling` for edges PLUS `sum-of-coupling` for node size, so a faithful graph
  command would fuse two analyses. The complexity direction independently proposes a
  `hotspot` command fusing revisions and complexity. Decide the concept ONCE, for both, or
  reject it.
- **E3. How does `semantics` describe a non-flat payload?** THE SHARPEST OPEN ITEM. The map
  is deliberately a flat `field -> string` so a Flint adapter is near-identity (decision D3
  of the spec-002 plan). A graph has node fields AND edge fields whose names can collide.
  Sectioning it (`nodes.*`, `edges.*`) breaks exactly the flatness that justified the
  design. Resolve this before writing any graph payload.
- **E4. `payloadKey` for a two-key payload.** A graph needs both `nodes` and `edges`. Decide
  between `payloadKeys(shape) []string` and a marshaler special-case. Note the deferred-shape
  panic branch is being deleted by cod-2le4, so this starts from a clean `payloadKey`.
- **E5. Does `row_count` survive for a non-table payload?** Or does each shape get its own
  count (`node_count`, `edge_count`). THIS IS THE FIRST GENUINE `schema_version` BUMP
  CANDIDATE: decision D1 kept the version at 1 precisely because the payload key was always
  `rows`, and a renamed count field is a real envelope break.
- **E6. `--rows` truncation for a graph.** "First N" is meaningless for a node set. Top-N
  nodes by degree with induced edges is an ANALYSIS decision, not a truncation rule. Today
  `truncate` slices a slice payload and leaves a non-slice payload alone, which is correct
  but inert.
- **E7. `--fields` projection into a non-flat payload.** The path collector walks marshaled
  JSON; a recursive payload would yield unbounded paths. Needs a depth rule or a different
  projection model.
- **E8. Transform interaction.** `--group` collapses paths to layer names and already
  degrades `entity` from `filepath` to `label`. Does a graph of grouped entities make sense,
  and if not does the command warn or refuse?
- **E9. Empty-result semantics per shape.** A graph with nodes but no surviving edges: empty
  or not? Exit 0 with a warning, or a zero-count result?

### Adapters

- **A1. Is Flint still the target?** Verify the project's maturity and schema stability
  before the skill commits to tracking an external schema. Then decide which analyses map to
  which of its roughly 30 tabular chart types. Flint does NOT cover the hierarchical
  (enclosure, treemap) or graph (coupling, communication) flagships, so those stay custom
  either way.
- **A2. Which adapters get built, and which are dropped?** Candidates: Flint
  `ChartAssemblyInput`, Vega-Lite, GraphML/DOT, CodeCharta `.cc.json`. Each is permanent
  maintenance. Note CodeCharta wants per-file metrics, so it inherits finding 1's file-access
  dependency.
- **A3. Do adapters replace or complement the existing renderers?** The five current scripts
  emit FINISHED ARTIFACTS (HTML, SVG, PNG). Flint and Vega-Lite emit SPECS that still need a
  renderer, so adopting them may ADD a step rather than remove one. Decide whether the skill
  gains a spec lane alongside the artifact lane, or switches.
- **A4. Adapter CLI contract.** stdin envelope to stdout spec; one generic adapter driven by
  `semantics` versus one per analysis; location under docs/skills/codelens/scripts/; and test
  expectations, noting every existing script there has a `*_test.py`.

### Process

- **P1. Epic or spec 003?** The decision count here rivals spec 002, which had its own plan
  with a decisions register. A docs/specs/003-*/plan.md may fit better than one epic ticket.
- **P2. Order versus the complexity direction.** If a hierarchy shape is ever wanted,
  complexity goes first and part of this epic depends on it.
- **P3. Batch the breaking changes.** If E3 and E5 both break the envelope, land them in one
  `schema_version: 2` release rather than two.

## Research needed before deciding

- Flint's current status and schema stability (A1).
- The CodeCharta `.cc.json` contract, and whether it is reachable at all without file
  metrics (A2).
- How Tornhill's own `csv_as_enclosure_json.py` handles the node-set problem. The enclosure
  contract already notes it mirrors that structure-versus-weights split, so the prior art is
  directly relevant to finding 1.
- What each candidate adapter would actually save relative to the existing renderers (A3),
  given the measurements in finding 2.

## Refinements already queued

- Realign docs/skills/codelens/references/catalog.md, narrowed per the note below: the
  static-versus-interactive split and any per-card `Command:` line whose pipeline gets
  simpler. The per-card `Formats:` lines do NOT change; they describe rendered artifacts.
- Update docs/skills/codelens/scripts/run.bash and docs/skills/codelens/scripts/digest.py if
  new commands appear in the fleet pipeline.
- The analysis count in README.md, AGENTS.md, and docs/cli-design.md if the command set
  grows. Note README.md already explains why 18 subcommands are described as 20 analyses;
  read that before "fixing" any count.

## Explicitly a separate epic, not this one

The complexity direction (reading file content, indentation-complexity trend, a `hotspot`
fusion command) from docs/research/complexity-analysis-synthesis.md and its three detailed
reports. It is larger and independent, and it breaks the read-only, log-only-input posture,
which is precisely why the `tree` shape was deferred INTO it rather than planned here.

## Acceptance Criteria

Epic-level and provisional; to be replaced when the epic is decomposed (or when it becomes
spec 003 per P1):

- Every shape in the declared enum is emitted by at least one command. Nothing is declared
  ahead of being emittable.
- Any shape that ships has a documented payload contract in docs/cli-design.md and is
  discoverable via `codelens schema --command CMD`.
- E3 is resolved explicitly, with the answer recorded as a decision rather than implied by
  the first payload written.
- The `row_count` question (E5) is resolved for any non-table payload, with any envelope
  break carried by a `schema_version` bump and recorded in CHANGELOG.md.
- Each skill reshaping script either consumes a native shape or is documented as
  deliberately bespoke.
- codelens still emits no chart language: every viz-spec encoding lives outside the binary.

## Notes

**2026-07-27T13:45:37Z**

Scope refinement from a verification sweep (2026-07-27), before the source scratch list was retired. Two items from that list resolve differently than written:

1. "Revisit references/embedding.md and references/reporting.md for format-era assumptions" is a NO-OP, already verified. embedding.md's "Format to target" section and table describe SVG / PNG / HTML / PDF rendered-artifact formats, not codelens output formats. reporting.md contains no format references at all. Nothing to do in either file, now or when shapes land.

2. "Revisit references/catalog.md cards" is narrower than it sounds. The per-card 'Formats:' lines are also about rendered artifacts (interactive HTML, or SVG/PNG chosen by the -o extension), so they do NOT change with the envelope. What genuinely may change once shapes and specs are emitted is the static-versus-interactive split and any per-card 'Command:' line whose pipeline gets simpler. Only catalog.md:129 carried a codelens --format flag, and that is handled by the skill ticket, not here.

**2026-07-27T15:06:43Z**

Body rewritten 2026-07-27 as a standalone parked record. Two decisions were settled and moved into 'Settled decisions': the matrix and series shapes are RETRACTED (nothing over table plus semantics), and the tree shape is DEFERRED to the complexity direction because a log-only tree cannot supply the enclosure node set (see finding 1). The shape enum is trimmed to table and text, with the reachable-only rule now in ADR 0008; docs landed the same day and the code half is ticket cod-2le4. Everything else is explicitly deferred and numbered E1-E9 (envelope), A1-A4 (adapters), P1-P3 (process) so it can be referenced without renumbering. Remaining scope is one candidate shape (graph), the semantics-for-non-flat-payload question (E3), and the adapters.

**2026-07-27T15:12:26Z**

CLOSED 2026-07-27 AS DEFERRED, NOT AS DONE. None of the work described in this ticket was implemented: no non-table shape is emitted, no viz-spec adapter exists, and every item under 'Open decisions' (E1-E9, A1-A4, P1-P3) is still open. The ticket is closed only to keep the active board clear, so do not read its presence in the closed list as evidence that any of it shipped.

Reopen with 'tk reopen cod-304f' when any of these becomes true:

- A graph payload is actually wanted, at which point decision E3 (how semantics describes a non-flat payload) must be settled BEFORE any payload is written, since it is the one that gets decided by accident otherwise.
- The complexity direction lands and brings working-tree file access with it, which is the prerequisite that made the tree shape deferrable rather than buildable (see finding 1).
- A viz-spec adapter is wanted for its own sake, independently of any new shape. A1 through A4 do not depend on the shape work and could be picked up alone.

The body above remains the authoritative record and needs no re-derivation: the settled decisions, the two findings with their measurements, and the numbered open decisions are all current as of closing. Per P1, consider promoting this to a docs/specs/003-* plan with its own decisions register rather than reopening it as a single epic, since the decision count rivals spec 002.

Sequencing note: the shape enum trim (cod-2le4) lands first, so whoever reopens this starts from a two-member enum (table, text) and a payloadKey with no deferred-shape panic branch. E4 is written on that assumption.
