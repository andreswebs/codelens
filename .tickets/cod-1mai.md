---
id: cod-1mai
status: closed
deps: []
links: []
created: 2026-07-29T03:44:15Z
type: feature
priority: 2
assignee: Andre Silva
tags: [codelens, skill, viz, run-bash, interactive]
---
# run.bash: emit the interactive HTML lane and every producible analysis, additively

`run.bash` never emits the interactive HTML artifacts, and never runs six of the
table-shaped analyses. Make it produce everything the skill can produce, additively,
without changing anything `report.py` consumes.

## Evidence

A full run against a 30,800-commit repository on 2026-07-28 produced **zero HTML
files**:

```text
22 json    18 analyses + tokei + schema captures + flint specs
14 svg     9 artifact-lane figures + 5 flint-lane figures
 2 md      digest.md, report.md
 0 html    <- nothing
```

Exactly three scripts in `docs/skills/codelens/scripts/` are referenced NOWHERE in
`run.bash`, and they are precisely the whole interactive lane:

```text
enclosure.py       the zoomable circle-packing map (the flagship visualization)
coupling_graph.py  change-coupling force graph
dev_network.py     communication / developer network
```

Every interactive card in SKILL.md's routing table currently gets a static substitute
instead: `treemap.py` stands in for the three enclosure maps, `pair_matrix.py` for both
graphs.

## Why this is worth fixing rather than accepting

The current behaviour is coherent by design: `run.bash` exists to feed `report.py`,
which embeds figures inline as SVG, and interactive HTML cannot be inlined that way.
`references/reporting.md` correctly calls what it builds the "degraded static" figures.

The problem is the consequence: **the flagship visualization is unreachable from the
one-command path.** `run.bash --repo X --out Y` gives you everything except the three
interactive artifacts, and nothing in the output directory hints they exist. Producing
them requires knowing to invoke `enclosure.py` by hand with the right weight column and
the tokei sidecar. SKILL.md steps 2 and 3 do document that, but a user who runs the
pipeline reasonably assumes the pipeline is the pipeline.

Second consequence, discovered the same way: because nothing invokes them, the three
interactive scripts are **untested end to end**. An end-to-end pipeline run exercises
neither. `enclosure.py` was verified working by hand afterward (9,276 files, 1.4MB
HTML); `coupling_graph.py` and `dev_network.py` remain unexercised by any automated
path.

The precedent for the fix already exists in the same file: the Flint spec lane is
additive, lands in its own subdirectory, is guarded by `command -v deno`, and leaves
everything above it untouched. Copy that shape.

## Design

All line numbers are against `docs/skills/codelens/scripts/run.bash` at commit
`62dca76`; re-locate by grep if they have drifted.

## Governing constraint: ADDITIVE ONLY

Nothing in this ticket may change a byte that `report.py` consumes. Concretely:
`${OUT}/figs/*.svg`, `${OUT}/summary.json`, `${OUT}/digest.md`, and every existing
`${OUT}/*.json` must be identical before and after. A run on a machine with no new
optional dependency must produce exactly what it produces today. The Flint lane
(lines 246 onward) is the model: its own subdirectory, its own stderr files, a
`command -v` guard, and a best-effort failure that warns without failing the run.

## 1. The five interactive HTML artifacts

New directory `${OUT}/interactive/`, and a helper mirroring `render_fig` (line 217)
but writing into it:

```bash
INTERACTIVE="${OUT}/interactive"
mkdir -p "${INTERACTIVE}"

render_interactive() {
    local name="${1}" script="${2}"
    shift 2
    echo_stderr "  html ${name}"
    uv run "${SCRIPT_DIR}/${script}" "${@}" 2>"${INTERACTIVE}/${name}.stderr" ||
        echo_stderr "[${REPO_NAME}] interactive ${name} failed (see interactive/${name}.stderr)"
}
```

Five artifacts from three scripts. The three `enclosure.py` calls mirror the existing
`treemap.py` calls at lines 225-230 exactly (same weights, same weight column, same
`--categorical` / `--invert`, same `EXCLUDES`), because they are the same three maps in
a different renderer:

