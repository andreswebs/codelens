---
id: cod-1d04
status: closed
deps: [cod-435u]
links: []
created: 2026-07-27T13:14:27Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-z4wu
tags: [codelens, spec-002, skill, docs]
---
# Skill: drop the format matrix, document shape and semantics

Update the codelens skill (docs/skills/codelens/) for the canonical envelope: drop
`--format json` from every example and script, rewrite the output section of the operating
reference, document `shape`/`semantics`/`transforms`, and remove the code-maat lineage
asides.

The skill is the canonical operations reference (AGENTS.md points at it, and it is what an
agent reads to learn how to run codelens), so a stale `--format json` there is worse than a
stale mention in a design document: it produces a command that exits 64.

Implements decision D16 from docs/specs/002-data-output/plan.md section 2 (drop the
code-maat lineage asides from the SKILL only, keeping them in README, ADRs,
docs/research/, and internal/command/testdata/README.md where GPL-3.0 inheritance and test
corpus provenance depend on them), and documents D5, D6, D12, and D15.

Scope note: this ticket does NOT retire the bespoke Python reshaping scripts
(scripts/enclosure.py's path-tree building, scripts/coupling_graph.py,
scripts/dev_network.py). Those depend on codelens emitting `tree` and `graph` shapes,
which is the deferred follow-on epic. Here the scripts only lose the `--format json` flag
from their invocations.

