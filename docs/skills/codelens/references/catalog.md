# Visualization catalog

One card per visualization: what it **consumes**, the **command** to build it, the
**formats** it emits, and how to **read** it. Loaded from [SKILL.md](../SKILL.md)
after routing. Column names are codelens JSON (snake_case), verified against build
eaece4f; re-check with `codelens schema --command <analysis>` after codelens changes.

---

## Hotspot enclosure map

- **Consumes:** `codelens revisions` -> `entity, n_revs`.
- **Sidecar:** `tokei --output json` (size). Optional; degrades to revisions-as-size.
- **Command:** `uv run scripts/enclosure.py --weights revs.json --structure tokei.json -o hotspots.html`
- **Formats:** interactive HTML (iframe embed; not exported to static).
- **Read:** big + hot (large, red) circles are the offender profile. Hotspots are
  1-5% of files yet 25-75% of defects. Contract: [enclosure.md](enclosure.md).

## Knowledge map

- **Consumes:** `codelens main-developer` -> `entity, main_dev, ownership`.
- **Sidecar:** `tokei --output json`.
- **Aliases:** author aliases split one person's ownership. The emitted log uses
  `--use-mailmap`, so a repo `.mailmap` collapses them for free; when there is no
  `.mailmap`, map aliases to a canonical name with `--team-map`.
- **Command:** `uv run scripts/enclosure.py --weights main-dev.json --weight-col main_dev --categorical --structure tokei.json -o knowledge.html`
- **Formats:** interactive HTML (iframe embed; not exported to static).
- **Read:** one color per developer, circles sized by tokei LOC. With `--structure`
  the node set is the tokei files (same as the hotspot and code-age maps, so all
  three are comparable); a file with no recorded author renders neutral grey
  (`(unowned)`). Single-color components are key-person dependencies; mixed
  components are shared effort. See [enclosure.md](enclosure.md).

## Code-age map

- **Consumes:** `codelens code-age` -> `entity, age_months`.
- **Sidecar:** `tokei --output json`.
- **Command:** `uv run scripts/enclosure.py --weights age.json --weight-col age_months --invert --structure tokei.json -o age.html`
- **Formats:** interactive HTML (iframe embed; not exported to static).
- **Read:** hot = recently changed (low age). Stable cores should be cool; churning
  old code that should be frozen is a smell.
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
- **Formats:** interactive HTML (iframe embed; not exported to static).
- **Read:** edges are files that change together. High-degree edges crossing
  architectural boundaries (group with `--group`) signal decay: copy-paste,
  unsupportive module boundaries, or producer-consumer.
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
- **Formats:** interactive HTML (iframe embed; not exported to static).
- **Read:** Conway litmus test. Dense intra-team links = healthy; many inter-team
  links = coordination bottleneck.

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
- **Read:** three ownership patterns: single developer, balanced (high main-dev
  ownership predicts fewer defects), many minor contributors (defect risk).

## Commit word cloud

- **Consumes:** `codelens parse` -> the `message` column.
- **Command:** `codelens parse --log git.log --format json | uv run scripts/commit_cloud.py -o cloud.svg`
- **Formats:** SVG or PNG (the -o extension picks the format).
- **Read:** dominant words show where time goes. Domain terms = good; "bug",
  "crash", "test", "bump" = drill deeper.

## Complexity trend

- **Consumes:** the **live repo** (not codelens): a repo path + a file path.
- **Command:** `uv run scripts/complexity_trend.py --repo . --file path/to/hotspot -o trend.svg`
- **Formats:** SVG or PNG (the -o extension picks the format).
- **Read:** indentation complexity over revisions. Shapes: deteriorating (act),
  refactored (dip = good), stable. Plot LOC alongside to see growth vs. thickening.

## Summary tiles

- **Consumes:** `codelens summary` -> `statistic, value`.
- **Command:** `uv run scripts/churn.py --summary summary.json -o summary.svg`
- **Formats:** SVG or PNG (the -o extension picks the format).
- **Read:** headline counts (commits, entities, authors) for a report header.
