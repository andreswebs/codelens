---
id: cod-n3qm
status: closed
deps: []
links: []
created: 2026-07-29T03:15:23Z
type: bug
priority: 2
assignee: Andre Silva
tags: [codelens, skill, complexity, bug]
---
# Bug: complexity_trend.py aborts on one unresolvable revision, and mis-pairs renamed paths

`complexity_trend.py` aborts the whole analysis when a single historical revision's
path cannot be resolved, and its rename-tracking mis-pairs a rename's OLD path with
the wrong commit, so the abort fires on any file that has ever been renamed.

Found 2026-07-28 on a 30,800-commit repository. The complexity-trend figure was the
only one of fourteen that failed in a full `run.bash` run, and it failed for both
candidate hotspots that were tried.

## Reproduction

Any file whose history crosses a directory rename. Observed:

```text
complexity_trend.py: git show 1e39a75ab70918f3694621cfabd038310ea804c2:
Keeper.AdminServer/src/AdminServer.Infrastructure/DefaultInfrastructureModule.cs:
fatal: path '...' does not exist in '1e39a75ab...'
```

The file was renamed `Keeper.AdminServer/src/...` to
`Keeper.AdminServer/adminSrc/...`. 385 revisions were enumerated; ONE unresolvable
path killed the chart.

## Two independent defects

1. **A single `git show` failure is fatal.** `git()` does `raise SystemExit(2)` on any
   non-zero git exit, so one bad revision of 385 discards the entire series. Even with
   a perfect rename parser, a long history with copies, merges, and case-only renames
   will occasionally fail to resolve; a miss should drop that data point, not the
   figure.
2. **The rename path/commit pairing is off by one.** `enumerate_revs` produced
   `path=Keeper.AdminServer/src/...` for commit `1e39a75a`, but
   `git log -1 --name-status 1e39a75a -- <current-path>` reports
   `A  Keeper.AdminServer/adminSrc/...` at that commit. The old path was attributed to
   a commit where only the new path exists.

Defect 1 alone would have degraded gracefully to a slightly shorter series. Defect 2
alone would be invisible with a tolerant fetch. Together they produce a total failure.

## Design

## Defect 1: make a per-revision fetch failure non-fatal

`git()` is a shared helper that exits on any git error:

```python
def git(repo: str, *args: str) -> str:
    r = subprocess.run(["git", "-C", repo, *args], capture_output=True, text=True)
    if r.returncode != 0:
        print(f"complexity_trend.py: git {' '.join(args)}: {r.stderr.strip()}", file=sys.stderr)
        raise SystemExit(2)
    return r.stdout
```

That is right for the initial `git log` (no history, nothing to do) and wrong for the
per-revision `git show`. Give the fetch a tolerant variant that returns `None` on
failure, skip that revision, and count the skips.

Then: if EVERY revision fails, that is still a real error (exit 2, nothing to plot). If
some fail, plot what resolved and report the count on stderr, e.g.
`skipped 3 of 385 revisions whose path could not be resolved`. That matches the
best-effort posture the rest of the pipeline already has, where `run.bash` skips a
figure with no data rather than failing the run.

## Defect 2: the rename pairing

`enumerate_revs` parses `git log --follow --name-status --format=%H\t%ad`. Its
docstring states the intent correctly: "each revision must be fetched at the path it
carried *then*", and for a rename line `R<score>\t<old>\t<new>` it takes the new side
as "the path this commit produced".

The empirical result contradicts that, so the pairing of status lines to the preceding
header needs re-deriving against real output rather than trusted. Note the parser takes
only the FIRST status line per commit (`elif path is None`), which is a second
assumption worth testing: verify what `--follow --name-status` emits for a commit that
touches the followed file AND others, and for a commit that renames it as part of a
bulk directory move.

Suggested more robust approach, which sidesteps out-parsing `--follow` entirely: keep
`--follow` only to get the revision LIST, then resolve each revision's path by asking
git directly per revision, falling back across the known historical names. Combined
with defect 1's tolerant fetch, an occasional unresolved path then costs one point.

## Interaction with the exclude fix (already landed)

`run.bash` selects the complexity file as `revisions.json` row 0, which is
exclude-filtered. Adding `**/Migrations/**`, `**/l10n/gen/**`, and `**/*.g.dart` to the
built-in excludes changed that selection from a generated EF Core snapshot to
`DefaultInfrastructureModule.cs`, a real hotspot.

THAT DID NOT FIX THIS TICKET: the new selection fails too, for the rename reason above.
The exclude change only means the failure is now about a file worth charting.

Consider whether `run.bash` should fall back to the next candidate when the top one
fails, so a single awkward file does not cost the whole figure.

## Acceptance Criteria

- A per-revision path-resolution failure SKIPS that revision instead of aborting.
- The number of skipped revisions is reported on stderr; the figure still renders.
- Every revision failing is still a hard error (exit 2, no output file).
- `complexity_trend.py --repo <keeper-core> --file
  Keeper.AdminServer/adminSrc/AdminServer.Infrastructure/DefaultInfrastructureModule.cs`
  produces an SVG. That file crosses a `src` to `adminSrc` rename and is the
  regression case.
- A test covering a fixture repo whose target file has been renamed at least once,
  asserting the series spans both sides of the rename.
- `run.bash` renders the complexity figure on a repository with renamed hotspots.
- `ruff` clean; `make build` green.


## Notes

**2026-07-29T03:34:11Z**

Fixed by removing the fragile dependency, not by patching it.

complexity_trend.py:
- Added git_try() (tolerant sibling of git(), returns None on non-zero) and
  fetch_at(repo, rev, candidates), which tries each name the file ever carried
  against a revision and returns (path, source) or None. git() stays fatal for
  the initial log.
- enumerate_revs() now returns ((rev, date) oldest-first, candidate paths). It
  mines --follow --name-status ONLY for header lines (the commit list) and the
  set of path fields (the names). It no longer pairs a status line to its
  header, so defect 2 cannot recur by construction.
- Skipped revisions are counted and reported on stderr; the figure still
  renders. Zero resolvable revisions is still exit 2 with no output file.
- The 'wrote ...' line now reads '(N revisions across M paths: a, b)', naming
  the paths the series was read at.

run.bash: the complexity figure now tries the top 5 hotspots in rank order
until one renders, instead of betting on revisions.json row 0.

Tests (complexity_trend_test.py, 7 total, all green):
- test_series_spans_both_sides_of_a_directory_rename: the src -> adminSrc
  regression shape; asserts both paths are in the reported series.
- test_unresolvable_revision_is_skipped_not_fatal: delete-then-readd; asserts
  'skipped 1 of 3 revisions' and exit 0 with an SVG.
- test_every_revision_unresolvable_is_fatal: exit 2, no output file.

For the next person: defect 2 (the rename mis-pairing) could NOT be reproduced.
Probes across bulk directory moves, rename+rewrite, path swaps, merge-side
renames, and delete/readd all showed the old pairing to be correct. What DOES
reproduce the reported fatal abort is a delete-then-readd history, where the
listed revision has no resolvable path under any name. If the field report's
mis-pairing resurfaces, the resolution path is now per-revision and the
name-set mining is order-tolerant, so it should degrade to a skipped point
rather than a failure. Verified end-to-end via run.bash against a fixture repo
with a renamed hotspot and a delete/readd gap: 10 of 11 revisions plotted,
1 skipped and reported, both rename sides present. ruff and make build green.
See docs/specs/learnings.md (cod-n3qm).
