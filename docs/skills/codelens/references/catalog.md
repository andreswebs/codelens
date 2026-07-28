# Visualization catalog

One card per visualization: what it **consumes**, the **command** to build it, the
**formats** it emits, and how to **read** it. Loaded from [SKILL.md](../SKILL.md)
after routing. Column names are codelens JSON (snake_case), verified against build
eaece4f; re-check with `codelens schema --command <analysis>` after codelens changes.

Each card's **Read:** line is a hook; the full reading - the investigative funnel,
the heuristics table, the misuse guardrails, and how to phrase the finding - is in
[interpretation.md](interpretation.md).

## Two lanes, and what each one draws

Cards come from one of two lanes, and every card names its own:

- The **artifact lane** is the five renderer scripts (`enclosure.py`,
  `treemap.py`, `coupling_graph.py`, `dev_network.py`, `churn.py`, `fractal.py`,
  `commit_cloud.py`, `complexity_trend.py`, `pair_matrix.py`). Each writes a
  finished artifact: interactive HTML, or static SVG/PNG. Needs `uv` only.
- The **spec lane** is `flint_spec.py`, which writes a Flint chart *spec*; a
  downstream renderer turns it into SVG or PNG. Needs `uv` plus either Deno or an
  attached flint-chart MCP client. See [flint.md](flint.md) for both rendering
  paths, the MCP stdio config, and the truncation footgun.

The split between them is fixed, so that no chart grows two cards that disagree
about how to draw it. The spec lane **replaces**:

- `churn.py`'s two chart bodies - the churn trend and the summary tiles.
- The chart half of `pair_matrix.py` - the coupling and communication heatmaps
  (`--chart-type Heatmap` on the spec-lane cards below).

The spec lane does **not** replace, for structural reasons rather than
incidental ones:

- `enclosure.py` and `treemap.py`. Flint's `ChartEncoding.field` is a singular
  string, so its Treemap gets one hierarchy level and its Sunburst two; a
  filesystem path needs arbitrary depth. Flint having a treemap is not the point.
- `commit_cloud.py`. Flint has no word cloud on either backend.
- `complexity_trend.py` and `fractal.py`. These keep their bespoke forms until
  the spec lane is proven; a normalized stacked bar is a *substitute* for a
  fractal figure, not an equivalent.
- The glob filtering, the tokei structure join, and the area-domination warning.
  Those are analysis concerns, not chart concerns, and no chart language has a
  notion of them.

Before adding a card, find which lane already draws that chart.

---

## Hotspot enclosure map

- **Consumes:** `codelens revisions` -> `entity, n_revs`.
- **Sidecar:** `tokei --output json` (size). Optional; degrades to revisions-as-size.
- **Command:** `uv run scripts/enclosure.py --weights revs.json --structure tokei.json -o hotspots.html`
- **Static:** `uv run scripts/treemap.py --weights revs.json --structure tokei.json -o hotspots.svg` (embeddable SVG/PNG; same flags and structure-first node set as `enclosure.py`).
- **Formats:** interactive HTML (iframe embed); static counterpart below.
- **Read:** the **offender profile** is big + hot, but colour (change) is the lead
  signal and size (LOC) the severity multiplier - a large pale circle is
  complex-but-stable. Scope out generated files first (false positives); large
  reference/spec files can dominate the map by size (the tool warns `dominant:` over
  10% of mapped LOC, `--exclude` them). Heuristics and full reading:
  [interpretation.md](interpretation.md). Contract: [enclosure.md](enclosure.md).

## Knowledge map

- **Consumes:** `codelens main-developer` -> `entity, main_dev, ownership`.
- **Sidecar:** `tokei --output json`.
- **Aliases:** author aliases split one person's ownership. The emitted log uses
  `--use-mailmap`, so a repo `.mailmap` collapses them for free; when there is no
  `.mailmap`, map aliases to a canonical name with `--team-map`.
