---
id: cod-quj5
status: closed
deps: [cod-tyux]
links: []
created: 2026-07-14T03:52:51Z
type: chore
priority: 3
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-6]
---

# P6-2 docs: codelens skill (operate + visualize)

Ship a `codelens` agent skill in the house format (YAML frontmatter + Markdown)
that does two things: encode the operating expertise for the codelens CLI (the
same invariants as the AGENTS.md guide, for skill-aware agents), and turn analysis
output into the behavioral-code visualizations from *Your Code as a Crime Scene*
(2nd ed.): hotspot enclosure maps, change-coupling and communication graphs, churn
and complexity trends, knowledge and ownership maps.

## Delivered

- Skill: [docs/skills/codelens/](../docs/skills/codelens/) (house skill format;
  `skill_ref.py validate` passes). `SKILL.md` frames both branches (operate,
  visualize) and holds the five-step visualization pipeline plus routing table.
- Operating guide: [docs/skills/codelens/references/operating.md](../docs/skills/codelens/references/operating.md)
  encodes the CLI invariants (canonical pipe workflow, `print-log-command`,
  runtime schema discovery, the 20-analysis catalog with aliases, output formats,
  `--fields`/`--rows` bounding, `--group`/`--team-map`/`--temporal-period`
  transforms, analysis-period heuristics, exit-code taxonomy). Self-contained so
  the skill works in any repo, and consistent with the shipped `AGENTS.md`.
- Visualization: `references/` (catalog, enclosure contract, embedding), seven
  self-contained `scripts/` (`uv run`, PEP 723), and D3 templates in
  `assets/templates/`. All ten visualizations implemented; input contracts
  verified against codelens build eaece4f via `codelens schema --command`.
- Design and rationale: [docs/skill-design.md](../docs/skill-design.md).

## Notes

- `operating.md` mirrors AGENTS.md rather than depending on it, since the skill
  runs against other repositories where AGENTS.md is absent; `codelens schema`
  remains the runtime source of truth for exact flags and columns.
- `coupling_graph.py` and `dev_network.py` are verified on synthetic fixtures
  because this repo's history is too shallow to clear the coupling and
  communication thresholds; re-verify on a mature repo.
- Global `~/.markdownlint.yaml` gained `MD003: atx` so markdownlint `--fix` no
  longer rewrites skill headings to setext.

## Acceptance Criteria

- [x] Skill valid and discoverable (`skill_ref.py validate` green; valid frontmatter).
- [x] Operating invariants encoded, consistent with AGENTS.md and the shipped surface.
- [x] Referenced commands match the shipped codelens surface (verified via schema).
- [x] markdownlint-clean.
