---
id: cod-6kzk
status: open
deps: [cod-eex1]
links: []
created: 2026-07-28T16:43:35Z
type: feature
priority: 2
assignee: Andre Silva
tags: [codelens, spec-003, skill, viz, python]
---
# flint_spec.py: emit a Flint ChartAssemblyInput from a codelens envelope

Add `docs/skills/codelens/scripts/flint_spec.py`: reads a codelens result envelope on
stdin, writes a Flint `ChartAssemblyInput` on stdout.

Spec: docs/specs/003-flint-adapter/plan.md sections 3.1 to 3.6 and 6.1 (ticket 1 of four).
Read that plan; this ticket summarises it but the plan carries the measured evidence.

## Why this is a spec emitter and not a renderer

Emitting a `ChartAssemblyInput` is JSON construction and needs NO Flint dependency in
any language (decision F1). That is what keeps this a zero-dependency Python script
consistent with the other five scripts in that directory. Rendering is ticket 2.

Relevant facts, all VERIFIED by running `flint-chart@0.4.0` under Deno rather than by
reading its docs:

- Flint's ECharts `Network Graph` consumes a FLAT EDGE TABLE (`x` = source, `y` = target,
  `size` = edge weight), which is exactly what `coupling` and `communication` already
  emit. No new codelens output shape is needed anywhere in this work.
- Flint is NOT on PyPI, and `packages/flint-py` is a source-only preview implementing
  the Vega-Lite backend only. So Python cannot call Flint at all; hence the channel
  table in this ticket.
- THE ASSEMBLER SILENTLY DISCARDS an encoding channel a template does not declare. No
  exception, no warning, no `_warnings` entry. Two of six planned override rows were
  wrong for exactly this reason and compiled cleanly.

## Prerequisite context from spec 004

Depends on the aggregation-role vocabulary ticket so that the measure-precedence rule
in section 3.3 is written over roles (additive versus intensive) rather than a
hardcoded list of semantic names. Use `additive` / `intensive` as the concepts; reading
them from `--schema` at runtime is a later ticket in spec 004.

## Design

## CLI

```text
uv run scripts/flint_spec.py [--schema FILE] [--chart-type NAME]
                             [--backend vegalite|echarts]
                             [--width N] [--height N] [-o OUT]
```

Envelope on stdin. Follow the conventions of the existing scripts: PEP 723 inline
metadata header, module docstring carrying usage and an exit-code line, argparse,
`-o/--out`, `ruff.toml` governs lint. Exit codes `0` ok, `2` usage or rejected envelope,
`3` empty payload.

`--schema FILE` takes `codelens schema --command CMD` output and is OPTIONAL. Supplied,
it populates `field_display_names` from each column's `desc` so axes read "number of
distinct revisions" instead of `n_revs`. ABSENT, degrade to raw column names WITHOUT
error. Graceful degradation is the whole reason the flag is optional.

NOTE `--data-url` was considered and CUT. Flint's `parseJsonRows` accepts a bare array,
`{values: [...]}`, `{data: [...]}` or `{data: {values: [...]}}`, but NOT `{rows: [...]}`,
which is the codelens envelope. Data is always inlined into `data.values`.

## Accept and reject (3.1)

Accept `shape: "table"` with a non-empty payload. Reject with a stderr diagnostic:
`shape: "text"` (print-log-command), `ok: false`, `schema` output, and `parse` — all
exit `2`. Empty payload exits `3`.

`parse` is excluded deliberately: it is an event dump whose row count scales with the
log, and its `loc_added`/`loc_deleted` columns are absent when numstat is absent. The
accepted consequence is that `commit_id`, `text`, and `flag` are mapped but never
exercised by a chart.

## Semantic type map (3.2) — 12 semantics to 9 Flint types

| codelens          | Flint annotation                                            |
| ----------------- | ----------------------------------------------------------- |
| `filepath`        | `"Name"`                                                    |
| `person`          | `"Name"`                                                    |
| `text`            | `"Name"` (never bound to a channel)                         |
| `label`           | `"Category"`                                                |
| `flag`            | `"Boolean"`                                                 |
| `commit_id`       | `"ID"` (never bound to a channel)                           |
| `date`            | `"Date"`                                                    |
| `count`           | `"Count"`                                                   |
| `loc`             | `{"semanticType": "Quantity", "unit": "lines"}`              |
| `duration_months` | `{"semanticType": "Duration", "unit": "months"}`             |
| `percentage`      | `"Percentage"` — BARE, no annotation                         |
| `ratio`           | `{"semanticType": "Percentage", "intrinsicDomain": [0, 1]}`   |

### The asymmetry in the last two rows is load-bearing: do NOT "fix" it

`intrinsicDomain` is a GATE, not an override: Flint checks only its PRESENCE before
running `detectPercentageRepresentation` on the data, and ignores its VALUE for that
decision. Measured axis format:

```text
data                          bare Pct   id[0,100]   id[0,1]
percentage spread 0..100      ,.12~g     ,.12~g      ,.12~g
percentage all values <= 1    ,.12~g     .0~%        .0~%
ratio 0..1                    ,.12~g     .0~%        .0~%
```

`.0~%` multiplies by 100. So annotating a `percentage` column whose surviving values are
all 0 or 1 (a filtered ownership or coupling table) renders `1` (one percent) as
`"100%"`. Annotating it CAUSES the error; leaving it bare prevents it. The cost is a
missing `%` suffix on the axis, which is unobtainable safely; the axis title carries the
meaning.

`ratio` keeps the annotation because there the transform is exactly what is wanted:
`0.34` renders as `34%`.

