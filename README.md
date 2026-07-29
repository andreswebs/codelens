# codelens

`codelens` turns a git history into structured, self-describing JSON: 18 analysis
commands over a git log, one machine-readable envelope per run, and enough type
information in that envelope for a downstream renderer to draw a chart without
knowing anything about your repository.

It is strictly read-only. It never runs git, never writes files, and has no side
effects. You generate a git log yourself and pipe it in.

It is also agent-first. Every analysis is discoverable at runtime, results and
diagnostics never share a stream, and the input format lives in the tool rather
than in your memory.

## Why

Evolutionary analysis reads a repository's history to find where change actually
concentrates: which files churn together, which are owned by one person, which have
gone quiet. The analyses are well established. What `codelens` adds is an I/O
surface an agent or a script can drive without guessing:

- **One canonical JSON envelope** on stdout, always the same shape, never affected
  by whether stdout is a terminal.
- **Semantic typing.** Each output column is tagged with what it _means_
  (`filepath`, `person`, `count`, `loc`, `percentage`, `duration_months`, ...), not
  just its JSON type. This is what lets a renderer pick a size channel over a colour
  channel correctly, and it is information only the tool that produced the data has.
- **Runtime introspection.** `codelens schema` describes every command, flag,
  column, error code and exit code, so nothing has to be memorised or hardcoded.
- **Coded errors on stderr only**, with a documented exit-code taxonomy.
- **Context discipline** via `--fields` and `--rows`, so a result fits in a budget.

## Install

