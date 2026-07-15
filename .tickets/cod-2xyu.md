---
id: cod-2xyu
status: closed
deps: [cod-gijq, cod-9yuv]
links: [cod-84ky, cod-a1gr, cod-a6wv]
created: 2026-07-15T10:01:20Z
type: feature
priority: 1
assignee: Andre Silva
tags: [codelens, viz-skill, reporting, feature]
---
# Skill: markdown report assembler (sequenced findings report)

Turn a full codelens run into one sequenced findings **report**: a single
self-contained plain-markdown document that walks the crime-scene investigation in
the book's order, embeds the static figures, and carries the agent's interpretation
of each analysis. Plain markdown only - NO MARP, no slide deck (an earlier MARP
decision was reverted). The structure, flow, and interpretation are fixed; only the
prose is authored per run.

## Decision and generation model

Hybrid: `scripts/report.py` assembles the report deterministically (fixed sequence,
embedded figures, headline numbers pulled from the analysis JSON) and slots in an
agent-authored **findings file** (the interpretation prose). Structure/order/figures
= script; prose = agent. This preserves the skill's determinism boundary while
keeping the reading a matter of judgment (the book insists findings are probabilistic
and context-dependent).

Depends on the degraded-renderers ticket (embeds its SVG figures) and the
interpretation ticket (findings follow `references/interpretation.md` and its
guardrails).

## `scripts/report.py`

- Inputs: the analysis JSONs (for headline numbers and to know which analyses ran),
  the rendered degraded figures (SVG files from `treemap.py`/`pair_matrix.py` and the
  existing static charts), and the agent-authored findings file.
- Output: `report.md` - a flowing narrative document: an H1 title, H2 sections in the
  fixed sequence below, prose, and each figure embedded as **inline `<svg>`** so the
  file is self-contained (one portable file, no external asset references).
- Follow the Python static-lane conventions (PEP 723 header, `uv run`, a
  `die(msg,code)` helper, exit codes 0 ok / 2 usage / 3 empty, a trailing
  `wrote {out} (...)` stderr line, an envelope-or-bare JSON loader).
- **Self-containment via inline SVG caveat (state in the ticket):** inline `<svg>` in
  markdown renders under pandoc/HTML pipelines but GitHub's markdown sanitizer strips
  it. The report targets HTML/pandoc rendering, not GitHub preview. (Read each figure
  SVG and inline its `<svg>...</svg>`; strip any XML prolog/doctype.)

## Findings file (agent-authored)

One block per analysis: `headline` (one line) + `interpretation` (prose per
`interpretation.md`) + optional `now_what`. Plus report-level `title`, `subtitle`
(and analysis window), `executive_summary` (business framing), and closing
`risk_choices`. Use a simple, documented shape (markdown-with-headings or YAML);
`report.py` slots each block into its section. A missing block yields a clear
placeholder, not a crash.

## Fixed sequence (H2 sections; book funnel + Ch-7 business-first)

1. Title + analysis window.
2. Executive summary + situational awareness (summary numbers; business headline,
   Red/Yellow/Green if provided).
3. Hotspots (treemap + top offenders).
4. Complexity trend (per top hotspot).
5. Change coupling, file + component (pair_matrix) - architecture.
6. Knowledge / ownership (treemap by owner) + social disclaimer.
7. Fractal / fragmentation.
8. Communication / Conway network (pair_matrix) + team-aggregation note + disclaimer.
9. Code-age (treemap by age) - stabilization framing.
10. Churn trend - business backdrop.
11. Commit word cloud - labeled heuristic / conversation-starter.
12. Closing - risk choices (accept / prioritise low-risk / mitigate) + next actions.

Lead/support emphasis (from the interpretation reference): summary, hotspots,
complexity trend, coupling, knowledge map are the leads; code-age, fractal, Conway,
churn, word cloud are supporting.

## Guardrails `report.py` MUST enforce

- A "not for performance evaluation" disclaimer on the social section (knowledge,
  fractal, Conway).
- Default team/component aggregation language for the Conway section.
- The commit word cloud labeled "heuristic only".

These come from `interpretation.md`'s misuse-guardrails section; the report cannot
omit them.

## `references/reporting.md` (new)

