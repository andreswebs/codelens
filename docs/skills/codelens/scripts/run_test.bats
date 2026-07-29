#!/usr/bin/env bats
#
# Orchestration tests for run.bash. codelens, git, tokei, uv, deno and the
# figure scripts are stubbed on PATH, so what is under test is the script's
# control flow -- where output lands, what it reports, and what it refuses to
# destroy -- never the analyses themselves.

setup() {
    # bats presets BATS_LIB_PATH to the system locations; bats-support and
    # bats-assert are commonly installed from npm instead, so append that root
    # rather than replacing what bats already resolved.
    local npm_root
    npm_root="$(npm root --global 2>/dev/null || true)"
    if [ -n "${npm_root}" ]; then
        export BATS_LIB_PATH="${BATS_LIB_PATH:-}:${npm_root}"
    fi
    bats_load_library bats-support
    bats_load_library bats-assert

    RUN_BASH="${BATS_TEST_DIRNAME}/run.bash"

    STUBS="${BATS_TEST_TMPDIR}/stubs"
    REPO="${BATS_TEST_TMPDIR}/repo"
    INVOCATION="${BATS_TEST_TMPDIR}/invocation"
    export STUB_STATE="${BATS_TEST_TMPDIR}/state"
    export STUB_LOG="${BATS_TEST_TMPDIR}/git.log"
    export STUB_REVISIONS='{"schema_version":1,"ok":true,"rows":[{"entity":"src/app.py","n_revs":9}]}'

    mkdir -p "${STUBS}" "${REPO}/.git" "${REPO}/src" "${INVOCATION}" "${STUB_STATE}"
    : >"${REPO}/src/app.py"
    printf -- '--abc1234--2026-01-01--Dev--subject\n\n1\t0\tsrc/app.py\n' >"${STUB_LOG}"

    write_stubs
    PATH="${STUBS}:${PATH}"
    export PATH
}

# The stubs mirror only the surface run.bash actually calls. codelens honours
# CODELENS_SCHEMA_FAIL so a schema capture can be made to fail on its first or
# on its second occurrence, which is what bug 3 turned on.
write_stubs() {
    cat >"${STUBS}/codelens" <<'STUB'
#!/usr/bin/env bash
set -o errexit -o nounset -o pipefail
case "${1}" in
print-log-command)
    echo "cat ${STUB_LOG}"
    ;;
schema)
    analysis="${3}"
    seen="${STUB_STATE}/schema-${analysis}.seen"
    if [[ "${CODELENS_SCHEMA_FAIL:-}" == "${analysis}" ]]; then
        if [[ "${CODELENS_SCHEMA_FAIL_AFTER:-0}" -eq 0 || -f "${seen}" ]]; then
            echo "stub: schema ${analysis} failed" >&2
            exit 1
        fi
    fi
    : >"${seen}"
    printf '{"schema_version":1,"ok":true,"command":"%s","row_schema":[]}\n' "${analysis}"
    ;;
revisions)
    printf '%s\n' "${STUB_REVISIONS}"
    ;;
*)
    printf '{"schema_version":1,"ok":true,"rows":[]}\n'
    ;;
esac
STUB

    cat >"${STUBS}/git" <<'STUB'
#!/usr/bin/env bash
set -o errexit -o nounset -o pipefail
if [[ "${1}" == "log" ]]; then
    echo "2026-01-01"
fi
STUB

    cat >"${STUBS}/tokei" <<'STUB'
#!/usr/bin/env bash
echo '{}'
STUB

    cat >"${STUBS}/deno" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB

    # uv run SCRIPT ARGS...: digest.py is the one script whose behaviour matters
    # here, because bug 3 killed the run through it. Everything else just needs
    # to produce whatever -o names.
    cat >"${STUBS}/uv" <<'STUB'
#!/usr/bin/env bash
set -o errexit -o nounset -o pipefail
shift
script="$(basename "${1}")"
shift
if [[ "${script}" == "digest.py" ]]; then
    out="${1}"
    schema=""
    while [[ "${#}" -gt 0 ]]; do
        [[ "${1}" == "--schema" ]] && schema="${2}"
        shift
    done
    if [[ -n "${schema}" ]]; then
        python3 -c 'import json, sys; json.load(open(sys.argv[1]))' "${schema}" ||
            exit 2
    fi
    : >"${out}/digest.md"
    exit 0
fi
out=""
while [[ "${#}" -gt 0 ]]; do
    [[ "${1}" == "-o" ]] && out="${2}"
    shift
