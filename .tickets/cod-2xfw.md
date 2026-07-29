---
id: cod-2xfw
status: closed
deps: []
links: []
created: 2026-07-29T04:16:34Z
type: bug
priority: 2
assignee: Andre Silva
tags: [codelens, skill, run-bash, bash, bug]
---
# run.bash: fix four silent-failure bugs and align with the bash conventions

Four bugs and three convention deviations in
`docs/skills/codelens/scripts/run.bash`, found by a review against the `bash` skill
after an end-to-end run on a 30,800-commit repository.

Baseline: `run.bash` at 407 lines, on top of commit `62dca76` plus the interactive-lane
work from cod-1mai. **Line numbers below are indicative; grep for the quoted code**,
because the file has been changing.

Every bug below was reproduced, not inferred. `shellcheck` reports only one finding on
this file and it is a false positive (see design section 8), so none of these are
mechanically detectable.

## Why they matter together

Three of the four share a failure signature: **the script keeps going and reports
success while quietly producing less than it should.** `run.bash` is deliberately
best-effort so one bad figure never costs a run, and that is right, but the same
tolerance currently hides a corrupted output path, a silently skipped figure, and a
missing digest. The fixes are small; the point is to make the remaining failures
audible.

## Severity order

1. `--out` resolves against the ANALYZED repo, so output can land in someone else's
   checkout. Observed: 48MB written into an unrelated work repository.
2. A path interpolated into a Python program breaks on a quote in the path, and the
   breakage is completely silent.
3. `emit_spec` can truncate a schema file that a later consumer was already committed
   to using, costing `digest.md`.
4. No bash-version preflight, though the script requires bash 4.4+ and macOS ships 3.2.

## Design

## 1. BUG: `--out` resolves against the analyzed repo

`cd "${REPO}"` (line ~93) runs before `mkdir -p "${OUT}/figs"` (line ~96), and `OUT` is
never made absolute. So `--out out/` while analyzing another checkout writes the entire
output tree into THAT repository.

Observed: `--out .local/test/analysis-keeper-core` while analyzing a symlinked work repo
put 48MB inside it. Nothing warned, because `.local/`-style paths are usually gitignored,
so `git status` stayed clean.

Resolve before the `cd`, using the invocation directory:

```bash
INVOCATION_DIR="$(pwd)"
```

placed with the other early assignments, then immediately after argument parsing and
before `cd "${REPO}"`:

```bash
# --out is relative to where the user invoked us, not to the repo we cd into below.
[[ "${OUT}" = /* ]] || OUT="${INVOCATION_DIR}/${OUT}"
```

Use `[[ ... = /* ]]` (glob match) rather than `realpath`: the directory does not exist
yet, and BSD `realpath` has no `--canonicalize-missing`. Do not try to normalize `..`;
prefixing is sufficient and portable.

Keep the absolute-path guidance already in `references/reporting.md`, but soften it from
a warning to a note once this lands.

## 2. BUG: path interpolated into a Python program, failing silently

Line ~276:

```bash
mapfile -t HOTSPOTS < <(python3 -c "import json; r=json.load(open('${OUT}/revisions.json')).get('rows',[]); print('\n'.join(row['entity'] for row in r[:5]))" 2>/dev/null || true)
```

Two defects in one line.

**(a) The path is interpolated into program source.** Reproduced with an apostrophe in
the output path:

```text
File "<string>", line 1
  import json; r=json.load(open('/tmp/o'brien/revisions.json'))...
                                ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
SyntaxError: invalid syntax. Perhaps you forgot a comma?
```

This is the same rule the skill states for jq, one layer out: bind the value, never
interpolate it. Pass it as `argv` instead (verified working):

```bash
mapfile -t HOTSPOTS < <(
    python3 -c 'import json, sys
rows = json.load(open(sys.argv[1])).get("rows", [])
print("\n".join(r["entity"] for r in rows[:5]))' "${OUT}/revisions.json" 2>/dev/null
)
```

Note the single-quoted program: no shell expansion happens inside it at all, which is
what makes the class of bug impossible rather than merely fixed. Splitting it across
lines also brings the line under a readable length; it is currently ~200 characters.

