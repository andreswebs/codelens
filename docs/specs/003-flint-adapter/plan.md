# Implementation plan: Flint chart-spec adapter

Status: FINAL (decisions resolved 2026-07-28). Ready to ticket.

All eleven questions in section 8 are resolved: five by executing Flint under Deno
(Q1, Q2, Q3, Q6, Q9) and six by decision (Q4, Q5, Q7, Q8, Q10, Q11). Q11 belongs in
the second group but was DISCOVERED by the first, which is the pattern worth carrying
forward.

Revision 3 corrected the per-analysis override table, two of whose six rows named
channels that do not exist (3.4.1), cut `--data-url` (3.5), and resolved Q11 by
validating encodings in two layers (F12). Revision 2 corrected the `percentage` and
`ratio` mappings, which were backwards (3.2.1). Both corrections were found by
executing Flint rather than reading it, which is why 3.4.1 ends with a rule about
compiling fixtures.

Scope: one new skill script, `flint_spec.py`, that reads a codelens result envelope
on stdin and writes a Flint `ChartAssemblyInput` on stdout, plus the rendering path
that turns that spec into an artifact. This is the "adapter" half of the parked epic
cod-304f (its decisions A1 through A4), and it is deliberately independent of the
"non-table shapes" half (E1 through E9).

Inputs read: Flint's published sources at `microsoft/flint-chart` 0.4.0
(`packages/flint-js/src/core/type-registry.ts`, `field-semantics.ts`,
`echarts/templates/graph.ts`, `packages/flint-mcp/src/render/data-source.ts`, the
per-backend chart references, and `agent-skills/flint-chart-author/SKILL.md`); the
live behaviour of `flint-chart@0.4.0` under Deno; `codelens schema --command` for all
17 table-shaped analyses; the skill under `docs/skills/codelens/`;
`docs/specs/002-data-output/plan.md` decisions D3 and D13; and epic cod-304f
decisions A1 through A4.

## 1. Verified current state (2026-07-27)

Facts established by reading Flint's shipped sources and registries rather than its
prose. Provenance is given because two of these contradict Flint's own
documentation.

### 1.1 Flint

| Fact                                                                                                                       | Source                                        |
| -------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| npm `flint-chart` is at 0.4.1; MIT; Microsoft                                                                              | npm registry                                  |
| `flint-chart` is NOT on PyPI (404)                                                                                         | PyPI                                          |
| `packages/flint-py` is a source-only preview, "PyPI publishing planned for a later release"                                | its README                                    |
| `flint-py` implements the VEGA-LITE BACKEND ONLY                                                                           | its README, module tree                       |
| Vega-Lite backend: 36 chart templates, 6 categories                                                                        | `docs/reference-vegalite.md`                  |
| ECharts backend: 37 templates, 10 categories, including Treemap, Sunburst, Tree, Sankey, Network Graph, Calendar Heatmap   | `docs/reference-echarts.md`                   |
| The semantic type registry has 44 entries                                                                                  | `packages/flint-js/src/core/type-registry.ts` |
| `ChartEncoding.field` is a singular `string`, EXCEPT that `x` and `y` also accept an array of field names as a documented reshape | `docs/api-reference.md`, and the authoring skill's "the only built-in reshape is the array form on `x`/`y`" |
| MCP server exposes 5 tools: `render_chart`, `compile_chart`, `validate_chart`, `list_chart_types`, `create_chart_view`     | `packages/flint-mcp/README.md`                |
| MCP renders in-process, no browser: Vega-Lite via headless `vega.View`, ECharts server-side SVG, PNG via `@resvg/resvg-js` | same                                          |
| `flint-chart-mcp/render` is an importable entry point: `renderChart(input, backend, {format, scale})`                      | same                                          |
| `data.url` accepts local `.json`, `.csv`, `.tsv`; remote URLs never fetched                                                | same                                          |
| An official authoring skill ships at `agent-skills/flint-chart-author/SKILL.md`                                            | that file                                     |

Two corrections to Flint's prose, both of which changed a decision here:

1. `docs/design-semantics.md` describes the `Physical` category as taking average
   aggregation, which made `Quantity` look unusable for line counts. The registry
   says `Quantity` is `aggRole: 'additive'` with `zeroBaseline: 'meaningful'`. The
   prose describes `Temperature`, the category's other member, which is the
   `intensive` one. `Quantity` is safe.
2. `docs/design-semantics.md` lists T2 types (`PersonName`, `String`, `Product`,
   `Company`) that are NOT in the registry. The agent skill's shorter list is the
   accurate one. There is no person-specific type; `person` maps to `Name`.

Treat `packages/flint-js/src/core/type-registry.ts` as the contract. Where prose and
registry disagree, the registry wins, and this plan follows it.

### 1.1.1 Facts established by EXECUTING Flint (2026-07-27)

Everything above was read. Everything below was run, under
`deno run npm:flint-chart@0.4.0`. This distinction earned its keep: reading the
registry corrected the prose, and running the compiler then corrected the registry
reading (3.2.1).

| Fact                                                                                                                       | How                                    |
| -------------------------------------------------------------------------------------------------------------------------- | -------------------------------------- |
| `Network Graph` consumes an EDGE TABLE: `x` = source, `y` = target, `size` = edge weight                                    | compiled a `coupling` row set          |
| Its output is `series.type: 'graph'`, `layout: 'circular'`, with derived `nodes` and `links`                                | same                                   |
| `layout` defaults to `circular` explicitly so rendering is DETERMINISTIC (no force simulation), which makes golden tests viable | `echarts/templates/graph.ts` doc comment |
| Node size is weighted degree: it SUMS the edge weight per node, including when that weight is a `Percentage`                | `graph.ts`, and observed in output     |
| Self-edges (`src === tgt`) are silently dropped                                                                            | `graph.ts`                             |
| `Category`, `Name`, and `Unknown` have byte-identical registry entries AND compile to identical Vega-Lite                   | compiled all four; diffed              |
| Overflow truncation is real and lands on a PRIVATE `_warnings` field, not `warnings`                                        | 300-row bar chart: 91 kept, 209 omitted |
| The truncated chart self-labels the omission in its axis domain (`...209 items omitted`)                                    | same                                   |
| `vlRecommendCharts(values, semantic_types)` exists and returns `[{chartType, encodings}]`                                   | called it                              |
| Deno's default supply-chain guard REFUSES `flint-chart@0.4.1` as published under 24h ago                                    | `deno run` error                        |

