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
`%h %ad %aN %s`, `--no-renames`). The trailing `%s` subject is what powers the
`messages` analysis and the word cloud; 3-field logs still parse (subject `-`).

## Discover a command at runtime

Never guess flags or column names.

```sh
codelens schema                     # every command, aliases, exit codes
codelens schema --command coupling  # summary, flags, row_schema, error_codes, exit_codes
```

`schema --command` is authoritative, including columns that appear only with
`--verbose`. It also describes the helper commands themselves
(`schema --command print-log-command`, `schema --command schema`,
`schema --command version`), so their flags and exit codes are discoverable at
runtime like any analysis; helpers carry no `row_schema`.

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

Helpers: `print-log-command`, `schema`, `version`.

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

Global flags that reshape the input before analysis:

- `--group FILE` (`--group-format text|json`): map files to architectural layers.
  Text lines are `pattern => name`; unanchored patterns are path-prefix matches,
  anchored (`^...`) are full expressions; unmatched files are dropped. Use to run
  any analysis at the component level.
- `--team-map FILE` (`--team-map-format csv|json`): map authors to teams
  (`author,team`); unmapped authors pass through. Resolve author aliases with a
  repo `.mailmap` first. Feeds the communication network's Conway view.
- `--temporal-period N`: collapse commits into sliding N-day change sets before
  analysis. Intended for coupling, where per-commit granularity is too narrow
  across teams working in days or weeks.

## Analysis period

Scope the git log by date (`--after=` on the log command). Heuristics: one year is
a good default; a month for very high-churn repos; a window around a major event
(reorg, redesign) to measure its impact. Too much history buries recent trends.

## Errors and exit codes

Errors are a JSON envelope on stderr (`{ok: false, error: {code, message,
hint}}`); `--format text` renders `✗ <message>` plus a `hint:` line instead.

| Exit | Meaning               | Examples                                                              |
| ---- | --------------------- | --------------------------------------------------------------------- |
| 0    | success (incl. empty) | any analysis that ran                                                 |
| 2    | usage error           | unknown flag/subcommand, bad value, `messages` without `--expression` |
| 3    | input error           | empty or unparseable log, malformed `--group`/`--team-map`            |
| 1    | internal              | a bug; prints a trace only under `--debug`                            |
