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

The default reads the checked-out branch's history, avoiding commits from
unmerged branches or dated after `HEAD`. Pass
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
codelens schema                     # every command, aliases, exit codes, error inventory
codelens schema --command coupling  # summary, flags, row_schema, error_codes, common_error_codes, exit_codes
```

`schema` (no `--command`) also carries an `errors` inventory: every error the
binary can emit as `{code, exit_code, hint}`, sorted by code, so an agent can
map any code it sees to its exit code and remedy without a second lookup.

`schema --command` reports the error surface as two lists. `error_codes` is only
the codes distinctive to that command; `common_error_codes` is the tool-level
baseline any invocation can produce (input, global-option, and output-layer
failures), reported identically on every command. Enumerate a command's full
error surface as `error_codes + common_error_codes`.

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

## Output and shaping

codelens emits exactly one thing on stdout: a single self-describing JSON
envelope, identical whether or not stdout is a terminal. There is no `--format`
flag and no alternate serialization. The envelope carries `schema_version, ok,
analysis, shape, semantics`, optionally `transforms` and `params`, then
`row_count` and the payload (`rows` for a table). `--rows N` truncation adds
`total_count` and `truncated: true`. An empty-but-valid result is `ok: true,
row_count: 0, rows: []`, exit 0. Column keys are snake_case.

Analyses that declare flags also carry a `params` object (after `transforms`)
echoing every declared flag at its effective value, defaults included, so a
run is self-documenting: `coupling`, `sum-of-coupling`, `code-age`, and
`messages`. Flagless analyses omit `params`.

`shape` is a fixed, per-command member of the closed set `table`, `text`. Every
analysis is `shape: "table"`; the sole exception is `print-log-command`, which is
`shape: "text"` and emits a bare command line by design, so it stays
copy-pasteable. The payload key follows from the shape (`rows` for a table). The
set names only the shapes codelens emits, so a shape read from `schema` can
always be relied on; new shapes appear as the analyses that need them ship.

`semantics` maps each emitted column to a member of a closed 12-entry
vocabulary. It exists so a renderer or a downstream chart spec can be derived
without domain knowledge, because codelens authored the data and knows what each
column means:

| Semantic          | Meaning                                                     |
| ----------------- | ----------------------------------------------------------- |
| `filepath`        | A repository path, splittable on `/`                        |
| `person`          | An actor name (an individual or, under `--team-map`, a team) |
| `date`            | Calendar date, `YYYY-MM-dd`                                 |
| `commit_id`       | Opaque commit identifier                                    |
| `text`            | Free prose, never a plottable category                      |
| `label`           | Categorical name                                            |
| `flag`            | Boolean                                                     |
| `count`           | Tally of things (a frequency measure)                       |
| `loc`             | Line count (a size measure)                                 |
| `percentage`      | Integer 0-100                                               |
| `ratio`           | Float 0-1                                                   |
| `duration_months` | Whole calendar months                                       |

`percentage` is 0-100 while `ratio` is 0-1, and `loc` is distinct from `count`
so a renderer can pick a size channel over a frequency channel.

`transforms` appears only when a pipeline transform ran (it is omitted for a
pass-through run) and records which of `include`, `exclude`, `group`,
`temporal_period`, and `team_map` were active. It also justifies an adjusted
semantic: under `--group`, `entity` is reported as `label`, not `filepath`,
because a layer name is not a splittable path; `--team-map` leaves `author` as
`person`, since a team name and a person name are both opaque actor labels.

`schema --command CMD` returns `shape` and a per-column `semantic`, so both a
column's meaning and its semantic type are discoverable at runtime. The map
tracks flags, not data: `coupling` declares four semantics without `--verbose`
and seven with it.

- Bound large output: `--rows N` (after sorting) and
  `--fields rows.entity,rows.n_revs`. Under `--fields`, `schema_version`, `ok`,
  and `shape` are always retained, `transforms` is retained when present, and
  `semantics` is narrowed to the projected fields.
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
  layer names. A malformed glob is a usage error (exit 64). Note `*` and `?` do not
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
  across teams working in days or weeks. On any other analysis with count or
  line-count columns, codelens warns on stderr (`temporal_period_recounts`)
  that those columns tally overlapping windows rather than commits, naming the
  affected columns in `details`.

## Calendar rollups (downstream)

Calendar bucketing is deliberately not an engine feature. The transforms above
rewrite the input before analysis runs, which nothing downstream could do;
rolling output rows up to months is a presentation step, so it stays in jq. Two
recipes cover the temporal readings in
[interpretation.md](interpretation.md); both bucket on `date[:7]`, the `YYYY-MM`
prefix of the `date` semantic.

Momentum (monthly commit and churn totals, from `absolute-churn`):

```sh
codelens absolute-churn --log git.log | jq '
  .rows
  | group_by(.date[:7])
  | map({month: .[0].date[:7], commits: (map(.commits) | add),
         added: (map(.added) | add), deleted: (map(.deleted) | add)})'
