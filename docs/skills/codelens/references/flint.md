# Flint spec lane

The five renderer scripts emit finished artifacts (HTML, SVG, PNG). The spec
lane emits a **chart spec**: `flint_spec.py` reads a codelens result envelope
on stdin and writes a Flint (flint-chart) `ChartAssemblyInput` on stdout. A
downstream renderer turns that spec into an artifact; there are two rendering
paths, documented below as alternatives, not as a choice the skill made once.
The spec-lane cards in [catalog.md](catalog.md) say which charts the lane
produces and what each replaces.

## Emitting a spec

```sh
codelens schema --command coupling > coupling.schema.json
codelens coupling --log git.log --rows 100 |
  uv run scripts/flint_spec.py --schema coupling.schema.json -o coupling.flint.json
```

- The chart type and backend default per analysis (the catalog cards name
  them); `--chart-type` overrides, and an unknown name lists the valid set.
- `--schema FILE` takes `codelens schema --command CMD` output and populates
  the spec's field display names from each column's `desc`, so axes read
  "number of distinct revisions" instead of `n_revs`. Optional: omitted, the
  spec degrades to raw column names. Capture it once per analysis, as
  `run.bash` does.
- `--backend vegalite|echarts` targets a backend; `--width`/`--height` set the
  base size; `-o` writes to a file. Exit codes match the other scripts: `0`
  ok, `2` usage or a rejected envelope, `3` empty payload.
- The emitted spec carries **no backend field**. `flint_spec.py` reports the
  chosen backend only on stderr (`emitted Network Graph (echarts) ...`), so a
  pipeline must pass `--backend` to the renderer explicitly or accept its
  inference (first backend declaring the chart type, Vega-Lite preferred);
  chart types like Heatmap exist on both backends.

## Bound rows with codelens, not with Flint

Both codelens and Flint truncate, and they truncate differently. codelens
`--rows` cuts by the analysis's own ranking, a deliberate top-N (coupling
sorts by degree descending, communication by strength). Flint cuts by axis
sort order within a canvas budget, a layout accident: measured at 0.4.0, a
300-row bar chart kept 91 values and omitted 209. Prefer `--rows`.

A genuine footgun: Flint's truncation lands on a **private `_warnings`**
field of the assembled spec, and the chart self-labels the omission in its
axis domain. A rendered chart showing `...209 items omitted` is Flint's
canvas budget at work, not a codelens bug. `flint_render.ts` re-emits every
`_warnings` entry as a one-line JSON diagnostic on stderr so the deterministic
lane surfaces truncation the way codelens surfaces its own warnings.

## Rendering path 1: Deno, deterministic and headless

`flint_render.ts` renders a spec in-process with no MCP client attached, and
is the only headless route to the ECharts-only templates (the coupling and
communication `Network Graph` charts). It pins `flint-chart-mcp@0.4.0`.

The pin is re-verified **by hand**: a six-item upgrade checklist sits directly
above the `import` in `scripts/flint_render.ts`, and nothing in the test suite
enforces it. Read it before bumping either Flint pin. It covers the 9 semantic
type names, the `Network Graph` source/target convention, the private
`_warnings` field, whether `intrinsicDomain` is a gate or an override, the 6
default chart-type names, and the declared channel lists. That last item is the
only scheduled parity check for `flint_spec.py`'s channel table, which is a copy
of Flint data no unit test can verify. The checklist also records why the pin is
0.4.0 rather than latest: Deno's default `minimumDependencyAge` guard refuses a
version published under 24 hours ago.

```sh
deno run --allow-env --allow-read --allow-sys --allow-ffi --allow-write \
  scripts/flint_render.ts --backend echarts --format svg \
  -o coupling-graph.svg <coupling.flint.json
```

Always pass the permission flags shown (`--allow-write` only for `-o`). PNG
needs `sys` and `ffi` for the resvg native binding, and Vega-Lite SVG output
**changes** when the ffi-loaded canvas module is unavailable (vega falls back
to approximate text metrics silently), so one canonical flag set is what keeps
renders reproducible. `--format png` with `--scale N` multiplies PNG
resolution. Exit codes: `0` ok, `2` usage or a rejected spec.

The renderer re-validates every encoding channel against Flint's own declared
template channels, so a spec produced by a stale channel table fails loudly
here rather than rendering with a channel silently dropped.

### First run downloads a large dependency tree

Flint pulls vega, vega-lite, echarts, plotly, chart.js, and native bindings for resvg
and canvas. The first `flint_render.ts` invocation therefore prints a few hundred
`Download https://registry.npmjs.org/...` lines to stderr and takes noticeably longer
than later ones; Deno caches it globally afterwards, so subsequent runs are quiet and
fast. In a `run.bash` fleet run this cost lands entirely on the first repository.

Two consequences worth knowing. The noise is on stderr and interleaves with the render
summary, so read the LAST line (`rendered <chartType> (<backend>) svg: WxH`) rather than
scanning stderr for problems. And the first run needs network: on an offline machine
`command -v deno` still succeeds while the render fails, which `run.bash` reports
per-figure and treats as non-fatal, leaving the specs in `OUT/flint/` for the MCP path.

## Rendering path 2: MCP, for agent and interactive use

The flint-chart MCP server renders in-process with no browser: run it with
`npx -y flint-chart-mcp`. Five tools:

- `render_chart`: returns PNG or SVG inline.
- `compile_chart`: returns the compiled backend spec without rendering.
- `validate_chart`: a pre-render check on a spec.
- `list_chart_types`: the template inventory per backend.
- `create_chart_view`: an editable live view in hosts supporting MCP App UIs.

This project ships **no `.mcp.json`** and does not register the server:
registering an MCP server is host-level state the user owns, and a
project-scoped server would only help people working inside this repository
rather than users of the skill. Documentation is therefore the only way this
path is discovered, so here is the stdio config verbatim; add it to your MCP
client's configuration:

```jsonc
{ "mcpServers": {
    "flint-chart": { "command": "npx", "args": ["-y", "flint-chart-mcp"] } } }
```

The MCP path is available only when a client is attached, which is why it
cannot be the deterministic path and why `run.bash` uses Deno.

## run.bash integration

`scripts/run.bash` wires the lane additively: it captures each analysis's
schema and emits specs into `OUT/flint/` (needing only `uv`, like the rest of
the skill), then renders them to `OUT/figs/flint-*.svg` when `deno` is on
PATH. Without Deno the specs still land for the MCP path and everything else
runs unchanged.