Document: the fixed sequence template; the findings-file schema; the inline-SVG
self-containment approach and the GitHub-sanitizer caveat; and the guardrails the
generator enforces. No MARP.

## SKILL.md

Add a final, optional step "Compose the report" pointing at `reporting.md`. The
skill description already claims "reports", so no description change is required
(confirm the wording still fits).

## TDD plan (/tdd)

Drive `report.py` with fixture JSONs, fixture SVG figures, and a fixture findings
file; assert on the emitted markdown:

1. `test_report_sections_in_order`: emits an H1 then the expected H2 sections in the
   fixed order; section count matches the analyses present.
2. `test_report_inlines_svg`: a figure SVG -> an inline `<svg>` inside its section;
   no external image reference anywhere (self-contained).
3. `test_report_findings_slotted`: findings blocks land in the correct sections;
   headline + interpretation present.
4. `test_report_guardrails_present`: the social disclaimer and the word-cloud
   "heuristic" label always appear.
5. `test_report_missing_findings_placeholder`: absent findings -> a visible
   placeholder, exit 0 (not a crash).
6. `test_report_empty_inputs`: no analyses -> exit 3.

Vertical slices: sequence/skeleton first, then SVG inlining, then findings slotting,
then guardrails.

## Files touched

```text
docs/skills/codelens/scripts/report.py            new (assembler)
docs/skills/codelens/scripts/report_test.py       new
docs/skills/codelens/references/reporting.md       new (template, findings schema, guardrails)
docs/skills/codelens/SKILL.md                      new final step "Compose the report"
```

## Acceptance criteria

- `report.py` produces one self-contained plain-markdown `report.md` (figures inline
  as SVG, no external references), sequenced per the template, business-first, with
  the agent findings slotted in.
- The social disclaimer, Conway team-aggregation language, and word-cloud "heuristic"
  label are always present.
- No MARP syntax anywhere (no front-matter directive `marp:`, no slide separators as
  slide breaks, no MARP image/background directives).
- Interactive HTML artifacts are still emitted separately (this is an additional
  output, not a replacement).
- `reporting.md` and the new SKILL step document it; the TDD cases pass; Markdown
  passes markdownlint per project standard.

## References

- `docs/skills/codelens/SKILL.md` (pipeline, determinism boundary),
  `references/embedding.md` (inline SVG), `references/catalog.md`
- Depends on: the degraded-renderers ticket (`treemap.py`/`pair_matrix.py` SVGs) and
  the interpretation-reference ticket (`interpretation.md` findings + guardrails)
- Skills: `/tdd`, `/llm-coding` (no speculative options; plain markdown only)

## Notes

**2026-07-15T10:43:57Z**

Implemented + verified end-to-end. New scripts/report.py assembles ONE self-contained plain-markdown report (no MARP, no slides): H1 title + 11 fixed H2 sections in the book funnel (exec summary/situational-awareness -> hotspots -> complexity -> coupling -> knowledge -> fractal -> communication -> code-age -> churn -> word cloud -> recommended actions). Hybrid model: report.py pins sequence/embedding/numbers, the agent supplies prose via a findings markdown file (reserved ## keys; missing block -> neutral placeholder, never a crash). Figures from --figures-dir (conventional stems) are embedded INLINE as <svg> (XML prolog/doctype stripped) so the file is self-contained with zero external image refs; --summary renders situational-awareness tiles. Guardrails are injected unconditionally and cannot be omitted: social 'not a productivity ranking' disclaimer on knowledge/fractal/communication, team-aggregation note on communication, 'heuristic only' label on the word cloud. Exit 0/2/3 (3 = nothing to assemble). New references/reporting.md documents the pipeline, sequence, findings schema, inline-SVG self-containment + GitHub-sanitizer caveat, and the guardrails; SKILL.md gains an optional step 6 'Compose the report'. TDD: report_test.py (7) asserts section order, inline-svg + no external refs, findings slotting, guardrails always present, missing-findings placeholder, summary tiles, empty-input exit 3. Verified on keeper-core: assembled an 11-section report inlining 9 real figures (0 external refs), and pandoc rendered it to a 1.5MB self-contained HTML with all 9 SVGs intact. All 5 script suites green; ruff/ty/strict-pyright clean; markdownlint clean; skill_ref validate passes.
