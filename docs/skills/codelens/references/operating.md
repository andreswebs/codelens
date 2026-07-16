# Operating the codelens CLI

How to drive `codelens` to produce analysis results. Loaded from
[SKILL.md](../SKILL.md) for step 2, and whenever the deliverable is an analysis
result itself rather than a visualization. This is the canonical operating
reference for codelens; the repo's `AGENTS.md` points here. `codelens schema` is
the runtime source of truth for exact flags and columns.

`codelens` is read-only: it mines a git log on stdin, emits a JSON envelope on
stdout, and never runs git or writes files.

## Canonical workflow

Ask codelens for the log command, generate the log, pipe it into one analysis:

```sh
eval "$(codelens print-log-command)" > git.log      # add --after=YYYY-MM-DD to scope
codelens <analysis> --log git.log > result.json      # stdin is the default input
```

`print-log-command` emits the required `git log` (`--numstat`, four fields
`%h %ad %aN %s`, `--no-renames`, `--use-mailmap`). The trailing `%s` subject is
what powers the `messages` analysis and the word cloud; 3-field logs still parse
(subject `-`).

The default reads the checked-out branch's history, matching code-maat and
avoiding commits from unmerged branches or dated after `HEAD`. Pass
`print-log-command --all` for cross-branch history (all refs), at the cost of
merge- and branch-tip noise.

### Resolving author aliases

When one person commits under several names, ownership and communication maps
inflate. Resolve aliases in this order:

1. Zero-config: the emitted log uses `--use-mailmap`, so a repo `.mailmap`
   collapses aliases automatically. This is the recommended first step and a safe
   no-op when the repo has no `.mailmap`.
2. Escalation: when there is no `.mailmap`, or a team-level rollup is wanted, use
   `--team-map` to map authors (or aliases) to canonical identities or teams.

## Discover a command at runtime

Never guess flags or column names.

```sh
codelens schema                     # every command, aliases, exit codes
codelens schema --command coupling  # summary, flags, row_schema, error_codes, exit_codes
```

`schema --command` is authoritative, including columns that appear only with
`--verbose`. It also describes the helper commands themselves
(`schema --command print-log-command`, `schema --command schema`), so their
flags and exit codes are discoverable at runtime like any analysis; helpers
carry no `row_schema`. The build version is the `--version` flag (bare output),
not a subcommand.

## Analyses

| Command                       | Alias                  | Purpose                                                      |
| ----------------------------- | ---------------------- | ------------------------------------------------------------ |
| `revisions`                   |                        | change frequency per entity                                  |
| `authors`                     |                        | distinct authors per entity                                  |
| `coupling`                    |                        | logical (temporal) coupling between entity pairs             |
| `sum-of-coupling`             | `soc`                  | sum of coupling per entity                                   |
| `summary`                     |                        | overview counts for the mined data                           |
| `absolute-churn`              | `abs-churn`            | lines added/deleted per date                                 |
| `author-churn`                |                        | lines added/deleted per author                               |
| `entity-churn`                |                        | lines added/deleted per entity                               |
| `entity-ownership`            |                        | per-author churn contribution to each entity                 |
| `main-developer`              | `main-dev`             | main developer per entity by lines added                     |
| `main-developer-by-revisions` | `main-dev-by-revs`     | main developer by revision count                             |
| `refactoring-main-developer`  | `refactoring-main-dev` | main developer by lines removed                              |
| `entity-effort`               |                        | each author's revision share per entity                      |
| `fragmentation`               |                        | author fragmentation (fractal value) per entity              |
| `communication`               |                        | heuristic communication strength between author pairs        |
| `code-age`                    | `age`                  | age in months since last modification                        |
| `messages`                    |                        | entity frequency for a commit-message regex (`--expression`) |
| `parse`                       | `identity`             | dump parsed records in log order (debug/interop)             |

Helpers: `print-log-command`, `schema`. The build version is printed by the
`--version` flag (bare version string), not a subcommand.

## Output formats and shaping

`--format json` (default) wraps rows in a self-describing envelope
(`schema_version, ok, analysis, row_count, rows`); `--rows N` truncation adds
`total_count` and `truncated: true`. An empty-but-valid result is `ok: true,
row_count: 0, rows: []`, exit 0. Column keys are snake_case.

Analyses that declare flags also carry a `params` object (after `analysis`)
echoing every declared flag at its effective value, defaults included, so a
run is self-documenting: `coupling`, `sum-of-coupling`, `code-age`, and
`messages`. Flagless analyses omit `params`.

- `--format`: `json` | `ndjson` (one row/line, no envelope) | `csv`
  (code-maat-compatible kebab-case headers) | `table` (human terminal).
- Bound large output: `--rows N` (all formats, after sorting) and
  `--fields rows.entity,rows.n_revs` (json only; always keeps `schema_version`
  and `ok`).
- Diagnostics go to stderr; stdout is only results, so piping into a JSON parser
  is safe.

## Pipeline transforms

Global flags that reshape the input before analysis. They run in a fixed order,
`filter -> group -> temporal -> team-map`, each a no-op when its flag is absent:

- `--include GLOB` / `--exclude GLOB` (both repeatable): keep or drop entities by
  gitignore-style path glob (`**` supported), matched against the full entity
  path. Precedence is exclude-after-include: with any `--include`, an entity must
  match at least one include to survive, then any `--exclude` match drops it; with
  no includes, all entities are included and only excludes apply. Filtering runs
  first, before grouping, so globs match raw file paths (`**/Migrations/**`), not
  layer names. A malformed glob is a usage error (exit 2). Note `*` and `?` do not
  cross `/`; use `**` to span directories.
