# 001 - Initial Implementation Plan

Implementation plan for the first complete version of `codelens`: a git2-log
analyzer that ports code-maat's 20 analyses with an agent-first I/O surface.

This plan is the bridge between the design and the tickets. It defines the
package build order, a phased sequence with sliceable tasks, the test strategy,
and a definition of done. Each task (`P<phase>-<n>`) is scoped to become one
ticket.

## References

- Design: [../../cli-design.md](../../cli-design.md) - authoritative surface,
  formats, errors, schema.
- Reference: [../../research/code-maat.md](../../research/code-maat.md) - data
  model, log format, per-analysis algorithms, fixtures.
- House style: sibling `terminology` repo (`internal/output`, `internal/terr`,
  `schema --command`) - the patterns to mirror/port.

## Guiding constraints

- **Quality gate.** Every task lands with `make validate` green (fmt-check,
  vet, lint, test). No `_ =` error silencing (golangci-lint v2, standard +
  revive). Exported symbols documented.
- **GPL-3.0 corpus reuse.** codelens is GPL-3.0 (matches the original), so
  code-maat's fixtures, sample logs, and expected outputs are ported directly
  as the regression oracle. Attribute the origin in ported testdata.
- **Registry-driven surface.** The command tree, per-command help, and `schema`
  are all generated from one analysis registry. Adding an analysis = adding a
  descriptor; no wiring drift.
- **JSON is canonical.** JSON envelope is the tested contract; `csv` is
  best-effort parity; `table`/`ndjson` are presentation.
- **Vertical slice early.** Prove the whole spine (parse → pipeline → analysis →
  all output formats → schema) on one analysis before fanning out to 20.

## Target package layout

```text
src/
  cmd/codelens/            main; builds the urfave/cli command tree from the registry
  internal/
    version/               (exists) build version - keep
    terr/                  coded errors: Code/Message/Hint/ExitCode; Coded, Detailed ifaces
    output/                envelope types, EmitJSON/NDJSON/CSV/Table, fields, registry, schema
    model/                 Modification, ChangeSet, Options
    gitlog/                tokenizer + git2(+%s) parser -> []model.Modification
    pipeline/              parse -> group -> temporal -> teammap orchestration
    transform/
      group/               layer mapping (=> text + JSON), anchoring, drop-unmatched
      temporal/            sliding N-day window grouping
      teammap/             author->team (CSV + JSON), unmapped passthrough
    analysis/              registry + one file per analysis (Run + descriptor)
      internal helpers:    groupby, distinct, orderby, round (ceil/int/centi)
```

## Phase 0 - Foundations

Goal: an installable binary with the output/error spine in place and the
placeholder removed. No analyses yet.

- **P0-1 Remove `greet` placeholder; wire root app.** Root `cli.Command` with
  name/usage/version; empty subcommand set; `main` maps returned errors to exit
  codes via `output.ExitCodeFor`. Keep slog to stderr.
