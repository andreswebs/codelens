# codelens

codelens mines a git log and runs any of 20 evolutionary code analyses (coupling,
hotspots, churn, ownership, code age, and more), emitting a self-describing JSON
envelope by default. It reads the log on stdin and is strictly read-only: it never
runs git, writes files, or produces side effects.

## Operating codelens

The `codelens` skill is the canonical reference for running the tool and
visualizing its output:

- [docs/skills/codelens/SKILL.md](docs/skills/codelens/SKILL.md) - skill entry
  point (operate and visualize).
- [docs/skills/codelens/references/operating.md](docs/skills/codelens/references/operating.md)
  - CLI operating guide: the canonical pipe workflow, `print-log-command`, runtime
    schema discovery, the analyses catalog, output formats, `--fields`/`--rows`
    bounding, the `--group`/`--team-map`/`--temporal-period` transforms, and the
    exit-code taxonomy.

At runtime, `codelens schema` and `codelens schema --command CMD` are the source
of truth for a command's flags and columns. Do not guess them.

## Building and contributing

`make build` is the canonical gate (validate, then compile); land every change
with it green. See [CLAUDE.md](CLAUDE.md) for the full build, validate, and
contribution guide.

## Repository map

- [docs/skills/codelens/](docs/skills/codelens/) - operate and visualize skill
  (canonical operations reference).
- [docs/skill-design.md](docs/skill-design.md) - visualization skill design and
  rationale.
- [docs/cli-design.md](docs/cli-design.md) - authoritative CLI design.
- [docs/specs/](docs/specs/) - requirements, plan, and the learnings log.
- [docs/research/](docs/research/) - code-maat port reference and urfave/cli
  reference.
- [docs/adr/](docs/adr/) - architecture decision records (numbered, sequential).
- [CLAUDE.md](CLAUDE.md) is a symlink to this file.
