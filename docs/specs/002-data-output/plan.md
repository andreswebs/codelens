# Implementation plan: canonical shape-aware data output

Status: FINAL (decisions resolved 2026-07-27). Ready to ticket.

Scope: implement ADR 0008 (one canonical, shape-aware JSON envelope; drop the
`--format` matrix) and lay the groundwork for the "codelens as a code
visualization data engine" direction.

Inputs read: `docs/adr/0008-canonical-output-representation.md`,
`docs/adr/0006-output-contract.md`, `docs/cli-design.md` sections 4.2/6/7/8, the
Go tree (`internal/output`, `internal/analysis`, `internal/command`), and the
skill under `docs/skills/codelens/`.

## 1. Verified current state (2026-07-26)

The structural refactors this plan was queued behind have all landed:

- The CLI delegate now lives in `internal/command/` (moved out of
  `cmd/codelens/`), behind the `Deps` seam. `cmd/codelens/main.go` is a
  one-liner.
- Error handling is on the `terr` registry with unique-per-failure codes;
  `schema` carries `common_error_codes` and an `errors` inventory.
- Golden end-to-end tests live in `internal/command/golden_test.go` with
  `testdata/<scenario>.{out,err,exit}` triples.
- All 78 tickets in `.tickets/` are `closed`. Clean slate for new tickets.
- `CHANGELOG.md` has an open `[Unreleased]` section (Keep a Changelog + SemVer),
  pre-1.0.

Format surface as it exists today:

| Location                                        | What                                                                                                                    |
| ----------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `internal/command/commands.go:38-43`            | `--format` global flag, default `json`                                                                                  |
| `internal/command/commands.go:175`              | `output.Emit(..., cmd.String("format"), ...)`                                                                           |
| `internal/command/commands.go:235-243`          | `columnNames(d)`, only used for csv/table                                                                               |
| `internal/command/root.go:27,37`                | `var format string` bound into `globalFlags`                                                                            |
| `internal/output/format.go`                     | `Emit`, `emitNDJSON`, `emitCSV`, `emitTable`, `rowMaps`, `cellString`, `snakeToKebab`, `rowObjects`, `errUnknownFormat` |
| `internal/output/format_test.go`                | ~32 format references                                                                                                   |
| `internal/output/types.go`                      | `Result` (no `shape`/`semantics`)                                                                                       |
| `internal/analysis/analysis.go:22-29`           | `Column{Name,Type,Desc}` (no `semantic`)                                                                                |
| `internal/analysis/schema.go:34-50`             | `commonErrorCodes` includes `unknown_format`                                                                            |
| `internal/analysis/schema.go:66-81`             | `CommandSchema` (no `shape`)                                                                                            |
| `internal/command/registry_guard_test.go:10-32` | sentinel count 22, itemises `unknown_format`                                                                            |
| `internal/command/error_format_test.go`         | asserts JSON errors under `--format text/table/json`                                                                    |
| goldens                                         | `authors_ndjson`, `authors_csv`, `authors_table`, `unknown_format`, `format_error_{text,table,json}`                    |
| docs                                            | `docs/cli-design.md`, `README.md` (~lines 103-170, 221, 247), `docs/skills/codelens/**`, `docs/specs/**`                |

Note: `format_error_{text,table,json}` are all the same `empty_log` (exit 65)
envelope; they existed only to prove `--format` never affects stderr.

Column inventory (measured, not estimated): 71 (analysis, column) pairs over 28
distinct column names. `expression` and `verbose` are flags, not columns.

## 2. Decisions register

Every decision below is settled. Numbering follows the draft's open questions so
prior discussion stays traceable.

