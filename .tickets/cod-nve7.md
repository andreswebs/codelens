---
id: cod-nve7
status: closed
deps: [cod-6kzk]
links: []
created: 2026-07-28T16:45:20Z
type: feature
priority: 2
assignee: Andre Silva
tags: [codelens, spec-003, skill, viz, deno]
---
# flint_render.ts: headless Deno renderer for Flint specs, with warning and channel checks

Add `docs/skills/codelens/scripts/flint_render.ts`: renders a Flint spec to SVG or PNG,
headless, with no MCP client attached.

Spec: docs/specs/003-flint-adapter/plan.md sections 5 and 6.2 (ticket 2 of four).

## Why Deno and not Node

The skill-builder convention for a self-contained TypeScript script is Deno with inline
`npm:` specifiers, chosen over Node precisely because Node needs a `package.json` plus
`npm install` and thereby breaks self-containment. This is the first TypeScript script in
the skill; it sits beside the Python ones as a sibling under the same rule.

## Why this lane is required, not optional

Flint's ECharts-only templates are unreachable from Python: `flint-chart` is not on PyPI,
and `packages/flint-py` is a source-only preview implementing the Vega-Lite backend only.
The `Network Graph` charts for `coupling` and `communication` are ECharts-only, so
without this script they cannot be rendered on the deterministic path at all.

MCP (`npx -y flint-chart-mcp`) also renders, but only when a client is attached, so it
cannot serve `run.bash`. Both paths are documented; neither replaces the other.

## Design

## Entry point

Flint's MCP package exports its render core, which is the shared SSR recipe:

```ts
import { renderChart } from 'npm:flint-chart-mcp@0.4.0/render'
const { buffer, warnings } = await renderChart(input, 'echarts', { format: 'png', scale: 2 })
```

Backends: `vegalite`, `echarts`, `chartjs`. Note `chartjs` renders PNG only (its engine
has no SVG output). How it renders, all in-process with no browser: Vega-Lite compiles to
Vega and runs through a headless `vega.View` to SVG; ECharts renders server-side to SVG;
PNG comes from `@resvg/resvg-js`.

## PIN 0.4.0, NOT 0.4.1

Deno's default `minimumDependencyAge` supply-chain guard REFUSES a version published
under 24 hours ago. Verified: `deno run` on `flint-chart@0.4.1` failed with
"blocked by the minimum dependency age policy". Weakening that default for a patch
release is a bad trade. Do not bump without reading section 5.1 of the plan.

## MUST re-emit `_warnings` on stderr

Flint truncates independently of codelens, and it reports on a PRIVATE field. Verified: a
300-row bar chart keeps 91 values and omits 209, recording:

```json
{"severity":"warning","code":"overflow",
 "message":"209 of 300 values in 'entity' were omitted (showing first 91 in sort order).",
 "channel":"y","field":"entity"}
```

That lands on `_warnings` (underscore-prefixed), NOT `warnings`, on the assembled spec.
The chart also self-labels the omission in its axis domain (`...209 items omitted`), so
it is not invisible, but only whoever reads `_warnings` or looks at the picture sees it.

This script must read `_warnings` and re-emit each entry on stderr, so the deterministic
lane surfaces truncation the way codelens surfaces its own warnings. `_warnings` being
private is an accepted risk recorded in the plan's section 9; item 3 of the upgrade
checklist covers it.

## MUST re-validate encodings against Flint (F12, layer 2)

This script HAS the Flint dependency, so unlike `flint_spec.py` it can ask Flint what a
template accepts:

```ts
import { vlGetTemplateChannels, ecGetTemplateChannels } from 'npm:flint-chart@0.4.0'
```

Check every key in the spec's `encodings` against the declared channel list for its
`chartType`, and error on an unknown one. This is the AUTHORITATIVE half of the
validation; `flint_spec.py`'s table is a copy that no unit test can verify, because
decision Q8 ruled out a Deno test lane in CI.

Useful side effect: because this errors on an unknown channel, a Flint upgrade that
renames one fails loudly the first time anything renders. That is the drift detection Q8
declined to build in CI, arriving free on the render path. It does NOT cover a spec that
is emitted and never rendered, which is the accepted residual gap.

## Determinism

The ECharts `Network Graph` golden is safe to pin. Verified from
`echarts/templates/graph.ts`: `layout` defaults to `circular` explicitly "so the render
is deterministic (no force simulation needed to settle)". A `force` layout is available
as a chart property; do NOT make it the default, or goldens become unstable.

## Permissions

Deno needs explicit flags. Keep them minimal and document the exact invocation in the
module docstring, matching how the Python scripts document theirs.

## Acceptance Criteria

- `npm:flint-chart-mcp@0.4.0/render` pinned; renders SVG and PNG for `vegalite` and
  `echarts` (note `chartjs` is PNG-only if offered at all).
- Runs headless with NO MCP client attached.
- Re-emits every `_warnings` entry on stderr, one per line, so truncation is visible on
  the deterministic lane.
- Re-validates every encoding channel against `vlGetTemplateChannels` /
  `ecGetTemplateChannels` and errors on an undeclared one.
- Renders each of the 6 override fixtures from the previous ticket.
- The ECharts `Network Graph` output is byte-stable across runs (circular layout).
- A test file beside it, matching the convention that every script in that directory has
  one.
- Exit codes consistent with the Python scripts: `0` ok, `2` usage.
- `make build` green.


## Notes

**2026-07-28T18:12:32Z**

Added docs/skills/codelens/scripts/flint_render.ts (Deno, pinned npm:flint-chart-mcp@0.4.0/render) plus flint_render_test.ts (15 subprocess tests; run: deno test --allow-run --allow-read --allow-write --allow-env flint_render_test.ts). Renders SVG (default) and PNG (--scale) for vegalite and echarts; chartjs not offered. Backend inferred from Flint's template registries, vegalite preferred, --backend overrides (KPI Card is vegalite-only; Heatmap/Bar Chart exist on both). F12 layer 2 done via vl/ecGetTemplateChannels: undeclared channel or unknown chartType exits 2 naming the declared set. _warnings re-emitted: renderChart's result.warnings IS the assembled spec's _warnings (verified in package source), each emitted as a one-line JSON stderr diagnostic; test pins the overflow entry. Network Graph SVG byte-stable across processes (circular layout); NOT stable within one process (zrender id counters), so the golden test uses two invocations. All 6 override fixtures render. Gotcha for ticket 3 (run.bash): always pass --allow-env --allow-read --allow-sys --allow-ffi — vega-lite SVG output CHANGES if the ffi canvas module cannot load (silent text-metrics fallback); documented in the docstring. Exit codes 0/2; render errors caught, no stack traces. Learnings recorded in docs/specs/learnings.md.
