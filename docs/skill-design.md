# codelens visualization skill: design

Design and rationale for the `codelens` agent skill, which turns `codelens` CLI
output into the visualizations from Adam Tornhill's _Your Code as a Crime Scene_
(2nd ed., 2024). This document captures the decisions and their reasons. The
skill itself lives in [`skills/codelens/`](skills/codelens/); operational detail
(the pipeline, per-visualization cards, the enclosure contract) is there, not
duplicated here.

Status: design finalized against the first codelens implementation (build
eaece4f). Every script's input contract has been verified against the live
`codelens schema` and real analysis runs. The remaining work is implementing the
scaffolded scripts, not design.

## Goal

`codelens` (see [specs/001-initial-implementation/requirements.md](specs/001-initial-implementation/requirements.md))
mines a git log and emits structured JSON for 20 evolutionary analyses. This
skill consumes that JSON and renders embeddable artifacts: static SVG/PNG for
reports and slides, and interactive HTML for exploration. The recurring subject
is the hotspot: complex code that changes often.

## Data model: two halves

Tornhill's visualizations draw on two data sources, and the split drives the
whole design:

1. Evolution and social data, mined from the git log. This is exactly what
   codelens produces.
2. Size and complexity data (lines of code, indentation complexity). codelens
   does not produce this; it is a log miner, not a source parser.

Each visualization therefore falls into one of three buckets:

- Pure codelens: coupling, churn, fractal figures, ownership/knowledge map,
  communication network, code age, word cloud.
- codelens plus a size sidecar: the flagship hotspot enclosure map.
- Out of scope for now: anything needing per-revision source analysis
  (method-level X-Ray, Code City). The complexity trend is the one exception we
  keep, via a self-contained git-history driver.

## Visualization catalog

Each visualization maps to a codelens analysis, a chart type, and a script. The
authoritative per-visualization contract (columns, sidecars, formats, how to
read the result) is in [skills/codelens/references/catalog.md](skills/codelens/references/catalog.md).

| Visualization                    | codelens analysis                | Chart                       | Interactive |
| -------------------------------- | -------------------------------- | --------------------------- | ----------- |
| Hotspot enclosure map (flagship) | `revisions` (+ tokei)            | circle packing              | yes         |
| Knowledge map                    | `main-developer` (+ tokei)       | circle packing, color = dev | yes         |
| Code-age map                     | `code-age` (+ tokei)             | circle packing, color = age | yes         |
| Change-coupling graph            | `coupling`, `sum-of-coupling`    | chord / force graph         | yes         |
| Communication network            | `communication`                  | force graph                 | yes         |
| Churn trend                      | `absolute-churn` (and variants)  | time series                 | no          |
| Fractal figures                  | `entity-effort`, `fragmentation` | nested rectangles           | no          |
| Commit word cloud                | `parse` (message column)         | word cloud                  | no          |
| Complexity trend                 | none (live repo)                 | line chart                  | no          |
| Summary tiles                    | `summary`                        | KPI tiles                   | no          |

Two notes: the enclosure family (hotspot, knowledge, age) is one generalized
script, differing only by the weight column. The word cloud is fed by
`codelens parse` (commit subjects), not the `messages` regex analysis.

## Toolchain: two lanes

- Python static lane: standalone `uv run` scripts with PEP 723 inline
  dependencies, emitting SVG/PNG/PDF. Used for churn, fractal, word cloud,
  complexity trend, summary. Fits the repo's Python conventions.
- Interactive HTML lane: one HTML file per interactive visualization, D3 loaded
  from a CDN and rendering from an inlined JSON data blob. Used for the enclosure
  family, coupling, and communication network. These are for live viewing and
  iframe embedding; they are not exported to static images.

Vega-Lite was considered and rejected: it is a third toolchain and does not earn
its keep beside the two above (and it is a poor fit for circle packing).

## Sidecars

