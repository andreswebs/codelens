---
id: cod-84ky
status: closed
deps: []
links: [cod-2xyu]
created: 2026-07-15T20:37:51Z
type: bug
priority: 2
assignee: Andre Silva
tags: [codelens, viz-skill, reporting, bug, friction]
---
# Bug: report.py findings report - drop em-dash titles and fix H1/##title ambiguity

Two defects in `scripts/report.py` (the findings-report assembler created in
cod-2xyu), found while assembling reports across the 29-repo fleet. Both had
post-hoc workarounds; fix them at the source since we own the script.

## F3: section titles hard-code em-dashes

`report.py`'s `SECTIONS` list hard-codes an em-dash separator in the titles (the
Hotspots, Complexity trend, Change coupling, Code age, and Churn titles all used a
spaced em-dash), and `HEURISTIC_NOTE` opened with one too. This conflicts with the
skill's no-em-dash house style, so the fleet run normalized the separator to a colon
in every assembled report after the fact.

### Fix F3

- Replace the em-dash separator with a spaced hyphen (` - `) in the `SECTIONS`
  titles and in `HEURISTIC_NOTE`. (`SOCIAL_DISCLAIMER` and `TEAM_NOTE` carry no
  em-dashes; confirm.)
- `report_test.py::test_sections_in_order` matches title prefixes before the dash
  (for example `## 2. Hotspots`), so it should keep passing; re-run to confirm.

## F4: `# H1` vs `## title` ambiguity can blank the title

`parse_findings` (report.py:103-118) lets both a `# H1` line and a `## title`
block populate the same `title` key. During the fleet run subagents did both (a
duplicate subtitle) or only `## title`; when a `## title` block was later stripped,
the H1 was lost and `assemble` fell back to the generic "Codebase evolutionary
analysis". `reporting.md` compounds this by listing `title` as a reserved `## key`
AND showing the `# H1` line as the title.

### Fix F4

- Code: make the `# H1` line the sole title source. In `parse_findings`, a `## title`
  heading must not create or override the `title` key (skip it; it becomes an ignored
  unknown key). The existing `# H1 -> title` behavior stays.
- Doc: in `reporting.md`, remove `title` from the reserved-key list and state plainly
  that the `# H1` line is the report title. (The example already shows the H1 as the
  title.)
- Test: add a `report_test.py` case asserting that a `## title` block does NOT override
  the H1, and that the H1 renders as the title.

## Files touched

- `docs/skills/codelens/scripts/report.py`
- `docs/skills/codelens/scripts/report_test.py`
- `docs/skills/codelens/references/reporting.md`

## Acceptance criteria

- No em-dash in any `report.py` section title or note; titles use ` - `.
- A findings file with only a `# H1` renders that title; a stray `## title` block is
  ignored and never overrides or blanks the H1.
- `reporting.md`'s reserved-key list no longer includes `title` and states the `# H1`
  line is the title.
- `report_test.py` covers both fixes; all script test suites green; ruff + ty +
  strict pyright clean; edited Markdown passes markdownlint.

## References

- `docs/skills/codelens/scripts/report.py` (SECTIONS, HEURISTIC_NOTE, parse_findings,
  assemble), `report_test.py`, `docs/skills/codelens/references/reporting.md`
- Created report.py in cod-2xyu. Origin: fleet-run friction log (F3, F4).

## Notes

**2026-07-15T21:01:12Z**

Implemented and verified. F3 (em-dashes) landed as part of a repo-wide em-dash
sweep: report.py's SECTIONS titles and HEURISTIC_NOTE now use a spaced hyphen
(" - ") instead of em-dashes; report_test.py's prefix assertions were unaffected.
F4: parse_findings now makes the `# H1` line the sole title source and skips a
`## title` block entirely (it can no longer override or blank the H1); reporting.md
drops `title` from the reserved-key list and states the H1 is the title and its
only source. Two new report_test.py cases: a `## title` block does not override the
H1, and a `## title` block without an H1 is ignored (generic fallback used, block
text never becomes the title). Verified: ruff + ty + strict pyright clean on
report.py and report_test.py; report_test.py 9 tests green; all six script test
suites green; reporting.md markdownlint clean; skill_ref.py validate passes.
