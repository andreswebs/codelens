# Code Maat: Port Reference

Reference material for the Go port (`codelens`). This captures the original
tool's behavior, data model, log format, and the exact algorithm of every
analysis so the port can reproduce results faithfully. It is descriptive of the
original (Clojure) implementation; design decisions for the port live in
[cli-design.md](../cli-design.md).

Original: `code-maat` by Adam Tornhill, GPL-3.0, pinned at `1.0.5-SNAPSHOT`
(vendored under `.local/refs/code-maat`).

## 1. Scope decisions carried from the design

The port is **git2-only** and keeps **all 20 analyses**. Two consequences drive
the reference below:

- Only the `git2` log format is parsed. The other five VCS parsers (git legacy,
  hg, svn, p4, tfs) are out of scope.
- The stock `git2` format (`--%h--%cd--%cn`) carries **no commit message**, so
  the original's `messages` analysis fails on git2 by design (it throws "Cannot
  do a messages analysis without commit messages"). To keep all 20 analyses on a
  single format, the port **extends the git2 format with the commit subject**
  (`%s`). See [§3](#3-log-format) and the design doc. This is the one place the
  port's input format deviates from stock git2, and it is a strict superset.

## 2. Data model

Every parser reduces the log to a flat sequence of **modification records**, one
per (commit, file) pair. The analyses consume this uniform shape:

| Field         | Type       | Source         | Notes                                                           |
| ------------- | ---------- | -------------- | --------------------------------------------------------------- |
| `entity`      | string     | file path      | the changed module/file                                         |
| `rev`         | string     | commit hash    | identifies the logical change set                               |
| `date`        | string     | `YYYY-MM-dd`   | normalized commit date                                          |
| `author`      | string     | committer name | may be remapped to a team                                       |
| `message`     | string     | commit subject | `"-"` in stock git2; real subject in the port's extended format |
| `loc-added`   | string int | numstat        | `"-"` for binary files → treated as 0                           |
| `loc-deleted` | string int | numstat        | `"-"` for binary files → treated as 0                           |

`loc-added`/`loc-deleted` are present only when numstat data exists (always true
for the git2 format, which uses `--numstat`). Churn/ownership analyses assert
their presence and error clearly if missing.

## 3. Log format

### 3.1 Stock git2 generation command

```sh
git log --all --numstat --date=short \
  --pretty=format:'--%h--%ad--%aN' --no-renames --after=${SINCE} > logfile.log
```

(The source comments also show a `-M -C` / `%cd`/`%cn` variant; the parser only
depends on the `--<hash>--<date>--<name>` prelude shape and the numstat lines.)

### 3.2 Port's extended format (adds the subject)

To support the `messages` analysis on a single format, `print-log-command`
emits:

```sh
git log --all --numstat --date=short \
  --pretty=format:'--%h--%ad--%aN--%s' --no-renames --after=${SINCE}
```

The parser treats the 4th `--`-delimited prelude field as the message subject
and everything from field 4 onward (subjects may contain `--`) is joined back;
when absent (stock 3-field format), `message` defaults to `"-"` and `messages`
degrades exactly as the original.

### 3.3 On-disk shape

Entries are separated by blank lines. Each entry is:

```text
--586b4eb--2015-06-15--Adam Tornhill--Fix the parser
35      0       src/code_maat/parsers/git2.clj
-       -       docs/logo.png
```

- Prelude line: `--<hash>--<date>--<author>[--<subject>]`.
- Zero or more numstat lines: `<added>\t<deleted>\t<path>`.
- Binary files present `-` for added/deleted → normalized to `0` in churn.
- A commit with no file changes (e.g. a merge) yields no rows.

### 3.4 Merge / pull-request preludes

The original grammar is `entry = <prelude*> prelude changes`: a single entry may
carry **several stacked prelude lines** (as produced for some merges/PRs); all
but the **last** are discarded, and the last one supplies rev/date/author. The
port must replicate this "take the last prelude" behavior.

### 3.5 Parser engineering notes

- The original tokenizes the stream into blank-line-separated chunks and parses
  each chunk independently (an Instaparse memory workaround). The Go port has no
  such constraint but should still stream chunk-by-chunk to bound memory.
- Date parsing: input `YYYY-MM-dd` is parsed and re-emitted as `YYYY-MM-dd`
  (canonical). The port can keep dates as `YYYY-MM-dd` strings and parse to
  `time.Time` only where arithmetic is needed (age, temporal grouping).
- Encoding: original honors `--input-encoding` (default UTF-8). Port keeps a
  `--input-encoding` flag for parity.

## 4. Pipeline

```text
log text (stdin or --log)
  -> parse            git2 -> []Modification
  -> group            optional (--group): remap entity to a layer; DROP unmatched
  -> temporal-period  optional (--temporal-period N): sliding N-day changesets
  -> team-map         optional (--team-map): remap author -> team
  -> analysis         one of 20; produces rows
  -> output           json (default) | ndjson | csv | table; --fields; --rows
```

Order matters and matches the original (`parse-commits-to-dataset` in `app.clj`).

## 5. Auxiliary transforms

### 5.1 Grouping (`--group`)

Maps files to architectural layers. Spec lines are `pattern => name`.

- If `pattern` starts with `^`, it is used as a regex verbatim.
- Otherwise it is anchored as `^<pattern>/` (prefix match on a path segment).
- Each entity is remapped to the **first** matching group's name.
- **Entities matching no pattern are dropped** from the analysis.

Port accepts both the `=>` text form and a JSON array `[{"pattern","name"}]`
(chosen by extension/sniff).

### 5.2 Temporal period (`--temporal-period N`)

Collapses commits within a **sliding window of N days** into single logical
change sets:

- Pad the date range so every day between first and last commit exists.
- Slide a window of `N` days (step 1) over the days.
- Within each window, merge all commits; set every record's `rev` to the
  window's latest date; **dedupe by entity** (a file counts once per window).
- Remove empty windows.

`N` must be a positive integer (original validates `\d+`). Meaningful for
coupling/soc only; the same physical commit is intentionally counted in multiple
overlapping windows, which is wrong for hotspot/count analyses. The original
warns this "will probably not work with author's analyses."

### 5.3 Team mapping (`--team-map`)

Replaces each `author` with its team. Input is CSV `author,team` (parity) or
JSON. **Authors absent from the map are kept as-is** (each becomes its own
team), so mapping omissions surface quickly. Applied before analysis so all
social metrics compute at the team level.

## 6. The 20 analyses

Notation: output columns are listed in emission order. "sort" is the original
ordering (the port must match it so `--rows` truncation is deterministic).
Tuning options that affect the analysis are noted.

### Author / organizational

**`authors`** (default) - authors per entity.
Group by entity; count distinct authors; count revisions (row count per entity).
Columns: `entity, n-authors, n-revs`. Sort: by `[n-authors, n-revs]` desc.

**`revisions`** - change frequency per entity.
Group by entity; count distinct `rev`.
Columns: `entity, n-revs`. Sort: `n-revs` desc.

**`entity-effort`** - each author's revision share of each entity.
Per entity: total revs; per author revs.
Columns: `entity, author, author-revs, total-revs`. Sort: stable by author-revs
desc within entity name asc.

**`main-dev-by-revs`** - main developer per entity by revision count.
Per entity, author with most revs; `ownership = added/total` (centi precision,
here added/total are rev counts).
Columns: `entity, main-dev, added, total-added, ownership`. Sort: entity asc.

**`fragmentation`** - fractal value per entity.
`fractal = 1 - Σ (author_revs / total_revs)²`, range 0 (one author) → ~1 (many).
Columns: `entity, fractal-value, total-revs`. Sort: `[fractal-value, total-revs]`
desc.

**`communication`** - shared-work strength between author pairs.
From per-author revisions grouped by entity, count co-occurring author pairs;
self-pairs carry each author's total commit count. For each distinct pair:
`average = ceil(avg(commits_a, commits_b))`,
`strength = int(percentage(shared / average))`.
Columns: `author, peer, shared, average, strength`. Sort: `[strength, author]`
desc.

### Coupling

**`coupling`** - logical (temporal) coupling between entity pairs.
For each rev, form the change set of entities; drop change sets larger than
`--max-changeset-size` (default 30); enumerate unordered pairs; count shared
revs per pair and total revs per module.
`average-revs = avg(revs_a, revs_b)`; `degree = int(percentage(shared / average-revs))`;
emitted `average-revs = ceil(average)`.
Threshold filter (`within-threshold?`): `revs >= min-revs` AND
`shared >= min-shared-revs` AND `degree >= min-coupling` AND
`floor(degree) <= max-coupling`.
Columns: `entity, coupled, degree, average-revs`. With `--verbose`, appends
`first-entity-revisions, second-entity-revisions, shared-revisions`. Sort:
`[degree, average-revs]` desc.
Defaults: `min-revs 5, min-shared-revs 5, min-coupling 30, max-coupling 100,
max-changeset-size 30`.

**`soc`** - sum of coupling per entity.
For each rev's change set of size `k`, every entity in it gains `k-1`; sum per
entity; keep entities with `soc > min-revs` (**strict `>`**, unlike coupling's
`>=`).
Columns: `entity, soc`. Sort: `[soc, entity]` desc.

### Churn (require loc data)

**`abs-churn`** - churn per date.
Group by date; sum added, sum deleted, count distinct revs.
Columns: `date, added, deleted, commits`. Sort: `[date, added, deleted]` asc.

**`author-churn`** - churn per author.
Columns: `author, added, deleted, commits`. Sort: `[author, added]` asc.

**`entity-churn`** - churn per entity.
Columns: `entity, added, deleted, commits`. Sort: `added` desc.

**`entity-ownership`** - churn per (entity, author).
Columns: `entity, author, added, deleted`. Sort: `entity` asc.

**`main-dev`** - main developer per entity by lines added.
Per entity, author with most added lines; `total-added = Σ added`;
`ownership = added / max(total-added, 1)` (centi precision).
Columns: `entity, main-dev, added, total-added, ownership`. Sort: entity asc.

**`refactoring-main-dev`** - main developer by lines **removed**.
Same as `main-dev` but ranks by deleted lines.
Columns: `entity, main-dev, removed, total-removed, ownership`. Sort: entity asc.

### Meta / support

**`summary`** - overview counts.
Rows: `number-of-commits` (distinct revs), `number-of-entities` (distinct
entities), `number-of-entities-changed` (total rows), `number-of-authors`
(distinct authors).
Columns: `statistic, value`.

**`age`** - months since last modification.
`now` = `--time-now` (`YYYY-MM-dd`) or current date. Per entity, consider only
changes strictly before `now`; take latest date; `age = months between latest
and now`.
Columns: `entity, age-months`. Sort: `age-months` asc.

**`messages`** - entity frequency for commit-message matches.
Requires a regex via `--expression`. Keep rows whose `message` matches; count
per entity. **Errors if the log has no messages** (i.e. stock git2). Works in
the port because of the extended format (§3.2).
Columns: `entity, matches`. Sort: `[matches, entity]` desc.

**`parse`** (was `identity`) - dump parsed modification records.
Emits the raw `[]Modification` after grouping/temporal/team transforms, before
any analysis. A debug/interop escape hatch. Columns: the full record shape.

## 7. Math and precision details

- `average(a, b) = (a + b) / 2` (rational in the original; the port must match
  rounding - `average-revs` is emitted as `ceil(average)`, `degree` as
  `int(percentage)` i.e. truncation).
- `as-percentage(v) = v * 100`.
- `ratio->centi-float-precision(v)` = round to 2 significant digits via
  `BigDecimal` with precision 2, then to float. Used for `ownership`. The port
  should reproduce 2-significant-digit rounding, not 2 decimal places (e.g.
  `0.834 → 0.83`, `0.0834 → 0.083`). Verify against fixtures.
- `int(...)` truncates toward zero; `ceil`/`floor` as usual.

These rounding rules are load-bearing for regression parity; port them exactly
and pin them with the ported fixtures.

## 8. Fixtures to port (regression parity)

The original's test corpus is the port's oracle. Priority items under
`.local/refs/code-maat/test`:

- `parsers/git2_test.clj` - git2 grammar cases (empty, PR/merge preludes, binary
  numstat, blank lines).
- `analysis/*_test.clj` - one per analysis, with `test_data.clj` shared inputs;
  these encode expected columns, ordering, and rounding.
- `end_to_end/simple_git2.txt`, `end_to_end/git2_live_data_test*.clj`,
  `roslyn_git.log`, `mono_git.log` - full-pipeline expectations, including
  `--group` (layer definitions in `end_to_end/*layers-definition.txt`) and team
  maps (`mono_git_team_map.csv`).
- `app/time_based_grouper_test.clj` - sliding-window semantics.
- `app/grouper_test.clj` - grouping/anchoring rules.
- `app/team_mapper_test.clj` - unmapped-author-passthrough.

Because codelens is GPL-3.0 (matching the original), these fixtures and expected
outputs can be ported directly. Port them as Go table tests / `testdata` golden
files. The JSON envelope is the canonical assertion; `--format csv` is spot-
checked against representative original CSVs for best-effort parity (not a
frozen byte-for-byte contract). Where the original's README and code disagree on
columns (e.g. churn's `:commits`), trust the code.

## 9. Known quirks and audit-relevant behaviors (originals)

Carried here so the port fixes rather than reproduces them:

- Errors and stack traces printed to **stdout**, corrupting piped CSV. Port:
  diagnostics to stderr; traces only under `--debug`.
- Generic parse error `"<vcs>: Failed to parse ... is it a valid logfile?"`.
  Port: named errors (`parse_error`, line/context in `details`).
- `--verbose-results` silently ignored by 19 of 20 analyses. Port: `--verbose`
  exists only on `coupling`.
- Version string hardcoded in the usage banner. Port: real `--version` from
  build info.
- `-o/--outfile` writes anywhere unsandboxed. Port: dropped (stdout only).
- In-memory processing; the README recommends `-Xmx4g`. Port: native Go, stream
  parsing, far lower footprint; no JVM flags.
- `messages` throws on git2. Port: extended format keeps it working (§3.2).
- README column lists occasionally lag the code (e.g. churn's `commits`
  column). Port: schema is generated from the code, so it cannot drift.
  </content>