- **P0-2 `internal/terr`.** Port terminology's coded-error model: `New(code,
exit, hint, msg)`, `Coded` and `Detailed` interfaces, wrapping helpers. Unit
  tests for code/exit/hint resolution and `errors.As` unwrapping.
- **P0-3 `output` envelope + emit.** Result envelope struct (`schema_version`,
  `ok`, `analysis`, `params`, `row_count`, `total_count`, `truncated`, `rows`);
  `EmitJSON`. Error envelope +
  `EmitError(w, format, err)` and `ExitCodeFor(err)` (0/1/2/3 taxonomy incl.
  usage-error classification from urfave messages).
- **P0-4 `output` field projection.** Port `ValidateFields`/`ProjectFields`
  (`--fields`), retaining `schema_version`+`ok`. Tests incl. nested `rows.*`.
- **P0-5 `output` registry.** `RegisterEnvelope`/`RegisterExitCodes` +
  accessors, so `schema` can reflect per-command shape. Tests.

Exit criteria: `codelens version` works; `codelens` with no args prints help and
exits 0; error envelope + exit codes covered by tests.

## Phase 1 - git log parser

Goal: turn a log (stdin or file) into `[]model.Modification`, faithfully.

- **P1-1 `model` types.** `Modification{Entity, Rev, Date, Author, Message,
LocAdded, LocDeleted (with present flag)}`; `Options`.
- **P1-2 Tokenizer.** Split the reader into blank-line-separated entries;
  streaming; encoding honored via `--input-encoding`.
- **P1-3 git2(+%s) entry parser.** Parse prelude `--hash--date--author[--subject]`
  (take the LAST of stacked preludes for merge/PR entries; join subject fields
  containing `--`) and numstat lines (`added\tdeleted\tpath`, binary `-` → 0,
  present flag false when no numstat). Emit one Modification per file.
- **P1-4 Parse errors.** Named `parse_error`/`empty_log` (terr) with entry
  index + offending line in `details`; hint → `print-log-command`. Reject
  disallowed control characters (e.g. NUL) as an input error.
- **P1-5 Fixtures.** Port git2 fixtures from `.local/refs/code-maat/test`
  (`git2_test.clj`, `simple_git2.txt`, PR/merge, binary, empty). Golden tests
  for the parsed record set.

Exit criteria: representative code-maat git2 logs parse to the expected record
sets (golden).

## Phase 2 - Vertical slice (`authors` end-to-end)

Goal: de-risk the full spine on one analysis, including every output format and
schema. This phase builds all the generic output/CLI machinery.

- **P2-1 Analysis descriptor + registry.** `analysis.Descriptor{Name, Aliases,
Summary, Flags, RowSchema, ErrorCodes, Run(func([]Modification, Opts)
(Envelope, error))}`; a registry the command tree and `schema` read from.
  `Name` is the descriptive canonical name; `Aliases` holds the terse code-maat
  originals (design §4.1). The command tree registers both; `schema` reports the
  canonical name plus its aliases.
- **P2-2 `authors` analysis.** Group by entity; distinct author count; rev
  count; sort `[n-authors, n-revs]` desc. Row schema with descriptions.
- **P2-3 CLI subcommand generation.** Build one `cli.Command` per registered
  descriptor (registering its aliases); attach global flags (`--log`,
  `--format`, `--fields`, `--rows`, `--input-encoding`, `--debug`, `--group`,
  `--group-format`, `--team-map`, `--team-map-format`, `--temporal-period`) and
  the descriptor's per-command flags. Read stdin by default.
- **P2-4 Output formats (generic).** `--format json|ndjson|csv|table`:
  - json: full envelope (default); on `--rows` truncation adds `total_count` +
    `truncated: true`.
  - ndjson: one row object per line, no wrapper, uniformly (scalar analyses like
    summary too); envelope metadata absent.
  - csv: original kebab-case headers, original column + row order.
  - table: aligned human columns.
    `--rows N` truncates after sort (all formats); `--fields` projects (json
    only).
- **P2-5 `schema --command authors`.** Emit flags + envelope + row_schema +
  error_codes + exit_codes from the descriptor/registry. `codelens schema`
  lists all commands.
- **P2-6 Golden tests for the slice.** `authors` against a ported fixture across
  all four formats + `--fields`/`--rows`; schema output snapshot.

Exit criteria: `git log … | codelens authors` produces the correct JSON; all
formats, `--fields`, `--rows`, and `schema --command authors` verified. The spine
is frozen; remaining analyses are additive.

## Phase 3 - Transforms (pipeline stages)

Goal: the three optional pipeline stages, wired in original order (group →
temporal → teammap), exercised via the `authors` slice.

- **P3-1 `pipeline`.** Compose parse → group → temporal → teammap → analysis,
  each stage a no-op when its flag is absent. Order per reference doc §4.
- **P3-2 `transform/group`.** Parse `=>` text or JSON specs, selected by
  `--group-format` (default `text`); anchoring rule (`^…` verbatim else
  `^<p>/`); first-match remap; **drop unmatched entities**. Patterns compiled
  with a complexity/size bound (usage error on invalid/oversized). Port
  `grouper_test.clj` + layer-definition fixtures.
- **P3-3 `transform/temporal`.** Sliding N-day window; pad date range; merge +
  set rev=latest date + dedupe by entity; remove empty windows; validate N is a
  positive int. Port `time_based_grouper_test.clj`.
- **P3-4 `transform/teammap`.** CSV (`author,team`) or JSON, selected by
  `--team-map-format` (default `csv`); unmapped authors passthrough. Port
  `team_mapper_test.clj`.

Exit criteria: each transform matches its ported fixtures; `authors --group …`
and `--temporal-period …` produce expected aggregated results.

## Phase 4 - Remaining analyses

Goal: the other 19 analyses, added as descriptors. Batched by shared
helpers/dependencies. Each task: run fn + row_schema + per-command flags +
ported fixtures (json golden canonical, csv spot-check).

Shared helper task first:

- **P4-0 Analysis helpers.** `groupby`, `distinct`, `orderby` (stable, matching
  original sort semantics), and rounding (`ceil`, `int` truncation,
  `ratio->centi-float-precision` = 2 significant digits). Unit-tested against
  reference doc §7 examples.

Batch A - counts (no loc):

- **P4-1 `revisions`** - `[entity, n-revs]`, sort n-revs desc.
- **P4-2 `summary`** - `[statistic, value]`, 4 rows.
- **P4-3 `parse`** (alias `identity`) - dump `[]Modification` in log order (as
  parsed, unsorted); loc columns only when present.

Batch B - coupling family:

- **P4-4 coupling algos** - change-set-per-rev, pair enumeration,
  shared/total-rev frequencies, `max-changeset-size` skip, `within-threshold?`.
- **P4-5 `coupling`** - `[entity, coupled, degree, average_revs]` + `--verbose`
  extra columns; flags min-revs/min-shared-revs/min-coupling/max-coupling/
  max-changeset-size; sort `[degree, average_revs]` desc.
- **P4-6 `sum-of-coupling`** (alias `soc`) - `[entity, soc]`; `soc > min-revs`
  (strict); sort `[soc, entity]` desc.

Batch C - churn family (require loc; assert + `input_error` when missing):

- **P4-7 churn core** - `sum-by-group` (added/deleted/commits), binary→0.
- **P4-8 `absolute-churn`** (alias `abs-churn`) - `[date, added, deleted,
commits]`, sort `[date, added, deleted]` asc.
- **P4-9 `author-churn`** - `[author, added, deleted, commits]`.
- **P4-10 `entity-churn`** - `[entity, added, deleted, commits]`, sort added desc.
- **P4-11 `entity-ownership`** - `[entity, author, added, deleted]`.
- **P4-12 `main-developer`** (alias `main-dev`) - by added lines; `[entity,
main-dev, added, total-added, ownership]`.
- **P4-13 `refactoring-main-developer`** (alias `refactoring-main-dev`) - by
  removed lines; `[entity, main-dev, removed, total-removed, ownership]`.

Batch D - effort family:

- **P4-14 effort core** - per-entity per-author revs + total revs.
- **P4-15 `entity-effort`** - `[entity, author, author-revs, total-revs]`
  (stable double-sort).
- **P4-16 `main-developer-by-revisions`** (alias `main-dev-by-revs`) - `[entity,
main-dev, added, total-added, ownership]`.
- **P4-17 `fragmentation`** - `1 - Σ(ai/nc)²`; `[entity, fractal-value,
total-revs]`.
- **P4-18 `communication`** - author-pair shared work; `[author, peer, shared,
average, strength]`.

Batch E - time / messages:

- **P4-19 `code-age`** (alias `age`) - whole calendar months (UTC) since latest
  change strictly before `now`; `--time-now` else current UTC date; `[entity,
age-months]` asc.
- **P4-20 `messages`** - `--expression` regex over subject (compiled with a
  complexity/size bound); `[entity, matches]`; usage error if `--expression`
  missing or invalid; input error if log has no messages.

Exit criteria: all 20 analyses match ported fixtures; `codelens schema` lists
the 20 analyses alongside `schema`, `print-log-command`, and `version`.

## Phase 5 - Auxiliary commands & surface finish

- **P5-1 `print-log-command`.** Emit the extended git2 command (with `%s`); no
  args. Optionally accept `--after DATE` to inline a date window.
- **P5-2 `--version` / `version`.** Confirm both paths use `internal/version`.
- **P5-3 Error-code registration.** Each descriptor registers its `error_codes`
  and `exit_codes`; verified surfaced by `schema`.
- **P5-4 Usage-error mapping.** Confirm unknown flag/subcommand, missing/invalid
  values, and `messages` w/o `--expression` map to exit 2 with coded envelope.

Exit criteria: full command surface matches the design doc §4.

## Phase 6 - Agent knowledge & docs

- **P6-1 `AGENTS.md`** (repo root): stdin-pipe workflow, `print-log-command`,
  "learn a command via `schema --command CMD`", `--fields`/`--rows` guidance,
  exit-code table, format guidance.
- **P6-2 codelens skill file** (house skill format, YAML frontmatter + Markdown)
  encoding the same invariants.
- **P6-3 README rewrite** for codelens (install, quick start, analyses table,
  examples) - replace code-maat-oriented content.
- **P6-4 Fill `CLAUDE.md`** project description (currently a template stub) and
  confirm `make build` guidance matches.

Exit criteria: an agent with only `AGENTS.md` + `schema` can drive every
analysis without external docs.

## Cross-cutting: test strategy

- **Unit** per package (terr, output/fields, gitlog, transforms, analysis
  helpers, each analysis).
- **Golden fixtures** ported from code-maat (`testdata/`), JSON envelope as the
  canonical assertion; `csv` spot-checked against original CSVs.
- **End-to-end** table tests: `stdin log → codelens <analysis> → envelope`, one
  per analysis, plus `--group`/`--temporal-period`/`--team-map` combinations
  from the ported end-to-end suite (roslyn/mono logs).
- **Schema conformance**: a test asserting every registered analysis has a
  non-empty row_schema and that `schema --command` reflects its actual flags.
- **`make validate`** is the gate for every task; `make test-race` in CI.

## Sequencing & dependencies

```text
P0 (foundations)
  └─> P1 (parser) ──┐
                    ├─> P2 (vertical slice: needs P0 output + P1 parser)