```bash
render_interactive hotspots enclosure.py \
    --weights "${OUT}/revisions.json" --weight-col n_revs \
    "${STRUCT[@]}" "${EXCLUDES[@]}" \
    --json-out "${INTERACTIVE}/hierarchy.json" \
    -o "${INTERACTIVE}/hotspots.html"
render_interactive knowledge enclosure.py \
    --weights "${OUT}/main-dev.json" --weight-col main_dev --categorical \
    "${STRUCT[@]}" "${EXCLUDES[@]}" -o "${INTERACTIVE}/knowledge.html"
render_interactive age enclosure.py \
    --weights "${OUT}/code-age.json" --weight-col age_months --invert \
    "${STRUCT[@]}" "${EXCLUDES[@]}" -o "${INTERACTIVE}/age.html"
render_interactive coupling coupling_graph.py \
    --coupling "${OUT}/coupling.json" --soc "${OUT}/soc.json" \
    -o "${INTERACTIVE}/coupling.html"
render_interactive network dev_network.py \
    "${DN_COMM[@]}" -o "${INTERACTIVE}/network.html"
```

Details that matter:

- `--json-out` on the FIRST enclosure call only. `references/enclosure.md` documents the
  hierarchy as "reusable across the enclosure family (hotspot / knowledge / age) since
  only `weight` changes", so one copy is the right number. Emitting it three times would
  write three near-identical multi-megabyte files.
- `coupling_graph.py --soc` is optional and supplies node size from
  `sum-of-coupling`; `soc.json` is already produced at line 167, so pass it.
- DO NOT pass `--min-degree` or `--min-strength`. Both default to `0.0`, and the JSON is
  already threshold-filtered by codelens itself (`coupling` defaults to
  `--min-coupling 30`). Passing them again would filter twice.
- `dev_network.py` accepts `--schema` for the aggregation guard on its node-size totals.
  `${SCHEMA_DIR}/communication.schema.json` is already captured at line 187, so build a
  guarded array exactly as `PM_COUPLING` does at lines 195-200:

```bash
declare -a DN_COMM=(--communication "${OUT}/communication.json")
if [[ -s "${SCHEMA_DIR}/communication.schema.json" ]]; then
    DN_COMM+=(--schema "${SCHEMA_DIR}/communication.schema.json")
fi
```

## 2. FIX A LATENT BUG WHILE HERE: guard `--structure`

`enclosure.py` and `treemap.py` both crash with an unhandled `FileNotFoundError`
traceback when `--structure` points at a missing file:

```text
FileNotFoundError: [Errno 2] No such file or directory: '/nonexistent.json'
```

`run.bash` tolerates tokei failing (line 177: `|| echo_stderr "tokei failed"`) but then
passes `--structure "${OUT}/tokei.json"` UNCONDITIONALLY to all three `treemap.py`
calls. So on any machine where tokei fails, all three static enclosure figures die with
a traceback instead of degrading to the documented no-tokei mode.

Introduce one guarded array and use it for BOTH lanes:

```bash
declare -a STRUCT=()
if [[ -s "${OUT}/tokei.json" ]]; then
    STRUCT=(--structure "${OUT}/tokei.json")
fi
```

Then replace `--structure "${OUT}/tokei.json"` in the three existing `treemap.py` calls
with `"${STRUCT[@]}"`. This is behaviour-preserving when tokei succeeds, which is the
only case the goldens and the reference report cover, so it does not violate the
additive-only constraint. Note `set -o nounset`: an empty array expansion is safe under
`"${STRUCT[@]}"` on bash 4.4+, but if the project must support older bash, append to an
already-populated array as the `PM_COUPLING` comment at lines 192-194 explains.

## 3. Run the six missing analyses and emit their specs

`run.bash` runs 11 of the 17 table-shaped analyses. Not run: `author-churn`, `authors`,
`entity-churn`, `entity-ownership`, `main-developer-by-revisions`,
`refactoring-main-developer`.

`flint_spec.py` is generic over analyses and handles ALL SIX. Verified 2026-07-28 by
piping each into it:

```text
authors                      -> Scatter Plot (vegalite)
entity-ownership             -> Bar Chart (vegalite)
entity-churn                 -> Bar Chart (vegalite)
author-churn                 -> Bar Chart (vegalite)
main-developer-by-revisions  -> Bar Chart (vegalite)
refactoring-main-developer   -> Bar Chart (vegalite)
```

