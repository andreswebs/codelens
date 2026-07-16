---
id: cod-258k
status: closed
deps: []
links: [cod-a1gr, cod-gijq]
created: 2026-07-15T20:37:52Z
type: feature
priority: 2
assignee: Andre Silva
tags: [codelens, viz-skill, feature, friction]
---
# Feature: treemap area-domination warning for large reference-data files

The hotspot treemap sizes each rectangle by tokei LOC, so a few large reference-data
or spec files (for example `naics_*.json`, `public/v0/openapi.yaml`) dominate the map
area and drown the real code even when they are not hot. Generic generated-file globs
miss them because names/paths are repo-specific.

Decision (settled via a /question-me interview): the dominating files are a mix of
artifacts that slipped past the excludes AND legitimately huge reference data; size
alone cannot classify them. Therefore the tool must **surface** the domination and let
the analyst decide, and must never silently alter the map.

## Behavior (treemap.py)

- Default = **warn, do not alter**. Rectangle area stays true tokei LOC.
- Trigger: any single mapped file whose LOC is **> 10% of total mapped LOC**. The
  total is the post-exclude, structure-first node set, so the check is iterative:
  exclude one offender, re-run, and the next one surfaces honestly.
- Payload: list each offender, capped at the **top 5**, as
  `dominant: <path> <pct>% (<LOC> LOC)`.
- Channel: **stderr**; exit code stays **0**.
- Scope: all colour modes (hotspots, `--categorical` knowledge, `--invert` age),
  **gated on `--structure` being present** (skip when area degrades to the weight,
  where a file dominating by area is the real signal, not noise). Implement in
  `treemap.py`; if `treemap.py` and `enclosure.py` share the structure-loading path,
  put the check there so `enclosure.py` warns too; otherwise `treemap.py` is required
  and `enclosure.py` parity is a follow-up.

## Explicitly rejected (do not build)

- No size-threshold auto-exclude flag: it reintroduces the silent-drop risk (a legit
  large source file would vanish with no trace). The existing `--exclude` is the one
  remedy.
- No log-scale or capped-area rendering: it breaks the treemap's "area = quantity"
  contract, a worse distortion than the domination it hides.

## Skill docs (ship with the feature)

- `operating.md`: a "reference data" exclude recipe - when the warning fires, add the
  named path to `--exclude` and re-run - placed beside the authored-only excludes
  guidance. NOTE: the doc-fixes ticket edits the same operating.md excludes section;
  coordinate so they do not collide.
- `interpretation.md`: a reading note in the hotspot/enclosure block - treemap area =
  tokei LOC, so large reference/spec files occupy area without being hot, and the
  warning names them. Fold in **F6**: raw tokei language totals mislead (platform-api
  read as 1.26M "JSON" LOC vs 35k PHP, almost all generated). Add a one-line hook from
  the catalog card.

## TDD plan (/tdd)

- Fixture weights + tokei structure with one file > 10% of total LOC: assert a
  `dominant:` line on stderr naming that file with its pct and LOC; exit 0.
- No file over threshold: no `dominant:` line emitted.
- `--structure` absent: no domination warning regardless of weight skew.
- Warning fires in `--categorical` mode too.
- More than five offenders: capped at five.
- Percentages are computed on the post-exclude node set: a file dropped by `--exclude`
  does not count toward the total and cannot trigger the warning.

## Files touched

- `docs/skills/codelens/scripts/treemap.py`, `treemap_test.py`
- `docs/skills/codelens/scripts/enclosure.py`, `enclosure_test.py` (only if the
  structure loader is shared; else a noted follow-up)
- `docs/skills/codelens/references/operating.md` (exclude recipe)
- `docs/skills/codelens/references/interpretation.md` (reading note, F6)
- `docs/skills/codelens/references/catalog.md` (one-line hook)

## Acceptance criteria

- `treemap.py` emits the domination warning per the spec (> 10% single file, top 5,
  `dominant:` on stderr, exit 0), in all colour modes, only when `--structure` is
  given; area and colouring are unchanged.
- `enclosure.py` warns too if it shares the structure loader (otherwise a follow-up is
  noted in the ticket on close).
- `operating.md` has the reference-data exclude recipe; `interpretation.md` has the
  reading note with F6 folded in; the catalog card has the hook.
- No auto-exclude flag and no area rescale exist.
- `treemap_test.py` (and `enclosure_test.py` if touched) cover the cases; script suites
  green; ruff + ty + strict pyright clean; edited Markdown passes markdownlint.

## References

- `docs/skills/codelens/scripts/treemap.py`, `enclosure.py`
- `docs/skills/codelens/references/operating.md`, `interpretation.md`, `catalog.md`
- Degraded renderers created in cod-gijq; `--exclude` flag from cod-a1gr (used by the
  recipe). Origin: fleet-run friction log (F5, F6).

## Notes

**2026-07-16T12:30:45Z**

Implemented and verified. Added `warn_domination(leaves)` to BOTH `treemap.py` and
`enclosure.py` (the leaf-building is identical across the two self-contained scripts,
so parity was done here, not deferred): it sums `leaf["size"]` (tokei LOC) over the
post-exclude node set and prints `dominant: <path> <pct>% (<LOC> LOC)` on stderr for
each file over 10% of the total, most-dominant first, capped at five; exit code stays
0 and the map is never altered. Called from `main` only when `--structure` is given
(when area degrades to the weight, a file dominating by area is the real signal). No
size-threshold auto-exclude and no area rescale were added (both explicitly rejected;
`--exclude` is the one remedy). Tests: treemap_test.py gained a TestDomination class
(6 cases: warns with pct+LOC, no-warning under threshold, gated on --structure,
categorical mode, capped at five, excluded file not counted) -> 15 total;
enclosure_test.py gained 2 parity cases -> 13 total. Docs: operating.md gained a
"Reference-data domination" recipe beside the authored-only section;
interpretation.md's hotspot block gained a reading note with F6 folded in (tokei area
vs hotness; the 1.26M "JSON" LOC vs 35k PHP example); catalog.md's hotspot card gained
a one-line hook. Verified: ruff + ty clean; strict pyright clean on treemap.py and
enclosure.py with matplotlib+squarify resolved in a venv (a bare run reports 16
pre-existing matplotlib Unknowns in draw(), unrelated to this change, confirmed by a
stash check); all six script suites green; edited Markdown markdownlint clean;
skill_ref.py validate passes.