| #   | Decision                                                                                                                                                                                                                                                                                                                                                           |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| D1  | `schema_version` stays `1`. The envelope stays backward compatible (new keys only; the `table` payload is still `rows`); removing the format matrix is a CLI-surface break, recorded in `CHANGELOG.md`, not a `schema_version` event.                                                                                                                              |
| D2  | No transitional error for `--format`. It becomes `unknown_flag` (exit 64), pinned by one new golden (`removed_format_flag`) so a re-introduction fails the suite.                                                                                                                                                                                                  |
| D3  | A semantic is a bare closed enum string; range and unit are implied by the name and documented once. This is the flat `field -> string` map Flint's `semantic_types` expects, keeping the adapter near-identity.                                                                                                                                                   |
| D3a | Numeric measures split four ways: `count` (tallies), `loc` (line counts), `percentage` (0-100 ints), `ratio` (0-1 floats). Separating `loc` from `count` is what lets a renderer pick a size channel over a frequency channel without domain knowledge.                                                                                                            |
| D3b | Closed vocabulary is 12 entries: `filepath`, `person`, `date`, `commit_id`, `text`, `label`, `flag`, `count`, `loc`, `percentage`, `ratio`, `duration_months`. Plus `text` as a _shape_ for `print-log-command` (D5), which is a separate enum.                                                                                                                    |
| D3c | Semantics are assigned per (analysis, column), never per column name globally: `added`/`total_added` are `loc` in `main-developer` but `count` in `main-developer-by-revisions`.                                                                                                                                                                                   |
| D4  | Under `--group`, the emitted `semantics` reports `entity` as `label`, not `filepath`. The envelope gains a `transforms` object recording the active pipeline transforms. `schema --command` keeps declaring the untransformed default; the schema states the command's default, the envelope states this run.                                                      |
| D4a | Rule governing D4: degrade a semantic only when a _structural affordance_ is lost. `filepath -> label` qualifies (path splitting is gone). `--team-map` does not: `author` stays `person`, because a team name and a person name are both opaque categorical actor labels.                                                                                         |
| D4b | `transforms` is its own key, omitted when the pipeline was a pass-through, recording `include`, `exclude`, `group`, `temporal_period`, `team_map`. `params` keeps its current meaning (per-analysis tuning only) so existing goldens and `--fields params.*` are unaffected.                                                                                       |
| D5  | Meta commands keep bare-text stdout, but `print-log-command` declares `shape: "text"` so an agent learns from `schema --command` that stdout is not JSON. `schema` declares no shape (it is the introspection surface, not an analysis result); `--version` is a flag and already off the schema path.                                                             |
| D6  | Under `--fields`: `shape` is always retained; `semantics` is retained filtered to the surviving payload fields (empty object when none survive); `transforms` is always retained when present, since it justifies an adjusted semantic. `schema_version` and `ok` keep their existing forced retention.                                                            |
| D7  | The payload is a single `Payload any` field written under a shape-derived key by a custom `MarshalJSON`. A `table` result can never emit a `nodes` key, and vice versa.                                                                                                                                                                                            |
| D7a | Consequence of D7: `--fields` path validation moves off struct reflection. Valid paths are those of the marshaled JSON, unioned with `<payload_key>.<column>` for every declared column. This also fixes a latent gap: `--fields rows.entity` stays valid on an empty result, because the column set comes from the schema rather than the data.                   |
| D8  | The envelope is built from an explicit `output.Meta{Analysis, Shape, Semantics, Transforms, Columns}` assembled in `internal/command` from the descriptor, so `output` never imports `analysis` and no envelope can be built missing its required fields.                                                                                                          |
| D9  | `semantics` stays in every envelope (resolved by D6): a piped envelope must be self-describing without a second `schema` call.                                                                                                                                                                                                                                     |
| D10 | ADR 0008 is restyled to match the numbered sequence (`# 0008: Canonical output representation`, no frontmatter), its stale "0006 is authoritative" preamble deleted, and 0006 gains an "amended by 0008" back-pointer. ADR 0001 is normalized to the same style. **Both ADR restyles happen immediately, outside the ticketed phases.**                            |
| D11 | `docs/specs/001-initial-implementation/{requirements,plan}.md` get a one-line "superseded by ADR 0008" note; their text is not rewritten. Spec 001 is a delivered historical record, and closed tickets reference its requirement IDs. `learnings.md` is appended to, never rewritten.                                                                             |
| D12 | Shape is fixed per command; `CommandSchema.Shape` is a string. `coupling` stays a table of pairs, and a graph view is a downstream derivation from the pair table plus semantics. No `--shape` flag, no per-shape sibling commands.                                                                                                                                |
| D13 | Viz-spec export lives in skill scripts that consume the canonical envelope on stdin. codelens stays strictly the data plane; no `export` verb in the binary, no new artifact to distribute. The existing bespoke reshaping scripts become thin `shape`+`semantics`-driven adapters.                                                                                |
| D14 | Six tickets (section 8), markdown plan plus `tk` tickets. No EARS spec, no `tasks.json`.                                                                                                                                                                                                                                                                           |
| D15 | The `semantics` map lists every declared column minus those excluded by an invocation flag. Governing rule: **semantics track flags, never data.** So `coupling` without `--verbose` omits its three verbose columns, while `parse` always lists `loc_added`/`loc_deleted`/`binary` (their absence is per-row). The map is deterministic for a given command line. |
| D16 | Drop code-maat lineage asides from the skill only. Keep them in README, ADRs, `docs/research/`, and `internal/command/testdata/README.md`, where GPL-3.0 inheritance and test-corpus provenance depend on them.                                                                                                                                                    |