So add them via the EXISTING `run_analysis` and `emit_spec` helpers, and render them
with the existing `render_flint`. Bound the entity-ranked ones with `--rows` on the same
reasoning the current `emit_spec` comment gives (lines 267-273): a deliberate top-N by
the analysis's own ranking beats Flint's canvas-budget truncation.

`authors` deserves a note: it is the only one whose override is a Scatter Plot rather
than a bar, and `entity` is deliberately unbound on that chart (see spec 003 section
3.4.1). Do not add a colour channel to it; at this repo's scale that requests thousands
of colours.

Apply `EXCLUDES` to all six: every one is entity-centric or author-centric over
entities, matching the rule stated in the comment at lines 161-164.

## 4. Explicitly OUT of scope, with reasons

Record these so a reviewer does not read them as omissions:

- **`messages`** stays unrun. `codelens schema --command messages` shows
  `--expression` is `required=true`, and there is no defensible default regex. It is a
  targeted query, not a survey.
- **`--vendor-d3` for `coupling_graph.py` and `dev_network.py`.** All three scripts
  inject D3 through the same `{{D3}}` template placeholder, but only `enclosure.py` has
  `--vendor-d3` to inline a local bundle; the other two hardcode
  `D3_CDN = '<script src="https://cdn.jsdelivr.net/npm/d3@7"></script>'`. So two of
  three artifacts cannot be made offline-capable. That is a real consistency gap and a
  separate ticket; this one must not silently start vendoring for one script only.
- **PNG duplicates of the static figures.** The static scripts pick format from the `-o`
  extension and would need a second invocation each. Doubling render time for a format
  nothing currently consumes is not justified here.
- **`report.py`.** It must not learn about `${OUT}/interactive/`. Inline SVG is its
  contract; HTML artifacts are for browsing and iframe embedding.

## 5. Documentation to update

- `references/reporting.md` step 1: it currently describes `run.bash` as producing "the
  analyses, the degraded static figures, and a grounding `digest.md`". Add the
  interactive artifacts and say plainly that `report.py` does not consume them.
- `SKILL.md` step 6: same sentence, and it is worth stating that the one-command path
  now yields the interactive lane too, since the current text sends a reader to steps 2
  and 3 to get it by hand.
- `run.bash`'s own header comment (lines 3-6) describes what it produces; extend it.
- `references/catalog.md`: the interactive cards can now name the pipeline path as well
  as the by-hand invocation.

## Acceptance Criteria

## Interactive artifacts

- `${OUT}/interactive/` contains five HTML files: `hotspots.html`, `knowledge.html`,
  `age.html`, `coupling.html`, `network.html`.
- Each opens in a browser and renders. `hotspots.html` shows a zoomable circle-packing
  map; `coupling.html` and `network.html` show force graphs.
- `${OUT}/interactive/hierarchy.json` is written exactly once, by the hotspots call.
- Each artifact has its own `${OUT}/interactive/<name>.stderr`, and a single failure
  warns without failing the run, matching `render_fig` and `emit_spec`.

## The additive guarantee (the criterion that matters most)

- Run the pipeline against a fixture repo before and after the change: every
  `${OUT}/figs/*.svg`, `${OUT}/*.json`, and `${OUT}/digest.md` is BYTE-IDENTICAL.
  This is the regression test for the whole ticket.
- `report.py` output is unchanged, and it still knows nothing about
  `${OUT}/interactive/`.
- A run with no `deno` on PATH still behaves exactly as today plus the new HTML.

## The tokei guard

- With `tokei.json` present, the three `treemap.py` figures are byte-identical to
  today's (proving the `STRUCT` array refactor is behaviour-preserving).
- With `tokei.json` absent or empty, `treemap.py` and `enclosure.py` both degrade to the
  documented no-structure mode instead of emitting a `FileNotFoundError` traceback.
  Test by deleting the file between the analysis and figure phases, or by pointing the
  run at a repo where tokei fails.

## The six added analyses

- `${OUT}/` gains JSON for `author-churn`, `authors`, `entity-churn`,
  `entity-ownership`, `main-developer-by-revisions`, `refactoring-main-developer`.
- Each has a Flint spec under `${OUT}/flint/`, and an `${OUT}/figs/flint-*.svg` when
  `deno` is on PATH.
- `authors` renders as a Scatter Plot with `entity` unbound; the others as Bar Charts.
- `EXCLUDES` is applied to all six.
- `messages` is still NOT run, and a comment says why.

