---
id: cod-a6wv
status: closed
deps: []
links: [cod-a1gr, cod-2xyu, cod-ymnw]
created: 2026-07-15T20:37:53Z
type: task
priority: 2
assignee: Andre Silva
tags: [codelens, viz-skill, reporting, tooling]
---
# Task: promote fleet reporting pipeline (run.bash + digest.py) into the codelens skill

Promote the fleet reporting pipeline (built while driving codelens across a 29-repo
fleet) into the codelens skill: `scripts/digest.py` and `scripts/run.bash`, plus
wiring them into the report docs. Done and verified in this session; recorded here for
the design lineage and closed on creation.

## What shipped

### scripts/digest.py (+ digest_test.py)

Condenses a repo's per-analysis JSON into a compact `digest.md`: summary counts,
window dates, hotspots split code vs docs/config, change-coupling pairs, ownership
concentration, most-fragmented files, code-age range, churn totals + biggest period,
author-communication pairs, and commit vocabulary. It is the grounding input for a
findings write-up, so no one reads the multi-megabyte analysis JSON. Conformed to the
Python static lane: PEP 723 header, `from __future__ import annotations`, full type
annotations with `cast` at the JSON boundary, `die()`/exit codes 0/2/3, argparse
(positional data dir, `-o` default `<dir>/digest.md`, presence-gate exits 3 when no
analysis files), trailing `wrote ...` stderr line. New `digest_test.py` (10 cases).
The original carried no client specifics; promotion was conforming it to the lane.

### scripts/run.bash

Single-repo driver that automates `reporting.md` step 1: generates windowed +
full-history logs, runs every analysis (the built-in generated-file excludes on the
entity-centric analyses, `communication`/`summary` unfiltered, `code-age` on full
history), renders the degraded static figures under `figs/` with the conventional
stems, runs tokei, and writes `digest.md`. Read-only against the repo and best-effort:
a figure with no data is skipped, not fatal. Genericized from the client run:
`SCRIPT_DIR` derived from `BASH_SOURCE` (no hardcoded skill path), portable
`months_before` (GNU and BSD/macOS `date`), a `--full-history` flag (the F2 remedy for
stale/front-loaded repos), and `--repo`/`--out`/`--months`/`--exclude` flags. Follows
the /bash conventions (strict mode, `echo_stderr`, quoted `${braces}`, long flags);
shellcheck + shfmt clean.

### Docs wired

`reporting.md` pipeline step 1 now leads with `bash scripts/run.bash --repo PATH --out
out/` and step 2 points at `out/digest.md` as the findings grounding; `SKILL.md` step
6 names `run.bash`. The previously hand-waved "run the analyses and render the figures
into one directory" step now has a real driver.

### Not promoted

`run-fleet.sh` (the multi-repo loop) is client-specific (mani inventory, engagement
paths); the generic loop over `run.bash` is trivial and left to the operator.

## Verification (done)

- digest.py: ruff + ty + strict pyright clean; 10 tests green.
- run.bash: shellcheck + shfmt clean; smoke-tested end-to-end against the codelens repo
  (all analyses + digest produced; 8/10 figures rendered; the two skipped were
  legitimate empty-input coupling/communication on a young single-author repo);
  `--full-history` path exits 0.
- Docs: markdownlint clean (repo-local `.markdownlint.yaml`).

## Acceptance criteria (met)

- `scripts/digest.py` + `digest_test.py` present; three checkers + tests green.
- `scripts/run.bash` present; shellcheck/shfmt clean; produces a report-ready directory
  (analysis JSON + `figs/` + `digest.md`) that `report.py` consumes.
- `reporting.md` step 1 and `SKILL.md` step 6 reference `run.bash`.

## References

- `docs/skills/codelens/scripts/run.bash`, `scripts/digest.py`, `scripts/digest_test.py`
- `docs/skills/codelens/references/reporting.md`, `docs/skills/codelens/SKILL.md`
- Feeds `report.py` (cod-2xyu); applies the `--exclude` flags from cod-a1gr. Origin:
  fleet-run friction log (F11).

