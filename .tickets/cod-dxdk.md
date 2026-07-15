---
id: cod-dxdk
status: closed
deps: []
links: []
created: 2026-07-15T03:40:57Z
type: bug
priority: 2
assignee: Andre Silva
tags: [codelens, viz-skill, bug, friction]
---
# Bug: complexity_trend.py aborts on renamed files

`docs/skills/codelens/scripts/complexity_trend.py` aborts with a git fatal and
writes no output for any file that was renamed at some point in its history. This
hits most real hotspots, because files move as a codebase is refactored.

## Reproduction

Pick any file that was renamed during its life (in the test-drive:
`Keeper.AdminServer/providerSyncSrc/Keeper.ProviderSync.Core/Services/EmployeeStatusService.cs`)
and run:

```sh
uv run docs/skills/codelens/scripts/complexity_trend.py \
  --repo /path/to/repo --file <renamed-file> -o trend.svg
```

Observed: exit 2, no output file, stderr:

```text
complexity_trend.py: git show <sha>:<current-path>: fatal: path '<current-path>'
exists on disk, but not in '<sha>'
```

## Root cause

The script enumerates revisions with `git log --follow` (line ~65), which
**follows renames**, so the returned commit list includes commits from before the
file reached its current path. It then fetches each historical version with `git
show <rev>:<args.file>` (line ~77) using the **current** path. For any commit
predating a rename, that path does not exist in the tree, so `git show` fails; the
`git()` helper treats any non-zero git exit as fatal and does `raise SystemExit(2)`
(lines ~32-37), aborting the whole run with no chart written.

## Decision: approach A (resolve the historical path per revision)

Chosen over "skip failing revs" (drops pre-rename history silently, rev count no
longer matches) and "drop `--follow`" (loses all history before the last rename).
A complexity trend exists to show a file's evolution across its whole life, and
renames are common, so the fix must preserve pre-rename history. `--follow`
already surfaces the rename information; the script just needs to map each
revision to the path the file had **at that revision** and `git show` that path.

### Implementation

Replace the current two-step (`git log --follow --format=%H\t%ad` then `git show
<rev>:<current-path>`) with a single enumeration that also yields the path at each
revision:

- Run `git log --follow --name-status --format=%H\t%ad --date=short -- <file>`.
  For each commit block, parse the status line: a rename appears as `R<score>\t
  <old-path>\t<new-path>`; adds/modifies appear as `A`/`M\t<path>`. Track, walking
  newest to oldest, the path the file carried at each commit (the "new-path" side
  of a rename becomes the path for that commit and all newer ones until the next
  rename; the "old-path" applies to older commits).
- Reverse to oldest-first (as today).
- For each `(rev, date, path_at_rev)`, call `git show <rev>:<path_at_rev>`; this
  never hits the missing-path fatal.
- Keep the existing `indentation()` measurement and the matplotlib rendering
  unchanged.

Do **not** silently swallow genuine `git show` failures: only the rename-path
mismatch is expected. Keep `git()`'s fatal behavior for real errors; the fix is to
pass the correct path, not to ignore failures.

Note: the codelens-generated log uses `--no-renames` (see
`src/cmd/codelens/printlogcommand.go`), but `complexity_trend.py` reads the live
repo with its own `git` invocations and is independent of that log, so it is free
to use `--follow`/`--name-status`.

## TDD plan (/tdd)

Tests drive a real temporary git repo (the script shells out to `git`, so a fixture
repo is the honest test double; do not mock `git`). Use one `git init` fixture per
case, built with a helper that commits content and performs `git mv`.

Vertical slices, one test then one implementation step at a time:

1. `test_trend_no_rename`: a file created and modified in place across 3 commits ->
   script exits 0, writes the `-o` file, reports 3 revisions. (Tracer bullet;
   guards against regressing the happy path.)
2. `test_trend_across_one_rename`: create `a.py`, commit twice, `git mv a.py b.py`,
   commit twice more; run with `--file b.py` -> exit 0, output written, **4**
   revisions (all of them, including the two under the old name). This is the
   reproduction of the bug and the core fix.
3. `test_trend_across_two_renames`: `a -> b -> c`; run with `--file c` -> exit 0,
   all revisions counted.
4. `test_trend_missing_file`: `--file does/not/exist` -> exit 3 (`no history`),
   preserving the current no-history contract.

Assert on exit code, output-file existence, and the trailing `wrote ... (N
revisions)` count on stderr (the script's observable behavior). Do not assert on
matplotlib internals.

## Files touched

```text
docs/skills/codelens/scripts/complexity_trend.py          rename-aware enumeration
docs/skills/codelens/scripts/complexity_trend_test.py     new (or tests/ per repo convention)
```

Confirm the repo's convention for Python script tests before adding the file;
follow whatever pattern the other `scripts/*.py` use (there may be none yet, in
which case a sibling `*_test.py` runnable via `uv run` is acceptable, matching the
PEP 723 inline-metadata style already in the scripts).

## Acceptance criteria

- Running the trend on a file that was renamed one or more times exits 0 and
  writes the chart, counting every revision across the renames (old and new
  paths).
- A file with no history still exits 3 with the `no history` message.
- A genuine git failure (not a rename-path mismatch) still surfaces as a fatal,
  not a silent skip.
- The four TDD cases pass; the script remains a single self-contained `uv run`
  file with inline PEP 723 metadata.
- Any Markdown touched passes markdownlint per project standard.

## References

- `docs/skills/codelens/scripts/complexity_trend.py` (current implementation)
- `docs/skills/codelens/references/catalog.md` (Complexity trend card)
- Skills: `/tdd` (fixture-repo tests, vertical slices), `/llm-coding` (surgical,
  no speculative features)

## Notes

**2026-07-15T04:27:15Z**

Fixed rename abort via approach A (resolve historical path per revision). New enumerate_revs() runs 'git log --follow --name-status --format=%H\t%ad --date=short -- <file>' and reads the path at each commit straight from its status line: rename block R<score>\told\tnew uses the NEW side (the path that commit's tree holds, so 'git show <rev>:<new>' never hits the missing-path fatal); copies (C<score>) likewise; all others use <status>\t<path>. Header lines detected by 2 tab-fields + 40-hex hash. git() fatal-on-nonzero behavior unchanged, so genuine git failures still surface. Tests in sibling complexity_trend_test.py (per enclosure.py convention) drive real git-init fixture repos with a git mv helper, no mocking; assert only on exit code, -o file existence, and trailing 'wrote ... (N revisions)' count. 4 cases: no-rename(3), one-rename(4), two-renames(5), missing-file(exit 3). make build green; ruff/ty clean (only pre-existing matplotlib import-resolution notes).