Via [Homebrew](https://brew.sh) (Linux and macOS):

```sh
brew tap andreswebs/tap
brew install andreswebs/tap/codelens
```

## Quick start

Ask `codelens` for the git command rather than memorising the log format, then pipe
the log straight in:

```sh
eval "$(codelens print-log-command)" | codelens coupling
```

Scoping flags forward through the helper to git, so a window needs no format
knowledge:

```sh
eval "$(codelens print-log-command --after=2024-01-01)" | codelens revisions --rows 10
```

Input defaults to stdin; `--log FILE` reads a file instead.

## What it analyses

18 commands, grouped by the question they answer. `codelens schema` is the
authoritative list, including the terse aliases kept for compatibility.

| Question                        | Commands                                                                                                                            |
| ------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| Where does change concentrate?  | `revisions`, `authors`, `code-age`                                                                                                  |
| What changes together?          | `coupling`, `sum-of-coupling`                                                                                                       |
| How much code moves, and where? | `absolute-churn`, `author-churn`, `entity-churn`                                                                                    |
| Who knows this code?            | `main-developer`, `main-developer-by-revisions`, `refactoring-main-developer`, `entity-ownership`, `entity-effort`, `fragmentation` |
| Who works alongside whom?       | `communication`                                                                                                                     |
| Everything else                 | `summary`, `messages`, `parse`                                                                                                      |

The ownership and communication analyses describe **coordination and key-person
risk, never individual performance**. That distinction is not decoration; it is the
difference between a useful reading and a harmful one.

## What it produces

One envelope per run:

```json
{
  "schema_version": 1,
  "ok": true,
  "analysis": "authors",
  "shape": "table",
  "semantics": {
    "entity": "filepath",
    "n_authors": "count",
    "n_revs": "count"
  },
  "row_count": 1,
  "rows": [{ "entity": "src/parser.go", "n_authors": 2, "n_revs": 2 }]
}
```

Three fields carry the self-description:

- **`shape`** names the payload topology, and the payload key follows from it
  (`table` carries `rows`). The set holds only what the binary actually emits, so a
  shape read from `schema` is never a promise the tool cannot keep.
- **`semantics`** maps each column to its meaning. A transform that destroys
  structure degrades the semantic: under `--group`, a splittable `filepath` becomes
  an opaque `label`. A transform that merely aggregates does not, so `--team-map`
  keeps `author` as `person`.
- **`transforms`** records which pipeline transforms ran, and is absent entirely on
  a pass-through run.

`schema` additionally publishes an **`aggregation_roles`** map, stating how a value
of each semantic may be combined: `count` and `loc` are `additive`, while
`percentage`, `ratio` and `duration_months` are `intensive` and must not be summed
into a reported total. Consumers use it to check an aggregation instead of assuming
one.

Errors are the same envelope with `ok: false` and a coded `error` object, on stderr.
Exit codes follow a fixed taxonomy: `0` success including empty results, `64` usage,
`65` data, `70` internal, `74` I/O.

Full CLI reference, including every flag, transform, error code and the analysis
catalog: [operating.md](docs/skills/codelens/references/operating.md).

## Visualizing

`codelens` is the data plane; rendering lives in an
[agent skill](docs/skills/codelens/SKILL.md) that ships with the repository. It
draws from two lanes, and the split between them is fixed so no chart grows two
competing recipes:

- The **artifact lane** is a set of self-contained Python scripts that write
  finished output: zoomable circle-packing maps (hotspot, ownership, code age),
  change-coupling and team-communication graphs as interactive HTML, and static
  SVG/PNG for churn trends, fractal effort figures, complexity trends, a commit word
  cloud and summary tiles. Needs `uv` only.
- The **spec lane** emits a [Flint](https://github.com/microsoft/flint-chart) chart
  spec, which Flint compiles to Vega-Lite or ECharts. It covers the edge tables as
  network graphs, the churn trend, summary KPI cards, a code-age histogram, and
  several charts the artifact lane has no counterpart for. Renders headless through
  Deno, or interactively through the flint-chart MCP server.

Semantic typing is what makes the spec lane cheap: because every column already
declares what it means, translating a result into a chart spec is close to a
rename rather than a per-analysis special case.

One command produces every analysis, both lanes, and a grounding digest for a
whole repository:

```sh
bash docs/skills/codelens/scripts/run.bash --repo "${REPO}" --out out/
```

From there, `report.py` assembles a single self-contained markdown report with a
fixed eleven-section sequence, figures embedded inline, and the coordination-risk
guardrails always present. See
[reporting.md](docs/skills/codelens/references/reporting.md).

### Installing the skill

Into any supported agent (Claude Code, Codex, Cursor and others) with the
[Vercel skills CLI](https://github.com/vercel-labs/skills):

```sh
npx skills add andreswebs/codelens
```

Or just this skill, globally, for Claude Code:

```sh
npx skills add andreswebs/codelens --skill codelens -g -a claude-code
```

## Documentation

- [docs/skills/codelens/](docs/skills/codelens/) - the skill: operating the CLI and
  rendering its output.
  - [operating.md](docs/skills/codelens/references/operating.md) - the canonical
    CLI reference.
  - [catalog.md](docs/skills/codelens/references/catalog.md) - every chart, its
    inputs and its lane.
  - [interpretation.md](docs/skills/codelens/references/interpretation.md) - how to
    read a result, and how not to misuse the social analyses.
  - [flint.md](docs/skills/codelens/references/flint.md) - the spec lane and both
    rendering paths.
  - [reporting.md](docs/skills/codelens/references/reporting.md) - the report
    pipeline.
- [docs/adr/](docs/adr/) - architecture decisions, including
  [0008](docs/adr/0008-canonical-output-representation.md) on the canonical
  envelope and [0002](docs/adr/0002-exit-code-taxonomy.md) on exit codes.
- [AGENTS.md](AGENTS.md) - repository map, build, and contribution guide.

## Provenance and influences

`codelens` began as a Go port of
[code-maat](https://github.com/adamtornhill/code-maat) by
[Adam Tornhill](https://github.com/adamtornhill), and the analyses and their
algorithms originate there. It still uses code-maat's test corpus (fixtures, sample
logs and expected outputs) as its regression oracle, which is what keeps the numbers
honest.

The visualization vocabulary draws on several sources. Tornhill's
[_Your Code as a Crime Scene_](https://isbnsearch.org/isbn/9798888650844) is where
the hotspot, knowledge-map and fractal-figure readings come from. The circle-packing
map is the same idea as GitHub Next's
[repo-visualization](https://githubnext.com/projects/repo-visualization/), with the
colour channel carrying an evolutionary metric rather than file type.
[Flint](https://github.com/microsoft/flint-chart) supplies the chart grammar for the
spec lane. Nothing here is tied to a single framework; the envelope is the contract,
and renderers are interchangeable.

## Authors

**Andre Silva** - [@andreswebs](https://github.com/andreswebs)

## License

[GPL-3.0-or-later](LICENSE).
