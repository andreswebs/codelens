---
id: cod-fj7t
status: closed
deps: [cod-435u]
links: []
created: 2026-07-27T13:12:56Z
type: task
priority: 2
assignee: Andre Silva
parent: cod-z4wu
tags: [codelens, spec-002, docs]
---
# Docs: README and cli-design for the canonical envelope

Bring the repository's own documentation in line with ADR 0008: remove the format matrix
from README.md, document `shape`/`semantics`/`transforms` and the projection rules in
docs/cli-design.md, and annotate the delivered spec-001 documents as superseded rather than
rewriting them.

This is deliberately a separate ticket from the code: the doc surface is coherent on its own
and reviewable as prose, and README.md's output section is the largest single piece of it
(roughly 70 lines). Pre-1.0 with everything landing in one release, so a brief window where
code is ahead of docs is acceptable.

Implements decisions D11 (annotate, do not rewrite, the delivered spec) and documents D1,
D3b, D4, D5, D6, D12, and D15 from docs/specs/002-data-output/plan.md section 2.

Two pre-existing doc bugs, unrelated to this rollout, get fixed here because they sit in the
exact lines being rewritten:

1. docs/cli-design.md sections 6.1 and 6.2 both cite "ADR 0003" for the output
   representation decision. ADR 0003 is error handling; the correct reference is ADR 0008.
   This is a leftover from when the ADR was unnumbered.
2. README.md (around line 247) claims that "With `--format text`, errors render as
   `✗ <message>` and a `hint:` line on stderr instead." There has never been a `text`
   format, and errors have always been the JSON envelope on stderr. The deleted
   `format_error_text` golden proved exactly that.

Note: the ADR restyling and the ADR 0006 back-pointer were completed separately, ahead of
this ticket, and are NOT in scope here.

