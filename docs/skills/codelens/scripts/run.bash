#!/usr/bin/env bash
#
# Drive a full codelens run for one repository: generate the logs, run every
# analysis, render the degraded static figures, and build the digest. The output
# directory is then ready for scripts/report.py (see references/reporting.md
# step 1, which this script automates).
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
# Requires codelens, git, tokei, and uv on PATH.

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-${0}}")" && pwd)"

echo_stderr() {
    echo "${*}" >&2
}

die() {
    echo_stderr "run.bash: ${*}"
    exit 2
}

# Resolve "N months before an anchor date" on both GNU and BSD/macOS date, so the
# window is computed the same way regardless of which date binary is on PATH.
months_before() {
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

for tool in codelens git tokei uv; do
    command -v "${tool}" >/dev/null 2>&1 || die "required tool not on PATH: ${tool}"
done

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
run_analysis() {
    local analysis="${1}" outfile="${2}"
    shift 2
    codelens "${analysis}" "${@}" --format json >"${OUT}/${outfile}" \
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
codelens parse "${WIN[@]}" --format json >"${OUT}/parse.json" 2>/dev/null || true

tokei --output json >"${OUT}/tokei.json" 2>/dev/null ||
    echo_stderr "[${REPO_NAME}] tokei failed"

# Render one figure; figure scripts print progress to stderr on success, so
# success is judged by exit code, never by stderr being empty.
render_fig() {
    local name="${1}" script="${2}"
    shift 2
    echo_stderr "  fig ${name}"
    uv run "${SCRIPT_DIR}/${script}" "${@}" 2>"${OUT}/figs/${name}.stderr" ||
        echo_stderr "[${REPO_NAME}] figure ${name} failed (see figs/${name}.stderr)"
}

render_fig hotspots treemap.py --weights "${OUT}/revisions.json" --weight-col n_revs \
    --structure "${OUT}/tokei.json" "${EXCLUDES[@]}" -o "${OUT}/figs/hotspots.svg"
render_fig knowledge treemap.py --weights "${OUT}/main-dev.json" --weight-col main_dev \
    --categorical --structure "${OUT}/tokei.json" "${EXCLUDES[@]}" -o "${OUT}/figs/knowledge.svg"
render_fig age treemap.py --weights "${OUT}/code-age.json" --weight-col age_months \
    --invert --structure "${OUT}/tokei.json" "${EXCLUDES[@]}" -o "${OUT}/figs/age.svg"
render_fig coupling pair_matrix.py --pairs "${OUT}/coupling.json" --a-col entity \
    --b-col coupled --weight-col degree -o "${OUT}/figs/coupling.svg"
render_fig network pair_matrix.py --pairs "${OUT}/communication.json" --a-col author \
    --b-col peer --weight-col strength \
    --note 'coordination risk, not a performance ranking' -o "${OUT}/figs/network.svg"
render_fig churn churn.py --churn "${OUT}/abs-churn.json" -o "${OUT}/figs/churn.svg"
render_fig summary churn.py --summary "${OUT}/summary.json" -o "${OUT}/figs/summary.svg"
render_fig fractal fractal.py --effort "${OUT}/effort.json" -o "${OUT}/figs/fractal.svg"

uv run "${SCRIPT_DIR}/commit_cloud.py" -o "${OUT}/figs/cloud.svg" <"${OUT}/parse.json" \
    2>"${OUT}/figs/cloud.stderr" || echo_stderr "[${REPO_NAME}] figure cloud failed"

# Complexity trend for the single hottest file (reads the live repo, best effort).
TOP="$(python3 -c "import json; r=json.load(open('${OUT}/revisions.json')).get('rows',[]); print(r[0]['entity'] if r else '')" 2>/dev/null || true)"
if [[ -n "${TOP}" && -f "${TOP}" ]]; then
    render_fig complexity complexity_trend.py --repo . --file "${TOP}" -o "${OUT}/figs/complexity.svg"
fi

# Compact per-analysis signal for grounding a findings write-up.
uv run "${SCRIPT_DIR}/digest.py" "${OUT}" >/dev/null 2>&1 ||
    echo_stderr "[${REPO_NAME}] digest failed"

echo_stderr "[${REPO_NAME}] done -> ${OUT}"