## 3. Target design

### 3.1 Envelope

```json
{
  "schema_version": 1,
  "ok": true,
  "analysis": "code-age",
  "shape": "table",
  "semantics": { "entity": "label", "age_months": "duration_months" },
  "transforms": { "group": true, "exclude": ["**/vendor/**"] },
  "params": { "time-now": "" },
  "row_count": 2,
  "rows": [ ... ]
}
```

Key order is fixed by the custom marshaler (D7): `schema_version, ok, analysis,
shape, semantics, transforms, params, row_count, total_count, truncated,
<payload_key>`. Payload last, so `head` on a large result shows the metadata.

### 3.2 Semantic vocabulary (D3b)

| Semantic          | Meaning                                                   | Example columns                                                                                                               |
| ----------------- | --------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `filepath`        | A repository path, splittable on `/`                      | `entity`, `coupled`                                                                                                           |
| `person`          | An actor name (individual or, under `--team-map`, a team) | `author`, `peer`, `main_dev`                                                                                                  |
| `date`            | Calendar date, `YYYY-MM-dd`                               | `date`                                                                                                                        |
| `commit_id`       | Opaque commit identifier                                  | `rev`                                                                                                                         |
| `text`            | Free prose, never a plottable category                    | `message`                                                                                                                     |
| `label`           | Categorical name                                          | `statistic`, grouped `entity`                                                                                                 |
| `flag`            | Boolean                                                   | `binary`                                                                                                                      |
| `count`           | Tally of things                                           | `n_revs`, `n_authors`, `commits`, `matches`, `soc`, `shared`, `average`, `average_revs`, `total_revs`, `author_revs`, `value` |
| `loc`             | Line count (a size measure)                               | `added`, `deleted`, `removed`, `loc_added`, `loc_deleted`, `total_added`, `total_removed`                                     |
| `percentage`      | Integer 0-100                                             | `degree`, `strength`                                                                                                          |
| `ratio`           | Float 0-1                                                 | `ownership`, `fractal_value`                                                                                                  |
| `duration_months` | Whole calendar months                                     | `age_months`                                                                                                                  |

Watch the D3c trap: in `main-developer-by-revisions`, `added` and `total_added`
count revisions and are therefore `count`, not `loc`.

Shape enum (separate from the above): `table`, `tree`, `graph`, `matrix`,
`series`, `text`. Only `table` and `text` are reachable after this plan.

### 3.3 Semantics single source of truth

`analysis.Column` gains a `Semantic` field. The envelope's `semantics` map is
derived from the descriptor's `RowSchema` (name to semantic), filtered per D15
and adjusted per D4/D4a; it is never declared twice. `schema --command CMD`
shows `semantic` per column and declares the untransformed default.

Conformance tests, in the style of `registry_guard_test.go`:

- every registered descriptor declares a `Shape` in the shape enum;
- every declared column has a non-empty `Semantic` in the 12-entry vocabulary;
- the envelope's `semantics` keys equal the descriptor's columns after the D15
  flag filter;
- the D4 adjustment applies exactly when its transform is active.

### 3.4 Removals

- `--format` flag, `errUnknownFormat`, `unknown_format` from `commonErrorCodes`
  and from the sentinel inventory (22 to 21 in `registry_guard_test.go`).
- `internal/output/format.go` and `format_test.go` in full; `output.Emit`
  collapses to the existing `output.EmitProjected`.
- `columnNames(d)` in `internal/command/commands.go` is deleted in ticket 1 and
  revived in ticket 2a for its new purpose (the D7a path seeding and D8 `Meta`).
- `var format string` plumbing in `root.go` and the `*string` parameter of
  `globalFlags`.
- Goldens `authors_ndjson`, `authors_csv`, `authors_table`, `unknown_format`,
  `format_error_{text,table,json}` (18 files).

Kept unchanged: `--fields`, `--rows`, the error envelope on stderr, warnings on
stderr, the exit-code taxonomy (minus the `unknown_format` code string).

## 4. Ticket 1: remove `--format`

Purely subtractive, so its golden diff is a clean deletion.

1. Delete `internal/output/format.go` and `format_test.go`. Nothing outside
   format.go calls `rowObjects`, so it goes with them.
