#!/usr/bin/env bash
#
# Drive a full codelens run for one repository: generate the logs, run every
# analysis, render the degraded static figures, build the interactive HTML
# artifacts, emit the Flint chart specs, and build the digest. The output
# directory is then ready for scripts/report.py (see references/reporting.md
# step 1, which this script automates).
#
# report.py consumes only the static SVG figures, summary.json and digest.md;
# the interactive lane under ${OUT}/interactive/ and the spec lane under
# ${OUT}/flint/ are additive, for browsing and for the MCP rendering path.
#
# Strictly read-only against the repository: it only reads git history and the
# working tree (tokei). It never runs git writes and never mutates the repo.
#
# Usage:
#   run.bash [--repo PATH] [--out DIR] [--months N] [--full-history]
#            [--exclude GLOB ...]
#
#   --repo PATH       repository to analyze (default: current directory)
#   --out DIR         output directory (default: ./codelens-report)
#   --months N        window size in months, ending at the repo's last commit
#                     (default: 12; ignored with --full-history)
#   --full-history    analyze all history instead of a window (for stale or
#                     front-loaded repos where a trailing window is nearly empty)
#   --exclude GLOB    extra generated/vendored path to exclude (repeatable); added
#                     to the built-in set below
#
# Requires bash 4.4+, and codelens, git, tokei, and uv on PATH.
#
# Command flags here are deliberately short (`mkdir -p`, `date -d`/`-v`,
# `mapfile -t`, `tokei --output` aside): the long forms are mostly GNU
# extensions that the BSD and macOS builds reject, and this script must run on
# both. Do not "modernize" them.

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-${0}}")" && pwd)"
INVOCATION_DIR="$(pwd)"

function echo_stderr() {
    echo "${*}" >&2
}

function die() {
    echo_stderr "run.bash: ${*}"
    exit 2
}

# Fail before any work, and report every problem in one pass rather than one per
# run. The bash floor is 4.4 because `mapfile` is a 4.0 builtin and expanding a
# possibly-empty array under `nounset` needs 4.4; macOS still ships 3.2.57.
function preflight() {
    local -a problems=()
    if ((BASH_VERSINFO[0] < 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] < 4))); then
        problems+=("bash 4.4+ required (found ${BASH_VERSION}); on macOS: brew install bash")
    fi
    local tool
    for tool in codelens git tokei uv; do
        command -v "${tool}" >/dev/null 2>&1 ||
            problems+=("required tool not on PATH: ${tool}")
    done
    if [[ "${#problems[@]}" -eq 0 ]]; then
        return 0
    fi
    local problem
    for problem in "${problems[@]}"; do
        echo_stderr "run.bash: ${problem}"
    done
    exit 2
}

preflight

# Resolve "N months before an anchor date" on both GNU and BSD/macOS date, so the
# window is computed the same way regardless of which date binary is on PATH.
function months_before() {
    local anchor="${1}" months="${2}"
    date -d "${anchor} -${months} months" +%F 2>/dev/null && return 0
    date -v"-${months}m" -j -f "%Y-%m-%d" "${anchor}" +%F 2>/dev/null && return 0
    die "cannot compute window date; neither GNU nor BSD 'date' worked"
}

REPO="."
OUT="./codelens-report"
MONTHS=12
FULL_HISTORY=false
declare -a EXTRA_EXCLUDES=()

while [[ "${#}" -gt 0 ]]; do
    case "${1}" in
    --repo)
        REPO="${2}"
        shift 2
        ;;
    --out)
        OUT="${2}"
        shift 2
        ;;
    --months)
        MONTHS="${2}"
        shift 2
        ;;
    --full-history)
        FULL_HISTORY=true
        shift
        ;;
    --exclude)
        EXTRA_EXCLUDES+=("${2}")
        shift 2
        ;;
    *)
        die "unknown argument: ${1}"
        ;;
    esac
done