Reference: docs/specs/002-data-output/plan.md section 7 (this ticket's step list) and
section 3.2 (the vocabulary table to reproduce). Skills: /documentation.

## Design

Depends on the two envelope tickets: the docs describe the shipped behaviour, so write them
against the real output. Generate the examples by RUNNING the built binary rather than
hand-writing JSON, so no example can be wrong.

All line numbers were verified at 2026-07-26. Re-locate by grep.

### 1. README.md

The affected regions, in file order:

- Line 101, `## Output formats`: retitle (for example `## Output`) and delete the whole
  four-row `--format` selection table (lines 103-110) and the "Select with `--format`"
  sentence. Keep the "JSON regardless of whether stdout is a terminal; nothing changes shape
  based on a TTY" point, which is still true and worth stating.
- Line 115, `### JSON envelope (default)`: drop "(default)" (there is no alternative) and
  update the example envelope to carry `shape` and `semantics`. Add a sentence for each new
  field. For `semantics`, state the reason it exists: it is what lets a downstream renderer
  derive a chart without domain knowledge, because codelens authored the data.
- Lines 137-174: DELETE the `### ndjson`, `### csv`, and `### table` subsections entirely,
  including their command examples and the note that "ndjson drops the envelope metadata".
- Line 175, `### Bounding output`: the closing sentence reads "`--fields` applies to JSON
  output only and always retains `schema_version` and `ok`. `--rows` applies to every format
  and truncates after sorting." Restate per D6: `--fields` always retains `schema_version`,
  `ok`, and `shape`, retains `transforms` when present, and narrows `semantics` to the
  projected fields. Drop both format qualifiers. Also refresh the projected-output example,
  which will now carry `shape` and a filtered `semantics`.
- Line 221, the `## Common flags` table: delete the `--format FMT` row.
- Line 247: delete the false `--format text` error paragraph outright (see the description);
  errors are always the JSON envelope on stderr.
- Line 305 area, `## Documentation`: check whether the doc list should now point at
  docs/adr/0008-canonical-output-representation.md.

Add a short subsection documenting `transforms`, since README is where a user learns that
`--group` changes what `entity` means. Cover the D4 rule in one or two sentences: a
transform that destroys a structural affordance degrades the semantic (`--group` makes
`entity` a `label`), one that merely aggregates does not (`--team-map` leaves `author` as
`person`).

Do NOT touch the `## The 20 analyses` section. The apparent mismatch (18 subcommands, "20
analyses") is already explained in place at lines 98-100: `coupling --verbose` and the
`parse` dump round out code-maat's 20 analysis functions. That is intentional, not a bug.

### 2. docs/cli-design.md

This document was already rewritten to the ADR 0008 target state and is AHEAD of the code,
so most of section 6 needs no change. Verify it against the shipped behaviour and fix only
these:

- Section 6.1 (line 175) and section 6.2: change "see ADR 0003" to
  `[ADR 0008](adr/0008-canonical-output-representation.md)`. Use the same relative-link
  style as the existing ADR 0002 reference at line 300.
- Section 7.2, the exit-code table (line 305): remove `unknown_format` from the exit-64
  code-string list.
- Section 8, the `schema --command` example (line 380): remove `"unknown_format"` from the
  `common_error_codes` array. The array is alphabetically sorted; keep it so. Confirm the
  rest of the example against real output from `codelens schema --command coupling`.
- Section 4.2, the global-flag table (line 88): confirm there is no `--format` row (there
  was none as of 2026-07-26) and that `--fields`/`--rows` wording carries no format
  qualifier.
- Section 6.2: add the `transforms` field and the D4/D4a rule, the D15 flag-gating rule, and
  the deliberate schema-versus-envelope asymmetry (the schema declares the command's
  default, the envelope declares the invocation). This asymmetry is the single most likely
  thing for a later reader to mistake for a bug, so state it as a decision.
- Section 6.2: add the 12-member semantic vocabulary as a table, reproducing
  docs/specs/002-data-output/plan.md section 3.2, including that `percentage` is 0-100 while
  `ratio` is 0-1, and that `loc` is distinguished from `count` so a renderer can pick a size
  channel over a frequency channel.
- Section 6: add the D5 note that `print-log-command` declares `shape: "text"` and emits a
  bare command line by design (so it stays copy-pasteable), and that `--version` prints a
  bare string. These are the two documented exceptions to "stdout carries only the
  canonical envelope".

### 3. docs/specs/001-initial-implementation/ (D11)

Annotate, do not rewrite. Add one note near the top of each file:

- requirements.md: the output-format requirements (around lines 113-119, the EARS
  requirements for `--format ndjson`, `--format csv`, and `--format table`) are superseded by
  ADR 0008.
- plan.md: same, for the format reference at line 32.

Wording should make clear the document is a delivered historical record: the requirements
describe what spec 001 shipped, and the format matrix was later removed by ADR 0008. Do not
edit the requirement bodies or renumber anything: closed tickets reference those requirement
IDs, and `docs/specs/learnings.md` cites them.

### 4. docs/specs/learnings.md

Append-only by convention (it is a chronological log). Append a section for this rollout;
do not edit the existing entries that describe the format work (P2-4 at line 273 and the
others). Cover what a future reader needs: why one representation replaced the matrix, why
`semantics` is the load-bearing addition, the flags-not-data rule, the structural-affordance
rule for transforms, and the schema-versus-envelope asymmetry.

### 5. docs/skill-design.md

Sweep for format-era assumptions. Known: line 18 describes codelens as emitting "structured
JSON for 20 evolutionary analyses", which is fine. Check for any `--format` reference or any
claim that the skill selects an output format.

### 6. Cross-file verification

```sh
grep -rn -- '--format' README.md AGENTS.md docs/ | grep -v 'group-format\|team-map-format\|pretty=format\|--format=%'
```

Expected remaining hits after this ticket, all legitimate:

- docs/adr/0006-output-contract.md and docs/adr/0008-canonical-output-representation.md:
  the historical record of the decision, including the back-pointer.
- docs/specs/001-initial-implementation/: the annotated historical spec.
- docs/specs/learnings.md: the append-only log.
- CHANGELOG.md: the removal entry.
- docs/research/code-maat.md: the tool codelens reimplements, whose own CSV output is
  described there.
- docs/skills/codelens/: cleared by the skill ticket, not this one.

Git's own `--format=` / `--pretty=format:` in the log-command examples is unrelated and
stays.

### Doc-style constraints (enforced)

- No em-dashes, no emojis.
- No local filesystem paths; every path is repo-relative and the document must read as
  standalone if published.
- Self-contained: do not reference a "house style", a sibling CLI, or a tool "family".
  code-maat references are allowed where they carry real information (licensing, the test
  corpus, the port's lineage in README and research), but they are not needed to explain
  the output contract.
- markdownlint every touched file:
  `markdownlint-cli2 --config .markdownlint.yaml --fix <file>` from the repo root (the
  project config is ATX-only).

### Files touched

```text
README.md                                              output section rewrite, flag table, error paragraph
docs/cli-design.md                                     ADR refs, unknown_format removal, vocabulary + transforms + text-helper notes
docs/specs/001-initial-implementation/requirements.md  superseded note
docs/specs/001-initial-implementation/plan.md           superseded note
docs/specs/learnings.md                                appended section
docs/skill-design.md                                   format-era sweep
```

## Acceptance Criteria

- README.md documents one JSON output with no format selection table and no ndjson/csv/table
  subsections; its envelope example carries `shape` and `semantics` and was generated by
  running the built binary, not hand-written.
- README.md's `--format FMT` flag-table row is gone, and the false `--format text` error
  paragraph is deleted.
- README.md states the `--fields` retention rules (D6) with no format qualifier on either
  `--fields` or `--rows`, and documents that `--group` changes what `entity` means.
- docs/cli-design.md cites ADR 0008 (not ADR 0003) for the output representation, carries the
  12-member semantic vocabulary as a table, documents `transforms`, the flags-not-data rule,
  the structural-affordance rule, and the schema-versus-envelope asymmetry as a decision.
- `unknown_format` appears nowhere in docs/cli-design.md (neither the exit-code table nor the
  `common_error_codes` example), and the schema example matches real
  `codelens schema --command coupling` output.
- docs/cli-design.md documents `print-log-command` (`shape: "text"`) and `--version` as the
  two deliberate bare-text exceptions.
- Both docs/specs/001-initial-implementation/ documents carry a superseded note; no
  requirement body was edited and no ID renumbered.
- docs/specs/learnings.md has an appended section; no existing entry was edited.
- `grep -rn -- '--format' README.md AGENTS.md docs/` returns only the legitimate hits
  enumerated in the design (ADRs, annotated spec-001, learnings, changelog, code-maat
  research, and the skill pending its own ticket), plus git's own `--format=`.
- No em-dashes, no emojis, no local filesystem paths in any touched file.
- markdownlint clean on every touched file.
- `make build` green.

## Notes

**2026-07-27T13:14:41Z**

Resolved ahead of this ticket, no action needed: a research sweep found a "family-wide taxonomy" phrasing in docs/cli-design.md section 7.2 and in docs/skills/codelens/references/operating.md, which referenced a tool family the docs must not assume. Both now read "the taxonomy in ADR 0002" and were fixed directly on 2026-07-27. Recorded here only so the finding is not re-reported. Note that "enclosure family" in the skill is a legitimate visualization-domain term, not a tool-family reference, and must not be swept.

**2026-07-27T14:27:37Z**

Brought repo docs in line with ADR 0008 (canonical shape-aware envelope). README.md: retitled 'Output formats' -> 'Output', deleted the --format selection table and the ndjson/csv/table subsections, updated the envelope example to carry shape+semantics, added a 'Transforms and what a column means' subsection (--group degrades entity to label; --team-map keeps author as person), restated the D6 --fields retention rules (retains schema_version/ok/shape, keeps transforms, narrows semantics), removed the --format flag-table row and the false '--format text' error paragraph, added the ADR 0008 doc pointer, and updated the schema-introspection blurb to mention shape + per-column semantic and the schema-vs-envelope default. All envelope examples were generated by running the built binary. docs/cli-design.md: fixed BOTH stale 'ADR 0003' output-representation citations to ADR 0008 (one linked a nonexistent adr/0003-canonical-output-representation.md file), removed unknown_format from the section 7.2 exit-code table and the section 8 common_error_codes example (verified the example now matches real coupling schema output), and expanded section 6.2 with the 12-member semantic vocabulary table, the transforms/structural-affordance rule, the flags-track-semantics-not-data rule, and the schema-vs-envelope asymmetry as a decision; added section 6.4 D6 projection rules and a new section 6.5 for the two bare-text exceptions (print-log-command shape:text, --version). spec-001 requirements.md and plan.md got superseded notes at the top (bodies/IDs untouched). Appended a rollout section to learnings.md. Swept docs/skill-design.md and removed --format from its global-flag list. Cross-file --format grep returns only legitimate hits (ADRs, research, annotated spec-001, append-only learnings, 'there is no --format flag' statements). markdownlint clean on all touched files; make build green.