## Hygiene

- `bash -n` clean, and `shellcheck -S warning` clean if available.
- The three interactive scripts are now exercised by a pipeline run, closing the
  end-to-end test gap that motivated this ticket.
- Docs updated per design section 5; markdownlint clean against the project config.
- `make build` green.


## Notes

**2026-07-29T04:00:33Z**

run.bash now emits the interactive lane and the six missing analyses, additively.

INTERACTIVE LANE: new render_interactive helper (mirrors render_fig) writes five HTML artifacts into ${OUT}/interactive/: hotspots/knowledge/age from enclosure.py (same weights, weight-col, --categorical/--invert and EXCLUDES as the three treemap.py calls), coupling.html from coupling_graph.py (--soc soc.json for node size), network.html from dev_network.py (--schema guarded via a new DN_COMM array, same shape as PM_COUPLING). --json-out on the hotspots call only, so hierarchy.json is written exactly once. No --min-degree/--min-strength: codelens already threshold-filtered both JSONs. Each artifact gets its own interactive/<name>.stderr; a failure warns and never fails the run. This closes the end-to-end test gap: coupling_graph.py and dev_network.py are now exercised by every pipeline run.

TOKEI GUARD (latent bug): tokei was best effort but --structure was passed unconditionally, so a failed tokei killed all three static maps. Two distinct failure modes confirmed: a MISSING file raises an unhandled FileNotFoundError traceback, an EMPTY one (what a failed tokei leaves, since the redirection creates the file first) gives a clean 'invalid JSON' error. One guarded STRUCT=() array covers both and is shared by the static and interactive map lanes. Verified: with a failing tokei stub, all six maps now degrade to no-structure mode and the run completes.

SIX ANALYSES: authors, author-churn, entity-churn, entity-ownership, main-developer-by-revisions, refactoring-main-developer, all with EXCLUDES, each with a Flint spec and a flint-*.svg. Chart types match the ticket (authors = Scatter Plot, rest = Bar Chart). messages still NOT run, with a comment saying why. DEVIATION FROM THE TICKET, deliberate: --rows 100 applied to authors and entity-churn ONLY. The other three sort by entity ASCENDING (internal/analysis/ownership.go:70, maindevbyrevs.go:77, refactoringmaindev.go:74), not by a rank, so --rows would silently cut alphabetically; the existing emit_spec comment's own rule says bound by the analysis's ranking, and Flint at least self-labels its truncation. author-churn is one row per author. Reasoning recorded in a comment above the calls.

ADDITIVE GUARANTEE: byte-identity as literally specified is unsatisfiable and was so before this change - matplotlib stamps <dc:date> with wall-clock time and emits random clip-path/xlink:href ids per run, and commit_cloud.py is nondeterministic (two runs on the same parse.json differ). Verified instead under a normalization that masks the date, [A-Za-z]+[0-9a-f]{10} ids and the --out path: every pre-existing *.json, digest.md, summary.json and figs/*.svg is identical before and after, except cloud.svg which differs run-to-run at HEAD too. report.py output identical under the same normalization and contains zero references to interactive/. Verified the no-deno path: interactive HTML still produced, specs emitted, no flint SVGs, no failure.

TESTING NOTE: tokei is not on PATH in this environment; a python stub emitting tokei --output json shape was used (as cod-8579 did). Ran against a purpose-built 60-commit, 3-author, 10-file fixture repo because this repo's own coupling and communication are empty at threshold, which would have left the two graph scripts unexercised again. Could NOT verify 'each opens in a browser and renders': no Chrome/Chromium is installed and the agent-browser skill forbids downloading one. Verified structurally instead - all five HTMLs parse, carry non-empty payloads (3 x 14-node hierarchy, coupling 6 nodes/5 links, network 3 nodes/6 ties), and their node/link keys match every field the force-network template accesses (d.group/d.label are ?? -guarded).

DOCS: run.bash header, references/reporting.md step 1, SKILL.md step 6, references/catalog.md (Pipeline: bullet on the five interactive cards plus the authors spec card). markdownlint clean. learnings.md entry covers the byte-identity gate, the two tokei failure modes, and the sort-before-bounding rule.

GATES: bash -n and shellcheck -S warning clean, all 8 python suites green, make build green.
