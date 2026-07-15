# codelens

`codelens` is an agent-first Go reimplementation of
[code-maat](https://github.com/adamtornhill/code-maat). It mines a git history
log and runs any of 20 evolutionary code analyses (coupling, hotspots, churn,
ownership, code age, and more), emitting a structured JSON envelope by default.

It is read-only: it never runs git, never writes files, and has no side
effects. You generate a git log yourself and pipe it in; `codelens` analyzes it
and prints results to stdout.

## Why

The original `code-maat` is a capable analysis engine wrapped in a CLI that is
hard for an agent to drive: results and stack traces land on stdout, the input
log format is undocumented at runtime, there is no schema introspection, and a
single global flag no-ops for most analyses. `codelens` keeps the analyses and
their numeric results while fixing the I/O surface:

- JSON envelope by default, with coded errors on stderr only.
- `schema` command so any analysis can be learned entirely at runtime.
- `print-log-command` so no one has to memorize the input format.
- Field projection (`--fields`) and row caps (`--rows`) for context discipline.
- Per-analysis flags, so each command exposes only what affects it.

## Install

Via [Homebrew](https://brew.sh) (both Linux and Mac supported):

```sh
brew tap andreswebs/tap
brew install andreswebs/tap/codelens
```

## Quick start

`codelens` reads a git log on stdin. Do not memorize the format; ask `codelens` for
the exact git command:

```sh
codelens print-log-command
# git log --all --numstat --date=short --pretty=format:'--%h--%ad--%aN--%s' --no-renames
```

Let the shell run that command and pipe its output straight into one analysis
subcommand, so the log format lives only in the tool:

```sh
eval "$(codelens print-log-command)" | codelens coupling
```

Scoping flags forward through the helper to git, so `--after=2024-01-01` and
friends work without hardcoding the log format:

```sh
eval "$(codelens print-log-command --after=2024-01-01)" | codelens coupling
```

The four fields (`%h`, `%ad`, `%aN`, `%s`) plus `--numstat` are what `codelens`
requires. The trailing `%s` (commit subject) is what makes the `messages`
analysis work; stock 3-field logs still parse, with the subject defaulting to
`-`.

Input defaults to stdin. Use `--log FILE` to read a file, or `--log -` to force
stdin explicitly:

```sh
codelens authors --log "${CODELENS_LOG}"
```

## The 20 analyses

Each analysis has a descriptive canonical name and, where `code-maat` used a
terse name, accepts that terse form as an alias. Run `codelens schema` for the
authoritative list.

| Canonical                     | Alias                  | Purpose                                               |
| ----------------------------- | ---------------------- | ----------------------------------------------------- |
| `authors`                     |                        | Number of distinct authors per entity                 |
| `revisions`                   |                        | Change frequency per entity                           |
| `coupling`                    |                        | Logical (temporal) coupling between entity pairs      |
| `sum-of-coupling`             | `soc`                  | Sum of coupling per entity                            |
| `summary`                     |                        | Overview counts for the mined data                    |
| `absolute-churn`              | `abs-churn`            | Lines added/deleted per date                          |
| `author-churn`                |                        | Lines added/deleted per author                        |
| `entity-churn`                |                        | Lines added/deleted per entity                        |
| `entity-ownership`            |                        | Per-author churn contribution to each entity          |
| `entity-effort`               |                        | Each author revision share per entity                 |
| `main-developer`              | `main-dev`             | Main developer per entity by lines added              |
| `main-developer-by-revisions` | `main-dev-by-revs`     | Main developer per entity by revision count           |
| `refactoring-main-developer`  | `refactoring-main-dev` | Main developer per entity by lines removed            |
| `fragmentation`               |                        | Author fragmentation (fractal value) per entity       |
| `communication`               |                        | Heuristic communication strength between author pairs |
| `messages`                    |                        | Entity frequency for commit-message regex matches     |
| `code-age`                    | `age`                  | Age in months since last modification                 |
| `parse`                       | `identity`             | Dump parsed modification records (debug/interop)      |

That is 18 analysis subcommands. `coupling`'s `--verbose` variant and the
`parse` dump round out code-maat's 20 analysis functions.

## Output formats

Select with `--format`:

| Format   | Use                                                                    |
| -------- | ---------------------------------------------------------------------- |
| `json`   | Default. Self-describing envelope with metadata.                       |
| `ndjson` | One row object per line, no envelope. Stream row-heavy results.        |
| `csv`    | code-maat-compatible headers (`kebab-case`) and column order. Interop. |
| `table`  | Aligned columns for a human terminal. Opt in only.                     |

JSON is the default regardless of whether stdout is a terminal. The output is
predictable; nothing changes shape based on a TTY.

### JSON envelope (default)

```json
{
  "schema_version": 1,
  "ok": true,
  "analysis": "authors",
  "row_count": 4,
  "rows": [
    { "entity": "src/code_maat/parsers/git2.clj", "n_authors": 2, "n_revs": 2 }
  ]
}
```

- `row_count` is the number of rows emitted.
- When `--rows N` truncates the result, the envelope also carries
  `total_count` (rows before the cap) and `truncated: true`, so a capped result
  is distinguishable from a complete one.
- An empty-but-valid result is `ok: true`, `row_count: 0`, `rows: []`, exit 0.
- Column keys are `snake_case`. The `parse` command dumps records in log order;
  every other analysis sorts deterministically.

### ndjson

```sh
codelens authors --log "${CODELENS_LOG}" --format ndjson
```

```text
{"entity":"src/code_maat/parsers/git2.clj","n_authors":2,"n_revs":2}
{"entity":"src/code_maat/parsers/git.clj","n_authors":1,"n_revs":2}
```

`ndjson` drops the envelope metadata (`analysis`, `row_count`, `total_count`);
use `json` when you need it.

### csv

```sh
codelens authors --log "${CODELENS_LOG}" --format csv
```

```text
entity,n-authors,n-revs
src/code_maat/parsers/git2.clj,2,2
src/code_maat/parsers/git.clj,1,2
```

### table

```sh
codelens authors --log "${CODELENS_LOG}" --format table
```

```text
entity                          n_authors  n_revs
src/code_maat/parsers/git2.clj  2          2
src/code_maat/parsers/git.clj   1          2
```

### Bounding output (`--fields`, `--rows`)

On a real repository, cap what you read and project only the columns you need:

```sh
codelens authors --log "${CODELENS_LOG}" \
  --fields rows.entity,rows.n_authors --rows 2
```

```json
{
  "ok": true,
  "schema_version": 1,
  "rows": [
    { "entity": "src/code_maat/parsers/git2.clj", "n_authors": 2 },
    { "entity": "src/code_maat/parsers/git.clj", "n_authors": 1 }
  ]
}
```

`--fields` applies to JSON output only and always retains `schema_version` and
`ok`. `--rows` applies to every format and truncates after sorting.

## Schema introspection

Never guess flags or column names. Learn them from `schema`:

```sh
codelens schema                    # list every command, its aliases and exit codes
codelens schema --command coupling # full contract for one command
```

`schema --command CMD` returns the command summary, its `flags`
(`name`, `type`, `default`, `required`, `desc`), its `row_schema`
(`name`, `type`, `desc` per output column), its `error_codes`, and its
`exit_codes`. This is the source of truth for what each analysis accepts and
emits, including columns that appear only with `--verbose`.

## Common flags

Global flags apply to every analysis subcommand; per-command flags are listed
by `schema --command CMD`. The ones you will reach for most:

| Flag                  | Meaning                                                          |
| --------------------- | ---------------------------------------------------------------- |
| `--log FILE`          | Read the log from FILE instead of stdin (`-` forces stdin).      |
| `--format FMT`        | `json` (default), `ndjson`, `csv`, or `table`.                   |
| `--fields PATHS`      | Project JSON fields, e.g. `rows.entity,rows.degree`.             |
| `--rows N`            | Cap output to N rows after sorting (0 = all).                    |
| `--group FILE`        | Map files to architectural layers (`--group-format text\|json`). |
| `--team-map FILE`     | Map authors to teams (`--team-map-format csv\|json`).            |
| `--temporal-period N` | Collapse commits into sliding N-day change sets.                 |
| `--expression REGEX`  | Required by `messages`: regex matched against commit subjects.   |
| `--time-now DATE`     | `code-age` only: `YYYY-MM-DD` "time zero" (default: today, UTC). |

## Errors and exit codes

All diagnostics go to stderr; stdout carries only results, so piping stdout
into a JSON parser is always safe. Errors are a JSON envelope:

```json
{
  "schema_version": 1,
  "ok": false,
  "error": {
    "code": "empty_log",
    "message": "the log is empty",
    "hint": "provide a non-empty git2 log on stdin or via --log"
  }
}
```

With `--format text`, errors render as `✗ <message>` and a `hint:` line on
stderr instead.

| Exit | Meaning               | Examples                                                                  |
| ---- | --------------------- | ------------------------------------------------------------------------- |
| 0    | success (incl. empty) | any analysis that ran                                                     |
| 2    | usage error           | unknown flag or subcommand, bad flag value, `messages` without expression |
| 3    | input error           | empty or unparseable log, malformed `--group`/`--team-map`                |
| 1    | internal / unexpected | a bug; the only path that prints a trace, and only under `--debug`        |

## Visualizing with the `codelens` skill

`codelens` ships an [agent skill](docs/skills/codelens/SKILL.md) that both drives
the CLI and turns its JSON into the crime-scene visualizations from Adam
Tornhill's [_Your Code as a Crime Scene_](https://isbnsearch.org/isbn/9798888650844).
The recurring subject is the hotspot: complex code that changes often.

It renders:

- Hotspot enclosure maps, knowledge/ownership maps, and code-age maps
  (circle-packing, from `revisions`, `main-developer`, `code-age`).
- Change-coupling and team-communication graphs (from `coupling`,
  `sum-of-coupling`, `communication`).
- Churn and complexity trends, fractal effort figures, a commit word cloud, and
  summary tiles.

Static charts render to SVG and PNG for slides and PDF; the enclosure and graph
views render to interactive HTML.

Install it into any supported agent (Claude Code, Codex, Cursor, and others)
with the [Vercel skills CLI](https://github.com/vercel-labs/skills):

```sh
npx skills add andreswebs/codelens
```

That discovers the skill under `docs/skills/codelens` and installs it. To add
only this skill, globally, for Claude Code:

```sh
npx skills add andreswebs/codelens --skill codelens -g -a claude-code
```

Once installed, ask the agent to visualize a repository and it follows the
skill's pipeline (frame the question, collect the log, build the artifact,
render, read the crime scene).

## Documentation

- [docs/skills/codelens/](docs/skills/codelens/) - the skill: operate the CLI
  and visualize its output, including the full analyses catalog and the
  visualization cards.
- [docs/skills/codelens/references/operating.md](docs/skills/codelens/references/operating.md)
  - the canonical CLI operating guide (the fuller reference behind this README).
- [AGENTS.md](AGENTS.md) - repository map and the build, validate, and
  contribution guide.

## Authors

**Andre Silva** - [@andreswebs](https://github.com/andreswebs)

## License

This project is licensed under the [GPL-3.0-or-later](LICENSE), matching
code-maat. It reuses code-maat's test corpus (fixtures, sample logs, and
expected outputs) as its regression oracle.

`codelens` is a port of [code-maat](https://github.com/adamtornhill/code-maat) by
[Adam Tornhill](https://github.com/adamtornhill). The analyses and their
algorithms originate there.
