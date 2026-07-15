---
id: cod-l1az
status: closed
deps: []
links: [cod-a1gr, cod-gijq]
created: 2026-07-15T03:40:57Z
type: bug
priority: 2
assignee: Andre Silva
tags: [codelens, viz-skill, bug, friction]
---
# Bug: enclosure node-set differs between size and categorical modes

`docs/skills/codelens/scripts/enclosure.py` picks a different **node set** (the
set of files drawn) depending on the map mode, so the hotspot/age maps and the
knowledge map disagree about which files exist. This surprised the operator during
the test-drive (hotspot map drew 9,145 files, knowledge map drew 11,017 for the
same repo) and it makes the maps non-comparable.

## Problem

In `main()` (around lines 154-179):

- **Size modes** (hotspot, code-age; numeric weight, `--structure` given): the node
  set is the **tokei structure** (`sizes`), and each file's weight defaults to `lo`
  when absent from the weights (`numeric.get(p, lo)`). So every source file tokei
  knows about is drawn; files never changed in the window appear cold.
- **Categorical mode** (knowledge map, `--categorical`): the node set is the
  **weights** (`{p: {...} for p, v in weights.items()}`), and `--structure` is
  ignored entirely (no size lookup, uniform `size: 1`).

Result: the two families draw different files and the knowledge map ignores tokei
sizes even when a sidecar is supplied.

## Decision: approach A (unify on structure-first for all modes)

Chosen over "unify on weights-first" (would silently change hotspot maps to
only-files-changed-in-window, dropping the faithful whole-codebase enclosure) and
"document the difference" (leaves the surprise in place). When `--structure` is
given, the tokei structure is the node set for **every** mode; an entity absent
from the weights renders neutral. When `--structure` is omitted, all modes degrade
to the weights as the node set (as the numeric degraded branch does today). One
node-set rule, one missing-weight rule, across both families.

### Behavior after the change

With `--structure`:

- Node set = tokei files (all modes). Circle radius = tokei `code` LOC.
- Numeric modes: weight = normalized value; a file absent from the weights gets the
  neutral end (cold) rather than `lo`-as-if-present. Keep `--invert` semantics.
- Categorical mode: `category` = the weight value; a file absent from the weights
  gets a dedicated sentinel category (e.g. `category: null` or a reserved
  `"(unowned)"` string) that the template renders in a neutral grey. The template
  at `docs/skills/codelens/assets/templates/circle-packing.html.jinja` must map the
  sentinel to a neutral swatch (confirm the template's categorical color logic and
  extend it; keep the existing per-developer colors for real categories).

Without `--structure` (degraded): node set = weights, for all modes (categorical
keeps uniform `size: 1`; numeric sizes by weight as today).

### Why this composes with the exclude feature

Once the map is structure-first consistently, filtering the tokei structure (the
approach in the include/exclude ticket) removes a file from **every** map
identically. The two tickets both edit `enclosure.py` `main()`; see the linked
include/exclude ticket and coordinate so the node-set change lands first (or in the
same change), since the exclude filter is applied to the same `sizes`/`weights`
dicts this ticket restructures.

## TDD plan (/tdd)

`enclosure.py` is stdlib-only and its observable output is the intermediate
hierarchy (exposed via `--json-out`) plus the trailing `wrote ... (N files)`
count. Test through those, not internals.

1. `test_size_mode_nodeset_is_structure`: weights covering 2 of 3 tokei files,
   numeric, `--structure` given -> hierarchy has all 3 leaves; the uncovered file
   has the neutral (cold) weight, not `lo`.
2. `test_categorical_uses_structure_when_given`: `--categorical` with `--structure`
   -> node set = the 3 tokei files (not just the weighted ones); each leaf has a
   `size` from tokei LOC (not uniform 1); the uncovered file carries the sentinel
   category.
3. `test_categorical_neutral_sentinel`: a file in tokei but absent from weights ->
   its leaf's `category` is the reserved sentinel.
4. `test_degraded_mode_unchanged`: no `--structure` -> node set = weights for both
   numeric and categorical (guard that the degraded path is untouched).
5. `test_count_matches_nodeset`: the `wrote ... (N files)` count equals the node-set
   size for each mode.

Vertical slices: write case 1, make it pass, then case 2, etc. Get to green before
touching the template color logic (a separate refactor step verified by rendering).

## Files touched

```text
docs/skills/codelens/scripts/enclosure.py                     unify node-set/leaf build
docs/skills/codelens/scripts/enclosure_test.py                new (repo convention)
docs/skills/codelens/assets/templates/circle-packing.html.jinja   neutral sentinel color
docs/skills/codelens/references/enclosure.md                  document the unified node-set rule
docs/skills/codelens/references/catalog.md                    knowledge-map card: now uses tokei sizes
```

## Acceptance criteria

- With `--structure`, all three maps (hotspot, code-age, knowledge) draw the same
  node set (the tokei files) and size circles by tokei LOC.
- A file present in tokei but absent from the weights renders neutral: cold for
  numeric modes, a reserved sentinel category shown in grey for categorical.
- Without `--structure`, every mode degrades to the weights as the node set, and
  the existing degraded numeric behavior is unchanged.
- `enclosure.md` and the knowledge-map catalog card document the unified rule;
  Markdown passes markdownlint per project standard.
- The TDD cases pass; `enclosure.py` stays stdlib-only and self-contained.

## References

- `docs/skills/codelens/scripts/enclosure.py` (`main()`, lines ~154-195)
- `docs/skills/codelens/assets/templates/circle-packing.html.jinja` (color logic)
- `docs/skills/codelens/references/enclosure.md`, `references/catalog.md`
- Linked: the path include/exclude ticket (both edit `enclosure.py`).
- Skills: `/tdd`, `/llm-coding`.

## Notes

**2026-07-15T04:03:48Z**

Unified enclosure.py node set structure-first (approach A). --structure now the node set for ALL modes incl. categorical (was weights-only, uniform size 1); circles sized by tokei code everywhere. Missing-weight rule uniform: numeric -> weight 0.0 (cold end; also fixes latent --invert bug where norm(lo)=1.0 drew unchanged files HOT on code-age); categorical -> reserved '(unowned)' sentinel (UNOWNED_CATEGORY). Template gained real categorical coloring via leafFill(): schemeTableau10 per dev, neutral grey #5a6270 for the sentinel, unchanged sequential scale for numeric. Degraded (no --structure) path untouched. Tests: enclosure_test.py drives the script as a subprocess over --json-out + the file count (stdlib-only, no runner in make build; run 'python3 -m unittest enclosure_test'). Docs: enclosure.md + knowledge-map catalog card document the unified rule. Unblocks cod-a1gr Part B (both edit main() / same sizes+weights dicts).
