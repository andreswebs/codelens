# Visualization catalog

One card per visualization: what it **consumes**, the **command** to build it, the
**formats** it emits, and how to **read** it. Loaded from [SKILL.md](../SKILL.md)
after routing. Column names are codelens JSON (snake_case), verified against build
eaece4f; re-check with `codelens schema --command <analysis>` after codelens changes.

Each card's **Read:** line is a hook; the full reading - the investigative funnel,
the heuristics table, the misuse guardrails, and how to phrase the finding - is in
[interpretation.md](interpretation.md).

---

## Hotspot enclosure map

- **Consumes:** `codelens revisions` -> `entity, n_revs`.
- **Sidecar:** `tokei --output json` (size). Optional; degrades to revisions-as-size.
- **Command:** `uv run scripts/enclosure.py --weights revs.json --structure tokei.json -o hotspots.html`
- **Static:** `uv run scripts/treemap.py --weights revs.json --structure tokei.json -o hotspots.svg` (embeddable SVG/PNG; same flags and structure-first node set as `enclosure.py`).
- **Formats:** interactive HTML (iframe embed); static counterpart below.
- **Read:** the **offender profile** is big + hot, but colour (change) is the lead
  signal and size (LOC) the severity multiplier - a large pale circle is
  complex-but-stable. Scope out generated files first (false positives). Heuristics
  and full reading: [interpretation.md](interpretation.md). Contract:
  [enclosure.md](enclosure.md).

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
- **Read:** Conway litmus test - aggregate authors to teams first (`--team-map`).
  Mostly intra-team links = healthy; inter-team links are *potential* coordination
  bottlenecks (the usual fix is cohesion, not reorg). Reading:
  [interpretation.md](interpretation.md).

## Churn trend

- **Consumes:** `codelens absolute-churn` -> `date, added, deleted, commits`
  (also `author-churn`, `entity-churn`).
- **Command:** `uv run scripts/churn.py --churn churn.json -o churn.svg`
- **Formats:** SVG or PNG (the -o extension picks the format).
- **Read:** added vs deleted over time. Sustained one-sided growth without deletion
  is accumulation; spikes flag large reworks.

## Fractal figures

- **Consumes:** `codelens entity-effort` -> `entity, author, author_revs,
total_revs`; `fragmentation` for the scalar.
- **Command:** `uv run scripts/fractal.py --effort effort.json -o fractal.svg`
- **Formats:** SVG or PNG (the -o extension picks the format).
- **Read:** three ownership patterns: single developer, balanced (higher main-dev
  ownership predicts fewer defects), many minor contributors (defect risk - the
  *count* of minor contributors is the stronger predictor). Reading:
  [interpretation.md](interpretation.md).

## Commit word cloud

- **Consumes:** `codelens parse` -> the `message` column.
- **Command:** `codelens parse --log git.log --format json | uv run scripts/commit_cloud.py -o cloud.svg`
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
- **Read:** headline counts (commits, entities, authors) for a report header.