- **Command:** `uv run scripts/enclosure.py --weights main-dev.json --weight-col main_dev --categorical --structure tokei.json -o knowledge.html`
- **Static:** `uv run scripts/treemap.py --weights main-dev.json --weight-col main_dev --categorical --structure tokei.json -o knowledge.svg`.
- **Formats:** interactive HTML (iframe embed); static counterpart below.
- **Read:** one color per developer, circles sized by tokei LOC. With `--structure`
  the node set is the tokei files (same as the hotspot and code-age maps, so all
  three are comparable); a file with no recorded author renders neutral grey
  (`(unowned)`). Single-color components are key-person dependencies; mixed
  components are shared effort. The main developer's ownership *degree* is the real
  signal, not just single-vs-mixed colour. See [enclosure.md](enclosure.md) and the
  reading in [interpretation.md](interpretation.md).

## Code-age map

- **Consumes:** `codelens code-age` -> `entity, age_months`.
- **Sidecar:** `tokei --output json`.
- **Command:** `uv run scripts/enclosure.py --weights age.json --weight-col age_months --invert --structure tokei.json -o age.html`
- **Static:** `uv run scripts/treemap.py --weights age.json --weight-col age_months --invert --structure tokei.json -o age.svg`.
- **Formats:** interactive HTML (iframe embed); static counterpart below.
- **Lane:** artifact, and **not** replaced (arbitrary path depth). The spec lane's
  [code-age histogram](#code-age-histogram-spec-lane) answers the distribution
  question the map cannot; the two are complements.
- **Read:** hot = recently changed (low age). Read through **stabilization**: stable
  cores are a virtue, and old code that still churns is a low-cohesion smell (no age
  threshold or "frozen" rule). Reading: [interpretation.md](interpretation.md).
- **Full history required:** run `code-age` against full history, not a window
  scoped with `--after`. Age is measured from the log's earliest commit, so a
  scoped window caps every file's reported `age_months` at the window length and
  the map looks uniformly young.

> **Authored-only maps (all three enclosure maps):** generated files
> (`**/Migrations/**`, `**/*.g.dart`, `**/*.Designer.cs`, lock files) dominate a
> monorepo and drown out real hotspots. Pass one shared `--exclude` glob set to
> **both** the `codelens` analysis and `enclosure.py`, so the weights and the drawn
> tokei structure agree; `enclosure.py` filters the structure `codelens` cannot
> see. `enclosure.py --include/--exclude` use the same gitignore-style globs
> (`**` supported, exclude-after-include). See operating.md, "Authored-only run".
> Do not exclude config or localization sources (`*.yml`, `*.arb`, `*.resx`) that
> are human-authored.

## Change-coupling graph

- **Consumes:** `codelens coupling` -> `entity, coupled, degree, average_revs`;
  `sum-of-coupling` for node weight.
- **Command:** `uv run scripts/coupling_graph.py --coupling coupling.json -o coupling.html`
- **Static:** `uv run scripts/pair_matrix.py --pairs coupling.json --a-col entity --b-col coupled --weight-col degree -o coupling.svg` (adjacency-matrix heatmap of the top-N most-coupled entities).
- **Formats:** interactive HTML (iframe embed); static counterpart below.
- **Lane:** artifact. The static heatmap here is what the spec lane replaces; see
  [Coupling edge chart](#coupling-edge-chart-spec-lane). The interactive HTML graph
  is not replaced.
- **Read:** edges co-change (degree = % shared commits; node weight =
  sum-of-coupling = architectural centrality). The signal is **surprising** coupling
  that crosses an architectural boundary (group with `--group`), not raw high degree -
  a test and its implementation sit near 100% and are benign. Causes: copy-paste
  (extract), unsupportive module boundaries (co-locate), producer-consumer (often
  legitimate). Reading: [interpretation.md](interpretation.md).
- **Empty result:** grouping to components dilutes per-pair degrees, so the
  default `--min-coupling 30` can filter everything and return `rows: []`. When
  that happens codelens emits a `coupling_all_filtered` warning on stderr naming
  the highest degree it actually observed; lower `--min-coupling` (around 5) to
  see the weaker component links.

## Communication network

- **Consumes:** `codelens communication` -> `author, peer, shared, strength`.
- **Aliases:** the emitted log's `--use-mailmap` collapses author aliases from a
  repo `.mailmap` first; with no `.mailmap`, use `--team-map` to map aliases to a
  canonical identity (in the test-drive this collapsed 34 -> 24 authors and
  removed a spurious self-tie).
- **Sidecar (optional):** `--team-map` to collapse authors to teams.
- **Command:** `uv run scripts/dev_network.py --communication comm.json -o network.html`
- **Static:** `uv run scripts/pair_matrix.py --pairs comm.json --a-col author --b-col peer --weight-col strength --note 'coordination risk, not a performance ranking' -o network.svg`.
- **Formats:** interactive HTML (iframe embed); static counterpart below.
- **Lane:** artifact. The static heatmap here is what the spec lane replaces; see
  [Communication edge chart](#communication-edge-chart-spec-lane). The interactive
  HTML graph is not replaced.
- **Read:** Conway litmus test - aggregate authors to teams first (`--team-map`).
  Mostly intra-team links = healthy; inter-team links are *potential* coordination
  bottlenecks (the usual fix is cohesion, not reorg). Reading:
  [interpretation.md](interpretation.md).

## Churn trend

- **Consumes:** `codelens absolute-churn` -> `date, added, deleted, commits`
  (also `author-churn`, `entity-churn`).
- **Command:** `uv run scripts/churn.py --churn churn.json -o churn.svg`
- **Formats:** SVG or PNG (the -o extension picks the format).
- **Lane:** artifact, and this chart body is replaced by the spec lane; see
  [Churn stacked bars](#churn-stacked-bars-spec-lane). Keep this one for a run
  with no Deno and no MCP client.
- **Read:** added vs deleted over time. Sustained one-sided growth without deletion
  is accumulation; spikes flag large reworks.

## Fractal figures

- **Consumes:** `codelens entity-effort` -> `entity, author, author_revs,
total_revs`; `fragmentation` for the scalar.
- **Command:** `uv run scripts/fractal.py --effort effort.json -o fractal.svg`
- **Formats:** SVG or PNG (the -o extension picks the format).
- **Lane:** artifact, and **not** replaced. The spec lane's
  [ownership share bars](#ownership-share-bars-spec-lane) are a substitute view of
  the same question, not an equivalent figure.
- **Read:** three ownership patterns: single developer, balanced (higher main-dev
  ownership predicts fewer defects), many minor contributors (defect risk - the
  *count* of minor contributors is the stronger predictor). Reading:
  [interpretation.md](interpretation.md).

## Commit word cloud

- **Consumes:** `codelens parse` -> the `message` column.
- **Command:** `codelens parse --log git.log | uv run scripts/commit_cloud.py -o cloud.svg`
- **Formats:** SVG or PNG (the -o extension picks the format).
- **Read:** heuristic only, a conversation starter. Dominant words show where time
  goes: domain terms = good; "bug", "crash", "revert", "bump" = drill deeper.
  Reading: [interpretation.md](interpretation.md).

## Complexity trend

- **Consumes:** the **live repo** (not codelens): a repo path + a file path.
- **Command:** `uv run scripts/complexity_trend.py --repo . --file path/to/hotspot -o trend.svg`
- **Formats:** SVG or PNG (the -o extension picks the format).
- **Read:** indentation complexity over revisions. Shapes: deteriorating (act),
  refactored (dip = good), stable. Overlay LOC: rising with LOC = growth by
  addition; complexity outpacing LOC = deterioration. Reading:
  [interpretation.md](interpretation.md).

## Summary tiles

- **Consumes:** `codelens summary` -> `statistic, value`.
- **Command:** `uv run scripts/churn.py --summary summary.json -o summary.svg`
- **Formats:** SVG or PNG (the -o extension picks the format).
- **Lane:** artifact, and this chart body is replaced by the spec lane; see
  [Summary KPI cards](#summary-kpi-cards-spec-lane). Keep this one for a run with
  no Deno and no MCP client.
- **Read:** headline counts (commits, entities, authors) for a report header.

---

## Spec lane

Every card below is **lane: spec**. Each is a two-step build: `flint_spec.py`
emits the spec, then one of the two rendering paths in [flint.md](flint.md) turns
it into an artifact. Shared conventions, stated once instead of on every card:

- **Command** is the spec step. Capture `codelens schema --command CMD` first and
  pass it as `--schema`, so axis labels come from each column's `desc` rather than
  the raw column name.
- Bound rows with codelens `--rows`, never by letting Flint truncate. `--rows` cuts
  by the analysis's own ranking; Flint cuts by axis sort order inside a canvas
  budget and self-labels the chart `...N items omitted`. See
  [flint.md](flint.md#bound-rows-with-codelens-not-with-flint).
- **Render** is the same two paths for all of them, so it is written once here:

  ```sh
  deno run --allow-env --allow-read --allow-sys --allow-ffi --allow-write \
    scripts/flint_render.ts --backend "$BACKEND" --format svg \
    -o chart.svg <chart.flint.json
  ```

  or hand the spec to the flint-chart MCP server's `render_chart`. Each card names
  its `$BACKEND`, because the spec carries none and Heatmap exists on both.
- **Formats:** the spec (`.flint.json`), then SVG or PNG from either path
  (`--format png --scale N` for a higher-resolution PNG). There is no interactive
  HTML in this lane; the nearest thing is the MCP server's `create_chart_view`,
  an editable live view in hosts that support MCP App UIs.

## Coupling edge chart (spec lane)

- **Consumes:** `codelens coupling` -> `entity, coupled, degree, average_revs`.
- **Command:** `codelens coupling --log git.log --rows 100 | uv run scripts/flint_spec.py --schema coupling.schema.json -o coupling.flint.json`
- **Chart:** `Network Graph`, **backend `echarts`** (`x: entity`, `y: coupled`,
  `size: degree`). ECharts-only, so Deno is the only headless route to it.
- **Alternate:** `--chart-type Heatmap --backend vegalite` is the adjacency matrix,
  and this is the chart that **replaces** `pair_matrix.py`'s coupling output.
- **Read:** as the [change-coupling graph](#change-coupling-graph) - surprising
  coupling across an architectural boundary, not raw high degree. The same
  `coupling_all_filtered` warning applies: with `--group`, lower `--min-coupling`.
  `flint_spec.py` exits `3` on an empty payload, which is that filter, not a bug.

## Communication edge chart (spec lane)

- **Consumes:** `codelens communication` -> `author, peer, shared, strength`.
- **Command:** `codelens communication --log git.log --rows 100 | uv run scripts/flint_spec.py --schema communication.schema.json -o network.flint.json`
- **Chart:** `Network Graph`, **backend `echarts`** (`x: author`, `y: peer`,
  `size: strength`).
- **Alternate:** `--chart-type Heatmap --backend vegalite`, which **replaces**
  `pair_matrix.py`'s communication output.
- **Read:** as the [communication network](#communication-network) - aggregate to
  teams with `--team-map` first, and never rank individuals. This lane has no
  equivalent of `pair_matrix.py --note`, so carry the "coordination risk, not a
  performance ranking" caveat in the surrounding prose instead.

## Churn stacked bars (spec lane)

- **Consumes:** `codelens absolute-churn` -> `date, added, deleted, commits`.
- **Command:** `codelens absolute-churn --log git.log | uv run scripts/flint_spec.py --schema absolute-churn.schema.json -o churn.flint.json`
- **Chart:** `Stacked Bar Chart`, **backend `vegalite`** (`x: date`,
  `y: [added, deleted]`). The two-measure bind is what keeps the
  refactor-versus-construction signal; one measure alone loses it.
- **Replaces:** `churn.py`'s churn-trend chart body.
- **Rows:** do **not** cap this one with `--rows`. `absolute-churn` sorts by date
  ascending, so a cap silently drops the oldest end of the trend; bound the window
  with the log's `--after` instead.
- **Read:** as the [churn trend](#churn-trend).

## Summary KPI cards (spec lane)

- **Consumes:** `codelens summary` -> `statistic, value`.
- **Command:** `codelens summary --log git.log | uv run scripts/flint_spec.py --schema summary.schema.json -o summary.flint.json`
- **Chart:** `KPI Card`, **backend `vegalite`** (`metric: statistic`,
  `value: value`), which compiles to a `vconcat` of one card per statistic. A bar
  chart of unrelated statistics on one shared scale is meaningless, which is why
  this is not the generic default.
- **Replaces:** `churn.py --summary`.
- **Rows:** unbounded. There are a handful of statistics and every one is a tile.
- **Read:** headline counts for a report header, as the
  [summary tiles](#summary-tiles).

## Code-age histogram (spec lane)

- **Consumes:** `codelens code-age` -> `entity, age_months`.
- **Command:** `codelens code-age --log git.log | uv run scripts/flint_spec.py --schema code-age.schema.json -o age.flint.json`
- **Chart:** `Histogram`, **backend `vegalite`** (`x: age_months`). The object here
  is the age *distribution*, not a per-file ranking.
- **Rows:** unbounded - a truncated distribution is a misleading one.
- **Full history required:** the same constraint as the
  [code-age map](#code-age-map). Age is measured from the log's earliest commit, so
  a windowed log caps every file's `age_months` and the histogram collapses into
  the young bins.
- **Read:** through **stabilization**. A mass in the old bins is a stable core and
  a virtue; a distribution with no old mass at all says nothing has settled.

## Author coordination scatter (spec lane)

- **Consumes:** `codelens authors` -> `entity, n_revs, n_authors`.
- **Command:** `codelens authors --log git.log --rows 200 | uv run scripts/flint_spec.py --schema authors.schema.json -o authors.flint.json`
- **Chart:** `Scatter Plot`, **backend `vegalite`** (`x: n_revs`, `y: n_authors`).
- **New chart:** the artifact lane has no counterpart, so nothing is replaced.
- **`entity` is deliberately unbound.** `detail` is not a Scatter Plot channel in
  Flint (it is silently dropped), and `color`/`shape`/`size` would each ask for
  hundreds of distinct values. The finding is the *shape of the cloud* - the
  high-revisions, many-authors corner is the coordination hotspot - not which point
  is which file. Read per-file identity off the `authors` table itself.

## Ranked lollipop (spec lane)

- **Consumes:** `codelens revisions` -> `entity, n_revs`; or `sum-of-coupling` ->
  `entity, soc`.
- **Command:** `codelens revisions --log git.log --rows 30 | uv run scripts/flint_spec.py --schema revisions.schema.json --chart-type 'Lollipop Chart' -o revisions.flint.json`
- **Chart:** `Lollipop Chart`, **backend `vegalite`**. Not a default: without
  `--chart-type` these analyses take the generic `Bar Chart`. Lollipops read better
  for a long ranked list.
- **New chart:** nothing is replaced. For hotspots as an area map, the
  [hotspot enclosure map](#hotspot-enclosure-map) remains the flagship - a ranked
  list is a different question from a sized map.
- **Read:** the head of the ranking, and where it flattens. A long flat tail means
  change is diffuse rather than concentrated in hotspots.

## Ownership share bars (spec lane)

- **Consumes:** `codelens entity-effort` -> `entity, author, author_revs,
  total_revs`; or `entity-ownership`.
- **Command:** `codelens entity-effort --log git.log --rows 60 | uv run scripts/flint_spec.py --schema entity-effort.schema.json --chart-type 'Stacked Bar Chart' -o effort.flint.json`
- **Chart:** `Stacked Bar Chart`, **backend `vegalite`**, and `flint_spec.py` sets
  `stackMode: normalize` automatically whenever a stacked bar has a dimension on
  `color`, so the shares are comparable across entities.
- **Substitute, not a replacement:** this is the ownership-fragmentation view; the
  [fractal figures](#fractal-figures) card still owns the fractal form.
- **One measure only.** These analyses carry both a component (`author_revs`) and
  its own total (`total_revs`); the adapter binds the component alone. Stacking a
  component together with its total is Flint's documented total-mixed-with-
  components hazard, and the resulting chart double-counts.
- **Read:** as the [fractal figures](#fractal-figures) - the *count* of minor
  contributors is the stronger defect predictor, and the social guardrails in
  [interpretation.md](interpretation.md) apply here too.
