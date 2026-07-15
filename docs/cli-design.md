# codelens CLI Design

`codelens` is an agent-first Go reimplementation of
[code-maat](https://github.com/adamtornhill/code-maat): it mines a git history
log and runs any of 20 evolutionary analyses (coupling, hotspots, churn,
ownership, code age, ...), emitting structured JSON by default.

This document is the authoritative design. Algorithmic reference for the
analyses and log format lives in [research/code-maat.md](research/code-maat.md).
The design deliberately fits the house style established by the sibling
`terminology` CLI: a `{schema_version, ok, ...}` envelope, coded errors, field
projection, and a `schema` introspection command.

## 1. Goals

1. **Faithful analyses.** Reproduce code-maat's 20 analyses and their numeric
   results, pinned by the original's test fixtures.
2. **Agent-first I/O.** Structured, self-describing, predictable - score highly
   on the agent-DX scale (target: agent-ready to agent-first).
3. **Fix the audit findings.** Every UX/agent defect found in code-maat is
   addressed by construction (see [§11](#11-audit-findings--resolutions)).
4. **House-style consistency.** Reuse `terminology`'s envelope/error/schema
   conventions so an agent that knows one knows the other.

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

| Flag                    | Default | Meaning                                           |
| ----------------------- | ------- | ------------------------------------------------- |
| `--log FILE`            | stdin   | Read log from FILE; `--log -` is explicit stdin   |
| `--input-encoding ENC`  | UTF-8   | Non-UTF-8 log encoding                            |
| `--format FMT`          | `json`  | `json` \| `ndjson` \| `csv` \| `table`            |
| `--fields PATHS`        | (all)   | Comma-separated JSON field projection             |
| `--rows N`              | (all)   | Cap output rows after sorting                     |
| `--include GLOB`        | -       | Keep only entities matching GLOB (repeatable)     |
| `--exclude GLOB`        | -       | Drop entities matching GLOB (repeatable)          |
| `--group FILE`          | -       | Layer-mapping file                                |
| `--group-format FMT`    | `text`  | `text` (`=>` lines) or `json`                     |
| `--team-map FILE`       | -       | author→team map                                   |
| `--team-map-format FMT` | `csv`   | `csv` or `json`                                   |
| `--temporal-period N`   | -       | Collapse commits into sliding N-day change sets   |
| `--debug`               | off     | Emit stack traces / verbose diagnostics to stderr |

Format for `--group`/`--team-map` is chosen by an **explicit** `*-format` flag
(no content sniffing or extension guessing), defaulting to the text/CSV form.

`--include`/`--exclude` are repeatable gitignore-style path globs (`**`
supported, matched against the full entity path via
`github.com/bmatcuk/doublestar/v4`). They are a pipeline transform that runs
**first**, before grouping, so globs match raw file paths (`**/Migrations/**`),
not the layer names grouping produces. Precedence is exclude-after-include: with
any `--include`, an entity must match at least one include to survive, then any
`--exclude` drops it; with no includes, all are included and only excludes apply.
A malformed glob is a usage error (exit 2). `*`/`?` do not cross `/`; use `**` to
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
  2), never an unbounded match or hang. `--include`/`--exclude` globs are
  length-bounded and validated the same way, and doublestar's matcher is
  backtracking-safe.
- **Control characters.** Log/definition content containing disallowed control
  characters (e.g. NUL) is an input error (exit 3).
- **Read-only.** All input files are opened read-only; results go only to
  stdout. There is no write surface to sandbox.
- **Time.** `code-age` computes whole calendar months in **UTC**; `--time-now`
  (else the current UTC date) is "time zero", making age reproducible in tests.

## 6. Output

### 6.1 Default: JSON envelope (all contexts)

Predictable regardless of TTY (matching `terminology`). Shape:

```json
{
  "schema_version": 1,
  "ok": true,
  "analysis": "coupling",
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
- Column keys are `snake_case` JSON (original used `kebab-case` CSV headers;
  `--format csv` restores the original headers for parity - see §6.4).
- Empty-but-valid result → `ok: true`, `row_count: 0`, `rows: []`, exit `0`.
- `row_count` is the number of rows emitted. When `--rows` truncates, the
  envelope also carries `total_count` (rows before the cap) and
  `truncated: true`, so an agent can distinguish a complete result from a capped
  one. Absent truncation, `truncated` is `false`/omitted and `total_count`
  equals `row_count`.

### 6.2 `ndjson`

One row object per line (no envelope wrapper), preceded by no header. Applied
**uniformly** to every analysis - including scalar-shaped ones like `summary`
(one `{statistic, value}` object per line). Envelope metadata (`analysis`,
`params`, `total_count`) is absent in this mode; use `json` when that metadata
is needed. Intended for row-heavy analyses (`revisions`, `coupling`,
`entity-effort`) on large repos so an agent can stream and bound its context.

### 6.3 `table`

Human-readable aligned columns for terminal use. Never the default (predictable

> pretty for agents); opt in with `--format table`.

### 6.4 `csv`

Byte-compatible with the original's CSV where feasible: original `kebab-case`
headers, same column order, same row ordering. This is the parity/interop path
(spreadsheets, existing scripts). The `parse` analysis emits the modification
records in **log order** (as parsed), across all formats - it is a passthrough
dump, not a sorted analysis.

### 6.5 Field projection (`--fields`)

Comma-separated JSON paths, validated against the envelope; invalid paths error
with the valid set listed (reusing terminology's `ValidateFields`/`ProjectFields`
approach). `schema_version` and `ok` are always retained. Example:
`--fields rows.entity,rows.degree`.

`--fields` applies to JSON output only; it is ignored for `ndjson`/`csv`/`table`.
`--rows N` applies to **every** format (truncation happens after sorting,
before formatting).

## 7. Errors and exit codes

### 7.1 Error envelope (stderr)

All diagnostics go to **stderr** (stdout carries only results). JSON error shape
mirrors `terminology`:

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

Errors are **always** emitted as this JSON envelope on stderr, for every
`--format` value including `text` and `table`: `--format` governs the results on
stdout, not diagnostics on stderr. There is no human-facing `✗ <message>` text
error path; a `text`/`table` caller reads the envelope's `message` and `hint`
fields directly.

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

| Code | Meaning                      | Examples                                                                                            |
| ---- | ---------------------------- | --------------------------------------------------------------------------------------------------- |
| 0    | success (incl. empty result) | any analysis that ran                                                                               |
| 2    | usage error                  | unknown flag/subcommand, missing/invalid flag value, `messages` without `--expression`              |
| 3    | input error                  | unparseable or empty log, malformed `--group`/`--team-map`, churn analysis on a log with no numstat |
| 1    | internal / unexpected        | bug; only path that prints a trace (with `--debug`)                                                 |

Coded errors carry a stable string `code` and their own exit code via a `terr`-
style package (ported from terminology's `internal/terr`). Usage errors are
classified from the CLI framework's messages, as terminology does.

## 8. Schema introspection

- `codelens schema` - lists all commands with one-line descriptions and their
  exit-code sets.
- `codelens schema --command CMD` - full, self-describing contract for `CMD`:

```json
{
  "schema_version": 1,
  "ok": true,
  "command": "coupling",
  "summary": "Logical (temporal) coupling between entity pairs",
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
    { "name": "entity", "type": "string", "desc": "module path" },
    { "name": "coupled", "type": "string", "desc": "co-changing module path" },
    {
      "name": "degree",
      "type": "int",
      "desc": "coupling strength, percent 0-100"
    },
    {
      "name": "average_revs",
      "type": "int",
      "desc": "avg revisions of the pair (ceil)"
    }
  ],
  "error_codes": ["parse_error", "empty_log"],
  "exit_codes": [0, 2, 3, 1]
}
```

`flags` and the envelope come from reflecting the registered command + envelope
struct (terminology's `registry` pattern). `row_schema` descriptions are the new
piece: a small per-analysis table of `{name, type, desc}` so column meanings are
machine-readable, closing the "columns documented only in prose" gap. The
`schema` command is what lets an agent learn a command entirely at runtime.

`CMD` covers the meta commands too: `schema --command schema|print-log-command`
returns each helper's contract (flags, error/exit codes) so nothing the binary
exposes is off the introspection path. Helpers take no log and emit no rows, so
their `aliases` and `row_schema` are empty. The build version is the `--version`
flag (bare output), not a subcommand, so it is not on the `schema` path.

## 9. Architecture

```text
cmd/codelens/            main; wires urfave/cli commands
internal/
  version/               (exists) build version
  terr/                  coded errors (code, message, hint, exit) - port from terminology
  output/                envelope build, EmitJSON/NDJSON/CSV/Table, fields, registry, schema
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
- A codelens skill file (YAML frontmatter + Markdown) mirroring the house skill
  format, encoding the same invariants for skill-aware agents.
- These move the agent-DX "knowledge packaging" axis off zero and make the tool
  usable with zero prompt stuffing.

## 11. Audit findings → resolutions

| code-maat finding                 | codelens resolution                                                         |
| --------------------------------- | --------------------------------------------------------------------------- |
| Errors/traces on stdout           | All diagnostics on stderr; stdout is results only                           |
| Stack traces leaked to users      | Traces only under `--debug`; otherwise one-line coded error                 |
| No `--version`                    | `codelens --version` prints the bare build version                          |
| Opaque `is it a valid logfile?`   | Named `parse_error` with entry/line `details` + hint to `print-log-command` |
| Input contract undiscoverable     | `codelens print-log-command` emits the exact git command                    |
| `--verbose-results` no-ops 19/20  | `--verbose` lives only on `coupling`                                        |
| CSV-only, prose-documented schema | JSON default + `schema --command` with per-column `row_schema`              |
| No introspection                  | `schema` command, self-describing                                           |
| Unsandboxed `-o` writes           | `--outfile` dropped; stdout + shell redirection                             |
| No agent knowledge                | `AGENTS.md` + skill file                                                    |
| Bespoke hand-generated input      | stdin pipe workflow + `print-log-command`                                   |
| Dead `:else` in main              | N/A (clean Go control flow)                                                 |

### Agent-DX scale: target

| Axis                        | code-maat | codelens target                                                             |
| --------------------------- | --------- | --------------------------------------------------------------------------- |
| 1 Machine-readable output   | 1         | 3 (JSON default, NDJSON streaming, structured errors)                       |
| 2 Raw payload input         | 0         | 1-2 (JSON `--group`/`--team-map`; input is a log, not an API payload)       |
| 3 Schema introspection      | 0         | 3 (full self-describing `schema`)                                           |
| 4 Context-window discipline | 1         | 3 (`--fields`, `--rows`, NDJSON, guidance in skill)                         |
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
3. **CSV parity: best-effort, JSON is canonical.** `--format csv` matches the
   original's headers, column order, and row order, but is not a frozen
   byte-for-byte contract; JSON is the source of truth. We port representative
   CSV goldens as spot-checks, not the entire corpus verbatim.

### Minor items for the implementation plan

- `ndjson` for scalar analyses (`summary`): emit rows as lines (no envelope
  wrapper), same as row analyses.
- `parse` output detail: include `loc-added`/`loc-deleted` only when present in
  the source records.