The last row is the reason this plan pins **0.4.0, not 0.4.1**: a version younger
than Deno's `minimumDependencyAge` cannot be installed without weakening a
supply-chain default, which is a bad trade for a patch release.

The node-sizing row is worth flagging beyond this plan: Flint sums an intensive
measure to size a node, exactly as `dev_network.py` does by hand. That is
independent corroboration of the ordinal-versus-value distinction in the
aggregation-role plan, and it means summing a `percentage` for a radius is
idiomatic rather than sloppy.

### 1.2 codelens

The exact column inventory the adapter must handle, from
`codelens schema --command CMD` at 2026-07-27. 17 table-shaped analyses; `parse`,
`schema`, and `print-log-command` are excluded (see 3.1).

```text
revisions                      entity[filepath] n_revs[count]
code-age                       entity[filepath] age_months[duration_months]
sum-of-coupling                entity[filepath] soc[count]
coupling                       entity[filepath] coupled[filepath] degree[percentage]
                               average_revs[count] first_entity_revisions[count]
                               second_entity_revisions[count] shared_revisions[count]
communication                  author[person] peer[person] shared[count] average[count]
                               strength[percentage]
absolute-churn                 date[date] added[loc] deleted[loc] commits[count]
author-churn                   author[person] added[loc] deleted[loc] commits[count]
entity-churn                   entity[filepath] added[loc] deleted[loc] commits[count]
entity-ownership               entity[filepath] author[person] added[loc] deleted[loc]
main-developer                 entity[filepath] main_dev[person] added[loc]
                               total_added[loc] ownership[ratio]
entity-effort                  entity[filepath] author[person] author_revs[count]
                               total_revs[count]
fragmentation                  entity[filepath] fractal_value[ratio] total_revs[count]
summary                        statistic[label] value[count]
messages                       entity[filepath] matches[count]
authors                        entity[filepath] n_authors[count] n_revs[count]
refactoring-main-developer     entity[filepath] main_dev[person] removed[loc]
                               total_removed[loc] ownership[ratio]
main-developer-by-revisions    entity[filepath] main_dev[person] added[count]
                               total_added[count] ownership[ratio]
```

Observations that drive the design:

- Only 4 of the 12 declared semantics never appear in a table-shaped analysis's
  schema: `commit_id`, `text`, and `flag` appear nowhere in the 17 above, and are
  reachable only through `parse`. The adapter still needs a mapping for them (the
  vocabulary is a closed contract) but no chart binds them.
- Two analyses are EDGE TABLES: `coupling` (two `filepath` columns) and
  `communication` (two `person` columns). They are the Network Graph and Heatmap
  candidates, and they are structurally detectable: two columns sharing one
  semantic.
- Three analyses are PART-OF-WHOLE pairs where a measure and its total sit side by
  side (`added`/`total_added`, `removed`/`total_removed`, `author_revs`/`total_revs`).
  Flint's authoring checklist explicitly warns against mixing a total level with its
  components on a stacked or colored channel, so these need care.
- `summary` is the only analysis whose category column is `label` rather than a
  path or a person, and the only KPI candidate.

### 1.3 The existing skill

Five renderer scripts totalling 1,231 lines, each with a `*_test.py` beside it, all
Python with PEP 723 inline metadata run via `uv run`. Conventions to match:
argparse, `-o/--out`, a module docstring carrying usage and an exit-code line, and
the exit taxonomy `0` ok, `2` usage, `3` empty. `ruff.toml` governs lint.

The skill-builder convention for a self-contained TypeScript script is Deno with
inline `npm:` specifiers, chosen over Node precisely because Node needs a
`package.json` plus `npm install` and thereby breaks self-containment.

## 2. Decisions register

Provisional. F-numbers are stable so they can be referenced without renumbering.

| ID  | Decision                                                                                                                                                                                                                                                     |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| F1  | The adapter EMITS A SPEC ONLY. Emitting a `ChartAssemblyInput` is JSON construction and needs no Flint dependency in any language, which is what keeps `flint_spec.py` a zero-dependency Python script consistent with the rest of the skill.                |
| F2  | Rendering is a SEPARATE, LATER STEP with two paths (section 5). The spec is the artifact `flint_spec.py` is responsible for, and it is useful on its own.                                                                                                    |
| F3  | codelens the binary is UNCHANGED. No `export` verb, no chart vocabulary in Go. This restates D13 and A2's constraint and is non-negotiable in this plan.                                                                                                     |
| F4  | Target the semantic type registry, not the template list. The registry is the stable surface; the template list grew by 56 entries in one minor release.                                                                                                     |
| F5  | The semantic map is the table in section 3.2: NINE semantics take a bare string and THREE take the object form (`loc`, `duration_months`, `ratio`). Revision 2 cut this from four object forms: `intrinsicDomain` turned out to be a gate rather than an override, so annotating `percentage` is actively harmful (3.2.1).                |
| F6  | Channel binding is a two-layer decision: a GENERIC POLICY from semantics (3.3) plus a small PER-ANALYSIS OVERRIDE table (3.4). Neither alone is sufficient; see F6 note below.                                                                               |
| F7  | Default backend is `vegalite`; `echarts` is selected only by the analyses that need an ECharts-only template. Reason: Vega-Lite is the only backend with a Python implementation, so a future in-process Python compile stays possible for the default path. |
| F8  | Data is ALWAYS inlined into `data.values`. codelens `--rows` already exists to bound a result, and a self-contained single-file spec is easier to test, cache, and inline in a report. Revision 2 CUT the `--data-url` escape hatch as underspecified surface (3.5).                      |
| F9  | The adapter NEVER binds `text` or `commit_id` to a channel. `text` is declared "never a plottable category" and `commit_id` maps to `ID`, whose registry `aggRole` is `identifier`.                                                                          |
| F10 | One invocation emits ONE spec. Multiple useful charts per analysis are obtained by multiple invocations with `--chart-type`, not by a spec array. Keeps the stdout contract a single JSON object, matching every other script.                               |
| F11 | The adapter is GENERIC over analyses, driven by `analysis` and `semantics` from the envelope. It does not hardcode column names. This is what makes it survive a new analysis being added.                                                                   |
| F12 | ENCODINGS ARE VALIDATED TWICE (Q11): `flint_spec.py` checks every channel against its own per-template table and exits `2` on an unknown one; `flint_render.ts` re-checks against Flint's own `vlGetTemplateChannels`/`ecGetTemplateChannels`. Needed because Flint silently discards an undeclared channel (3.4.1). F1 survives: the adapter still has no Flint dependency, it has a table.                                                                   |

