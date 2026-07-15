---
id: cod-zeva
status: closed
deps: []
links: [cod-3wut]
created: 2026-07-15T20:29:35Z
type: task
priority: 2
assignee: Andre Silva
tags: [codelens, viz-skill, docs, friction]
---
# Docs: fleet-run skill fixes - exclude scope, windowing, hotspot reading, stderr, findings-write gotcha

Documentation-only fixes to the `docs/skills/codelens` skill surfaced by a full
end-to-end run of `codelens` v0.0.2 across a 29-repo fleet. No code or script
changes: the tool and the promoted scripts already behave correctly (see the
pipeline-promotion ticket); only the prose that tells an operator how to run and
read codelens is wrong or incomplete. Split out so it can land immediately at zero
risk.

## F1: "authored-only" excludes must cover every entity-centric analysis

`operating.md` frames the generated-file exclude guidance around the hotspot and
coupling maps, so it is natural to filter only `revisions`. But regenerated
artifacts pollute the churn, effort, fragmentation and ownership analyses just as
badly. Concrete case from the fleet: platform-api's `absolute-churn` was dominated
by a single +852k-line spike (2026-04-27) from regenerating the `juris-rules` JSON
(top commit word `regenerate`, 1635x), which made the churn trend meaningless until
that generated JSON was excluded. The same pollution distorts `entity-effort`,
`fragmentation` and `main-developer`.

### Fix F1

- In `operating.md`, in the authored-only / include-exclude guidance, state that the
  same `--exclude` set must be passed to every **entity-centric** analysis:
  `revisions`, `coupling`, `sum-of-coupling`, `main-developer`, `code-age`,
  `absolute-churn`, `entity-effort`, `fragmentation`. It must **not** be passed to
  `communication` (an author graph) or `summary` (whole-repo counts), so authorship
  and totals stay whole. Use the juris-rules churn-spike as the motivating example.
- Cross-reference `scripts/run.bash` as the canonical authored-only implementation
  (it applies the built-in exclude set to exactly those entity-centric analyses and
  leaves communication/summary unfiltered).
- Echo the "applies to churn and effort too, not just the maps" point in
  `reporting.md` where excludes are mentioned.

## F2: a trailing window starves front-loaded / stale repos

Scoping a window with `--after` assumes activity clusters near the present. Repos
with an early burst and a late trickle of commits get a nearly empty window:
in the fleet, quantum had 17 in-window commits of 12252 total, checkout 8/238,
compliance-system 27/203, atomic-resources 10/267.

### Fix F2

- In `operating.md`'s analysis-period section, document the pitfall and the remedy:
  analyze **full history** for stale or front-loaded repos (there is no recency
  tension when the repo is inactive). Point at `scripts/run.bash --full-history`,
  which does this and already warns when the windowed log is empty.
- Record that **auto-widening** the window (auto-expanding when in-window commits
  fall below a threshold) was **considered and declined**: it silently changes the
  analysis window from a heuristic, making two runs of the same command
  incomparable. An explicit lever plus the empty-window warning is preferred.

## F7: human-authored config/docs legitimately top the hotspot list

By raw revision count, human-authored config and docs (for example `CLAUDE.md`,
`.env.example`, `.github/workflows/*.yml`, `composer.json`, `README`/docs files)
often outrank source files. The skill correctly says NOT to exclude human-authored
config (unlike generated artifacts), so this is a recurring interpretation burden.

### Fix F7

- In `interpretation.md`'s hotspot reading block, add a note: by raw revisions,
  human-authored config/docs can top the list; this is expected and must not be
  excluded. For a legible code hotspot list, use `scripts/digest.py`, which splits
  the hotspots into code vs docs/config, rather than excluding config.

## F8: figure scripts print success to stderr

The figure scripts (`churn.py`, `fractal.py`, `treemap.py`, `pair_matrix.py`,
`commit_cloud.py`, `complexity_trend.py`) print progress to **stderr on success**:
the trailing `wrote ... .svg (...)` line, and uv's `Installed N packages`. A wrapper
that treats non-empty stderr as failure will false-positive.

### Fix F8

- In `reporting.md`, note that figure-script success is judged by **exit code 0**,
  never by stderr being empty; `scripts/run.bash` follows this.
- Echo a one-line version in `operating.md`'s exit-code section.

## F10: host hooks may block writing findings.md (gotcha)

Observed with subagents in one engagement: a host hook guarded the Write tool on
files named `findings.md` / report files, forcing a fallback. This is environmental,
not a codelens constraint, but it is worth a gotcha so a blocked write is recognized.

### Fix F10

- In `reporting.md`'s findings-file section, add a gotcha: some host environments run
  a hook that guards writes to `findings.md` or report filenames. If a write is
  blocked, fall back to writing via a shell heredoc, or write to a differently-named
  file and rename. This is a host-environment behavior, not a codelens rule.

## Out of scope

- No change to `codelens` or to any script. run.bash/digest.py already implement the
  exclude-everywhere and `--full-history` behaviors (pipeline-promotion ticket). No
  auto-widen feature. No engine-level code-vs-docs classifier (F7 stays a reading
  note pointing at digest.py's existing split).

## Acceptance criteria

- `operating.md` authored-only guidance names all eight entity-centric analyses and
  states communication/summary are left unfiltered, with the juris-rules churn-spike
  example; `reporting.md` echoes it.
- `operating.md` analysis-period section documents the front-loaded/stale pitfall and
  the `--full-history` remedy, and records auto-widen as considered-and-declined.
- `interpretation.md` hotspot block has the docs/config reading note pointing at
  `digest.py`'s code-vs-docs split.
- `reporting.md` documents stderr-on-success (judge by exit code) and the findings.md
  write-hook gotcha; `operating.md` exit-code section echoes the stderr note.
- All edited Markdown passes `markdownlint-cli2` against the repo-local
  `.markdownlint.yaml`.

## References

- `docs/skills/codelens/references/operating.md` (authored-only excludes,
  analysis-period, exit codes)
- `docs/skills/codelens/references/reporting.md` (excludes echo, stderr note,
  findings-file gotcha)
- `docs/skills/codelens/references/interpretation.md` (hotspot reading block)
- `docs/skills/codelens/scripts/run.bash`, `scripts/digest.py` (canonical behaviors)
- Origin: fleet-run friction log (F1, F2, F7, F8, F10)

## Notes

**2026-07-15T20:43:41Z**

Docs-only, implemented and verified. operating.md: (F1) the Authored-only run
section now states the `--exclude` set must reach every entity-centric analysis
(revisions, coupling, sum-of-coupling, main-developer, code-age, absolute-churn,
entity-effort, fragmentation) and NOT communication/summary, with the juris-rules
+852k churn-spike example and a run.bash cross-reference; (F2) the Analysis period
section documents the front-loaded/stale pitfall (17/12252 example), the
`run.bash --full-history` remedy, and records auto-widen as considered-and-declined;
(F8) the exit-codes section gained a one-liner that the render scripts print success
to stderr, judge by exit code, pointing at reporting.md. reporting.md: (F8) a
"judge figures by exit code, not stderr" note in the Figures section; (F10) a
findings-file gotcha that a host hook may block writing findings.md, with the
heredoc/rename fallback. interpretation.md: (F7) a Hotspot map reading note that
human-authored config/docs legitimately top the list and must not be excluded,
pointing at digest.py's code-vs-docs split. New prose is em-dash-free (house style);
existing em-dashes left untouched (out of scope). markdownlint-cli2 (repo-local
.markdownlint.yaml) 0 errors on all three files; skill_ref.py validate passes
(exit 0). No code or script changes.