- Size sidecar: `tokei --output json`, per-file `code` (lines of code, excluding
  comments and blanks). Snapshot of the working tree, run from the log's root.
  Optional; the enclosure degrades to revisions-as-size when it is absent.
- Complexity metric: zero-dependency indentation ("negative space") analysis,
  the book's approach. Language-agnostic, stdin-friendly, no binary dependency.
  `lizard` is deferred: it is added only when method-level X-Ray (real
  per-function cyclomatic complexity) is built, not for the trend.

The complexity trend is not a codelens consumer. It is a git-history driver: the
plain `git` CLI plus a stdlib script (`git log --follow` to enumerate a file's
revisions, `git show <rev>:<file>` piped into the indentation metric). It reads
the live repo, beside codelens, rather than downstream of it. No git library is
needed.

## Flagship enclosure contract

Full contract in [skills/codelens/references/enclosure.md](skills/codelens/references/enclosure.md).
The essential design points:

- Role asymmetry: tokei defines the node set and circle radius (the whole current
  tree, sized by LOC); codelens `revisions` is a color overlay joined on path,
  defaulting to 0 for files with no recorded change. This mirrors Tornhill's
  `csv_as_enclosure_json.py` (`--structure` vs `--weights`) and is why stable and
  third-party files still appear, giving the whole-codebase view the book
  describes.
- Two modes: full (tokei present) and degraded (no tokei, node set and size come
  from the weight source). Same tree-builder and template downstream.
- Join on the raw path string after stripping a leading `./`; run tokei at the
  log's root so paths align. Normalize the numeric weight to 0.0 to 1.0.
- Tree built by splitting paths on `/`; leaf keys `size`/`weight` kept identical
  to Tornhill's script so his template is drop-in compatible.
- Improvement over the book: the output HTML inlines the data as a script blob
  (D3 loads from a CDN), so it opens directly with no `python -m http.server`
  CORS step.

## Formats and embedding

The full format-to-target matrix is in
[skills/codelens/references/embedding.md](skills/codelens/references/embedding.md).
Summary:

- SVG is the canonical static artifact: it embeds inline in HTML and Markdown,
  imports crisply into slides, and stays sharp in PDF.
- PNG is the raster fallback for the static charts (churn, fractal, word cloud,
  complexity trend, summary) where SVG cannot go.
- Interactive views (enclosure, coupling, network) are delivered as HTML for live
  viewing and iframe embedding only; they are not exported to static images. When
  a slide or PDF needs a picture, use the static visualizations.

## Skill structure

Standard skill layout under [skills/codelens/](skills/codelens/): `SKILL.md` at
the root, `references/`, `scripts/`, `assets/`. Skill name is `codelens`.

The skill covers two branches: operating the codelens CLI (running analyses) and
visualizing the output. Pure operation is a valid endpoint, not only a
visualization prerequisite.

Information hierarchy (per the skill-builder method):

- Inline in `SKILL.md`: the five-step visualization pipeline (frame, collect,
  build, render, read), each with a checkable completion criterion, plus a compact
  routing table and a pointer to the operating reference. Leading words: hotspot,
  crime scene.
- Disclosed to `references/`: the CLI operating guide (`operating.md`: canonical
  workflow, schema discovery, analyses catalog, output shaping, transforms, exit
  codes; self-contained and the canonical operating reference, which the repo
  `AGENTS.md` points to), the full
  per-visualization cards (`catalog.md`), the flagship contract (`enclosure.md`),
  and embedding mechanics (`embedding.md`). Each is reached by a pointer only when
  a branch needs it.

## Determinism boundary

The agent runs codelens and picks the visualization; the scripts pin the
transform and render so they never vary. Every script is self-contained: one
file, inline dependencies, one command, meaningful exit code, stable output.
Small shared helpers (tree-builder, path-join, normalization) are copied into
each script rather than imported, trading DRY for the portability and legibility
the self-contained rule buys.

## Codelens surface (verified)