F6 note, since it is the design's main judgment call. A purely generic policy
cannot know that `code-age` wants a Histogram (a distribution over one measure)
while `revisions` wants a ranked bar chart, because both are
`filepath` + one measure. A purely per-analysis table would violate F11 and break
on any new analysis. So: the generic policy produces a defensible default for any
envelope it has never seen, and the override table upgrades the 6 analyses whose
best chart is not the default.

F6 was CHALLENGED and CONFIRMED (Q3). Flint exports `vlRecommendCharts(values,
semantic_types)`, which would have replaced the generic layer with an upstream
recommender. Tested against seven real codelens tables, it is wrong or useless on
five:

| Analysis      | Flint recommends                       | Verdict                                                    |
| ------------- | -------------------------------------- | ---------------------------------------------------------- |
| `revisions`   | Bar Chart, x=entity, y=n_revs          | correct, and identical to the generic policy                |
| `authors`     | Scatter Plot, x=n_revs, y=n_authors    | correct, matching 3.4's override, but adds `color: entity`  |
| `code-age`    | Bar Chart                              | wrong: misses the distribution, which is the whole point    |
| `absolute-churn` | Line Chart on `added` only           | wrong: drops `deleted`, losing the refactor-vs-build signal |
| `summary`     | Bar Chart                              | wrong: unrelated statistics on one scale                    |
| `entity-ownership` | Scatter Plot, deleted vs added     | wrong: not an ownership view                                |
| `coupling`    | **Bar Chart**                          | wrong: never suggests Network Graph, even on ECharts        |

It also proposes `color: entity` on tables with hundreds of entities, which would
request hundreds of colours. It is tuned for small business data. The recommender is
NOT adopted; it agrees only on the case the generic policy already handles, and
adopting it would put a JavaScript dependency inside a Python script to replace
roughly twenty lines of rules.

## 3. Target design

### 3.1 What the adapter accepts and rejects

Accepts: any envelope with `shape: "table"` and a non-empty payload.

Rejects, with a diagnostic on stderr and a non-zero exit:

- `shape: "text"` (`print-log-command`). Not data.
- `ok: false`. An error envelope has no payload.
- `schema` output. It is a descriptor, not a result, and has no `semantics` map.
- `parse` (RESOLVED, Q4). It is a raw event dump whose row count is the log's size,
  and its `loc_added`/`loc_deleted` columns are absent when numstat is absent, so its
  schema is the one case where a declared column may not appear in a row. The
  adapter charts analyses, not event dumps. Note the deliberate consequence: `parse`
  is the only command carrying `commit_id`, `text`, and `flag`, so those three
  semantics are MAPPED (the vocabulary is a closed contract) but never exercised by
  any chart. That is accepted, not overlooked.
- An empty payload, which exits `3` to match the other scripts.

### 3.2 Semantic type map (F5)

Twelve codelens semantics to NINE distinct Flint types: `Name`, `Category`,
`Boolean`, `ID`, `Date`, `Count`, `Quantity`, `Duration`, `Percentage`. Three
semantics share `Name` and two share `Percentage`, which is why the map is
many-to-one and why the channel policy in 3.3, not the type map, carries the
distinctions the map drops.

| codelens          | Flint annotation                                              | Registry justification                                                                                          |
| ----------------- | ------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `filepath`        | `"Name"`                                                      | Categorical/Entity, nominal, dimension                                                                          |
| `person`          | `"Name"`                                                      | same; there is no `PersonName` in the registry                                                                  |
| `text`            | `"Name"`                                                      | never bound to a channel (F9)                                                                                   |
| `label`           | `"Category"`                                                  | entry is identical to `Name`'s; the choice is documentary                                                       |
| `flag`            | `"Boolean"`                                                   | Categorical/Coded, `domainShape: 'fixed'`                                                                       |
| `commit_id`       | `"ID"`                                                        | Identifier, `aggRole: 'identifier'`; never bound (F9)                                                           |
| `date`            | `"Date"`                                                      | Temporal/DateTime, `visEncodings: ['temporal']`                                                                 |
| `count`           | `"Count"`                                                     | Measure/GenericMeasure, additive, `formatClass: 'integer'`                                                      |
| `loc`             | `{"semanticType": "Quantity", "unit": "lines"}`               | additive, meaningful zero, `formatClass: 'unit-suffix'` so the unit is needed to render a sane label            |
| `duration_months` | `{"semanticType": "Duration", "unit": "months"}`              | additive, `unit-suffix`                                                                                         |
| `percentage`      | `"Percentage"` (BARE, no annotation)                          | `intrinsicDomain` must be OMITTED here; supplying it causes a 100x rendering error (3.2.1)                       |
| `ratio`           | `{"semanticType": "Percentage", "intrinsicDomain": [0, 1]}`   | `Percentage` is `intensive`, correct for a proportion; `Number` is `additive` and summing ratios is meaningless. The annotation is REQUIRED to get `0.34` rendered as `34%` |