Reference: docs/specs/002-data-output/plan.md section 8 (this ticket's step list).
Skills: /skill-builder (for the SKILL.md and references edits).

## Design

All line numbers were verified at 2026-07-26. Re-locate by grep rather than trusting them.

### 1. Mechanical: drop `--format json` from invocations

Four sites, all verified to still carry the flag:

```text
SKILL.md:47                     codelens <analysis> --log git.log --format json > data.json
references/catalog.md:129       codelens parse --log git.log --format json | uv run scripts/commit_cloud.py -o cloud.svg
scripts/run.bash:150            codelens "${analysis}" "${@}" --format json >"${OUT}/${outfile}"
scripts/run.bash:169            codelens parse "${WIN[@]}" --format json >"${OUT}/parse.json"
scripts/commit_cloud.py:18      codelens parse --format json | uv run scripts/commit_cloud.py -o cloud.svg   (docstring usage)
```

DO NOT TOUCH git's own format flags, which are unrelated and would break the log generation:

```text
scripts/run.bash:131            LAST="$(git log -1 --format=%as)"
scripts/complexity_trend.py:59  "--format=%H\t%ad"
```

### 2. `references/operating.md`: the output section

Current state at lines 94-112 (`## Output formats and shaping`):

- Retitle the section: it is no longer about formats. Something like `## Output and
  shaping`.
- Lines 96-99 open with "`--format json` (default) wraps rows in a self-describing
  envelope (`schema_version, ok, analysis, row_count, rows`)". Restate with no
  default-format framing: codelens emits one JSON envelope, and list its fields including
  the new `shape`, `semantics`, and `transforms`.
- Lines 106-107: DELETE the `--format` bullet and the "code-maat-compatible kebab-case
  headers" line entirely.
- Lines 108-110: the bounding bullet says `--rows N` applies to "all formats" and
  `--fields` is "json only". Both qualifiers go. Restate the D6 retention rules:
  `--fields` always keeps `schema_version`, `ok`, and `shape`, keeps `transforms` when
  present, and narrows `semantics` to the projected fields.
- Lines 101-104 describe the `params` object and which analyses carry it. Still accurate;
  verify the list (`coupling`, `sum-of-coupling`, `code-age`, `messages`) against
  `codelens schema` output rather than trusting it.

Add, once, in this section:

- `shape`: the closed set (`table`, `tree`, `graph`, `matrix`, `series`, `text`), that it
  is fixed per command, and that everything is `table` today except `print-log-command`,
  which is `text` and emits a bare command line by design (D5, D12).
- `semantics`: the 12-member vocabulary, and WHY it exists (it is what lets a renderer or
  a downstream chart spec be derived without domain knowledge). Include that `percentage`
  is 0-100 while `ratio` is 0-1, and that `loc` is distinct from `count` so a size channel
  is distinguishable from a frequency channel.
- `transforms` plus the D4 consequence an operator must know: under `--group`, `entity` is
  reported as `label`, not `filepath`, because a layer name is not a splittable path;
  `--team-map` leaves `author` as `person`.
- D15: `coupling` declares 4 semantics without `--verbose` and 7 with it (semantics track
  flags, not data).
- That `schema --command CMD` now returns `shape` and a per-column `semantic`, so column
  meanings AND their semantic types are discoverable at runtime. Reinforce the existing
  "schema is the source of truth, do not guess" stance.

### 3. `references/operating.md`: the errors section

Lines 219-223 currently read:

```text
Errors are **always** a JSON envelope on stderr (`{ok: false, error: {code,
message, hint}}`), for every `--format` value including `text` and `table`.
`--format` selects the results shape on stdout, not the diagnostics on stderr;
there is no `✗ <message>` text error path, so parse the envelope's `message` and
`hint` fields directly.
```

Cut both `--format` clauses. The surviving claim is simply that errors are always the JSON
envelope on stderr, there is no text error path, and a caller parses `message` and `hint`
directly.

### 4. Lineage and family asides (D16)

- `references/operating.md:26`: "The default reads the checked-out branch's history,
  matching code-maat and avoiding commits from unmerged branches or dated after `HEAD`."
  Drop "matching code-maat"; the operational reason (avoiding unmerged branches and
  post-HEAD commits) is the part a skill user needs and it stands on its own.
- Sweep the skill for other external-tool lineage asides:
  `grep -rn 'code-maat\|code_maat\|Tornhill\|family' docs/skills/codelens/`.

IMPORTANT distinction when sweeping: the `family` term in that grep now matches ONLY
legitimate hits. "enclosure family" (SKILL.md:53,
references/reporting.md:32, references/embedding.md:11, references/enclosure.md:7 and :95,
scripts/treemap.py, scripts/enclosure.py) means a family of VISUALIZATIONS within the
skill (hotspot / knowledge / code-age maps sharing one renderer). That is a legitimate
domain term and MUST NOT be removed. Only tool-family and external-lineage references go.

Two code-maat references in the skill need a judgement call rather than blind deletion:

- `references/enclosure.md:26-27`: "This mirrors Tornhill's `csv_as_enclosure_json.py`
  (`--structure` vs `--weights`)". This explains WHY the structure/weights split exists, so
  it carries real information. Prefer rewriting it to state the behaviour directly (files
  with no recorded change still appear, because structure and weights are separate inputs)
  and drop the attribution.
- Any place a column name or default threshold is justified by code-maat parity: keep the
  behaviour, drop the justification, since the skill user cannot act on the lineage.

### 5. Script test suites

`scripts/` has Python tests (`commit_cloud` has no test but `digest_test.py`,
`enclosure_test.py`, `report_test.py`, `complexity_trend_test.py` exist). Only
`scripts/run.bash` and the `scripts/commit_cloud.py` DOCSTRING change here, so no test
should break. Run whatever the skill's test entry point is to confirm; check
`scripts/ruff.toml` and any Makefile target or CI workflow under `.github/workflows/` that
exercises the skill scripts.

Note `scripts/run.bash:150` is inside a shell function whose failure path writes to
`${OUT}/${analysis}.stderr`; the flag removal must not disturb the redirections or the
`||` fallback on the following lines.

### 6. Verification

```sh
grep -rn -- '--format' docs/skills/codelens/ | grep -v 'format=%\|group-format\|team-map-format'
grep -rni 'ndjson\|kebab' docs/skills/codelens/
grep -rn 'csv' docs/skills/codelens/    # inspect each: --team-map-format csv is legitimate
```

Expected: zero hits for a codelens OUTPUT format. Legitimate survivors, enumerated so none
of them is "fixed" by mistake:

- git's own `--format=%as` (scripts/run.bash:131) and `--format=%H` (
  scripts/complexity_trend.py:59).
- The `--group-format` and `--team-map-format` input selectors, and any CSV mention that
  refers to a `--team-map` input file. These are INPUT parsing selectors and are unaffected
  by ADR 0008.
- `references/operating.md:238`: "line (valid NDJSON), they never change the exit code and
  never touch stdout". This describes WARNING DIAGNOSTICS on stderr, which genuinely do
  form an NDJSON stream (one JSON object per line, per ADR 0006). It is correct and MUST
  NOT be removed: the NDJSON that was deleted was a results serialization on stdout, an
  unrelated thing that happened to share the name.