# --out is relative to where the user invoked us, not to the repo we cd into
# below. A glob match, not realpath: the directory does not exist yet and BSD
# realpath has no --canonicalize-missing.
[[ "${OUT}" = /* ]] || OUT="${INVOCATION_DIR}/${OUT}"

cd "${REPO}" || die "cannot enter repo: ${REPO}"
[[ -d .git ]] || die "not a git repository: ${REPO}"
REPO_NAME="$(basename "$(pwd)")"
mkdir -p "${OUT}/figs"

# Built-in excludes: truly generated or vendored artifacts common across
# languages. Human-authored config and localization are intentionally NOT
# excluded (they are legitimate hotspots). Extra --exclude globs are appended.
declare -a EXCLUDES=(
    --exclude '**/composer.lock'
    --exclude '**/package-lock.json'
    --exclude '**/yarn.lock'
    --exclude '**/pnpm-lock.yaml'
    --exclude '**/Gemfile.lock'
    --exclude '**/poetry.lock'
    --exclude '**/go.sum'
    --exclude '**/vendor/**'
    --exclude '**/node_modules/**'
    --exclude '**/dist/**'
    --exclude '**/build/**'
    --exclude '**/generated/**'
    --exclude '**/*.min.js'
    --exclude '**/*.min.css'
    --exclude '**/*.postman_collection.json'
    --exclude '**/__snapshots__/**'
    --exclude '**/*.snap'
    --exclude '**/coverage/**'
    --exclude '**/storybook-static/**'
    # Framework codegen that lands in the source tree rather than under a
    # generated/ or build/ directory, so the globs above miss it. Each of these
    # topped a real hotspot ranking before being excluded.
    --exclude '**/Migrations/**'
    --exclude '**/l10n/gen/**'
    --exclude '**/*.g.dart'
)
if [[ "${#EXTRA_EXCLUDES[@]}" -gt 0 ]]; then
    for glob in "${EXTRA_EXCLUDES[@]}"; do
        EXCLUDES+=(--exclude "${glob}")
    done
fi

# Full history is always needed for code-age (age is measured from the log's
# earliest commit, so a windowed log caps every file's reported age). The windowed
# log is the full log too when --full-history is set.
eval "$(codelens print-log-command)" >"${OUT}/git-full.log"
if [[ "${FULL_HISTORY}" == true ]]; then
    cp "${OUT}/git-full.log" "${OUT}/git.log"
    echo_stderr "[${REPO_NAME}] window=full-history out=${OUT}"
else
    LAST="$(git log -1 --format=%as)"
    AFTER="$(months_before "${LAST}" "${MONTHS}")"
    eval "$(codelens print-log-command --after "${AFTER}")" >"${OUT}/git.log"
    echo_stderr "[${REPO_NAME}] last=${LAST} window=${MONTHS}mo after=${AFTER} out=${OUT}"
fi

if [[ ! -s "${OUT}/git.log" ]]; then
    echo_stderr "[${REPO_NAME}] WARN: windowed log is empty; try --full-history"
    exit 0
fi

WIN=(--log "${OUT}/git.log")
FULL=(--log "${OUT}/git-full.log")

# Run one analysis to a JSON file; a single failure is reported, not fatal, so a
# partial run still produces a usable output directory.
function run_analysis() {
    local analysis="${1}" outfile="${2}"
    shift 2
    codelens "${analysis}" "${@}" >"${OUT}/${outfile}" \
        2>"${OUT}/${analysis}.stderr" ||
        echo_stderr "[${REPO_NAME}] analysis ${analysis} failed (see ${analysis}.stderr)"
}

# Generated-file excludes apply to every entity-centric analysis, so churn,
# coupling, ownership, effort and fragmentation reflect authored code rather than
# regenerated artifacts. communication is an author graph and summary is a
# whole-repo count; both run unfiltered so authorship and totals stay whole.
run_analysis revisions revisions.json "${WIN[@]}" "${EXCLUDES[@]}"
run_analysis coupling coupling.json "${WIN[@]}" "${EXCLUDES[@]}"
run_analysis sum-of-coupling soc.json "${WIN[@]}" "${EXCLUDES[@]}"
run_analysis main-developer main-dev.json "${WIN[@]}" "${EXCLUDES[@]}"
run_analysis code-age code-age.json "${FULL[@]}" "${EXCLUDES[@]}"
run_analysis communication communication.json "${WIN[@]}"
run_analysis absolute-churn abs-churn.json "${WIN[@]}" "${EXCLUDES[@]}"
run_analysis entity-effort effort.json "${WIN[@]}" "${EXCLUDES[@]}"
run_analysis fragmentation fragmentation.json "${WIN[@]}" "${EXCLUDES[@]}"
run_analysis summary summary.json "${WIN[@]}"
# Entity- and author-centric surveys that feed the spec lane only; no static
# figure consumes them, so they are excluded-filtered like every other
# entity-centric analysis above.
run_analysis authors authors.json "${WIN[@]}" "${EXCLUDES[@]}"
run_analysis author-churn author-churn.json "${WIN[@]}" "${EXCLUDES[@]}"
run_analysis entity-churn entity-churn.json "${WIN[@]}" "${EXCLUDES[@]}"
run_analysis entity-ownership entity-ownership.json "${WIN[@]}" "${EXCLUDES[@]}"
run_analysis main-developer-by-revisions main-dev-by-revs.json "${WIN[@]}" "${EXCLUDES[@]}"
run_analysis refactoring-main-developer refactoring-main-dev.json "${WIN[@]}" "${EXCLUDES[@]}"
# messages is deliberately NOT run: its --expression is required and there is no
# defensible default regex. It is a targeted query, not a survey.

# parse runs unfiltered on purpose: it is a raw event dump, and both consumers
# (the commit word cloud and the digest's vocabulary section) read messages, not
# entities.
run_analysis parse parse.json "${WIN[@]}"

tokei --output json >"${OUT}/tokei.json" 2>/dev/null ||
    echo_stderr "[${REPO_NAME}] tokei failed"

# tokei is best effort, so --structure must be too: the map scripts raise an
# unhandled FileNotFoundError on a missing path, and both of them degrade to a
# documented no-structure mode when the flag is simply absent. Guarded once here
# and shared by the static and the interactive map lanes. Expanding a possibly
# empty array under `set -o nounset` needs bash 4.4+, which this script already
# requires for the `mapfile` below.
declare -a STRUCT=()
if [[ -s "${OUT}/tokei.json" ]]; then
    STRUCT=(--structure "${OUT}/tokei.json")
fi

# Per-analysis schema, captured once for the scripts that aggregate rows: they
# read its aggregation_roles map to check how a column may be combined (see
# scripts/aggregation.py). Best effort, because the guard degrades to a
# pass-through when a schema is unavailable; the flag is only added below when
# the capture produced a file, so a failed capture never fails a figure.
SCHEMA_DIR="${OUT}/schema"
mkdir -p "${SCHEMA_DIR}"

# Capture a command's schema at most once, write-then-move. Redirecting straight
# onto the target truncates it before codelens runs, so a failed re-capture would
# leave an empty file that a consumer guarded a hundred lines earlier has already
# committed to reading; a temporary file keeps the last good capture instead.
function capture_schema() {
    local analysis="${1}" errfile="${2}"
    local target="${SCHEMA_DIR}/${analysis}.schema.json"
    if [[ -s "${target}" ]]; then
        return 0
    fi
    local tmp="${target}.tmp"
    if codelens schema --command "${analysis}" >"${tmp}" 2>"${errfile}"; then
        mv "${tmp}" "${target}"
    else
        rm -f "${tmp}"
        echo_stderr "[${REPO_NAME}] schema ${analysis} failed (see ${errfile#"${OUT}/"})"
    fi
}

for analysis in coupling communication absolute-churn; do
    capture_schema "${analysis}" "${SCHEMA_DIR}/${analysis}.stderr"
done

# The guard's arguments are appended to an already-populated array rather than
# expanded on their own, so every expansion below stays non-empty and safe under
# `set -o nounset` on older bash.
declare -a PM_COUPLING=(
    --pairs "${OUT}/coupling.json" --a-col entity --b-col coupled --weight-col degree
)
if [[ -s "${SCHEMA_DIR}/coupling.schema.json" ]]; then
    PM_COUPLING+=(--schema "${SCHEMA_DIR}/coupling.schema.json")
fi

declare -a PM_NETWORK=(
    --pairs "${OUT}/communication.json" --a-col author --b-col peer
    --weight-col strength --note 'coordination risk, not a performance ranking'
)
if [[ -s "${SCHEMA_DIR}/communication.schema.json" ]]; then
    PM_NETWORK+=(--schema "${SCHEMA_DIR}/communication.schema.json")
fi

declare -a DN_COMM=(--communication "${OUT}/communication.json")
if [[ -s "${SCHEMA_DIR}/communication.schema.json" ]]; then
    DN_COMM+=(--schema "${SCHEMA_DIR}/communication.schema.json")
fi

declare -a DIGEST=("${OUT}")
if [[ -s "${SCHEMA_DIR}/absolute-churn.schema.json" ]]; then
    DIGEST+=(--schema "${SCHEMA_DIR}/absolute-churn.schema.json")
fi

# Render one figure; figure scripts print progress to stderr on success, so
# success is judged by exit code, never by stderr being empty.
function render_fig() {
    local name="${1}" script="${2}"
    shift 2
    echo_stderr "  fig ${name}"
    uv run "${SCRIPT_DIR}/${script}" "${@}" 2>"${OUT}/figs/${name}.stderr" ||
        echo_stderr "[${REPO_NAME}] figure ${name} failed (see figs/${name}.stderr)"
}

render_fig hotspots treemap.py --weights "${OUT}/revisions.json" --weight-col n_revs \
    "${STRUCT[@]}" "${EXCLUDES[@]}" -o "${OUT}/figs/hotspots.svg"
render_fig knowledge treemap.py --weights "${OUT}/main-dev.json" --weight-col main_dev \
    --categorical "${STRUCT[@]}" "${EXCLUDES[@]}" -o "${OUT}/figs/knowledge.svg"
render_fig age treemap.py --weights "${OUT}/code-age.json" --weight-col age_months \
    --invert "${STRUCT[@]}" "${EXCLUDES[@]}" -o "${OUT}/figs/age.svg"
render_fig coupling pair_matrix.py "${PM_COUPLING[@]}" -o "${OUT}/figs/coupling.svg"
render_fig network pair_matrix.py "${PM_NETWORK[@]}" -o "${OUT}/figs/network.svg"
render_fig churn churn.py --churn "${OUT}/abs-churn.json" -o "${OUT}/figs/churn.svg"
render_fig summary churn.py --summary "${OUT}/summary.json" -o "${OUT}/figs/summary.svg"
render_fig fractal fractal.py --effort "${OUT}/effort.json" -o "${OUT}/figs/fractal.svg"

uv run "${SCRIPT_DIR}/commit_cloud.py" -o "${OUT}/figs/cloud.svg" <"${OUT}/parse.json" \
    2>"${OUT}/figs/cloud.stderr" || echo_stderr "[${REPO_NAME}] figure cloud failed"

# Complexity trend for the hottest file (reads the live repo, best effort). A
# single awkward file must not cost the figure, so the top-ranked hotspots are
# tried in order until one renders.
declare -a HOTSPOTS=()
# The path is bound as argv, never interpolated into the program source: the
# program is single-quoted, so a quote or a `$` in the output path cannot
# rewrite it. Same rule as jq, one layer out.
mapfile -t HOTSPOTS < <(
    python3 -c 'import json, sys
rows = json.load(open(sys.argv[1])).get("rows", [])
for row in rows[:5]:
    print(row["entity"])' "${OUT}/revisions.json" 2>"${OUT}/figs/hotspot-rank.stderr"
)
if [[ "${#HOTSPOTS[@]}" -eq 0 ]]; then
    echo_stderr "[${REPO_NAME}] no hotspots to trend (revisions.json empty or unreadable; see figs/hotspot-rank.stderr); skipping complexity"
fi
for TOP in "${HOTSPOTS[@]}"; do
    [[ -f "${TOP}" ]] || continue
    echo_stderr "  fig complexity (${TOP})"
    if uv run "${SCRIPT_DIR}/complexity_trend.py" --repo . --file "${TOP}" \
        -o "${OUT}/figs/complexity.svg" 2>"${OUT}/figs/complexity.stderr"; then
        break
    fi
    echo_stderr "[${REPO_NAME}] figure complexity failed for ${TOP} (see figs/complexity.stderr), trying the next hotspot"
done

# Interactive lane (additive): the same three maps and two graphs as above, in
# their browsable renderers. report.py does not consume these -- it embeds
# figures as inline SVG, which an HTML artifact cannot be -- so they land in
# their own subdirectory and a failure here never costs the static lane.
INTERACTIVE="${OUT}/interactive"
mkdir -p "${INTERACTIVE}"

function render_interactive() {
    local name="${1}" script="${2}"
    shift 2
    echo_stderr "  html ${name}"
    uv run "${SCRIPT_DIR}/${script}" "${@}" 2>"${INTERACTIVE}/${name}.stderr" ||
        echo_stderr "[${REPO_NAME}] interactive ${name} failed (see interactive/${name}.stderr)"
}

# The hierarchy is reusable across the whole enclosure family (only `weight`
# changes between hotspots, knowledge and age), so --json-out is passed to the
# first call only rather than writing three near-identical copies.
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
# No --min-degree/--min-strength: codelens already threshold-filtered both JSONs
# (coupling defaults to --min-coupling 30), and filtering again would compound.
render_interactive coupling coupling_graph.py \
    --coupling "${OUT}/coupling.json" --soc "${OUT}/soc.json" \
    -o "${INTERACTIVE}/coupling.html"
render_interactive network dev_network.py \
    "${DN_COMM[@]}" -o "${INTERACTIVE}/network.html"

# Flint spec lane (additive): emit a chart spec per analysis, then render it
# headless when deno is on PATH. Without deno the specs still land in
# ${OUT}/flint/ for the MCP rendering path, and everything above ran
# unchanged. Each analysis's schema is captured into ${OUT}/schema/, the same
# place the aggregation guard reads, so axis labels come from column
# descriptions rather than raw column names.
FLINT="${OUT}/flint"
mkdir -p "${FLINT}"

function emit_spec() {
    local name="${1}" analysis="${2}"
    shift 2
    local schema="${SCHEMA_DIR}/${analysis}.schema.json"
    capture_schema "${analysis}" "${FLINT}/${name}.stderr"
    if [[ ! -s "${schema}" ]]; then
        echo_stderr "[${REPO_NAME}] flint spec ${name} skipped: no schema for ${analysis}"
        return 0
    fi
    codelens "${analysis}" "${@}" 2>>"${FLINT}/${name}.stderr" |
        uv run "${SCRIPT_DIR}/flint_spec.py" --schema "${schema}" \
            -o "${FLINT}/${name}.flint.json" 2>>"${FLINT}/${name}.stderr" ||
        echo_stderr "[${REPO_NAME}] flint spec ${name} failed (see flint/${name}.stderr)"
}

# Bound the ranked edge tables with codelens --rows, never with Flint's canvas
# budget: --rows truncates by the analysis's own ranking (a deliberate top-N,
# coupling and communication sort by degree/strength descending), while Flint
# truncates by axis sort order within a layout budget and self-labels the chart
# "...N items omitted". absolute-churn sorts by date ascending, so --rows would
# cut the oldest end of the trend; the --months window bounds it instead. The
# code-age histogram and summary cards need every row (a distribution and a
# handful of tiles), so they run unbounded.
emit_spec coupling coupling "${WIN[@]}" "${EXCLUDES[@]}" --rows 100
emit_spec network communication "${WIN[@]}" --rows 100
emit_spec churn absolute-churn "${WIN[@]}" "${EXCLUDES[@]}"
emit_spec summary summary "${WIN[@]}"
emit_spec age code-age "${FULL[@]}" "${EXCLUDES[@]}"
# The six survey analyses no figure consumes. Only the two that sort by a rank
# take --rows: authors ranks by n_authors descending and entity-churn by added
# descending, so a top-N is the analysis's own answer. entity-ownership,
# main-developer-by-revisions and refactoring-main-developer all sort by entity
# ascending, so --rows would silently cut alphabetically; they run unbounded and
# let Flint truncate within its canvas budget, which self-labels the omission.
# author-churn is one row per author, already bounded by team size.
emit_spec authors authors "${WIN[@]}" "${EXCLUDES[@]}" --rows 100
emit_spec entity-churn entity-churn "${WIN[@]}" "${EXCLUDES[@]}" --rows 100
emit_spec author-churn author-churn "${WIN[@]}" "${EXCLUDES[@]}"
emit_spec entity-ownership entity-ownership "${WIN[@]}" "${EXCLUDES[@]}"
emit_spec main-dev-by-revs main-developer-by-revisions "${WIN[@]}" "${EXCLUDES[@]}"
emit_spec refactoring-main-dev refactoring-main-developer "${WIN[@]}" "${EXCLUDES[@]}"

# Render each spec with the pinned Deno renderer. The backend is passed
# explicitly because the spec does not carry one (flint_spec.py reports it
# only on stderr) and Heatmap-style names exist on both backends. Always the
# full permission set: vega-lite SVG output changes if the ffi canvas module
# cannot load, so a reduced flag set silently breaks reproducibility.
if command -v deno >/dev/null 2>&1; then
    function render_flint() {
        local name="${1}" backend="${2}"
        [[ -s "${FLINT}/${name}.flint.json" ]] || return 0
        echo_stderr "  fig flint-${name}"
        deno run --allow-env --allow-read --allow-sys --allow-ffi --allow-write \
            "${SCRIPT_DIR}/flint_render.ts" --backend "${backend}" \
            -o "${OUT}/figs/flint-${name}.svg" <"${FLINT}/${name}.flint.json" \
            2>"${OUT}/figs/flint-${name}.stderr" ||
            echo_stderr "[${REPO_NAME}] figure flint-${name} failed (see figs/flint-${name}.stderr)"
    }
    render_flint coupling echarts
    render_flint network echarts
    render_flint churn vegalite
    render_flint summary vegalite
    render_flint age vegalite
    # authors is the one Scatter Plot of the six; the rest are Bar Charts.
    render_flint authors vegalite
    render_flint entity-churn vegalite
    render_flint author-churn vegalite
    render_flint entity-ownership vegalite
    render_flint main-dev-by-revs vegalite
    render_flint refactoring-main-dev vegalite
else
    echo_stderr "[${REPO_NAME}] deno not on PATH; Flint specs emitted to flint/, not rendered"
fi

# Compact per-analysis signal for grounding a findings write-up.
uv run "${SCRIPT_DIR}/digest.py" "${DIGEST[@]}" >/dev/null 2>&1 ||
    echo_stderr "[${REPO_NAME}] digest failed"

echo_stderr "[${REPO_NAME}] done -> ${OUT}"
