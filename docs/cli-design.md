# codelens CLI Design

`codelens` is an agent-first Go reimplementation of
[code-maat](https://github.com/adamtornhill/code-maat): it mines a git history
log and runs any of 20 evolutionary analyses (coupling, hotspots, churn,
ownership, code age, ...), emitting structured JSON by default.

This document is the authoritative design. Algorithmic reference for the
analyses and log format lives in [research/code-maat.md](research/code-maat.md).
The design is agent-first: a `{schema_version, ok, ...}` envelope, coded errors,
field projection, and a `schema` introspection command.

## 1. Goals

1. **Faithful analyses.** Reproduce code-maat's 20 analyses and their numeric
   results, pinned by the original's test fixtures.
2. **Agent-first I/O.** Structured, self-describing, predictable - score highly
   on the agent-DX scale (target: agent-ready to agent-first).
3. **Fix the audit findings.** Every UX/agent defect found in code-maat is
   addressed by construction (see [§11](#11-audit-findings--resolutions)).
4. **Predictable, self-describing surface.** One envelope, coded errors, and a
   `schema` introspection command, so an agent can learn the whole tool at
   runtime.

## 2. Non-goals

- No VCS other than git (git2 log format only). The parser is behind an
  interface so more can be added later, but there is no `--vcs` flag now.
- No direct VCS invocation. codelens consumes a log; it does not run `git`
  itself (beyond documenting the command via `print-log-command`).
- No visualization. Output is data; charting is downstream.
- No config file. Everything is flags + stdin.

## 3. Identity

| Property      | Value                              |
| ------------- | ---------------------------------- |
| Binary        | `codelens`                         |
| Module        | `github.com/andreswebs/codelens`   |
| Go            | 1.26                               |
| CLI framework | `urfave/cli/v3`                    |
| License       | **GPL-3.0** (matches the original) |

> License decision: codelens is **GPL-3.0**, matching code-maat. This lets the
> port reuse code-maat's test corpus directly - the `.clj` fixtures, sample
> logs, and expected outputs become the Go port's regression oracle with no
> derivation concern. The repo's placeholder UNLICENSE is replaced with
> GPL-3.0.

## 4. Command surface

```text
codelens <analysis> [flags]      # run one analysis, read log from stdin
codelens schema [--command CMD]  # machine-readable introspection
codelens print-log-command       # emit the git log command to generate input
codelens --version               # print the bare build version
codelens --help / <cmd> --help   # human help
```

### 4.1 Analysis subcommands (20)

One subcommand per analysis. Each has a **descriptive canonical name** and
accepts code-maat's **terse original as an alias** (parity + muscle memory).
Names without an alias below are identical to the original.

| Canonical          | Alias       | Canonical                     | Alias                  |
| ------------------ | ----------- | ----------------------------- | ---------------------- |
| `authors`          | -           | `main-developer`              | `main-dev`             |
| `revisions`        | -           | `refactoring-main-developer`  | `refactoring-main-dev` |
| `coupling`         | -           | `entity-effort`               | -                      |
| `sum-of-coupling`  | `soc`       | `main-developer-by-revisions` | `main-dev-by-revs`     |
| `summary`          | -           | `fragmentation`               | -                      |
| `absolute-churn`   | `abs-churn` | `communication`               | -                      |
| `author-churn`     | -           | `messages`                    | -                      |
| `entity-churn`     | -           | `code-age`                    | `age`                  |
| `entity-ownership` | -           | `parse`                       | `identity`             |

(That is 18 analysis commands; `coupling`'s `--verbose` variant and the `parse`
dump round out code-maat's 20 analysis functions. `parse` renames `identity`.
The full algorithmic mapping is in the reference doc §6, which uses the terse
names to describe the original.)

Each subcommand exposes **only the flags that affect it**. This is the core fix
for code-maat's "global flag that no-ops for 19 of 20 analyses" problem.

### 4.2 Global flags (all analysis subcommands)

| Flag                    | Default | Meaning                                         |
| ----------------------- | ------- | ----------------------------------------------- |
| `--log FILE`            | stdin   | Read log from FILE; `--log -` is explicit stdin |
| `--input-encoding ENC`  | UTF-8   | Non-UTF-8 log encoding                          |
| `--fields PATHS`        | (all)   | Comma-separated JSON field projection           |
| `--rows N`              | (all)   | Cap output rows after sorting                   |
| `--include GLOB`        | -       | Keep only entities matching GLOB (repeatable)   |
| `--exclude GLOB`        | -       | Drop entities matching GLOB (repeatable)        |
| `--group FILE`          | -       | Layer-mapping file                              |
| `--group-format FMT`    | `text`  | `text` (`=>` lines) or `json`                   |
| `--team-map FILE`       | -       | author→team map                                 |
| `--team-map-format FMT` | `csv`   | `csv` or `json`                                 |
| `--temporal-period N`   | -       | Collapse commits into sliding N-day change sets |

Format for `--group`/`--team-map` is chosen by an **explicit** `*-format` flag
(no content sniffing or extension guessing), defaulting to the text/CSV form.

`--include`/`--exclude` are repeatable gitignore-style path globs (`**`
supported, matched against the full entity path via
`github.com/bmatcuk/doublestar/v4`). They are a pipeline transform that runs
**first**, before grouping, so globs match raw file paths (`**/Migrations/**`),
not the layer names grouping produces. Precedence is exclude-after-include: with
any `--include`, an entity must match at least one include to survive, then any
`--exclude` drops it; with no includes, all are included and only excludes apply.
A malformed glob is a usage error (exit 64). `*`/`?` do not cross `/`; use `**` to
span directories. The pipeline order is `filter -> group -> temporal -> team-map`.

### 4.3 Per-analysis flags

| Subcommand(s)                 | Flag                   | Default    | Meaning                               |
| ----------------------------- | ---------------------- | ---------- | ------------------------------------- |
| `coupling`, `sum-of-coupling` | `--min-revs`           | 5          | Min revisions to include an entity    |
| `coupling`                    | `--min-shared-revs`    | 5          | Min shared revisions for a pair       |
| `coupling`                    | `--min-coupling`       | 30         | Min coupling degree (%)               |
| `coupling`                    | `--max-coupling`       | 100        | Max coupling degree (%)               |
| `coupling`                    | `--max-changeset-size` | 30         | Skip change sets larger than this     |
| `coupling`                    | `--verbose`            | off        | Add per-pair revision detail columns  |
| `revisions`, `authors`, ...   | `--min-revs`           | 5          | Where the analysis filters by revs    |
| `code-age`                    | `--time-now DATE`      | today      | `YYYY-MM-dd` "time zero"              |
| `messages`                    | `--expression REGEX`   | (required) | Regex matched against commit messages |

Canonical names are used throughout; each also accepts its terse alias (§4.1).
Whether `--min-revs` applies to a given analysis follows the original (see
reference doc §6); the schema output is the source of truth per command.

## 5. Input

- **Default: stdin.** The canonical workflow is a pipe:

  ```sh
  git log --numstat --date=short \
    --pretty=format:'--%h--%ad--%aN--%s' --no-renames --use-mailmap --after=2024-01-01 \
    | codelens coupling
  ```

- `--log FILE` reads a file; `--log -` forces stdin.
- The log format is the git2 format **extended with the commit subject** (`%s`),
  so the `messages` analysis works on the single supported format. The 3-field
  stock git2 log is still accepted (message defaults to `-`). See reference doc
  §3.
- The default reads the current branch's history and applies `.mailmap`
  (`--use-mailmap`, collapsing author aliases). `print-log-command --all` opts
  into all-refs history when cross-branch coverage is wanted.
- `codelens print-log-command` prints exactly the command above (minus the
  illustrative `--after`), so neither a human nor an agent has to memorize the
  format - this is the single biggest friction-killer versus the original.

### 5.1 Input safety

- **Bounded regexes.** `--expression` and grouping patterns are compiled with a
  size/complexity guard; an invalid or oversized pattern is a usage error (exit
  64), never an unbounded match or hang. `--include`/`--exclude` globs are
  length-bounded and validated the same way, and doublestar's matcher is
  backtracking-safe.
- **Control characters.** Log/definition content containing disallowed control
  characters (e.g. NUL) is a data error (exit 65).
- **Read-only.** All input files are opened read-only; results go only to
  stdout. There is no write surface to sandbox.
- **Time.** `code-age` computes whole calendar months in **UTC**; `--time-now`
  (else the current UTC date) is "time zero", making age reproducible in tests.

## 6. Output

### 6.1 Canonical output: one self-describing, shape-aware JSON envelope

codelens emits exactly one thing on stdout: a single JSON envelope, identical
regardless of TTY. There is no `--format` flag and no alternate serialization;
JSON is the one representation (see ADR 0003). Shape:

```json
{
  "schema_version": 1,
  "ok": true,
  "analysis": "coupling",
  "shape": "table",
  "semantics": {
    "entity": "filepath",
    "coupled": "filepath",
    "degree": "percentage",
    "average_revs": "count"
  },
  "params": { "min_coupling": 30, "min_shared_revs": 5, "...": "..." },
  "row_count": 2,
  "rows": [
    {
      "entity": "InfoUtils.java",
      "coupled": "Page.java",
      "degree": 78,
      "average_revs": 44
    },
    {
      "entity": "InfoUtils.java",
      "coupled": "BarChart.java",
      "degree": 62,
      "average_revs": 45
    }
  ]
}
```

- `params` echoes the effective tuning options (defaults included) so a result
  is self-documenting and reproducible.
- Column keys are `snake_case` JSON.
- `shape` and `semantics` are described in §6.2.
- Empty-but-valid result → `ok: true`, `row_count: 0`, `rows: []`, exit `0`.
- `row_count` is the number of rows emitted. When `--rows` truncates, the
  envelope also carries `total_count` (rows before the cap) and
  `truncated: true`, so an agent can distinguish a complete result from a capped
  one. Absent truncation, `truncated` is `false`/omitted and `total_count`
  equals `row_count`.

### 6.2 Shape and semantics

Every envelope is self-describing along two axes:

- `shape` is one of `table` | `tree` | `graph` | `matrix` | `series`, and the
  payload key follows from it (`rows` for `table`; `nodes` and `edges` for
  `graph`; and so on). Today every analysis is `shape: "table"` with a `rows`
  payload; the other shapes are introduced with the analyses that need them
  (hierarchy, graph, matrix, and time-series outputs).
- `semantics` maps each field to a semantic type (`filepath`, `percentage`,
  `count`, `duration`, `person`, ...). This is what codelens knows because it
  authored the data, and it is what lets a downstream renderer derive a chart
  without domain knowledge. The tabular payload is deliberately kept close to a
  chart-spec input (a values array plus its semantics), so a downstream adapter
  is near-identity.

The `parse` analysis emits the modification records in **log order** (as
parsed), a passthrough dump rather than a sorted analysis.

### 6.3 Viz-spec encodings are downstream

Chart-language encodings (for example Flint `ChartAssemblyInput`, Vega-Lite,
GraphML/DOT, CodeCharta `.cc.json`) are produced by consumers of the canonical
envelope, never by the core binary. `shape` and `semantics` exist so those
transforms are mechanical. codelens is the data plane; rendering is downstream.

### 6.4 Field projection (`--fields`)

Comma-separated JSON paths, validated against the envelope; invalid paths error
with the valid set listed. `schema_version` and `ok` are always retained.
Example: `--fields rows.entity,rows.degree`.

`--rows N` caps rows after sorting, before the envelope is built.

## 7. Errors and exit codes

### 7.1 Error envelope (stderr)

All diagnostics go to **stderr** (stdout carries only the canonical envelope).
JSON error shape:

```json
{
  "schema_version": 1,
  "ok": false,
  "error": {
    "code": "parse_error",
    "message": "git log entry 4: expected numstat, got \"foo\"",
    "hint": "generate the log with `codelens print-log-command`",
    "details": { "entry": 4, "line": "foo" }
  }
}
```

Errors are **always** emitted as this JSON envelope on stderr, independent of the
results on stdout. There is no human-facing text error path; every caller reads
the envelope's `message` and `hint` fields directly.

### 7.1a Warning diagnostics (stderr)

Non-fatal advisories are emitted as a single-line JSON diagnostic on stderr,
distinct from the error envelope by a `level` field (and the absence of `ok`):

```json
{
  "schema_version": 1,
  "level": "warning",
  "code": "empty_result_at_thresholds",
  "message": "grouped coupling returned no rows at the default thresholds",
  "hint": "lower --min-coupling or --min-shared-revs",
  "details": { "min_coupling": 30, "min_shared_revs": 5 }
}
```

One diagnostic is emitted per line, so multiple warnings form a valid NDJSON
stream on stderr. A warning never changes the exit code and never appears on
stdout; `hint` and `details` are omitted when empty.

### 7.2 Exit codes

codelens follows the taxonomy in
[ADR 0002](adr/0002-exit-code-taxonomy.md) (BSD `sysexits.h` range):

| Code | Code string(s)                                                                                                                                                                                                                                                       | Meaning                             | Examples                                                                                                                |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| 0    | -                                                                                                                                                                                                                                                                    | success (incl. empty result)        | any analysis that ran                                                                                                   |
| 64   | `unknown_command`, `unknown_flag`, `unknown_format`, `unknown_schema_command`, `invalid_value`, `missing_required_flag`, `invalid_field`, `invalid_glob`, `invalid_group`, `invalid_temporal_period`, `invalid_expression`, `invalid_time_now`, `invalid_after_date` | usage error (EX_USAGE)              | unknown flag/subcommand, missing/invalid flag value, `messages` without `--expression`, malformed glob/group            |
| 65   | `parse_error`, `invalid_control_char`, `empty_log`, `missing_messages`, `missing_metrics`, `invalid_temporal_date`, `invalid_team_map`                                                                                                                               | data error (EX_DATAERR)             | unparseable or empty log, disallowed control character, malformed `--team-map`, churn analysis on a log with no numstat |
| 70   | `internal_error`                                                                                                                                                                                                                                                     | internal / unexpected (EX_SOFTWARE) | a bug; an unexpected internal fault, reported as a one-line coded error like any other                                  |
| 74   | `log_open_failed`, `input_file_open_failed`                                                                                                                                                                                                                          | I/O error (EX_IOERR)                | unreadable `--log`, `--group`, or `--team-map` file                                                                     |

Coded errors carry a stable string `code` and their own exit code via a
dedicated `terr`-style package. Usage errors from the CLI framework are
classified from its messages; all resolve to exit 64.

## 8. Schema introspection

- `codelens schema` - lists all commands with one-line descriptions and their
  exit-code sets, plus an `errors` inventory: every error the binary can emit as
  `{code, exit_code, hint}`, derived from the coded-error registry (so it cannot
  drift from the sentinels that exist) and sorted by code.
- `codelens schema --command CMD` - full, self-describing contract for `CMD`:

```json
{
  "schema_version": 1,
  "ok": true,
  "command": "coupling",
  "summary": "Logical (temporal) coupling between entity pairs",
  "shape": "table",
  "flags": [
    {
      "name": "min-coupling",
      "type": "int",
      "default": 30,
      "required": false,
      "desc": "minimum coupling degree in percent"
    }
  ],
  "row_schema": [
    {
      "name": "entity",
      "type": "string",
      "semantic": "filepath",
      "desc": "module path"
    },
    {
      "name": "coupled",
      "type": "string",
      "semantic": "filepath",
      "desc": "co-changing module path"
    },
    {
      "name": "degree",
      "type": "int",
      "semantic": "percentage",
      "desc": "coupling strength, percent 0-100"
    },
    {
      "name": "average_revs",
      "type": "int",
      "semantic": "count",
      "desc": "avg revisions of the pair (ceil)"
    }
  ],
  "error_codes": ["empty_log"],
  "common_error_codes": [
    "input_file_open_failed",
    "internal_error",
    "invalid_control_char",
    "invalid_field",
    "invalid_glob",
    "invalid_group",
    "invalid_team_map",
    "invalid_temporal_date",
    "invalid_temporal_period",
    "invalid_value",
    "log_open_failed",
    "missing_required_flag",
    "parse_error",
    "unknown_flag",
    "unknown_format"
  ],
  "exit_codes": [0, 64, 65, 70, 74]
}
```

`flags` and the envelope come from reflecting the registered command + envelope
struct. `row_schema` is the self-describing piece: a per-analysis table of
`{name, type, semantic, desc}` so both column meanings and their semantic types
are machine-readable, closing the "columns documented only in prose" gap and
giving downstream renderers the semantics they need. `shape` declares the payload
topology. The `schema` command is what lets an agent learn a command entirely at
runtime.

`error_codes` and `common_error_codes` split the error surface in two.
`error_codes` carries only the codes distinctive to this command (for example
`messages` adds `invalid_expression` and `missing_messages`), while
`common_error_codes` is the tool-level baseline any invocation can produce: the
input, global-option, and output-layer failures, declared once and reported the
same on every command (analyses and helpers alike). An agent enumerates a
command's full reachable error surface as `error_codes + common_error_codes`.
The log-input code `empty_log` stays in `error_codes`; the baseline excludes it
to avoid double-reporting.

`CMD` covers the meta commands too: `schema --command schema|print-log-command`
returns each helper's contract (flags, error/exit codes) so nothing the binary
exposes is off the introspection path. Helpers take no log and emit no rows, so
their `aliases` and `row_schema` are empty. The build version is the `--version`
flag (bare output), not a subcommand, so it is not on the `schema` path.

## 9. Architecture

```text
cmd/codelens/            one-line main; hands os.Args[1:] + streams to command.Run
internal/
  command/               CLI delegate behind the Deps seam (ADR 0004):
                         run.go (framework-free contract), root/commands/
                         schema/printlogcommand (framework interior), usage.go
                         (usage-error classifier), errors.go (CLI-surface
                         sentinels), registry.go (ExitCodes)
  version/               (exists) build version
  terr/                  coded errors (code, message, hint, exit)
  output/                envelope build, JSON emit, fields, registry, schema
  gitlog/                git2(+subject) tokenizer + parser -> []Modification
  model/                 Modification, ChangeSet types
  pipeline/              parse -> filter -> group -> temporal -> team-map orchestration
  transform/
    filter/              path include/exclude globs (doublestar), runs first
    group/               layer mapping (text `=>` + JSON), anchoring rules
    temporal/            sliding-window day grouping
    teammap/             author->team (CSV + JSON)
  analysis/              one file per analysis; each: Run([]Modification, Opts) (Envelope, error)
    registry.go          name -> analysis descriptor {run, flags, row_schema, summary}
```

Design points:

- **Analysis registry.** Each analysis registers a descriptor (run fn, flag set,
  row schema, summary, error codes). The command tree, `schema`, and help are all
  generated from this registry - no drift, one place to add an analysis.
- **No Incanter.** The original leans on Incanter datasets; the port uses plain
  Go slices/maps and small helpers (group-by, distinct, order-by). Rounding
  helpers reproduce `ratio->centi-float-precision`, `ceil`, `int` truncation
  exactly (reference doc §7).
- **Streaming parse.** Tokenize the log by blank-line-separated entries and
  parse incrementally to bound memory; analyses that need the full set collect
  into memory (as the original does, but far cheaper in Go).
- **Determinism.** Sort orders match the original per analysis so `--rows`
  truncation is stable and fixtures match.

## 10. Agent knowledge packaging

- `AGENTS.md` at repo root: how to invoke, the stdin-pipe workflow, "always
  bound output with `--fields`/`--rows`", "learn a command with `codelens schema
--command CMD`", exit-code table, the `print-log-command` helper.
- A codelens skill file (YAML frontmatter + Markdown) encoding the same
  invariants for skill-aware agents.
- These move the agent-DX "knowledge packaging" axis off zero and make the tool
  usable with zero prompt stuffing.

## 11. Audit findings → resolutions

| code-maat finding                 | codelens resolution                                                          |
| --------------------------------- | ---------------------------------------------------------------------------- |
| Errors/traces on stdout           | All diagnostics on stderr; stdout is results only                            |
| Stack traces leaked to users      | No traces ever; every failure is a one-line coded error (silent posture)     |
| No `--version`                    | `codelens --version` prints the bare build version                           |
| Opaque `is it a valid logfile?`   | Named `parse_error` with entry/line `details` + hint to `print-log-command`  |
| Input contract undiscoverable     | `codelens print-log-command` emits the exact git command                     |
| `--verbose-results` no-ops 19/20  | `--verbose` lives only on `coupling`                                         |
| CSV-only, prose-documented schema | Single JSON representation + `schema --command` with per-column `row_schema` |
| No introspection                  | `schema` command, self-describing                                            |
| Unsandboxed `-o` writes           | `--outfile` dropped; stdout + shell redirection                              |
| No agent knowledge                | `AGENTS.md` + skill file                                                     |
| Bespoke hand-generated input      | stdin pipe workflow + `print-log-command`                                    |
| Dead `:else` in main              | N/A (clean Go control flow)                                                  |

### Agent-DX scale: target

| Axis                        | code-maat | codelens target                                                             |
| --------------------------- | --------- | --------------------------------------------------------------------------- |
| 1 Machine-readable output   | 1         | 3 (single self-describing JSON envelope, structured errors)                 |
| 2 Raw payload input         | 0         | 1-2 (JSON `--group`/`--team-map`; input is a log, not an API payload)       |
| 3 Schema introspection      | 0         | 3 (full self-describing `schema`)                                           |
| 4 Context-window discipline | 1         | 3 (`--fields`, `--rows`, guidance in skill)                                 |
| 5 Input hardening           | 0         | 2 (read-only inputs, no write surface, validated enums/regex bounds)        |
| 6 Safety rails              | 0         | 1-2 (read-only tool; `schema` as pre-flight; no destructive ops to dry-run) |
| 7 Agent knowledge packaging | 0         | 2-3 (AGENTS.md + skill)                                                     |

Target total ≈ 15-18 (agent-ready to agent-first). Axes 2 and 6 are inherently
capped for a read-only local log analyzer with no API payload or mutations.

## 12. Resolved decisions

1. **License: GPL-3.0.** Matches the original; the port reuses code-maat's test
   corpus (fixtures, sample logs, expected outputs) directly as its regression
   oracle. See [§3](#3-identity).
2. **`messages` kept via the `%s` extension.** `print-log-command` emits the
   git2 format plus the commit subject so all 20 analyses run on one format;
   stock 3-field logs still parse. See [§5](#5-input) and reference doc §3.2.
3. **One JSON representation.** Output is a single self-describing, shape-aware
   JSON envelope; there is no `--format` flag and no CSV, ndjson, or human table
   output. See [ADR 0003](adr/0003-canonical-output-representation.md). Numeric
   results are still pinned against the original's expected outputs; only the
   serialization differs.

### Minor items for the implementation plan

- `parse` output detail: include `loc-added`/`loc-deleted` only when present in
  the source records.