done
if [[ -n "${out}" ]]; then
    printf '{}\n' >"${out}"
fi
STUB

    chmod +x "${STUBS}"/*
}

@test "a relative --out lands under the invocation directory, not the analyzed repo" {
    cd "${INVOCATION}"
    run bash "${RUN_BASH}" --repo "${REPO}" --out rel-out --full-history
    assert_success
    assert [ -d "${INVOCATION}/rel-out/figs" ]
    assert [ -f "${INVOCATION}/rel-out/digest.md" ]
    refute [ -e "${REPO}/rel-out" ]
}

@test "an absolute --out is used verbatim" {
    local out="${BATS_TEST_TMPDIR}/abs-out"
    run bash "${RUN_BASH}" --repo "${REPO}" --out "${out}" --full-history
    assert_success
    assert [ -f "${out}/figs/hotspots.svg" ]
    assert [ -f "${out}/digest.md" ]
    refute [ -e "${REPO}/abs-out" ]
}

@test "an --out path containing an apostrophe still produces the complexity figure" {
    local out="${BATS_TEST_TMPDIR}/o'brien"
    run bash "${RUN_BASH}" --repo "${REPO}" --out "${out}" --full-history
    assert_success
    assert [ -f "${out}/figs/complexity.svg" ]
    refute_output --partial "SyntaxError"
    run cat "${out}/figs/hotspot-rank.stderr"
    refute_output --partial "SyntaxError"
}

@test "an empty revisions.json reports the skipped complexity figure instead of nothing" {
    export STUB_REVISIONS='{"schema_version":1,"ok":true,"rows":[]}'
    run bash "${RUN_BASH}" --repo "${REPO}" --out "${BATS_TEST_TMPDIR}/out" --full-history
    assert_success
    assert_output --partial "no hotspots to trend"
}

@test "a failing schema re-capture leaves the earlier capture valid and still writes the digest" {
    export CODELENS_SCHEMA_FAIL=absolute-churn
    export CODELENS_SCHEMA_FAIL_AFTER=1
    local out="${BATS_TEST_TMPDIR}/out"
    run bash "${RUN_BASH}" --repo "${REPO}" --out "${out}" --full-history
    assert_success
    assert [ -s "${out}/schema/absolute-churn.schema.json" ]
    run python3 -c 'import json, sys; json.load(open(sys.argv[1]))' \
        "${out}/schema/absolute-churn.schema.json"
    assert_success
    assert [ -f "${out}/digest.md" ]
}

@test "a schema capture that never succeeds leaves no empty file behind" {
    export CODELENS_SCHEMA_FAIL=authors
    local out="${BATS_TEST_TMPDIR}/out"
    run bash "${RUN_BASH}" --repo "${REPO}" --out "${out}" --full-history
    assert_success
    refute [ -e "${out}/schema/authors.schema.json" ]
    refute [ -e "${out}/schema/authors.schema.json.tmp" ]
    assert [ -f "${out}/digest.md" ]
}

@test "the bash version preflight runs before any output directory is created" {
    local guard mkdir_line
    guard="$(grep --line-number --fixed-strings 'BASH_VERSINFO' "${RUN_BASH}" | head -1 | cut -d: -f1)"
    mkdir_line="$(grep --line-number --fixed-strings 'mkdir -p "${OUT}/figs"' "${RUN_BASH}" | head -1 | cut -d: -f1)"
    assert [ -n "${guard}" ]
    assert [ -n "${mkdir_line}" ]
    assert [ "${guard}" -lt "${mkdir_line}" ]
    run grep --fixed-strings 'brew install bash' "${RUN_BASH}"
    assert_success
}

@test "--repo pointing at a non-git directory fails with a clear message" {
    mkdir -p "${BATS_TEST_TMPDIR}/plain"
    run bash "${RUN_BASH}" --repo "${BATS_TEST_TMPDIR}/plain" --out "${BATS_TEST_TMPDIR}/out"
    assert_failure
    assert_output --partial "not a git repository"
}

@test "an unknown argument fails rather than being ignored" {
    run bash "${RUN_BASH}" --repo "${REPO}" --out "${BATS_TEST_TMPDIR}/out" --nope
    assert_failure
    assert_output --partial "unknown argument: --nope"
}

@test "an empty windowed log exits 0 with the documented warning" {
    : >"${STUB_LOG}"
    run bash "${RUN_BASH}" --repo "${REPO}" --out "${BATS_TEST_TMPDIR}/out" --full-history
    assert_success
    assert_output --partial "windowed log is empty"
}