- The rendered-artifact format vocabulary: `references/embedding.md`'s "Format to target"
  section and table, and `references/catalog.md`'s per-card `Formats:` lines, are about
  SVG / PNG / HTML / PDF output artifacts of the visualization scripts. They have nothing
  to do with codelens's output representation and are out of scope here (verified
  2026-07-27: `references/reporting.md` has no format references at all).

### 7. Doc-style constraints (enforced)

- No em-dashes, no emojis.
- No local filesystem paths; the skill must read as standalone.
- markdownlint every touched markdown file:
  `markdownlint-cli2 --config .markdownlint.yaml --fix <file>` from the repo root.

### Out of scope

- Retiring the bespoke reshaping scripts, and the `references/catalog.md` card realignment
  (`Command`/`Formats` lines, the static-versus-interactive split): all deferred to the
  follow-on epic, since they depend on `tree` and `graph` shapes existing.
- `references/embedding.md` and `references/reporting.md` beyond the family/lineage sweep.
- Any change to the repo's own docs: the docs ticket owns those.

### Files touched

```text
docs/skills/codelens/SKILL.md                       :47 drop --format json
docs/skills/codelens/references/catalog.md          :129 drop --format json
docs/skills/codelens/references/operating.md        output section rewrite, errors section, :26 code-maat aside
docs/skills/codelens/references/enclosure.md        :26-27 attribution rewrite
docs/skills/codelens/scripts/run.bash               :150, :169 drop --format json
docs/skills/codelens/scripts/commit_cloud.py        :18 docstring usage
```

## Acceptance Criteria

- No `--format`, `ndjson`, `csv`, or `table` remains as a codelens OUTPUT reference anywhere
  under docs/skills/codelens/. Surviving hits are only git's own `--format=`, the
  `--group-format` / `--team-map-format` input selectors, and CSV as a `--team-map` input
  format.
- Every codelens invocation in the skill (SKILL.md, references/catalog.md, scripts/run.bash,
  the scripts/commit_cloud.py docstring) runs without `--format json` and works against the
  built binary.
- references/operating.md documents one JSON envelope with `shape`, `semantics`, and
  `transforms`; the 12-member vocabulary with its ranges; that `shape` is fixed per command
  and everything is `table` today except `print-log-command` (`text`); the D6 `--fields`
  retention rules with no format qualifier; and that `schema --command CMD` returns `shape`
  plus a per-column `semantic`.
- references/operating.md states the operator-visible consequence of `--group` (entity is
  reported as `label`, not `filepath`) and that `--team-map` leaves `author` as `person`.
- The errors section asserts errors are always the JSON envelope on stderr, with both
  `--format` clauses removed.
- The code-maat lineage aside at references/operating.md:26 is gone; the enclosure-family
  visualization terminology is INTACT (it is a domain term, not a tool-family reference).
- The skill's Python test suites pass; scripts/run.bash still redirects stderr per analysis
  and keeps its `||` fallback intact.
- No em-dashes, no emojis, no local filesystem paths in any touched file.
- markdownlint clean on every touched markdown file.
- `make build` green.


## Notes

**2026-07-27T14:20:38Z**

Skill updated for the canonical shape-aware envelope (ADR 0008). Dropped --format json from all four invocation sites (SKILL.md, references/catalog.md, scripts/run.bash x2, scripts/commit_cloud.py docstring); git's own --format= left intact. Rewrote references/operating.md 'Output and shaping': one JSON envelope, no default-format framing, documented shape (closed set, table everywhere except print-log-command=text), the 12-member semantic vocabulary as a table with the percentage-vs-ratio and loc-vs-count distinctions, transforms (group degrades entity to label; team-map keeps author as person), D6 --fields retention rules (keeps schema_version/ok/shape, retains transforms when present, narrows semantics), D15 coupling 4-vs-7 semantics, and that schema --command returns shape plus per-column semantic. Errors section: dropped both --format clauses. D16 lineage: removed 'matching code-maat' at operating.md:26 and rewrote the enclosure.md Tornhill csv_as_enclosure_json.py attribution to state the structure/weights behaviour directly. Left the enclosure-family visualization term (domain term, not tool-family) and the two book/D3-template Tornhill references (conceptual foundation + actionable drop-in-template info) as they carry real information. Verified: all six Python test suites pass, bash -n run.bash OK, markdownlint clean, make build green, and the SKILL example command runs exit 0 against the binary.