Entries that deserve a comment in the code because they look wrong:

- `ratio` becoming `Percentage` contradicts Flint's own dropped-type table, which
  recommends `Number` for a ratio. The registry's aggregation roles are the reason
  to override that advice: `Number` is `additive`, and summing ownership ratios is
  meaningless, whereas `Percentage` is `intensive`.
- `percentage` and `ratio` land on the SAME Flint type but are annotated
  ASYMMETRICALLY, which looks like an oversight and is not. See below.

#### 3.2.1 Why `percentage` is bare: a measured correction

Revision 1 asserted that `intrinsicDomain: [0, 100]` would tell Flint that a
`percentage` column is whole-number and thereby defeat its data-dependent
fractional detection. **That is false.** `intrinsicDomain` is a GATE, not an
override: `field-semantics.ts` reads only its PRESENCE before running
`detectPercentageRepresentation` on the data, and the annotation's VALUE is not
consulted for the representation decision at all.

Measured axis format, from compiling three data shapes against three annotations:

| data                        | bare `Percentage` | `intrinsicDomain [0,100]` | `intrinsicDomain [0,1]` |
| --------------------------- | ----------------- | ------------------------- | ----------------------- |
| `percentage` spread 0 to 100 | `,.12~g`          | `,.12~g`                  | `,.12~g`                |
| `percentage` all values <= 1 | `,.12~g`          | **`.0~%`**                | **`.0~%`**              |
| `ratio` 0 to 1               | `,.12~g`          | `.0~%`                    | `.0~%`                  |

`.0~%` multiplies by 100. So on a filtered ownership or coupling table where the
surviving values happen to be 0 or 1, annotating a `percentage` column makes Flint
render `1` (meaning one percent) as `"100%"`. The annotation does not prevent the
error revision 1 was worried about; it is the sole cause of it.

Consequences, and they are the operative rules for the implementation:

- **`percentage` gets a bare string.** The cost is real but small: no `%` suffix on
  the axis, because whole-number percent formatting is unobtainable without
  triggering the transform. The axis title carries the meaning instead.
- **`ratio` keeps its annotation.** Without it the axis shows `0.34`; with it,
  `34%`. Here the transform is exactly what is wanted, and the `all values <= 1`
  row of the table is the normal case rather than a hazard.
- The asymmetry is therefore load-bearing and must be commented, or a future reader
  will "fix" it into symmetry and reintroduce the 100x error.

This is the second time in this plan that Flint's prose disagreed with Flint's
behaviour, and the first time the registry ALSO disagreed with it. The operating
rule going forward: for anything that affects a rendered number, compile a spec and
read the output.

### 3.3 Generic channel policy (F6, F11)

Applied to the columns that survive F9, using only `semantics`:

1. Partition columns into DIMENSIONS (`filepath`, `person`, `label`, `date`,
   `flag`) and MEASURES (`count`, `loc`, `percentage`, `ratio`,
   `duration_months`).
2. If two dimensions share the same semantic, the table is an EDGE TABLE. Bind the
   first to `x`, the second to `y`, and the strongest measure to `size`. Default
   chart: `Network Graph` on `echarts`.
3. Otherwise, if there is exactly one dimension, bind it to the categorical axis and
   the primary measure to the value axis. Default chart: `Bar Chart`.
4. If there are two dimensions of different semantics (`entity` + `author`), bind
   the first to the categorical axis, the primary measure to the value axis, and the
   second dimension to `color`.
5. A `date` dimension always takes the temporal axis and upgrades the default chart
   to `Bar Chart` with `xAxisType: temporal`, or `Line Chart` when the caller asks.