**(b) The failure is inaudible.** `2>/dev/null || true` discards the error, `HOTSPOTS`
comes back empty, `for TOP in "${HOTSPOTS[@]}"` iterates zero times, and NOTHING is
printed. The complexity figure silently disappears. Drop the `|| true` (the process
substitution's exit status does not propagate to `mapfile` anyway, so it buys nothing)
and report the empty case:

```bash
if [[ "${#HOTSPOTS[@]}" -eq 0 ]]; then
    echo_stderr "[${REPO_NAME}] no hotspots to trend (revisions.json empty or unreadable); skipping complexity"
fi
```

Consider keeping stderr rather than `2>/dev/null`, redirected to
`"${OUT}/figs/complexity.stderr"`, so a malformed `revisions.json` is diagnosable
instead of merely announced.

## 3. BUG: `emit_spec` truncates a schema file a later consumer already committed to

The schemas for `coupling`, `communication` and `absolute-churn` are captured once at
lines ~214-217. The consumer arrays are then built with an existence guard:

```bash
declare -a DIGEST=("${OUT}")
if [[ -s "${SCHEMA_DIR}/absolute-churn.schema.json" ]]; then
    DIGEST+=(--schema "${SCHEMA_DIR}/absolute-churn.schema.json")
fi
```

Later, `emit_spec churn absolute-churn` re-captures the SAME path:

```bash
codelens schema --command "${analysis}" >"${SCHEMA_DIR}/${analysis}.schema.json" \
    2>"${FLINT}/${name}.stderr" && ...
```

`>` truncates before `codelens` runs. If this second capture fails, the file is left
EMPTY, and `DIGEST` still carries `--schema <that path>` from a guard evaluated a
hundred lines earlier.

Reproduced consequence, with an empty schema file:

```text
digest.py: invalid JSON in schema file ...: Expecting value: line 1 column 1 (char 0)
exit=2
```

`run.bash` then prints `digest failed` and proceeds to `done`, so the run "succeeds"
having lost `digest.md`, which step 2 of the reporting pipeline depends on as the sole
grounding for the findings prose.

Two independent fixes; do both.

**(a) Do not re-capture what is already captured.** Only the eight analyses added for
the survey lane need a schema; `coupling`, `communication` and `absolute-churn` already
have one. Guard it:

```bash
if [[ ! -s "${SCHEMA_DIR}/${analysis}.schema.json" ]]; then
    codelens schema --command "${analysis}" >"${SCHEMA_DIR}/${analysis}.schema.json" \
        2>"${FLINT}/${name}.stderr" || true
fi
```

**(b) Never truncate in place.** Write to a temporary file and move on success, so a
failed capture leaves the previous good file (or no file) rather than an empty one:

```bash
local tmp="${SCHEMA_DIR}/${analysis}.schema.json.tmp"
if codelens schema --command "${analysis}" >"${tmp}" 2>"${FLINT}/${name}.stderr"; then
    mv "${tmp}" "${SCHEMA_DIR}/${analysis}.schema.json"
else
    rm -f "${tmp}"
fi
```

Apply the same write-then-move to the initial capture loop at ~214-217, which has the
identical truncate-then-fail window.

While here: make the guards consistent. The consumer arrays test `-s` (non-empty), which
is the right test; keep it.

## 4. BUG: no bash-version preflight, but bash 4.4+ is required

`mapfile` is a bash 4.0+ builtin, and the comment at line ~200 already states the script
"needs bash 4.4+" for expanding a possibly-empty array under `nounset`. But the preflight
loop checks commands only:

```bash
for tool in codelens git tokei uv; do
    command -v "${tool}" >/dev/null 2>&1 || die "required tool not on PATH: ${tool}"
done
```

and the usage header says only "Requires codelens, git, tokei, and uv on PATH".

macOS ships bash 3.2.57, which has no `mapfile`. Reproduced:

```text
mf-test.bash: line 6: mapfile: command not found
exit=127
```

Under `errexit` that aborts mid-run: the analyses and static figures are already written,
but the interactive lane, the Flint lane and the digest never run. The user gets a
partial output directory and one cryptic line.

Add to the preflight, and collect problems rather than dying on the first (the skill:
"report every missing dependency, not just the first one found"):

```bash
if ((BASH_VERSINFO[0] < 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] < 4))); then
    die "bash 4.4+ required (found ${BASH_VERSION}); on macOS: brew install bash"
fi
```

`BASH_VERSINFO` is safe under `nounset` (always set by bash). Update the usage header to
state the bash requirement alongside the four commands.

## 5. Minor: `codelens parse` failure is silent

Line ~191 bypasses `run_analysis`:

```bash
codelens parse "${WIN[@]}" >"${OUT}/parse.json" 2>/dev/null || true
```

`parse.json` feeds both the commit word cloud and the digest's vocabulary section, so a
silent failure costs two outputs. Use the existing helper for consistency and reporting:

```bash
run_analysis parse parse.json "${WIN[@]}"
```

Check first whether `parse` was deliberately excluded from `EXCLUDES` (it is called
without them today, and that looks intentional since `parse` is a raw event dump); if so
keep that, and only change the error handling.

## 6. Convention: use the `function` keyword

All seven functions use the bare form (`echo_stderr() {`, `die() {`, `months_before() {`,
`run_analysis() {`, `render_fig() {`, `render_interactive() {`, `emit_spec() {`, plus
`render_flint()` nested in the deno branch). The skill requires `function name() {` so
every definition is greppable with `grep '^function '` without matching call sites.

## 7. Convention: `.editorconfig` contradicts the file

`.editorconfig` sets `indent_size = 2` under `[*]`, but the script is 4-space indented
and `shfmt --indent 4 --diff` reports no changes. The `bash` skill requires 4 and also
says to follow `.editorconfig`, so the config should be amended rather than the script
reformatted:

```ini
[*.bash]
indent_style = space
indent_size = 4
```

## 8. Silence the one shellcheck finding, as a false positive

`shellcheck` reports exactly one thing:

```text
SC2015 (info): Note that A && B || C is not if-then-else. C may run when A is true.
```

on `emit_spec`. It is a false positive here: `A && B | C || D` parses as
`A && (B|C) || D`, and `D` is only `echo_stderr`, so "C may run when A is true" is
precisely the intent. Per the skill, add a targeted disable on the line above rather than
leaving a standing finding or restructuring working code:

```bash
# shellcheck disable=SC2015  # the || branch is a logger, not an else
```

Re-check after the section 3 rewrite: if `emit_spec` no longer uses `A && B || C`, drop
the disable instead of carrying it.

## 9. DO NOT "fix" the short command flags

The skill prefers long flags, but this script is deliberately cross-platform:
`months_before` supports both GNU `date -d` and BSD `date -v`, and long flags are mostly
GNU extensions that BSD and macOS builds reject. `mkdir -p`, `cp`, `date -d`/-v and
`mapfile -t` must stay short. Add a one-line comment saying so, so a future reviewer
following the skill literally does not break macOS.

## 10. Missing: a BATS test

There is no `.bats` file anywhere in the repository, though every Python script beside
`run.bash` has a `*_test.py`. This is the 407-line orchestrator that produces every
artifact.

A first test file need not be exhaustive; the highest-value cases map directly to the
bugs above:

- `--out` relative path lands under the invocation directory, NOT under the repo
  (bug 1). Use `BATS_TEST_TMPDIR` for both the fake repo and the output.
- An output path containing an apostrophe still produces the complexity figure, or at
  minimum reports the skip (bug 2).
- A truncated schema file does not cost `digest.md` (bug 3).
- `--repo` pointing at a non-git directory fails with a clear message and non-zero exit.
- An unknown argument fails rather than being ignored (already correct; pin it).
- A repo whose windowed log is empty exits 0 with the documented warning (already
  correct; pin it).

Use `bats-assert` (`assert_success`, `assert_failure`, `assert_output --partial`) per the
skill. Stub `codelens`, `tokei` and `uv` on `PATH` so the test does not need a real
30,000-commit repository; the orchestration logic is what is under test, not the
analyses.

## Acceptance Criteria

## Bugs, each with a reproduction

- **`--out`**: `cd /tmp && bash run.bash --repo "${SOME_REPO}" --out rel/` writes to
  `/tmp/rel/`, and `${SOME_REPO}` gains nothing. An absolute `--out` behaves exactly as
  before.
- **Interpolation**: a run whose `--out` path contains an apostrophe produces the
  complexity figure normally. No `SyntaxError` reaches any stderr file.
- **Silent skip**: with an empty or malformed `revisions.json`, the run prints a
  message naming the skipped complexity figure instead of saying nothing.
- **Schema truncation**: simulate a failing second capture (for example by making
  `codelens schema --command absolute-churn` fail partway through the run); the
  previously captured schema file is still valid, and `digest.md` is still written.
- **bash version**: running under `/bin/bash` on macOS (3.2.57) fails IMMEDIATELY with a
  message naming the required version and the `brew install bash` remedy, before any
  output directory is created. It must not fail 250 lines in with
  `mapfile: command not found`.

## Behaviour that must not change

- With an absolute `--out`, bash 4.4+, and a well-formed repo, the full output tree is
  byte-identical to before: every `figs/*.svg`, `flint/*.json`, `interactive/*.html`,
  `*.json` and `digest.md`.
- The script stays strictly read-only against the analyzed repository: no git writes, no
  working-tree mutation. Bug 1's fix changes only where output lands.
- Best-effort semantics are preserved: a single failing analysis, figure, spec or
  interactive artifact still warns and continues rather than aborting. The point of this
  ticket is that failures become audible, not fatal.

## Hygiene

- `shellcheck` clean, with at most one targeted `# shellcheck disable=SC2015` carrying a
  reason comment (or none, if section 3's rewrite removes the construct).
- `shfmt --indent 4 --diff` reports no changes.
- All functions declared with the `function` keyword; `grep -c '^function ' run.bash`
  equals the number of functions.
- `.editorconfig` has a `[*.bash]` section with `indent_size = 4`, so the file and the
  config agree.
- The usage header lists the bash requirement alongside codelens, git, tokei and uv.
- A comment records why short command flags are deliberate (BSD/macOS `date`).

## Tests

- A `.bats` file exists for `run.bash`, using `BATS_TEST_TMPDIR` and `bats-assert`, with
  `codelens`/`tokei`/`uv` stubbed on `PATH`.
- It covers the five reproduction cases above plus the two existing behaviours worth
  pinning (unknown argument fails; empty windowed log exits 0 with a warning).
- `make build` green.


## Notes

**2026-07-29T04:24:33Z**

All four bugs fixed in docs/skills/codelens/scripts/run.bash, each pinned by a new BATS test.

1. --out: added INVOCATION_DIR=$(pwd) and '[[ "${OUT}" = /* ]] || OUT="${INVOCATION_DIR}/${OUT}"' immediately after arg parsing, before 'cd "${REPO}"'. Glob match, not realpath (dir does not exist yet; BSD realpath lacks --canonicalize-missing).
2. Python interpolation: the hotspot-ranking one-liner is now a single-quoted multi-line program taking the path as sys.argv[1], so no shell expansion happens inside it at all. Dropped '|| true'; stderr goes to figs/hotspot-rank.stderr instead of /dev/null, and an empty HOTSPOTS array now prints 'no hotspots to trend ... skipping complexity'. The python side prints one entity per line (a join+print on an empty list emits a newline, which would give mapfile one empty element and defeat the -eq 0 guard).
3. Schema truncation: new capture_schema() does both fixes at once - returns early if the target is already non-empty, and writes to .tmp then mv on success (rm -f on failure), so a failed capture never leaves an empty file a consumer already committed to. Both the initial 3-analysis loop and emit_spec go through it. emit_spec now skips with a message when no schema is available instead of feeding flint_spec.py a missing --schema. The SC2015 'A && B || C' construct is gone, so no shellcheck disable was needed.
4. bash preflight: new preflight() checks BASH_VERSINFO for 4.4+ AND the four commands, collects every problem and reports them all before exiting 2. Runs before argument parsing, so it fires before any output directory exists. Usage header now says 'Requires bash 4.4+, and codelens, git, tokei, and uv on PATH'.
5. 'codelens parse' now goes through run_analysis (parse.json, no EXCLUDES - kept unfiltered on purpose, it is a raw event dump whose consumers read messages), so a failure is reported and lands in parse.stderr.
6/9. All 10 function definitions use the 'function' keyword. Header comment records that short command flags are deliberate (BSD/macOS date -v, mkdir -p, mapfile -t).
7. NOT changed: .editorconfig already has '[*.{sh,bash}] indent_size = 4', which covers .bash. Adding a separate [*.bash] section would only duplicate it.
8. No shellcheck disable in the file; shellcheck and 'shfmt --indent 4 --diff' are both clean.

Tests: docs/skills/codelens/scripts/run_test.bats, 10 cases, all green. It is the repo's first BATS suite. make build gates Go only, so run it directly: 'bats docs/skills/codelens/scripts/run_test.bats' (needs bats + bats-support + bats-assert; the file appends 'npm root --global' to BATS_LIB_PATH because bats presets that variable, so a ':=' default never applies). Stubs codelens/git/tokei/uv/deno on PATH under BATS_TEST_TMPDIR. Six cases were red before the fixes (relative --out, apostrophe path, empty revisions, schema re-capture, no-empty-file-left, version-guard ordering) and four were already green as pins (absolute --out, non-git repo, unknown arg, empty windowed log).

Docs: references/reporting.md's 'PASS --out AS AN ABSOLUTE PATH' warning softened to a note now that relative paths are safe, and its requires line gained bash 4.4+. learnings.md gained a section.

Caveat: byte-identity of the full output tree against a real repository was not re-verified here - tokei is not installed in this environment, so the end-to-end path cannot run. Orchestration is covered by the stubbed suite; the only behavioural changes to the happy path are where a relative --out lands and the fact that already-captured schemas are not re-captured.
