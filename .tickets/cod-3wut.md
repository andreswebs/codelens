---
id: cod-3wut
status: closed
deps: []
links: [cod-9yuv]
created: 2026-07-15T03:40:57Z
type: task
priority: 3
assignee: Andre Silva
tags: [codelens, docs, viz-skill, friction]
---
# Docs: fix SVG/PNG format wording and warn code-age needs full history

Two documentation-only fixes surfaced by a full end-to-end run of `codelens`
v0.0.1 and the `docs/skills/codelens` visualization skill against a large real
repository. No code changes. These are split out from the code tickets so they
can land immediately at zero risk.

## Context

The visualization skill's SKILL.md and catalog describe each script's output
formats and each analysis's semantics. Two statements are wrong or incomplete and
misled the operator during the test-drive.

## Problem 1: "SVG and PNG" overstates what the static scripts write

`docs/skills/codelens/SKILL.md` step 4 says the static scripts "write SVG and PNG
directly", and `docs/skills/codelens/references/catalog.md` lists `Formats: SVG,
PNG` for the churn, fractal, word cloud, complexity trend, and summary cards. In
fact each script writes a **single** file whose extension selects the format
(`-o x.svg` **or** `-o x.png`, never both in one run). Verified: `churn.py`,
`fractal.py`, `commit_cloud.py`, `complexity_trend.py`, and `summary` via
`churn.py --summary` each emit exactly the one file named by `-o`.

### Fix 1

- In `docs/skills/codelens/SKILL.md` step 4, change the "Static" bullet so it
  reads that each script writes one file and the `-o` extension picks the format
  (`.svg` or `.png`); to get both, run the script twice.
- In `docs/skills/codelens/references/catalog.md`, change every `Formats: SVG,
  PNG` line to `Formats: SVG or PNG (the -o extension picks the format)`.

## Problem 2: code-age is silently capped by the log window

`code-age` reports age in months since last modification, measured from the log's
time zero. When the log is scoped with `--after` (the skill's own "one year is a
good default" heuristic in `docs/skills/codelens/references/operating.md`), every
entity's `age_months` is bounded by the window: a 12-month window makes every file
report as `<= 12` months old, regardless of true age. The code-age card gives no
warning, so the resulting code-age map looks uniformly young and is misread.

### Fix 2

- In `docs/skills/codelens/references/catalog.md`, add a caveat line to the
  "Code-age map" card: code-age should be run against **full history**, not a
  window scoped with `--after`, because age is measured from the log's earliest
  commit and a scoped window caps every file's reported age at the window length.
- In `docs/skills/codelens/references/operating.md`, in the "Analysis period"
  section, note the one exception to the "scope with `--after`" guidance:
  `code-age` wants full history.

## Out of scope

- No change to any script or to `codelens` itself. The complexity-trend PNG
  support and the analysis behavior are unchanged; this ticket only corrects the
  prose describing them.

## Acceptance criteria

- SKILL.md step 4 and every catalog `Formats:` line for a static script state
  "SVG or PNG (extension picks format)"; no doc claims a single run writes both.
- The code-age catalog card and the operating.md analysis-period section both warn
  that code-age needs full history and that `--after` caps reported age.
- All edited Markdown passes `markdownlint-cli2 --config ~/.markdownlint.yaml`
  (project standard; there is no repo-local config).

## References

- `docs/skills/codelens/SKILL.md` (step 4, output formats)
- `docs/skills/codelens/references/catalog.md` (static-script cards, code-age card)
- `docs/skills/codelens/references/operating.md` (analysis-period heuristics)

## Notes

**2026-07-15T04:33:28Z**

Docs-only. SKILL.md step 4 static bullet: each script writes one file, -o extension picks .svg or .png, run twice for both. catalog.md: all 5 static-script Formats lines changed to 'SVG or PNG (the -o extension picks the format)'; added a 'Full history required' caveat to the Code-age map card. operating.md Analysis period: noted code-age is the exception to --after scoping (age measured from earliest commit, window caps reported age). markdownlint-cli2 auto-discovered repo-local /workspace/.markdownlint.yaml (ticket assumed none) -> 0 errors. make build green.