```

Crisis cadence (distinct crisis commits per month, from `parse`):

```sh
codelens parse --log git.log | jq '
  [.rows[] | select(.message | test("revert|hotfix|emergency|rollback"; "i"))]
  | group_by(.date[:7])
  | map({month: .[0].date[:7], commits: ([.[].rev] | unique | length)})'
```

The source here must be `parse`, not `messages`: `parse` rows are one per
entity-record and carry `rev`, `date`, and `message`, so distinct commits are
recoverable (`unique` on `rev`); the `messages` analysis has already collapsed
to per-entity match counts, which multi-count commits that touch several files
and carry no dates. `parse` also emits the full message-bearing record stream,
so the same shape adapts to any message-regex-over-time question.

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
message, hint}}`). There is no text error path, so parse the envelope's
`message` and `hint` fields directly.

Exit codes follow the taxonomy in ADR 0002 (BSD `sysexits.h`):

| Exit | Meaning               | Examples                                                                                    |
| ---- | --------------------- | ------------------------------------------------------------------------------------------- |
| 0    | success (incl. empty) | any analysis that ran                                                                       |
| 64   | usage error           | unknown flag/subcommand, bad value, `messages` without `--expression`, malformed glob/group |
| 65   | data error            | empty or unparseable log, malformed `--team-map`, churn on a log with no numstat            |
| 70   | internal              | a bug; an unexpected internal fault, reported as a one-line coded error                     |
| 74   | I/O error             | unreadable `--log`, `--group`, or `--team-map` file                                         |

Non-fatal advisories are emitted as single-line JSON **warning** diagnostics on
stderr, distinguished from errors by `level: "warning"` (and no `ok` field):
`{schema_version, level: "warning", code, message, hint?, details?}`. One per
line (valid NDJSON), they never change the exit code and never touch stdout, so a
consumer reading results from stdout is unaffected.

The skill's Python render scripts follow the same convention: they print their
`wrote ...` summary (and uv's `Installed N packages`) to stderr on success, so a
wrapper must judge them by exit code, never by stderr being empty. See
[reporting.md](reporting.md).

### Inspecting stderr alone: a zsh gotcha, not a codelens bug

To read only the diagnostics, the usual idiom discards stdout while keeping stderr
on the pipe:

```sh
codelens coupling --log git.log 2>&1 >/dev/null | jq .
```

**In zsh that idiom is broken and makes codelens look like it violates the stream
contract.** zsh's `MULTIOS` option (on by default) treats a second redirection of
the same descriptor as a request to write to BOTH destinations, so stdout is teed
to `/dev/null` AND to the pipe. The results envelope then appears alongside the
warning, and the output reads as though the analysis result were emitted on stderr.
It was not: the tee happened in the shell, after codelens had already written each
stream correctly.

The tell is a second line whose payload is a full result envelope
(`"ok":true,"analysis":...`) rather than a diagnostic (`"level":"warning"`). Before
reporting a stream-separation bug, re-run one of these:

```sh
# portable and unambiguous: separate files, then inspect each
codelens coupling --log git.log >out.json 2>err.txt

# zsh, disabling the tee for this shell
unsetopt multios
codelens coupling --log git.log 2>&1 >/dev/null | jq .

# or run the pipeline under bash, where the idiom behaves as written
bash -c 'codelens coupling --log git.log 2>&1 >/dev/null' | jq .
```

Verified on zsh 5.9.2: the idiom yields 2 lines on the pipe, while all three
alternatives yield 1. Note `1>/dev/null 2>&1` is teed the same way, so reordering
the redirections is not a fix. Separate files are the safest default in a script
that must work under either shell.
