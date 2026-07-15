---
id: cod-gijq
status: closed
deps: []
links: [cod-l1az, cod-a1gr, cod-258k]
created: 2026-07-15T10:01:20Z
type: feature
priority: 1
assignee: Andre Silva
tags: [codelens, viz-skill, feature]
---
# Skill: degraded static renderers (treemap + pair matrix)

Three of the skill's visualization families (enclosure hotspot/knowledge/age maps,
the change-coupling graph, the communication network) render only as interactive D3
HTML. Those cannot be embedded in a static document. Add static, no-D3 renderers
that produce the same information as embeddable SVG from the analysis JSON, so the
markdown report assembler (sibling ticket) can inline them. The five existing static
charts (churn, fractal, word cloud, complexity trend, summary) already fit and are
untouched.

## Decision

Report-optimized static forms, rendered through the existing matplotlib "static
lane": enclosure family -> **treemap** (area = size, colour = weight); coupling and
communication -> **adjacency-matrix heatmap and/or ranked bar**. Faithful
circle-packing / force-directed replicas were rejected (layout-heavy; static
node-link graphs are unreadable hairballs). SVG is canonical (inline-embeddable in
the report); PNG remains available by output extension.

## The static lane (match these conventions)

From `scripts/churn.py`, `fractal.py`, `commit_cloud.py`, `complexity_trend.py`:

- PEP 723 header (`# /// script` ... `dependencies = [...]` ... `# ///`), run via
  `uv run`. `squarify` is ALREADY a dependency of `fractal.py`, so the treemap
  layout library is in-lane.
- `# pyright: reportUnknownMemberType=false` (add `reportMissingTypeStubs=false`
  when a dependency ships no stubs).
- `import matplotlib; matplotlib.use("Agg")` inside the function; `fig.savefig(out)`
  picks format by extension (`.svg` or `.png`).
- A `rows(path)` loader that accepts either a codelens envelope `{ "rows": [...] }`
  or a bare list, and `-` for stdin.
- A `die(msg, code)` helper; exit codes 0 ok, 2 usage/bad-input, 3 empty; a trailing
  stderr line `wrote {out} (<summary counts>)`.
- Palette: added `#4b9e5f`, deleted `#d1495b`; categorical uses `tab20` with a
  `color_of = {author: cmap(i % 20)}` sorted-author map.

## codelens JSON shapes (inputs)

- `revisions`: `entity, n_revs`
- `main-developer`: `entity, main_dev, added, total_added, ownership`
- `code-age`: `entity, age_months`
- `coupling`: `entity, coupled, degree, average_revs`
- `communication`: `author, peer, shared, average, strength`
- tokei sidecar: `{lang: {reports: [{name, stats: {code}}]}}`

## Renderer 1: `scripts/treemap.py` (enclosure family)

Static enclosure-family view. Flags MIRROR `enclosure.py`: `--weights` (analysis
JSON, or `-`), `--weight-col` (default `n_revs`), `--structure` (tokei JSON;
optional), `--categorical`, `--invert`, `--path-prefix`, `-o`.

- Layout via `squarify`: rectangle area = tokei `code` LOC; when `--structure` is
  omitted, degrade to sizing by the weight value.
- Colour: numeric modes -> a normalized heat scale over the weight (respect
  `--invert`, so low age = hot); `--categorical` -> per-owner `tab20` (reuse the
  `color_of` pattern from `fractal.py`).
- **Node-set rule:** follow the unified structure-first rule from the linked ticket
  `cod-l1az` - with `--structure`, the tokei files are the node set for every mode
  (a file absent from the weights renders neutral: cold for numeric, a reserved
  sentinel category for categorical); without `--structure`, the weights are the
  node set. Keep this identical to `enclosure.py` post-`cod-l1az` so the static and
  interactive maps agree.
- Honor the same include/exclude globs as `cod-a1gr` (linked) so an authored-only
  report matches the authored-only interactive maps - or accept already-filtered
  inputs; state which in the completion note.
- Emit SVG (canonical) or PNG by extension; trailing `wrote {out} (N files)`.

## Renderer 2: `scripts/pair_matrix.py` (coupling + communication)

Generic symmetric pair-strength renderer, reused for both graph families. Flags:
`--pairs FILE` (or `-`), `--a-col`, `--b-col`, `--weight-col`, `--top N` (default
e.g. 30), `-o`.

- Build the top-N entities by total involvement, render an annotated adjacency-matrix
  heatmap (`imshow`) of the weight, basename labels, and/or a ranked horizontal bar
  of the top pairs.
- Coupling: `--a-col entity --b-col coupled --weight-col degree`.
- Communication: `--a-col author --b-col peer --weight-col strength`. Carry the
  social guardrail from the interpretation ticket: label the axis/title as
  coordination (not performance), and note team aggregation.