2. `internal/command/commands.go`: drop the `--format` flag definition, call
   `output.EmitProjected(cmd.Root().Writer, res, cmd.String("fields"))`, delete
   `columnNames`.
3. `internal/command/root.go`: drop `var format string`; `globalFlags()` takes
   no argument.
4. `internal/analysis/schema.go`: drop `unknown_format` from `commonErrorCodes`
   and from the derivation comment above it.
5. `internal/command/registry_guard_test.go`: sentinel count 22 to 21; remove
   the `internal/output/format.go` line from the derivation comment.
6. `internal/command/error_format_test.go`: replace with an `empty_log`
   JSON-error-envelope test; rename the file to `error_envelope_test.go`.
7. `golden_test.go`: delete the six format scenarios; add `empty_log` (keeps
   exit-65 and `empty_log` coverage) and `removed_format_flag` per D2. Update
   the coverage comment block (lines ~57-67).
8. Regenerate with `go test ./internal/command/ -run TestGolden -update` and
   hand-review. Net: 18 golden files deleted, 6 added.
9. `internal/command/testdata/README.md`: drop "every format" and the format
   scenario names.

Gate: `make build` green.

## 5. Ticket 2a: shape and the payload marshaler

Structural refactor. The only new envelope key is `shape`.

1. `internal/analysis/analysis.go`: add `Descriptor.Shape`.
2. `internal/analysis/shape.go` (new): the shape enum and its validator.
3. Set `Shape: "table"` on all 20 analysis descriptors; `print-log-command`
   declares `text` (D5), `schema` declares none.
4. `internal/analysis/schema.go`: `CommandSchema.Shape`, populated by
   `Schema(d)` and by `MetaSchema` (which now needs a shape parameter for D5).
5. `internal/output/types.go`: `Result` becomes `Shape string` plus
   `Payload any`, with a custom `MarshalJSON` writing the payload under the
   shape-derived key and fixing the key order from section 3.1.
6. `internal/output/meta.go` (new): `output.Meta` per D8; `NewResult(meta,
payload)`.
7. `internal/output/fields.go`: replace struct reflection with marshaled-JSON
   path collection unioned with the declared column paths (D7a). Keep the
   wildcard matching. Force-retain `shape` (D6).
8. `internal/command/commands.go`: revive `columnNames` for `Meta.Columns`;
   build `Meta` from the descriptor; `truncate` now operates on `Payload`.
9. Tests: `--fields rows.entity` on an empty result stays valid (the D7a fix);
   the marshaler emits `rows` for `table`; a shape conformance test.
10. Regenerate the affected goldens; review the diff for exactly one new key.

Gate: `make build` green.

## 6. Ticket 2b: semantics and transforms

Content, on top of 2a's mechanics.

1. `internal/analysis/analysis.go`: add `Column.Semantic`.
2. `internal/analysis/semantics.go` (new): the 12-entry vocabulary, its
   validator, and `SemanticsOf(d Descriptor, activeFlags map[string]bool)`
   applying the D15 filter.
3. Fill `Semantic` on all 71 (analysis, column) pairs, honouring D3c.
4. `internal/analysis/schema.go`: `Column.Semantic` reaches
   `schema --command` output.
5. `internal/output/types.go`: add `Semantics map[string]string` and
   `Transforms map[string]any` (omitempty) to `Result` and the marshaler; add
   both to `output.Meta`.
6. `internal/command/commands.go`: build the transforms record from
   `pipelineConfig` (D4b) and apply the D4/D4a semantic adjustment.
7. `internal/output/fields.go`: filter `semantics` to surviving payload fields;
   force-retain `transforms` when present (D6).
8. Conformance tests per section 3.3, plus a `--group` test asserting
   `entity` becomes `label` while `schema --command` still says `filepath`, and
   a `--team-map` test asserting `author` stays `person`.
9. Goldens: regenerate; add a `--group` scenario so the D4 adjustment and the
   `transforms` record are frozen.
10. `CHANGELOG.md` `[Unreleased]`: `Removed` for the format matrix (breaking),
    `Added` for `shape`/`semantics`/`transforms`, and an explicit note that
    `schema_version` stays `1` and why (D1).

Gate: `make build` green.

## 7. Ticket 3: repo docs

1. `docs/cli-design.md`: fix the two "see ADR 0003" references in section 6
   (0003 is error handling; the right ADR is 0008). Remove `unknown_format`
   from the section 7.2 exit-code table and the section 8
   `common_error_codes` example. Document `shape`, `semantics`, `transforms`,
   the D6 projection rules, and the D5 text helpers. Add the vocabulary table
   from section 3.2.
