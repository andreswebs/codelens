---
name: codelens
description: "Operate the codelens CLI and visualize its output: mine a git log, run any of its 18 evolutionary analyses (hotspots, coupling, churn, ownership, code age, commit vocabulary), render a result as a chart, or assemble a whole-repository findings report. Use when running a codelens analysis, when turning one into embeddable SVG, interactive HTML, or PNG for docs and slides, or when asked to analyze how a code base has evolved."
---

# codelens code base visualization

Turn `codelens` output into charts that answer a question about a code base. The recurring subject is the **hotspot**: complex code that changes often. Every run follows one pipeline; the visualization chosen decides the data collected and the script run.

The readings draw on Adam Tornhill's _Your Code as a Crime Scene_ for the hotspot, knowledge-map and fractal-figure interpretations, among other sources.

`codelens` mines a git log and emits structured JSON. This skill both operates the
CLI and visualizes its output. To run analyses (the canonical pipe workflow,
runtime schema discovery, the analyses catalog, output shaping, transforms, and
exit codes), see [operating.md](references/operating.md); when the deliverable is
an analysis result itself, that reference is the whole job. To visualize, follow
the pipeline below, which consumes that JSON (plus, for two visualizations, a size
sidecar or the live repo) and renders an embeddable artifact.

## Pipeline

### 1. Frame the question, pick the visualization

Route with the table below to a single visualization and its script. Open its
card in [catalog.md](references/catalog.md) for the full contract (columns,
sidecars, formats, and how to read the result).

**Done when:** one visualization and one script are selected.

| Question                         | Visualization         | codelens analysis                                | Script                | Interactive |
| -------------------------------- | --------------------- | ------------------------------------------------ | --------------------- | ----------- |
| Where is the risky code?         | Hotspot enclosure map | `revisions`                                      | `enclosure.py`        | yes         |
| Who owns what?                   | Knowledge map         | `main-developer`                                 | `enclosure.py`        | yes         |
| How old is each part?            | Code-age map          | `code-age`                                       | `enclosure.py`        | yes         |
| What changes together?           | Change-coupling graph | `coupling`, `sum-of-coupling`                    | `coupling_graph.py`   | yes         |
| Do teams align with code?        | Communication network | `communication`                                  | `dev_network.py`      | yes         |
| How much churn, over time?       | Churn trend           | `absolute-churn`, `author-churn`, `entity-churn` | `churn.py`            | no          |
| How is effort shared per module? | Fractal figures       | `entity-effort`, `fragmentation`                 | `fractal.py`          | no          |
| What does the team talk about?   | Commit word cloud     | `parse` (message column)                         | `commit_cloud.py`     | no          |
| Is this hotspot deteriorating?   | Complexity trend      | none (live repo)                                 | `complexity_trend.py` | no          |
| What is the headline?            | Summary tiles         | `summary`                                        | `churn.py --summary`  | no          |

Those ten rows are the **artifact lane**: each script writes a finished HTML, SVG,
or PNG file and needs only `uv`. A second lane, `flint_spec.py`, writes a Flint
chart _spec_ that a downstream renderer turns into SVG or PNG, and covers several
charts the artifact lane has no counterpart for. Which lane draws which chart is
settled in
[catalog.md](references/catalog.md#two-lanes-and-what-each-one-draws); the two
rendering paths are in [flint.md](references/flint.md).

### 2. Collect the data

Generate a compatible log and run the chosen analysis:

```sh
eval "$(codelens print-log-command --after "${SINCE}")" > git.log
codelens <analysis> --log git.log > data.json
```

Discover any analysis's flags and columns with `codelens schema --command
<analysis>`. Collect a sidecar only when the card calls for one:

- **Size sidecar** (enclosure family): `tokei --output json > tokei.json`, run
  from the log's root.
- **Live repo** (complexity trend only): pass the repository path and file; this
  visualization reads git history directly, not codelens JSON.

The full CLI surface (analyses catalog, output shaping, transforms like
`--group`/`--team-map`/`--temporal-period`, analysis-period heuristics, exit
codes) is in [operating.md](references/operating.md).

**Done when:** the analysis JSON exists and is non-empty (and any sidecar JSON
exists).

### 3. Build the artifact

Run the card's one documented command, e.g.:

```sh
uv run scripts/enclosure.py --weights data.json --structure tokei.json -o hotspots.html
```

**Done when:** the script exits `0` and the named output file exists.

### 4. Render the requested formats

Which case applies is the `Interactive` column of the step 1 table.

- **Static:** the `-o` extension picks the format (`.svg` or `.png`), one per run;
  run the script twice for both. Use these for slides and PDF.
- **Interactive:** the script writes an `.html` file for live viewing and iframe
  embedding, and has no static export. For a picture of the same data, render its
  degraded counterpart (`treemap.py`, `pair_matrix.py`) per
  [embedding.md](references/embedding.md).
- **Spec lane:** a spec is not an artifact. Render it with `flint_render.ts` under
  Deno, or hand it to the flint-chart MCP server; [flint.md](references/flint.md)
  has both invocations, the permission flags Deno needs, and the MCP stdio config.

Target mechanics (inline SVG, iframe) are in [embedding.md](references/embedding.md).

**Done when:** an artifact exists in every requested format.

### 5. Read the result

State the finding in the visualization's own terms.
[interpretation.md](references/interpretation.md) is the reading authority: the
investigative funnel that orders an investigation, a reading block per
visualization, the heuristics table (every number, in one place), and the misuse
guardrails the social analyses must respect (never rank individuals; aggregate to
teams; findings are probabilistic). A visualization delivered without this reading
is incomplete.

**Done when:** the finding is named, in the terms `interpretation.md` gives - not
just the chart handed over.

### 6. Compose the report (optional)

When the deliverable is a sequenced findings report rather than a single chart,
three commands replace steps 1 to 4:

```sh
bash scripts/run.bash --repo "${REPO}" --out out/   # every analysis, figure, digest.md
# write findings.md: your reading of each analysis, grounded in out/digest.md
uv run scripts/report.py --findings findings.md --figures-dir out/figs/ \
  --summary out/summary.json -o report.md
```

`run.bash` covers both lanes, so this path yields the interactive artifacts too;
steps 2 and 3 are for tuning one by hand. `report.py` pins the investigative
sequence, embeds figures inline as SVG, and always emits the social-analysis
guardrails. Read each finding per
[interpretation.md](references/interpretation.md). The output inventory, the
findings-file format, and the figure stems are in
[reporting.md](references/reporting.md).

**Done when:** `report.py` exits `0` and `report.md` carries every section with its
findings and figures.

## Determinism boundary

The agent runs codelens and picks the visualization; the scripts pin the
transform and render so they never vary. Every script is self-contained: one
file, inline dependencies, one command, meaningful exit code, stable output.