Each script's input contract is pinned to codelens JSON column names and the
success envelope (`schema_version, ok, analysis, row_count, rows`). All 20
analyses are present, and the columns the scripts consume were confirmed against
`codelens schema --command <analysis>` on build eaece4f:

| Analysis          | Columns (snake_case)                                                 |
| ----------------- | -------------------------------------------------------------------- |
| `revisions`       | `entity, n_revs`                                                     |
| `main-developer`  | `entity, main_dev, added, total_added, ownership`                    |
| `code-age`        | `entity, age_months`                                                 |
| `coupling`        | `entity, coupled, degree, average_revs` (+ verbose `*_revisions`)    |
| `sum-of-coupling` | `entity, soc`                                                        |
| `communication`   | `author, peer, shared, average, strength`                            |
| `absolute-churn`  | `date, added, deleted, commits`                                      |
| `entity-effort`   | `entity, author, author_revs, total_revs`                            |
| `fragmentation`   | `entity, fractal_value, total_revs`                                  |
| `summary`         | `statistic, value`                                                   |
| `parse`           | `entity, rev, date, author, message, loc_added, loc_deleted, binary` |

`print-log-command` emits `git log --all --numstat --date=short
--pretty=format:'--%h--%ad--%aN--%s' --no-renames`. Two consequences: `--no-renames`
confirms codelens does not track renames (the enclosure rename edge case holds),
and a `--after <date>` argument is forwarded to git, so date scoping works through
the helper. Analysis-specific flags live on their commands (coupling thresholds,
`code-age --time-now`, `messages --expression`); the pipeline flags (`--log`,
`--format`, `--group`, `--team-map`, `--temporal-period`, `--rows`, `--fields`)
are global.

Re-run `codelens schema --command <analysis>` after any codelens change to catch
column drift before it reaches a script.

## Decisions

- D3 is loaded from a CDN (not vendored); output HTML needs network to render.
- Interactive views are not exported to static images; the static charts cover
  slides and PDF.
- Enclosure circle radius uses tokei `code` only (excludes comments and blanks).
- Enclosure keeps all languages by default, with a tokei `--exclude` opt-out.
- First cut ships all ten visualizations (no scaffolds deferred).

## Build state

All ten visualizations are implemented; `ruff` clean; `skill_ref.py validate`
passes.

| Piece                                        | State                                                                           |
| -------------------------------------------- | ------------------------------------------------------------------------------- |
| `SKILL.md`, `references/*.md`                | complete                                                                        |
| `scripts/enclosure.py`                       | verified against real `codelens revisions` + tokei (both modes, join confirmed) |
| `scripts/complexity_trend.py`                | verified (git driver + indentation, valid SVG)                                  |
| `scripts/churn.py`                           | verified (valid SVG)                                                            |
| `scripts/fractal.py`                         | verified against real `codelens entity-effort` (valid SVG)                      |
| `scripts/commit_cloud.py`                    | verified against real `codelens parse` (valid SVG)                              |
| `scripts/coupling_graph.py`                  | verified on synthetic coupling data (HTML)                                      |
| `scripts/dev_network.py`                     | verified on synthetic communication data (HTML)                                 |
| `assets/templates/circle-packing.html.jinja` | D3 v7 zoomable                                                                  |
| `assets/templates/force-network.html.jinja`  | D3 v7 force-directed                                                            |

Note: `coupling_graph.py` and `dev_network.py` were exercised on synthetic
fixtures because this repo's history is too shallow to clear the coupling and
communication thresholds; re-verify on a mature repo. The word-cloud script is
`commit_cloud.py`, not `wordcloud.py`, so it does not shadow the `wordcloud`
package it imports.

## References

- Book: `.local/refs/tornhill.2024.code-crime-scene.txt`.
- codelens tool: [specs/001-initial-implementation/](specs/001-initial-implementation/),
  [research/code-maat.md](research/code-maat.md), [cli-design.md](cli-design.md).
- Skill: [skills/codelens/](skills/codelens/).
