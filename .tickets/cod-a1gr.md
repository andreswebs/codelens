---
id: cod-a1gr
status: closed
deps: []
links: [cod-l1az, cod-gijq, cod-258k, cod-a6wv, cod-2xyu]
created: 2026-07-15T03:40:57Z
type: feature
priority: 1
assignee: Andre Silva
tags: [codelens, cli, viz-skill, feature, friction]
---
# Feature: path include/exclude globs for analyses and enclosure map

`codelens` has no way to include or exclude files by path, so on a real monorepo
the hotspot and coupling analyses are dominated by machine-generated files (EF
migration snapshots, generated localization `.dart`, `.Designer.cs`, lock files),
which were ~10.6% of all revisions in the test-drive. The only workaround today is
to hand-filter both the git log and the tokei sidecar with an external script.
Add a first-class path filter to `codelens` (a pipeline transform) and to
`enclosure.py` (which owns the external tokei structure `codelens` cannot see).

## Decision

- **Syntax:** glob, gitignore-style with `**` support. Users reason in path shapes
  (`**/Migrations/**`, `*.g.dart`), not regex; globs match the `.gitignore`/tokei
  mental model and avoid regex foot-guns. Matching is against the full entity path.
- **Two surfaces, one feature:** `codelens` gets global `--include`/`--exclude`
  flags implemented as a new pipeline transform; `enclosure.py` gets the same
  `--include`/`--exclude` globs applied to the tokei structure (and weights). Ship
  together so an "authored-only" run is one glob set passed to both.
- **Precedence (both surfaces):** if any `--include` is given, an entity must match
  at least one include to survive; then any `--exclude` match drops it
  (exclude-after-include). No includes means "all included", then excludes apply.

## Part A: `codelens` pipeline filter (Go)

### New transform package

Add `src/internal/transform/filter` mirroring `src/internal/transform/group`:

```go
// Package filter implements the path-filter transform: it keeps only the
// modifications whose entity matches the include/exclude glob rules, applied
// before grouping so raw file paths are matched (not layer names).
package filter

// Spec holds the compiled include and exclude globs.
type Spec struct { Includes, Excludes []glob }

// Compile validates and compiles the raw glob strings; a malformed glob is a
// usage error (exit 2), like ErrInvalidGroup in the group package.
func Compile(includes, excludes []string) (Spec, error)

// Apply keeps modifications whose Entity satisfies the include set (if any) and
// no exclude. Input is not mutated; a new slice is returned.
func Apply(mods []model.Modification, spec Spec) []model.Modification
```

Follow `group.go`'s conventions: a coded `terr` error (`ErrInvalidGlob`, exit 2,
usage error since globs come from flags, never the log), a `maxPatternLen` guard,
and an `Apply` that returns a fresh slice.

### Glob matching dependency (decided)

Go's `path.Match` does **not** support `**`. Use
`github.com/bmatcuk/doublestar/v4` (`doublestar.Match(pattern, path)`), a small,
widely-used, backtracking-safe `**` glob matcher. **This is the approved
decision**: add `github.com/bmatcuk/doublestar/v4` as a project dependency (the
second third-party dep after `urfave/cli/v3`) and use it for all glob matching in
the `filter` package. It is well-scoped and justified by correct `**` semantics;
do not hand-roll a matcher. Run `go get github.com/bmatcuk/doublestar/v4` and
commit the updated `go.mod`/`go.sum`.

### Pipeline wiring

- `src/internal/pipeline/pipeline.go`: add `FilterSpec filter.Spec` (or
  `*filter.Spec`) to `Config`, and run it **first** in `Apply`, before grouping:
  `filter -> group -> temporal -> teammap`. Filtering must precede grouping because
  a glob like `**/Migrations/**` matches file paths, not the layer names that
  grouping produces. Skip the stage when no globs are set (zero-value spec).
- `src/cmd/codelens/commands.go`: add `--include`/`--exclude` as
  **repeatable string-slice** global flags in `globalFlags` (`cli.StringSliceFlag`,
  usable multiple times), with clear `Usage` text noting gitignore-style globs and
  exclude-after-include precedence. In `pipelineConfig`, compile them via
  `filter.Compile(cmd.StringSlice("include"), cmd.StringSlice("exclude"))` and set
  `cfg.FilterSpec`. A malformed glob returns the transform's coded error unchanged
  (exit 2), matching how `--group` errors flow.

The flags are global (like `--group`/`--team-map`), so every analysis honors them
uniformly and each run's `params`/behavior stays self-documenting.

## Part B: `enclosure.py` structure filter (Python)

`codelens --exclude` filters analysis entities (the weights), but the enclosure
maps also read the external tokei structure, which `codelens` cannot touch, so an
excluded generated file would still be drawn (sized by tokei, just weightless).
Add matching `--include`/`--exclude` (repeatable) options to
`docs/skills/codelens/scripts/enclosure.py` that filter **both** the tokei
structure and the weights before `build_tree`, using the same gitignore-style glob
semantics (stdlib `fnmatch` does not do `**`; implement a small `**`-aware matcher,
or match with `pathlib.PurePath.full_match` on Python 3.13+, or a compact
`**`-to-regex translation). Apply exclude-after-include on the normalized paths
(`norm_path`).

This composes with the node-set unification in `cod-l1az` (linked): both edit
`enclosure.py` `main()` and operate on the same `sizes`/`weights` dicts. Land
`cod-l1az` first (or together) and apply the filter to the unified node set.

## Guidance (rides with this ticket)