- Symmetric fill (both (a,b) and (b,a) cells); empty input -> exit 3.
- Emit SVG (canonical) or PNG by extension; trailing `wrote {out} (...)`.

## TDD plan (/tdd)

Drive each script with fixture JSON; assert on observable output - the intermediate
hierarchy (add a `--json-out` to `treemap.py` mirroring `enclosure.py`), the
`wrote ... (...)` counts, exit codes, and that the output file exists and is
non-empty SVG. Do NOT assert matplotlib internals. Vertical slices, one test then
one implementation step:

1. `test_treemap_structure_first_nodeset`: weights covering 2 of 3 tokei files,
   numeric, `--structure` -> hierarchy has all 3 leaves; uncovered file is neutral.
2. `test_treemap_categorical_colours`: `--categorical` -> per-owner colours; node set
   per the structure-first rule.
3. `test_treemap_degraded_no_structure`: no `--structure` -> node set = weights;
   sized by weight.
4. `test_treemap_svg_and_png`: `.svg` and `.png` outputs both produced and non-empty.
5. `test_pair_matrix_topN_symmetric`: coupling fixture -> top-N selected; matrix is
   symmetric; degree values placed correctly.
6. `test_pair_matrix_communication_cols`: communication fixture via `--a-col author
   --b-col peer --weight-col strength` renders; title/label carries the coordination
   framing.
7. `test_pair_matrix_empty`: empty `rows` -> exit 3.

## Files touched

```text
docs/skills/codelens/scripts/treemap.py            new (squarify treemap; enclosure family)
docs/skills/codelens/scripts/treemap_test.py       new
docs/skills/codelens/scripts/pair_matrix.py        new (coupling + communication matrices/bars)
docs/skills/codelens/scripts/pair_matrix_test.py   new
docs/skills/codelens/references/catalog.md         note the static counterpart per interactive card
docs/skills/codelens/references/embedding.md       update: interactive families now have static SVG counterparts
```

Confirm the repo convention for Python script tests before adding files (follow the
pattern used by the ticket `cod-dxdk`/`cod-l1az` test work, or a sibling `*_test.py`
runnable via `uv run` with PEP 723 inline metadata).

## Acceptance criteria

- Each currently-interactive family (enclosure, coupling, network) has a static
  renderer producing an embeddable **SVG** (PNG available by extension) from the
  analysis JSON, with no D3 and no browser.
- `treemap.py` matches `enclosure.py`'s flags and the structure-first node-set rule
  (`cod-l1az`); `pair_matrix.py` serves both coupling and communication via column
  flags and carries the social guardrail framing for communication.
- Scripts match the static lane (PEP 723, Agg, envelope-or-bare loader, `die`/exit
  codes 0/2/3, `wrote ...` line, palette).
- The TDD cases pass; `embedding.md`/`catalog.md` note the static counterparts;
  Markdown passes markdownlint per project standard.

## References

- `docs/skills/codelens/scripts/enclosure.py` (flags + node-set to mirror),
  `fractal.py` (squarify + `tab20` colour pattern), `churn.py`/`commit_cloud.py`
  (lane conventions)
- `docs/skills/codelens/references/catalog.md`, `references/embedding.md`
- Linked: `cod-l1az` (enclosure node-set unification - mirror it), `cod-a1gr`
  (include/exclude globs). Consumed by the markdown report assembler ticket.
- Skills: `/tdd` (fixture-driven, vertical slices), `/llm-coding` (no speculative
  flags beyond what the report needs)

## Notes

**2026-07-15T10:25:00Z**

Implemented + verified. Two new self-contained scripts in the matplotlib static lane: treemap.py (enclosure family) reuses enclosure.py's exact helpers, glob include/exclude, and structure-first node set (missing weight -> cold / (unowned) sentinel), rendering a squarify treemap (area=LOC or degraded weight; colour = YlOrRd heat for numeric respecting --invert, or tab20 per category with grey unowned; --top default 50; --json-out for tests). pair_matrix.py is a generic symmetric top-N adjacency-matrix heatmap reused for coupling (--a-col entity --b-col coupled --weight-col degree) and communication (--a-col author --b-col peer --weight-col strength, with --note for the coordination guardrail); annotated, symmetric fill, --json-out. Both emit SVG (canonical) or PNG by extension, envelope-or-bare loader, die/exit 0/2/3, 'wrote ...' stderr line. TDD: treemap_test.py (9) + pair_matrix_test.py (4) via uv run, asserting node-set/sentinel/invert-cold/filter/top/symmetry/values, all green. Verified end-to-end on keeper-core data: hotspot/knowledge/code-age treemaps + coupling/communication matrices all render valid SVG/PNG (colour leads over size; config lockstep + l10n trio visible in the matrix). Docs: catalog.md gains a **Static:** command per interactive card; embedding.md notes the degraded counterparts. ruff check clean (ruff format not enforced in repo); markdownlint clean; skill_ref validate passes.
