---
id: cod-di2f
status: closed
deps: [cod-6kzk]
links: []
created: 2026-07-28T16:48:28Z
type: task
priority: 2
assignee: Andre Silva
tags: [codelens, spec-003, skill, docs]
---
# Wire the Flint spec lane into the codelens skill: catalog, run.bash, references

Wire the Flint spec lane into the codelens skill: catalog cards, `run.bash` steps, and
reference documentation for both rendering paths.

Spec: docs/specs/003-flint-adapter/plan.md sections 4, 5, 6.3 (ticket 3 of four).

## Why the skill gains a lane rather than switching

The five existing renderer scripts emit FINISHED ARTIFACTS (HTML, SVG, PNG). Flint emits
SPECS. The two coexist because three things make the artifact lane load-bearing
regardless:

1. The hierarchical flagships cannot be expressed in Flint at all.
   `ChartEncoding.field` is a singular string, so Flint's Treemap gets ONE hierarchy
   level and Sunburst two. A filesystem path needs arbitrary depth. `enclosure.py` and
   `treemap.py` survive on that ground, not on Flint lacking a treemap (it has one).
2. The MCP path is only available when a client is attached, so `run.bash` cannot depend
   on it.
3. The existing scripts do work Flint has no notion of: glob filtering, the tokei
   structure join, and the area-domination warning. Those are analysis concerns.

`commit_cloud.py` also stays: Flint has no word cloud on any backend.

## Design

## 1. `catalog.md` cards

Add cards for the charts the spec lane now produces. Keep the existing per-card
conventions: a `Command:` line, a `Formats:` line, and the static-versus-interactive
split. Be honest about WHICH LANE produces each artifact, since a reader choosing a chart
needs to know whether they need Deno or an MCP client.

Do not duplicate an existing card. Section 4 of the plan states what the spec lane
REPLACES (`churn.py`'s two chart bodies, and the chart half of `pair_matrix.py`) and what
it does not. That split must be stated IN THE SKILL, not only in the plan, or the catalog
will grow two cards that disagree about how to draw the same thing.

## 2. `run.bash`

- Capture `codelens schema --command CMD` per analysis, so `flint_spec.py --schema` is
  populated and axis labels come from each column's `desc` rather than raw column names.
- PREFER codelens `--rows` over letting Flint truncate. Both truncate, but `--rows`
  truncates by the analysis's own ranking while Flint truncates by axis sort order within
  a canvas budget: the former is a deliberate top-N, the latter a layout accident.
  Measured: 300 rows in, 91 kept, 209 omitted.
- Keep the existing pipeline working unchanged for anyone without Deno. The spec lane is
  additive.

## 3. `references/` documentation

Document BOTH rendering paths as alternatives, not as a choice the skill made once.

**Deno, for the deterministic path.** The `flint_render.ts` invocation, pinned at 0.4.0,
with its permission flags. Note that this is the only headless route to the ECharts-only
`Network Graph` charts.

**MCP, for agent and interactive use.** `npx -y flint-chart-mcp`. Five tools:
`render_chart` (PNG/SVG inline), `compile_chart`, `validate_chart` (a pre-render check the
skill has never had), `list_chart_types`, `create_chart_view` (an editable live view in
hosts supporting MCP App UIs).

The project ships NO `.mcp.json` and does not register the server (decision Q7):
registering an MCP server is host-level state the user owns, and a project-scoped server
would only help people working inside this repository rather than users of the skill.
DOCUMENTATION IS THEREFORE THE ONLY WAY ANYONE DISCOVERS THIS PATH, so carry the stdio
config verbatim:

```jsonc
{ "mcpServers": {
    "flint-chart": { "command": "npx", "args": ["-y", "flint-chart-mcp"] } } }
```

Also document, because it is a genuine footgun: Flint's truncation lands on a PRIVATE
`_warnings` field and the chart self-labels the omission, so a chart showing
"...209 items omitted" is Flint's canvas budget, not a codelens bug.

## 4. Counts and cross-references

If the analysis or command counts change anywhere, note that `README.md` already explains
why 18 subcommands are described as 20 analyses; read that before "fixing" any count.

## Acceptance Criteria

- `catalog.md` cards added for the spec-lane charts, each honest about which lane
  produces it and in what formats.
- Section 4's replaces/does-not-replace split stated IN THE SKILL, so the catalog cannot
  grow two cards that disagree.
- `run.bash` captures `codelens schema --command CMD` per analysis and passes it to
  `flint_spec.py --schema`.
- `run.bash` bounds results with codelens `--rows` rather than relying on Flint
  truncation.
- The existing pipeline still runs end to end for a user with no Deno installed.
- `references/` documents both rendering paths, and carries the MCP stdio config
  verbatim.
- No `.mcp.json` is committed.
- All touched markdown passes markdownlint against the project config.
- `make build` green.


## Notes

**2026-07-28T18:45:55Z**

Skill wiring for the Flint spec lane. catalog.md gained a 'Two lanes, and what each one draws' section stating plan section 4's replaces/does-not-replace split in the skill itself, plus eight spec-lane cards (coupling/communication edge charts, churn stacked bars, summary KPI cards, code-age histogram, authors scatter, ranked lollipop, ownership share bars). Four artifact-lane cards (change-coupling graph, communication network, churn trend, summary tiles) gained a 'Lane:' line cross-linking the spec-lane card that replaces their chart body; the code-age map and fractal cards say explicitly that they are NOT replaced, so no two cards can disagree about the same chart. Shared spec-lane conventions (schema capture, --rows over Flint truncation, the deno invocation, formats) are stated once in the section preamble rather than on every card. SKILL.md now names both lanes under the routing table and points step 4 and step 6 at flint.md; before this the reference was unreachable from the skill entry point. run.bash and references/flint.md were already in place from earlier work on this ticket; both were verified end to end on this repo: run.bash produces out/flint/*.flint.json plus out/figs/flint-*.svg with deno present, and with deno absent from PATH it emits the specs, logs 'deno not on PATH', and leaves every artifact-lane figure unchanged. coupling and communication specs fail on this repo only because those analyses return zero rows at default thresholds (coupling_all_filtered); the artifact-lane figures fail on the same data. No .mcp.json committed. flint_spec_test.py 42/42 and flint_render_test.ts 15/15 green; make build green. Note for the next person: flint_render_test.ts needs --allow-run in addition to the render permission set, since it spawns the script as a subprocess.