Also note `ratio` maps to `Percentage` rather than the literal-looking `Number` because
`Number` is `additive` and summing ownership ratios is meaningless, whereas `Percentage`
is `intensive`. This contradicts Flint's own dropped-type table; the registry wins.

## Generic channel policy (3.3)

Over the columns surviving the `text`/`commit_id` exclusion, using `semantics` only:

1. Partition into DIMENSIONS (`filepath`, `person`, `label`, `date`, `flag`) and
   MEASURES (`count`, `loc`, `percentage`, `ratio`, `duration_months`).
2. Two dimensions sharing the same semantic means an EDGE TABLE: first to `x`, second to
   `y`, strongest measure to `size`. Default `Network Graph` on `echarts`.
3. Exactly one dimension: it takes the categorical axis, primary measure the value axis.
   Default `Bar Chart`.
4. Two dimensions of different semantics: first to the categorical axis, primary measure
   to the value axis, second dimension to `color`.
5. A `date` dimension always takes the temporal axis.

Measure precedence: intensive first (`percentage`, `ratio`), then `loc`, then
`duration_months`, then `count`. Express this over aggregation ROLES, not a hardcoded
semantic list — a count is almost always a supporting quantity here (`total_revs`,
`commits`, `average_revs`) while a proportion is the result. `loc` prefers `size` over
the value axis when the chosen template has a `size` channel.

## Per-analysis overrides (3.4) — ALL SIX COMPILE-VERIFIED

| Analysis         | chartType           | Backend  | Encodings                          |
| ---------------- | ------------------- | -------- | ---------------------------------- |
| `code-age`       | `Histogram`         | vegalite | `x: age_months`                    |
| `summary`        | `KPI Card`          | vegalite | `metric: statistic, value: value`  |
| `authors`        | `Scatter Plot`      | vegalite | `x: n_revs, y: n_authors`          |
| `coupling`       | `Network Graph`     | echarts  | `x: entity, y: coupled, size: degree`   |
| `communication`  | `Network Graph`     | echarts  | `x: author, y: peer, size: strength`    |
| `absolute-churn` | `Stacked Bar Chart` | vegalite | `x: date, y: [added, deleted]`     |

Two of these were WRONG in an earlier revision and compiled silently:

- `summary` used `x`/`y`. KPI Card's channels are `metric`, `value`, `goal`. With the
  right channels it emits a `vconcat` of one card per row, which is what a
  multi-statistic summary wants; with `x`/`y` it emitted a `layer` spec.
- `authors` used `detail: entity`. `detail` is NOT a Scatter Plot channel; it was
  dropped silently. `entity` is deliberately UNBOUND now: `options.addTooltips` does not
  restore it (verified, `encoding.tooltip` stays undefined), and `color`/`shape`/`size`
  would each ask for hundreds of values. The finding in that chart is the shape of the
  cloud, not which point is which file.

`y: [added, deleted]` is CORRECT and verified: Flint folds it to
`__flint_series_key`/`__flint_series_value`, auto-binds `color` to the series key, and
expands 2 rows to 4. This is the documented array reshape, the one exception to
`ChartEncoding.field` being a singular string.

Secondary charts offered via `--chart-type` only: `coupling`/`communication` as
`Heatmap`; `entity-ownership`/`entity-effort` as `Stacked Bar Chart` with
`stackMode: normalize`; `revisions`/`sum-of-coupling` as `Lollipop Chart`.

HAZARD: `main-developer`, `refactoring-main-developer`, `main-developer-by-revisions`,
and `entity-effort` each carry a measure AND its own total. Binding both to a stacked or
coloured channel trips Flint's documented total-mixed-with-components rule. Bind the
RATIO (`ownership`) or the component alone, never component plus total.

## Channel validation (F12, layer 1)

Because Flint silently discards undeclared channels, carry a table keyed backend then
`chartType`. Every key in `encodings` must be a member; an unknown channel is a HARD
ERROR, exit `2`. Seed from these verified 0.4.0 values:

```text
vegalite  Bar Chart          x, y, color, column, row
vegalite  Stacked Bar Chart  x, y, color, column, row
vegalite  Histogram          x, color, column, row
vegalite  Scatter Plot       x, y, color, size, shape, opacity, column, row
vegalite  KPI Card           metric, value, goal
echarts   Network Graph      x, y, size
```

Add entries for any secondary chart type offered. The tests can only assert
SELF-CONSISTENCY (every chartType named in the override table has an entry; every
override's encodings are a subset of it). Parity with Flint is deliberately NOT asserted
here — that is ticket 2 at render time, and the upgrade checklist.

## Backend default

`vegalite`, with `echarts` selected only by templates that require it (Network Graph).
Vega-Lite is the only backend with any Python implementation, so a future in-process
Python compile stays possible on the default path.

## Acceptance Criteria

- Emits a valid `ChartAssemblyInput` for all 17 table-shaped analyses via the generic
  policy plus the 6 overrides.
- Rejects `shape: "text"`, `ok: false`, `schema` output, and `parse` with exit `2`;
  exits `3` on an empty payload; `0` otherwise.
- A COMPILED FIXTURE PER OVERRIDE ROW: each of the 6 has a recorded expected spec
  checked against `flint-chart@0.4.0`. This is the criterion that would have caught both
  broken rows.
- A regression test asserting `percentage` is annotated with a BARE string and `ratio`
  with `intrinsicDomain: [0, 1]`.
- A channel table plus tests asserting every override's encodings are a subset of its
  template's declared channels, and that an unknown channel exits `2`.
- `--schema` absent degrades to raw column names without error; present, populates
  `field_display_names` from column `desc`.
- No Flint dependency: the script runs under `uv run` with no npm or Deno involvement.
- `flint_spec_test.py` beside it, matching the other five scripts. `ruff` clean.
- `make build` green.

