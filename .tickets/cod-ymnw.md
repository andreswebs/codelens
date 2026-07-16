---
id: cod-ymnw
status: closed
deps: []
links: [cod-a6wv, cod-2xyu]
created: 2026-07-16T12:41:35Z
type: task
priority: 2
assignee: Andre Silva
tags: [codelens, viz-skill, docs, reporting]
---
# Docs: harvest findings-writing guidance into reporting.md

Harvest the reusable authoring guidance from the fleet session's `findings-spec.md`
(the prompt handed to findings-writing subagents) into the codelens skill, so the
skill documents how to WRITE a good findings block, not just its format. Done and
recorded here; closed on creation.

## Context

The 29-repo fleet run drove findings-writing subagents with a `findings-spec.md`
prompt. Assessed for incorporation: about half was genuinely additive (authoring
discipline the skill had no home for), and about half either duplicated the skill
(reserved-key list, guardrails) or carried client specifics (Bizee/Particle41,
`data/github/<repo>/` paths, atomic 24-month, full-history list). reporting.md
already defines the findings-file *format* and interpretation.md defines how to
*read*; neither covered how to *write* the prose. The verbatim spec was also already
stale (it still listed the `title` reserved key that cod-84ky removed), which is
exactly the single-source-of-truth drift to avoid.

## What shipped

Added a tight "Writing a findings block" subsection to
`references/reporting.md` (in the Findings file section, loaded at SKILL.md step 6),
generic and pointer-only:

- Ground every claim in the digest's numbers; name the specific file, coupled pair,
  author, or fractal/degree value with its metric; no filler.
- Be honest about thin signal (do not invent findings).
- Separate generated/reference data from authored code, pointing at the
  reference-data note in operating.md.
- The `risk_choices` triad shape (`Accept:` / `Prioritise now:` /
  `Mitigate over time:`).
- Keep blocks tight (2 to 6 sentences), read per interpretation.md, and honour its
  guardrails (pointer, not a re-listing).
- A closing line naming the digest-first, one-agent-per-repo subagent workflow.

Deliberately NOT copied: the reserved-key list and guardrails (owned by reporting.md
and interpretation.md; kept as single sources of truth) and all client specifics. No
standalone `findings-spec.md` was added to the skill: a verbatim copy would
re-duplicate two references and rot.

## Acceptance criteria (met)

- `reporting.md` has a "Writing a findings block" subsection with the additive
  authoring directives, referencing interpretation.md for guardrails and
  operating.md for the reference-data note.
- No reserved-key list or guardrail text is duplicated; no client-specific content.
- Markdown passes markdownlint; skill_ref.py validate passes; no em-dashes.

## References

- `docs/skills/codelens/references/reporting.md` (Findings file section)
- Source: the fleet session's `findings-spec.md` (external, not added to the skill)
- Related: cod-2xyu (report.py + reporting.md), cod-a6wv (digest-first pipeline).