Measure precedence, when several measures are present and one must be chosen:
`percentage` and `ratio` first (they are the analysis's headline proportion), then
`loc`, then `duration_months`, then `count`. Rationale: a count is almost always a
supporting quantity in these analyses (`total_revs`, `commits`, `average_revs`),
whereas a proportion is the result.

`loc` prefers `size` over the value axis when a `size` channel exists in the chosen
template. This is where the `loc`/`count` distinction is actually consumed; see the
explainer.

### 3.4 Per-analysis overrides

The 6 analyses whose best chart is not what 3.3 produces. Everything else takes the
generic default.

| Analysis         | chartType           | Backend  | Encodings                                 | Why the default is wrong                                                                                                                   |
| ---------------- | ------------------- | -------- | ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `code-age`       | `Histogram`         | vegalite | `x: age_months`                           | The interesting object is the DISTRIBUTION of age, not a per-file ranking. This is the chart that answers the stabilisation question.      |
| `summary`        | `KPI Card`          | vegalite | `metric: statistic, value: value`         | One row per metric; a bar chart of unrelated statistics on one scale is meaningless. KPI Card emits one card per row via `vconcat`.        |
| `authors`        | `Scatter Plot`      | vegalite | `x: n_revs, y: n_authors`                 | Two measures and one identifying dimension. The generic rule would drop one measure. `entity` is deliberately UNBOUND; see 3.4.1.          |
| `coupling`       | `Network Graph`     | echarts  | `x: entity, y: coupled, size: degree`     | Reached by rule 2, but pinned here so the `degree` choice over `average_revs` is explicit.                                                 |
| `communication`  | `Network Graph`     | echarts  | `x: author, y: peer, size: strength`      | Same.                                                                                                                                      |
| `absolute-churn` | `Stacked Bar Chart` | vegalite | `x: date, y: [added, deleted]`            | Added against deleted is the point; a single measure loses the refactor-versus-construction signal that the churn analysis exists to show. |

Every row above has been COMPILED against `flint-chart@0.4.0` and its encodings
checked against the template's declared channel list. Two rows were wrong before that
check and are corrected here; 3.4.1 records what was wrong and why it was invisible.

#### 3.4.1 The override table was verified by compiling it, and two rows were wrong

Recorded because the failure mode generalizes, and because the corrected values look
arbitrary without it.

**The assembler silently ignores an encoding channel the template does not
declare.** No exception, no warning, not even a `_warnings` entry. A misnamed channel
simply vanishes from the output.

Declared channels, read from `vlGetTemplateChannels` / `ecGetTemplateChannels` at
0.4.0, for the templates this plan uses:

| chartType           | Backend  | Declared channels                                     |
| ------------------- | -------- | ----------------------------------------------------- |
| `Bar Chart`         | vegalite | `x, y, color, column, row`                            |
| `Stacked Bar Chart` | vegalite | `x, y, color, column, row`                            |
| `Histogram`         | vegalite | `x, color, column, row`                               |
| `Scatter Plot`      | vegalite | `x, y, color, size, shape, opacity, column, row`      |
| `KPI Card`          | vegalite | `metric, value, goal`                                 |
| `Network Graph`     | echarts  | `x, y, size`                                          |

The two corrections:

- **`summary` / KPI Card.** The plan said "per-statistic", which was hand-waving
  rather than a specification. KPI Card's channels are `metric`, `value`, `goal`:
  there is no `x` or `y`. Compiling with `x`/`y` yields a `layer` spec; compiling
  with `metric`/`value` yields a `vconcat` of one card per row, which is exactly what
  a multi-statistic summary wants. The two outputs are not equal, so the wrong
  channels produced a genuinely different and worse chart, silently.
- **`authors` / Scatter Plot.** `detail` is NOT a Scatter Plot channel in Flint,
  though the published Vega-Lite reference lists it for other templates. Compiled
  output was `encoding keys = x,y`: `detail: entity` was dropped without a word.

`entity` on the `authors` scatter is therefore unbindable in any useful way.
`options: {addTooltips: true}` does not restore it (verified: `encoding.tooltip`
stays `undefined` at assemble time), and the only channels that would accept it are
`color`, `shape`, and `size`, each of which asks for hundreds of distinct values on a
real repository. So it is left unbound deliberately: the FINDING in that chart is the
shape of the cloud, where high revisions plus many authors marks a coordination
hotspot, not which point is which file. The per-file identity is available from the
`authors` table itself.

**Consequence for the implementation, and it is not small.** F1 keeps
`flint_spec.py` free of any Flint dependency, so the adapter cannot ask Flint what a
template accepts. The answer (F12, Q11) is to validate twice: the adapter checks
against the table above and exits `2` on an unknown channel, and `flint_render.ts`
re-checks against Flint itself, since it has the dependency and its answer is
authoritative rather than a copy.

The operating rule, generalizing 3.2.1 from numbers to structure: **every row of the
override table needs a compiled fixture before it is trusted.** That is the natural
content of ticket 1's test suite, and it is cheap.

Secondary charts worth offering via `--chart-type`, not as defaults:

- `coupling` and `communication` as `Heatmap`, which is what `pair_matrix.py`
  produces today and works on both backends.
- `entity-ownership` and `entity-effort` as `Stacked Bar Chart` with
  `stackMode: normalize`, which is the ownership-fragmentation view and a legitimate
  substitute for the fractal figures.
- `revisions` and `sum-of-coupling` as `Lollipop Chart`, which reads better than
  bars for a long ranked list.

Note the F-checklist hazard for `main-developer`,
`refactoring-main-developer`, `main-developer-by-revisions`, and `entity-effort`:
each carries a measure and its own total. Binding both to a stacked or colored
channel trips Flint's documented total-mixed-with-components rule. The adapter must
bind the RATIO (`ownership`) or the component alone, never component plus total.

### 3.5 CLI contract

```text
uv run scripts/flint_spec.py [--schema FILE] [--chart-type NAME]
                             [--backend vegalite|echarts]
                             [--width N] [--height N]
                             [-o OUT]
```

`--data-url` was specified in revision 1 and is **CUT from v1**. It was never fully
specified: Q2 established that Flint cannot read a codelens envelope directly (its
`parseJsonRows` accepts a bare array or a `values`/`data` key, and the envelope's key
is `rows`), so the flag needs a sidecar file, and nothing said who writes it or how
the renderer resolves a relative path. Since F8 makes inlining the default anyway,
the flag was surface without a consumer.

Anyone who wants an external data file can produce one with no adapter support:

```sh
jq '.rows' "${RESULT}" > "${ROWS}"    # a bare array, which Flint accepts
```

and edit `data` in the emitted spec. If a real need appears, the flag returns with
the sidecar step specified.

Envelope on stdin, spec on stdout or to `-o`. Reading stdin rather than a path
argument matches decision D13's "skill scripts consuming the envelope on stdin" and
lets the canonical pipe work without a temporary file. `--chart-type` overrides
3.4; an unknown name is a usage error listing the valid names for the backend.

`--schema FILE` (RESOLVED, Q5) takes the output of
`codelens schema --command CMD` and is OPTIONAL. Supplied, it gives the adapter the
per-column `desc` strings for `field_display_names`, so axes read "number of distinct
revisions" instead of `n_revs`. Omitted, the adapter degrades to raw column names.
Graceful degradation is what keeps it optional: no pipeline is broken by not passing
it, and `run.bash` captures it once per analysis.

The same flag is how the aggregation-role vocabulary reaches Python if that plan
lands, so the second-input question is settled once for both specs rather than twice.

Exit codes, matching the existing scripts: `0` ok, `2` usage or a rejected envelope
per 3.1, `3` empty payload.

### 3.6 Truncation reporting (Q9)

Flint truncates independently of codelens. Measured: a 300-row bar chart keeps 91
values and omits 209, recording it on the assembled spec's **private `_warnings`**
field, `{severity, code: 'overflow', message, channel, field}`, and labelling the
axis domain `...209 items omitted`.

So truncation is not silent, but it is only visible to whoever reads `_warnings` or
looks at the picture. Two obligations follow:

1. `flint_render.ts` MUST read `_warnings` and re-emit each entry on stderr, so the
   deterministic lane surfaces it the way codelens surfaces its own warnings.
2. The skill should prefer bounding with codelens `--rows` over letting Flint
   truncate, because `--rows` truncates by the analysis's own ranking while Flint
   truncates by axis sort order within a canvas budget. The former is a deliberate
   top-N; the latter is a layout accident.

`_warnings` is underscore-prefixed and therefore not a public contract. That is an
accepted risk under Q8's answer, and it is on the upgrade checklist in section 5.

## 4. What this does and does not replace

Replaces, if the spec lane is adopted: `churn.py`'s two chart bodies, and the chart
half of `pair_matrix.py`.

Does NOT replace, and the reasons are structural rather than incidental:

- `enclosure.py` and `treemap.py`. `ChartEncoding.field` is singular, so Flint's
  Treemap gets one hierarchy level and Sunburst two. A filesystem path needs
  arbitrary depth. This is the finding that corrects A1's original wording.
- The tokei structure join and the area-domination warning. Analysis concerns, not
  chart concerns.
- `commit_cloud.py`. Flint has no word cloud on any backend.
- `complexity_trend.py` and `fractal.py` keep their bespoke forms until the spec
  lane is proven; a normalized stacked bar is a substitute for a fractal figure, not
  an equivalent.

## 5. Rendering paths (F2)

Two, documented as alternatives rather than a choice the skill makes once.

**MCP, for agent and interactive use.** `npx -y flint-chart-mcp`. `render_chart`
returns PNG or SVG inline; `validate_chart` gives the skill a pre-render check it
has never had; `create_chart_view` opens an editable live view in hosts that support
MCP App UIs. Available only when a client is attached, so it cannot be the
deterministic path.

RESOLVED (Q7): the skill **documents** this config and does not ship one. No
`.mcp.json` is committed. Registering an MCP server is host-level state the user
owns, a skill writing to their client config is a side effect outside its remit, and
a project-scoped server would only help people working inside this repository rather
than users of the skill. The reference must therefore carry the stdio config
verbatim, since documentation is the only way anyone discovers this path.

**Deno, for `run.bash`.** A self-contained `flint_render.ts` importing the render
core:

```ts
import { renderChart } from "npm:flint-chart-mcp@0.4.0/render";
```

Pinned at **0.4.0**, not 0.4.1: Deno's default `minimumDependencyAge` supply-chain
guard refuses a version published under 24 hours ago, and weakening that default for
a patch release is a bad trade. This is the only path that reaches ECharts-only
templates without an MCP client, so it is required for the Network Graph charts
rather than optional.

### 5.1 Upgrade checklist (Q8)

RESOLVED: pin the version and re-verify by hand on upgrade. No Deno contract test in
CI, which keeps JavaScript out of the test suite at the cost of not detecting drift
until someone runs this list. The blast radius is bounded because `flint_spec.py`
has no Flint dependency: only the renderer and the type and chart-type NAMES are
exposed.

Before changing the pin, re-verify:

1. All 9 target semantic types still resolve: `Name`, `Category`, `Boolean`, `ID`,
   `Date`, `Count`, `Quantity`, `Duration`, `Percentage`.
2. `Network Graph` still reads `x` as source and `y` as target (compile a two-column
   edge table and inspect `series[0].links`).
3. `_warnings` still carries overflow entries on the assembled spec (section 3.6
   depends on a private field).
4. `intrinsicDomain` is still a gate rather than an override, per the table in 3.2.1;
   if this changes, the `percentage` mapping should be revisited.
5. The 6 default chart-type names in 3.4 are unchanged.
6. The declared channel lists in 3.4.1 are unchanged, and `flint_spec.py`'s channel
   table still matches them. Per F12 this table is a COPY of Flint data that no unit
   test can verify (Q8 ruled out a Deno test lane), so this item is the only scheduled
   parity check. `flint_render.ts` catches drift at render time, but only for specs
   that are actually rendered.

## 6. Ticket breakdown

The verification spike that used to be ticket 2 has been PERFORMED, and its findings
are folded into sections 1.1.1, 3.2.1, 3.4.1, 3.6, and the F6 note; it is not a
ticket. Q11 must be answered before ticket 1 is written, because it determines whether
ticket 1 carries a channel table.

### 6.1 Ticket 1: `flint_spec.py`

Sections 3.1 to 3.6, plus whatever Q11 decides. No Flint dependency; every test
asserts on emitted JSON.

Acceptance criteria:

- Emits a valid `ChartAssemblyInput` for all 17 table-shaped analyses, using the
  generic policy (3.3) plus the 6 overrides (3.4).
- Rejects `shape: "text"`, `ok: false`, `schema` output, and `parse` with exit `2`;
  exits `3` on an empty payload; exits `0` otherwise. Matches the existing scripts'
  taxonomy.
- **A COMPILED FIXTURE PER OVERRIDE ROW.** Each of the 6 overrides has a recorded
  expected spec, checked against `flint-chart@0.4.0`. This is the criterion 3.4.1
  argues for and the one that would have caught both broken rows.
- A regression test asserting `percentage` is annotated with a BARE string and `ratio`
  with `intrinsicDomain: [0, 1]` (3.2.1). This asymmetry is the rule most likely to be
  "corrected" into a bug.
- **A channel table** keyed backend then `chartType`, holding each template's declared
  channel set, seeded from 3.4.1. Every key in `encodings` is checked against it, and an
  unknown channel exits `2` (F12).
- Tests for the table that assert SELF-CONSISTENCY only: every `chartType` named in
  3.4 has an entry, and every override's encodings are a subset of that entry. Parity
  with Flint is deliberately NOT asserted here, because Q8 ruled out a Deno test lane;
  it is checked by ticket 2 at render time and by section 5.1 item 6 on upgrade.
- `--schema` absent degrades to raw column names without error; present, populates
  `field_display_names` from column `desc`.

### 6.2 Ticket 2: `flint_render.ts`

Section 5's Deno path.

Acceptance criteria:

- `npm:flint-chart-mcp@0.4.0/render` pinned; renders SVG and PNG for both backends.
- Re-emits every `_warnings` entry on stderr (3.6), so truncation is visible on the
  deterministic lane.
- Runs with no MCP client attached.
- Determinism: the ECharts `Network Graph` golden is stable, which 1.1.1 confirms is
  safe because `layout` defaults to `circular` rather than a force simulation.
- **Re-validates encodings against Flint** via `vlGetTemplateChannels` /
  `ecGetTemplateChannels`, erroring on a channel the template does not declare (F12,
  layer 2). This is the authoritative half of the check and the only thing that
  detects a post-upgrade channel rename.

### 6.3 Ticket 3: skill wiring

Acceptance criteria:

- `catalog.md` cards for the new charts, with the static-versus-interactive split
  honest about which lane produces each.
- `run.bash` captures `codelens schema --command CMD` per analysis so `--schema` is
  populated, and prefers codelens `--rows` over Flint truncation (3.6).
- `references/` documents both rendering paths and carries the MCP stdio config
  verbatim (Q7), since documentation is the only way that path is discoverable.
- Section 4's replaces/does-not-replace split is stated in the skill, not only here,
  so the catalog does not grow two cards that disagree.

### 6.4 Ticket 4: upgrade checklist

Section 5.1's six-item checklist recorded wherever the pin lives, so a future
upgrader finds it without reading this plan. Item 6 (channel-table parity) exists
because F12 accepts a copy of Flint data that nothing automatically re-checks.

### 6.5 Order and dependencies

Ticket 1 gates tickets 2 and 3, which are independent of each other. Ticket 4 can
land any time after ticket 2 fixes the pin. Nothing gates ticket 1: all eleven
questions are resolved.

If the aggregation-roles plan (004) is adopted, its steps 1 to 3 land BEFORE ticket 1
so that 3.3's measure precedence is written over the role vocabulary rather than a
hardcoded list of semantic names. See that plan's section 11.

Ticket 1 alone is not user-visible value; see section 9.

## 7. Related documents

- The reasoning behind section 3.2, including why the object form is used and why the
  `loc`/`count` split survives a many-to-one type map, is in the semantics explainer.
  It is published alongside this plan as
  `docs/specs/003-flint-adapter/semantics-and-the-near-identity-adapter.md`, and has
  been corrected against 3.2.1: it originally carried revision 1's inverted
  `intrinsicDomain` claim.
- `docs/specs/004-aggregation-roles/plan.md` shares this plan's `--schema`
  decision (Q5) and is argued to land before ticket 1; see its section 11.
- Decisions D3, D3a, D3b, D3c in `docs/specs/002-data-output/plan.md` are the
  origin of the `semantics` contract this adapter consumes.
- Epic cod-304f carries the answered A1 and A3 and the still-open A2 and A4.

## 8. Questions

Q1 through Q10 resolved 2026-07-27 and Q11 on 2026-07-28. Five were answered by
executing Flint under Deno (Q1, Q2, Q3, Q6, Q9) and six by decision (Q4, Q5, Q7, Q8,
Q10, Q11). Kept in full, with their answers, because the reasoning is the record:
three of them overturned a conclusion this plan had already written down.

Q11 was opened 2026-07-28 by the review that compiled the override table, and
resolved the same day.

### Resolved

### Answered by running Flint

- **Q1. Does `Network Graph` read `x` as source and `y` as target?** **YES.**
  `echarts/templates/graph.ts` documents "Data model (each row = one edge): x
  (nominal): source node name, y (nominal): target node name, size (quantitative,
  optional): edge weight", and compiling a real `coupling` row set yields
  `series.type: 'graph'` with correct `links` and degree-sized `nodes`. This was the
  highest-stakes question in the plan: had `x`/`y` been coordinates, the adapter
  would have collapsed back into epic cod-304f's E1 through E4. **The decoupling of
  A1-A4 from E1-E9 is confirmed.**
- **Q2. Does `data.url` accept a bare JSON array?** **YES, but not the codelens
  envelope.** `parseJsonRows` accepts a bare array, `{values: [...]}`,
  `{data: [...]}`, or `{data: {values: [...]}}`, and rejects everything else. The
  envelope's key is `rows`, so it is not directly usable. `--data-url` therefore
  needs a rows-only sidecar, and `jq '.rows' result.json` produces one with no
  Python involved. F8's inlining remains the default.
- **Q3. Is the generic-plus-override split right?** **YES, and the alternative was
  tested rather than assumed.** See the F6 note in section 2 for the seven-analysis
  comparison against `vlRecommendCharts`. It is wrong on five, never proposes
  Network Graph, and would put JavaScript inside a Python script.
- **Q6. Confirm the registry reading empirically.** **DONE, and it found an error.**
  `Quantity` is confirmed additive. But `intrinsicDomain` does NOT override the
  fractional heuristic; it gates it, and annotating a `percentage` column causes the
  exact 100x error the annotation was added to prevent. Section 3.2.1 has the
  measured table. This is the single most valuable outcome of the whole spike.
- **Q9. Does codelens `--rows` interact with Flint's overflow filtering?** **BOTH
  TRUNCATE, and Flint's is reported.** Measured: 300 rows in, 91 kept, 209 omitted,
  recorded on the private `_warnings` field and labelled in the axis domain. Section
  3.6 makes surfacing it an obligation of `flint_render.ts` and prefers codelens
  `--rows` because it truncates by the analysis's own ranking rather than by a
  layout budget.

### Answered by decision

- **Q4. Does `parse` get an adapter?** **NO, exclude it, but keep its semantics
  mapped.** The adapter charts analyses, not event dumps. Accepted consequence:
  `commit_id`, `text`, and `flag` are mapped but never exercised by a chart, because
  the vocabulary is a closed contract and partial coverage of it would be worse.
  Recorded in 3.1.
- **Q5. Should the adapter read column `desc` for `field_display_names`?** **YES,
  via an OPTIONAL `--schema FILE`.** Scripts may take a second input, degrading
  gracefully to raw column names when it is absent. This also settles the same
  question for the aggregation-role plan, which needs the identical channel to reach
  Python. Recorded in 3.5.
- **Q7. Does the project ship an MCP client config?** **NO, document only.** No
  `.mcp.json` is committed. Recorded in section 5 with the reasoning.
- **Q8. How is version drift handled?** **Pin plus a documented review trigger**, no
  Deno contract test in CI. Keeps JavaScript out of the test suite; accepts that
  drift is undetected until someone runs the checklist in 5.1. The accepted risk is
  concentrated in the private `_warnings` field, which is item 3 on that list.
- **Q11. How does the adapter validate its encodings against a template's declared
  channels? RESOLVED: BOTH a Python channel table AND a check in the renderer.** The
  problem is in 3.4.1: Flint silently discards an encoding channel a template does not
  declare, and two of six override rows were wrong for exactly that reason and
  compiled cleanly. F1 keeps `flint_spec.py` free of any Flint dependency, so it
  cannot ask Flint what a template accepts.

  The two layers do different jobs, which is why neither alone was chosen:

  1. **`flint_spec.py` carries a channel table** keyed by backend then `chartType`,
     holding the declared channel set. Every key in `encodings` must be a member;
     an unknown channel is a HARD ERROR, exit `2`, because it can only come from a bad
     override row or a `--chart-type` the encodings do not fit. This catches the
     mistake at the point it is made, with no Flint dependency.
  2. **`flint_render.ts` re-checks against Flint itself**, calling
     `vlGetTemplateChannels` / `ecGetTemplateChannels` on the spec it is handed. It has
     the dependency, so its answer is authoritative rather than a copy.

  **The honest limit on layer 1, which must not be overstated.** Q8 ruled out a Deno
  test lane, so the Python table's PARITY WITH FLINT cannot be asserted by a unit
  test. `flint_spec_test.py` can only check self-consistency: that every `chartType`
  named in the override table has an entry, and that every override's encodings are a
  subset of it. Parity is checked by layer 2 at render time and by the section 5.1
  checklist on upgrade.

  **A useful side effect.** Because layer 2 errors on an unknown channel, a Flint
  upgrade that renames a channel fails loudly the first time anything is rendered.
  That is the drift detection Q8 declined to build in CI, arriving free on the render
  path. It does not cover a spec that is emitted and never rendered, which is the
  residual gap and is recorded in section 9.

- **Q10. Is `Category` versus `Name` worth distinguishing?** **YES, keep it.**
  Verified that `Category`, `Name`, and `Unknown` have byte-identical registry
  entries AND compile to identical Vega-Lite, so the distinction is purely
  documentary TODAY. Kept anyway: it costs nothing, it makes the emitted spec
  self-describing about which column was a path and which was a bucket name, and it
  is already correct if Flint ever differentiates Entity subtypes.

### What the spike changed

Worth stating plainly, because it is an argument for spiking before building:

| Conclusion in revision 1                         | After running Flint                                      |
| ------------------------------------------------ | -------------------------------------------------------- |
| `intrinsicDomain` defeats fractional detection    | It causes the error instead; `percentage` must go bare    |
| Four semantics use the object form                | Two do                                                   |
| Network Graph roles unknown, might collapse scope | Confirmed; scope holds                                    |
| Flint truncation might be silent                  | It reports, on a private field                            |
| Pin 0.4.1                                         | Pin 0.4.0; Deno refuses versions under 24h old            |

## 9. Risks

- **Pre-1.0 dependency.** 0.4.0 added 56 templates. The registry is more stable
  than the template list, which is why F4 targets it, but neither is frozen.
- **Two toolchains.** The skill becomes Python plus Deno. Now firmly justified: Q1
  confirmed that Network Graph consumes an edge table, and it is an ECharts-only
  template, so the Deno lane is the only headless route to the `coupling` and
  `communication` charts. This risk is accepted rather than open.
- **Spec lane without a renderer.** If neither rendering path is wired up, the
  adapter emits specs nothing consumes. Ticket 1 alone is not user-visible value;
  it must land with ticket 2 or the MCP path documented.
- **A private field in the contract.** Section 3.6 depends on `_warnings`, which is
  underscore-prefixed and therefore outside Flint's public API. Q8's answer (pin plus
  manual checklist) means a rename would be caught by a human, not a test. The
  degradation is quiet: truncation would stop being reported on stderr while the
  chart would still self-label the omission.
- **A channel table that nothing automatically re-checks.** F12 accepts a copy of
  Flint's declared channels inside `flint_spec.py`, and Q8's no-Deno-test-lane answer
  means no unit test can verify it against Flint. Two things narrow the exposure:
  `flint_render.ts` re-checks authoritatively, and section 5.1 item 6 is a scheduled
  manual check. The residual gap is a spec that is EMITTED AND NEVER RENDERED, where a
  stale table would pass silently. Given that emitting a spec nobody renders has no
  value anyway (section 9's first bullet), this is accepted.
- **An asymmetry that invites a regression.** `percentage` bare and `ratio`
  annotated (3.2.1) looks like an inconsistency and is not. It is called out here
  because the natural instinct is to make it symmetric, and doing so reintroduces a
  100x error in rendered ownership and coupling values. Ticket 1 carries a regression
  test for exactly this.
- **Duplicate charting.** Two ways to draw a churn chart invites drift. Section 4
  must be stated in the skill, not just here, or the catalog will grow two cards
  that disagree.