- `--group FILE` (`--group-format text|json`): map files to architectural layers.
  Text lines are `pattern => name`; unanchored patterns are path-prefix matches,
  anchored (`^...`) are full expressions; unmatched files are dropped. Use to run
  any analysis at the component level. Grouping `coupling` to components dilutes
  per-pair degrees, so the default `--min-coupling 30` may filter everything and
  return an empty result; lower `--min-coupling` (around 5). When every candidate
  pair is filtered, codelens warns on stderr (`coupling_all_filtered`) with the
  highest degree it observed.
- `--team-map FILE` (`--team-map-format csv|json`): map authors to teams
  (`author,team`); unmapped authors pass through. Resolve author aliases with a
  repo `.mailmap` first. Feeds the communication network's Conway view.
- `--temporal-period N`: collapse commits into sliding N-day change sets before
  analysis. Intended for coupling, where per-commit granularity is too narrow
  across teams working in days or weeks.

## Authored-only run

On a real monorepo, hotspot and coupling analyses are dominated by
machine-generated files (migration snapshots, generated localization, designer
files, lock files). Exclude them with one shared glob set passed to both the
`codelens` analysis and the enclosure map, so the weights and the drawn structure
agree:

```sh
GENERATED='--exclude **/Migrations/** --exclude **/*.g.dart
  --exclude **/*.Designer.cs --exclude **/*.lock --exclude **/package-lock.json'

git log --numstat --date=short \
  --pretty=format:'--%h--%ad--%aN--%s' --no-renames --use-mailmap \
  | codelens revisions $GENERATED > revisions.json

python3 enclosure.py --weights revisions.json --weight-col n_revs \
  --structure tokei.json $GENERATED -o hotspots.html
```

Exclude only truly generated artifacts. Config (`appsettings*.json`, `*.yml`) and
localization sources (`*.arb`, `*.resx`) are human-authored and should not be
excluded by default.

The same `--exclude` set must reach **every entity-centric analysis**, not only
the hotspot and coupling maps. `revisions`, `coupling`, `sum-of-coupling`,
`main-developer`, `code-age`, `absolute-churn`, `entity-effort`, and
`fragmentation` are all distorted when a generated file is regenerated. In one
fleet repo a single +852k-line commit that regenerated a `juris-rules` JSON blob
(top commit word `regenerate`) dominated `absolute-churn` and skewed effort,
fragmentation, and ownership until that path was excluded. Do **not** pass the
excludes to `communication` (an author graph) or `summary` (whole-repo counts), so
authorship and totals stay whole. `scripts/run.bash` is the canonical
implementation: it applies its built-in exclude set to exactly those
entity-centric analyses and leaves `communication` and `summary` unfiltered.

### Reference-data domination

Even after the generated-file globs, a few large reference-data or spec files (for
example `naics_*.json`, `public/v0/openapi.yaml`) can occupy most of a treemap,
because area is tokei LOC, not change. `treemap.py` and `enclosure.py` warn on
stderr for any single file over 10% of total mapped LOC:

```text
dominant: public/v0/openapi.yaml 34% (12040 LOC)
```

The map is never altered; the tool only names the offender so you decide. When a
named file is reference data rather than code you maintain, add its path to the same
`--exclude` set and re-run. The check is computed on the post-exclude node set, so
each re-run surfaces the next offender until the map reads as code. There is
deliberately no size-threshold auto-exclude (it would silently drop a legitimately
large source file) and no area rescale (it would break the treemap's area-as-size
contract); the explicit `--exclude` is the one remedy.

## Analysis period

Scope the git log by date (`--after=` on the log command). Heuristics: one year is
a good default; a month for very high-churn repos; a window around a major event
(reorg, redesign) to measure its impact. Too much history buries recent trends.

A trailing window assumes activity clusters near the present. A stale or
front-loaded repo, with an early burst and a late trickle of commits, gets a
nearly empty window: in one fleet a repo had 17 in-window commits out of 12,252.
For a stale or inactive repo, analyze **full history** instead, since there is no
recency tension when nothing recent is happening. `scripts/run.bash --full-history`
does this and warns when the windowed log is empty. Auto-widening the window when
in-window commits fall below a threshold was considered and declined: it silently
changes the analysis window from a heuristic, making two runs of the same command
incomparable. An explicit lever plus the empty-window warning is preferred.

The one exception is `code-age`: run it against full history, not a window scoped
with `--after`. Age is measured from the log's earliest commit, so a scoped window
caps every file's reported age at the window length.

## Errors and exit codes

Errors are **always** a JSON envelope on stderr (`{ok: false, error: {code,
message, hint}}`), for every `--format` value including `text` and `table`.
`--format` selects the results shape on stdout, not the diagnostics on stderr;
there is no `✗ <message>` text error path, so parse the envelope's `message` and
`hint` fields directly.

| Exit | Meaning               | Examples                                                              |
| ---- | --------------------- | --------------------------------------------------------------------- |
| 0    | success (incl. empty) | any analysis that ran                                                 |
| 2    | usage error           | unknown flag/subcommand, bad value, `messages` without `--expression` |
| 3    | input error           | empty or unparseable log, malformed `--group`/`--team-map`            |
| 1    | internal              | a bug; prints a trace only under `--debug`                            |

Non-fatal advisories are emitted as single-line JSON **warning** diagnostics on
stderr, distinguished from errors by `level: "warning"` (and no `ok` field):
`{schema_version, level: "warning", code, message, hint?, details?}`. One per
line (valid NDJSON), they never change the exit code and never touch stdout, so a
consumer reading results from stdout is unaffected.

The skill's Python render scripts follow the same convention: they print their
`wrote ...` summary (and uv's `Installed N packages`) to stderr on success, so a
wrapper must judge them by exit code, never by stderr being empty. See
[reporting.md](reporting.md).