2. `README.md`: rewrite the output section (~lines 100-170), the global-flag
   table row (~221), and the stale `--format text` error paragraph (~247).
   This is the largest remaining doc surface.
3. `docs/specs/001-initial-implementation/{requirements,plan}.md`: add the D11
   superseded note; do not rewrite.
4. `docs/specs/learnings.md`: append a section; never rewrite.
5. `docs/skill-design.md`: sweep for format-era assumptions.
6. markdownlint every touched file
   (`markdownlint-cli2 --config .markdownlint.yaml --fix <file>`).

Not in this ticket: the ADR restyles and the 0006 back-pointer, done
immediately per D10.

## 8. Ticket 4: codelens skill

Line numbers below were current at 2026-07-26. Re-locate by grep rather than
trusting them. Use the `skill-builder` skill.

Section A (drop `--format json`):

- `docs/skills/codelens/SKILL.md:47`
- `docs/skills/codelens/references/catalog.md:129`
- `docs/skills/codelens/scripts/run.bash:150,169`
- `docs/skills/codelens/scripts/commit_cloud.py:18`

Leave git's own `--format=` in `scripts/complexity_trend.py:59` and
`scripts/run.bash:131`.

Section B (`references/operating.md`):

- `:96-99` restate as "codelens emits one JSON envelope" (no default-format
  framing).
- `:106-107` delete the `--format` bullet and the kebab-case/code-maat line.
- `:108-110` reword `--rows`/`--fields` so neither is qualified by a format.
- `:220-221` errors: drop "for every `--format` value including `text` and
  `table`" and the "`--format` selects the results shape" clause.
- Add, once, `shape` / `semantics` / `transforms`, the D6 projection rules, and
  that everything is `shape: "table"` today except `print-log-command`.
- Add that `schema --command CMD` returns `shape` and per-column `semantic`.

Section D (D16): drop the "matching code-maat" aside at
`references/operating.md:26` and sweep the skill for other lineage asides.

Verification: no `--format`, `ndjson`, `csv`, or `table` remains as a codelens
output reference under `docs/skills/codelens/`; the skill's Python test suites
pass for any touched script; `make build` green; markdownlint clean.

## 9. Ticket 5: deferred epic

Not planned in detail. Tracked so it is not forgotten:

- Per-shape payload schemas for `tree`, `graph`, `matrix`, `series`, and which
  new analyses introduce them. Under D12 these arrive as genuinely new analyses,
  not as alternate views of existing ones.
- Whether `row_count` stays `row_count` for a non-table payload, or each shape
  gets its own count. First real `schema_version` bump candidate.
- Shape-aware `--rows` truncation (today it slices a payload slice).
- Skill-side viz-spec adapters per D13 (Flint `ChartAssemblyInput`, Vega-Lite,
  GraphML/DOT, CodeCharta `.cc.json`), driven by `shape` + `semantics`.
- Skill-side reshaping cleanups once shapes land: retiring the path-tree
  building in `scripts/enclosure.py`, simplifying `scripts/coupling_graph.py`
  and `scripts/dev_network.py`, realigning `references/catalog.md` cards, and
  revisiting `references/embedding.md` and `references/reporting.md`.
- The complexity epic (reading file content, indentation complexity trend,
  `hotspot` fusion command) from `docs/research/complexity-*`. Separate epic.

## 10. Immediate actions (outside the tickets)

Per D10, before or alongside ticket creation:

1. Restyle `docs/adr/0008-canonical-output-representation.md` to
   `# 0008: Canonical output representation`, drop the frontmatter, delete the
   stale "0006 is authoritative" preamble, and reference 0006 in prose.
2. Add an "amended by 0008" back-pointer to
   `docs/adr/0006-output-contract.md`.
3. Normalize `docs/adr/0001-keep-churn-and-effort-separate.md` to the same
   numbered-title, no-frontmatter style.
4. markdownlint all three.

## 11. Ticket order and dependencies

| #   | Ticket                                                          | Depends on |
| --- | --------------------------------------------------------------- | ---------- |
| 1   | Remove `--format` and the alternate serializers                 | -          |
| 2a  | `shape` and the payload marshaler                               | 1          |
| 2b  | `semantics` and `transforms`                                    | 2a         |
| 3   | Repo docs: cli-design, README, specs notes                      | 2b         |
| 4   | codelens skill: drop the format matrix, document the new fields | 2b         |
| 5   | Epic: non-table shapes and viz-spec adapters                    | 3, 4       |

Every ticket lands with `make build` green.