Add an "authored-only run" recipe to `docs/skills/codelens/references/operating.md`
and the enclosure cards in `references/catalog.md`: pass the same
`--exclude '**/Migrations/**' --exclude '*.g.dart' ...` set to both the `codelens`
analysis and `enclosure.py`, replacing the external-script workaround. List the
common generated-file globs as a starting point, and note that config
(`appsettings*.json`, `*.yml`) and localization sources (`*.arb`, `*.resx`) are
human-authored and should **not** be excluded by default.

## TDD plan (/tdd)

### Go (`filter` package + pipeline + CLI)

1. `TestFilter_ExcludeDropsMatches`: mods for `a.cs`, `x/Migrations/m.cs` ->
   `--exclude '**/Migrations/**'` keeps only `a.cs`.
2. `TestFilter_IncludeThenExclude`: include `**/*.cs`, exclude `**/*.Designer.cs`
   -> `.cs` kept, `.Designer.cs` dropped; a `.dart` file dropped (not included).
3. `TestFilter_NoGlobsKeepsAll`: empty spec is a no-op passthrough.
4. `TestFilter_BadGlob`: an uncompilable glob -> `ErrInvalidGlob` (exit 2).
5. `TestPipeline_FilterBeforeGroup`: a filter dropping `**/Migrations/**` plus a
   group mapping -> Migrations files are gone before grouping (assert via the
   grouped entity set), proving order.
6. CLI e2e: `revisions --exclude '**/Migrations/**'` on a fixture log ->
   excluded entities absent from `rows`; a second `--exclude` also applies
   (repeatable flag).

### Python (`enclosure.py`)

1. `test_enclosure_exclude_filters_structure_and_weights`: tokei + weights each
   containing a `Migrations/` file -> `--exclude '**/Migrations/**'` -> that file
   is in neither the node set nor the leaves; the `wrote ... (N files)` count drops
   accordingly.
2. `test_enclosure_include_then_exclude`: include `**/*.cs`, exclude
   `**/*.Designer.cs` on structure+weights.

Vertical slices: build the `filter` package first (cases 1-4), wire the pipeline
(case 5), wire the CLI (case 6), then the Python `enclosure.py` cases last.

## Files touched

```text
src/internal/transform/filter/filter.go          new: Compile, Apply, ErrInvalidGlob
src/internal/transform/filter/filter_test.go      new
src/internal/pipeline/pipeline.go                  Config.FilterSpec; filter runs first
src/internal/pipeline/pipeline_test.go             filter-before-group order
src/cmd/codelens/commands.go                       --include/--exclude StringSlice flags; compile in pipelineConfig
src/cmd/codelens/*_test.go                          CLI e2e
src/go.mod / src/go.sum                             + github.com/bmatcuk/doublestar/v4 (if chosen)
docs/skills/codelens/scripts/enclosure.py           --include/--exclude on structure+weights
docs/skills/codelens/scripts/enclosure_test.py      Python filter cases
docs/skills/codelens/references/operating.md         authored-only recipe; document the flags
docs/skills/codelens/references/catalog.md           enclosure cards: exclude generated files
docs/cli-design.md                                   document the global filter flags + pipeline order
```

## Acceptance criteria

- `codelens <analysis> --exclude GLOB [--exclude GLOB ...] [--include GLOB ...]`
  filters entities by gitignore-style globs for every analysis, applied before
  grouping; a malformed glob is a usage error (exit 2).
- Include/exclude precedence is exclude-after-include, and the same semantics apply
  in `enclosure.py`.
- `enclosure.py --exclude GLOB` removes matching files from both the tokei
  structure and the weights, so excluded files are drawn on no map.
- Passing one shared glob set to both `codelens` and `enclosure.py` reproduces the
  authored-only maps from the test-drive without any external filter script.
- The chosen glob approach (doublestar or the internal matcher) is stated; if
  doublestar, `go.mod`/`go.sum` are updated and `make build` stays green.
- operating.md/catalog.md/cli-design.md document the flags and the authored-only
  recipe; Markdown passes markdownlint per project standard.

## References

- `src/internal/transform/group/group.go` (pattern for the new package)
- `src/internal/pipeline/pipeline.go` (`Config`, `Apply` order)
- `src/cmd/codelens/commands.go` (`globalFlags`, `pipelineConfig`)
- `docs/skills/codelens/scripts/enclosure.py` (`read_structure`, `main`)
- Linked: `cod-l1az` (both edit `enclosure.py`; land node-set unification first)
- `docs/cli-design.md` (transforms), `references/operating.md` (pipeline transforms)
- Skills: `/golang` (new package, coded errors, RE2/glob safety), `/tdd`,
  `/llm-coding` (no speculative options beyond include/exclude)

## Notes

**2026-07-15T04:13:48Z**

Implemented path include/exclude globs on both surfaces. Go: new src/internal/transform/filter package (Compile/Apply, ErrInvalidGlob exit 2) using github.com/bmatcuk/doublestar/v4 (2nd direct dep); wired as first pipeline stage (filter->group->temporal->team-map) via Config.FilterSpec; global --include/--exclude StringSlice flags compiled in pipelineConfig. Python: enclosure.py gets matching --include/--exclude via a glob_to_regex() translator (stdlib 3.12 lacks ** glob), applied to BOTH the tokei structure and weights before build_tree, with an empty-node-set guard (exit 3). Precedence exclude-after-include on both. Docs updated (cli-design.md flags+pipeline order, operating.md authored-only recipe, catalog.md enclosure note); markdownlint clean. Tests: filter_test.go, pipeline FilterBeforeGroup, 3 CLI e2e, 4 enclosure_test.py cases. make build green; ruff+ty clean. GOTCHA: doublestar matches full path and */? do not cross /, so use **/ to span dirs (bare *.g.dart matches root only).