P0 ────────────────┘
P2 ─> P3 (transforms use pipeline + authors slice)
P2 ─> P4-0 helpers ─> P4 batches A–E (each batch independent; parallelizable)
P4 ─> P5 (surface finish)
P4/P5 ─> P6 (docs reflect final surface)
```

- Critical path: P0 → P1 → P2 → P4-0 → longest P4 batch → P5 → P6.
- Parallelizable once P2 + P4-0 land: the five P4 batches, and P3.

## Definition of done (this spec)

1. `git log … | codelens <analysis>` works for all 20 analyses; JSON default,
   plus `ndjson`/`csv`/`table`, `--fields`, `--rows`.
2. `codelens schema` and `schema --command CMD` are fully self-describing
   (flags, row_schema, error/exit codes) for every command.
3. `print-log-command` and `version`/`--version` implemented.
4. Errors on stderr as coded envelopes; exit codes 0/2/3/1 as specified; traces
   only under `--debug`.
5. Ported fixtures pass; JSON goldens are the contract, csv spot-checks pass.
6. `AGENTS.md` + skill + README + CLAUDE.md complete.
7. `make build` (validate + compile) green; GPL-3.0 headers/attribution on
   ported testdata.

## Risks & mitigations

- **Rounding drift** (degree/average-revs/ownership). Mitigation: P4-0 rounding
  helpers pinned to reference §7 examples before any analysis uses them.
- **Sort-order mismatches** breaking `--rows` determinism. Mitigation: encode
  each analysis's exact sort in its fixture golden; stable sorts.
- **Merge/PR prelude edge cases** in the parser. Mitigation: port the specific
  git2 fixtures that cover stacked preludes early (P1-5).
- **Temporal sliding-window semantics** (double-counting by design). Mitigation:
  port `time_based_grouper_test.clj` verbatim; document that it's coupling-only.
- **CSV parity scope creep.** Mitigation: JSON is the contract; csv is
  best-effort and spot-checked, explicitly not byte-frozen.
