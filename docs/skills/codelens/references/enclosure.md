# Enclosure diagram: data contract

The flagship visualization: a zoomable circle-packing map where each file is a
circle sized by lines of code and colored by a weight (change frequency for a
hotspot map, ownership for a knowledge map, age for a code-age map). Built by
[`enclosure.py`](../scripts/enclosure.py). Loaded from
[SKILL.md](../SKILL.md) via the catalog when building any enclosure family map.

## Two inputs

**Structure source (optional): `tokei --output json`.** Defines the node set and
the circle radius. Shape: top-level keyed by language, each with a `reports`
array of `{stats: {code, comments, blanks}, name: <path>}`; `Total` is skipped.
Per-file size is `stats.code`; path is `name`. Run tokei from the log's root so
`name` is repo-relative with forward slashes, matching codelens `entity`.

**Weight source (required): a codelens analysis JSON.** The color overlay,
joined on path. The envelope is `{schema_version, ok, analysis, row_count,
rows: [...]}`; rows carry snake_case columns (`entity`, `n_revs`, `main_dev`,
`age_months`, ...). The weight column is selectable (`--weight-col`).

## Role asymmetry

The two sources are not symmetric. Tokei is the **skeleton** (which files exist,
how big); codelens is a **color overlay** joined onto it, defaulting to 0 for
files with no recorded change. This mirrors Tornhill's `csv_as_enclosure_json.py`
(`--structure` vs `--weights`) and is why stable and third-party files still
appear, as cool circles, giving the whole-codebase view the book describes.

## Two modes

|                  | Full mode (tokei present)          | Degraded mode (no tokei)             |
| ---------------- | ---------------------------------- | ------------------------------------ |
| Node set         | every file tokei sees              | only entities with a recorded change |
| Radius (`size`)  | tokei `code`                       | the weight value                     |
| Color (`weight`) | joined, normalized; 0 if unmatched | normalized weight                    |
| Whole tree shown | yes                                | no (changed files only)              |

Same tree-builder and template downstream; only leaf population differs.

## Join and normalization

- Join key: the raw path string, after stripping a leading `./` from both sides.
- Normalize the numeric weight to `0.0 .. 1.0` as `value / max(value)` across all
  matched files. `0.0` is coolest, `1.0` the hottest.
- Categorical weight (knowledge map): carry the category (e.g. `main_dev`) on the
  leaf instead of a normalized number; the client assigns one color per category.

## Tree building

Both inputs reduce to a flat `path -> {size, weight}` map. Split each path on
`/`; create an intermediate `{name, children: []}` node per directory segment;
the file is a leaf `{name, size, weight}`. Weight lives only on leaves;
directories take their radius from D3's `.sum()` over descendants.

## D3 leaf shape

D3 zoomable circle packing consumes a nested hierarchy. Leaf keys are kept
identical to Tornhill's script so his template is drop-in compatible:

```json
{
  "name": "codelens",
  "children": [
    {
      "name": "src",
      "children": [{ "name": "coupling.go", "size": 210, "weight": 0.34 }]
    },
    { "name": "README.md", "size": 40, "weight": 0.0 }
  ]
}
```

`d3.hierarchy(data).sum(d => d.size)` sets radii; `d.data.weight` drives color.

## Artifacts

1. `hotspots.json` - the hierarchy above. Reusable across the enclosure family
   (hotspot / knowledge / age) since only `weight` changes.
2. `<name>.html` - the JSON injected as a `<script>` blob, with D3 loaded from a
   CDN. Opens directly in a browser (no `python -m http.server` CORS step the book
   needs); requires network to fetch D3. For live viewing and iframe embedding
   only, not exported to a static image.

## Edge cases

- **Renames:** codelens does not track them; a renamed file's history splits, and
  old-path revisions will not join the current tree (dropped in full mode).
- **History-only files** (deleted, or outside the current tree): present in the
  weight source, absent from tokei; excluded in full mode, shown in degraded mode.
- **Path-root drift:** if the log was generated from a subdirectory, entities and
  tokei names lose a shared root. Run tokei at the log's root, or pass
  `--path-prefix`.
- **Non-code files:** tokei buckets Markdown/JSON/YAML as languages; kept by
  default (whole-codebase view), excludable with tokei's `--exclude`.
- **`--group`:** grouped entities are component names with no `/`, giving a
  shallow root -> component tree. Works unchanged, just flatter.

## Defaults

- Size = tokei `code` (excludes comments and blanks).
- Keep all languages; exclude noise with tokei's own `--exclude` when needed.
