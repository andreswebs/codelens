---
id: cod-1kww
status: closed
deps: []
links: []
created: 2026-07-29T03:14:09Z
type: bug
priority: 2
assignee: Andre Silva
tags: [codelens, skill, digest, bug]
---
# Bug: digest.py counts commit vocabulary per changed file instead of per commit

`digest.py`'s commit-vocabulary section counts each commit message once per CHANGED
FILE instead of once per COMMIT, because `codelens parse` emits one row per
(commit, file) pair.

Found 2026-07-28 by an end-to-end run against a 30,800-commit repository, where the
digest's vocabulary list visibly disagreed with the word cloud rendered from the same
data in the same report.

## Why this is worse than a wrong number

It REORDERS the ranking rather than merely scaling it, because the inflation factor is
each term's average files-per-commit, which varies wildly by term. Measured on a repo
averaging 5.0 files per commit:

| term       | digest.md | actual (per commit) | inflation |
| ---------- | --------- | ------------------- | --------- |
| `dev`      | 2,901     | 20                  | **145x**  |
| `merge`    | 6,401     | 182                 | 35x       |
| `refactor` | 3,304     | 546                 | 6.1x      |
| `revert`   | 1,290     | 83                  | 15x       |
| `feat`     | 7,373     | 1,266               | 5.8x      |

`dev` reads as the 5th most common term when it appears in 20 of 7,254 commits. Terms
used in wide commits (merges, sweeping renames, generated-file regeneration) float to
the top and displace the real vocabulary.

## Why it matters beyond cosmetics

`references/reporting.md` documents `digest.md` as THE grounding for the findings
prose: "Write each block from `out/digest.md` (open the raw JSON only for a number the
digest omits)." So this defect propagates directly into every report written from a
digest. It did exactly that on discovery: the draft findings asserted "`revert` at
1,290 is high enough to be worth a look at whether a specific area is churning
through reverts", when `revert` is in 83 commits (1.1 percent) and is a non-finding.

The report also then contradicted its own figure, because `commit_cloud.py` dedupes
correctly and the two sat on the same page.

## Design

## Root cause

`digest.py`, commit-vocabulary section:

```python
words: Counter[str] = Counter()
for r in load_rows(d / "parse.json"):
    msg = str(r.get("message") or "").lower()
    for tok in re.findall(r"[a-z][a-z0-9_-]{2,}", msg):
        if tok not in STOP:
            words[tok] += 1
```

`load_rows(parse.json)` yields one row per (commit, file). The message is repeated on
every row for the same commit, so a commit touching 50 files contributes its message
50 times.

`commit_cloud.py` already does this correctly and is the reference implementation:

```python
for msg in by_rev.values():          # <- deduped by revision
    for tok in TOKEN.findall(msg.lower()):
```

## Fix

Dedupe by `rev` before counting, mirroring `commit_cloud.py`:

```python
by_rev: dict[str, str] = {}
for r in load_rows(d / "parse.json"):
    by_rev[str(r["rev"])] = str(r.get("message") or "")
words: Counter[str] = Counter()
for msg in by_rev.values():
    for tok in re.findall(r"[a-z][a-z0-9_-]{2,}", msg.lower()):
        if tok not in STOP:
            words[tok] += 1
```

Note `parse.json`'s `rev` column is the commit short hash (semantic `commit_id`), and
its `message` column is the commit subject (semantic `text`), constant per rev.

## Consider also reporting the denominator

The counts become far easier to read against a total. `digest.md` already reports
commits in its summary block, so the vocabulary line could carry a percentage, which
is what makes "`feat` in 17.5 percent of commits" a usable statement and "`feat`
1,266" a bare number. Optional, but it is the difference between a number and a
finding.

## Cross-check while fixing

Two other `digest.py` sections read `parse.json`; confirm neither has the same
per-row/per-commit confusion. Every other section reads an aggregating analysis
(`revisions.json`, `coupling.json`, ...) where one row is already one entity or pair,
so they are not at risk.

This is the SECOND defect of this shape in this file: cod-b9td was the analysis-window
sampling bug, also a misreading of the git log's structure. Worth a moment's thought
about whether the parse-derived sections deserve a shared helper that yields one item
per commit.

## Acceptance Criteria

- The commit-vocabulary section counts each commit message exactly once, regardless of
  how many files the commit touched.
- On a fixture with a known multi-file commit, the reported count for a term in that
  commit's message is 1, not the file count. That is the regression test.
- The digest's top-terms ranking agrees with `commit_cloud.py`'s ranking on the same
  input, since both then count per revision. A test asserting the two agree would pin
  this permanently.
- The other `digest.py` sections are confirmed unaffected.
- `ruff` clean; `make build` green.


## Notes

**2026-07-29T03:23:26Z**

Fixed by collapsing parse.json rows by `rev` before tallying commit vocabulary.

Implementation: new `commit_messages(path)` helper in digest.py returns one subject per commit, mirroring commit_cloud.py's dedupe (keys on `rev`, falls back to the message when `rev` is absent, as commit_cloud does). The vocabulary section now iterates that instead of raw rows. Extracted as a named helper rather than inlined because the ticket flagged this as the second parse-structure misreading in this file (after cod-b9td); any future parse-derived section should call it.

Cross-check done: parse.json is read in exactly ONE place in digest.py (the vocabulary section). Every other section reads an aggregating analysis where one row is already one entity/pair, so none were at risk. Confirmed by grepping all load_rows call sites.

Denominator added (the ticket's optional item): the section now emits a leading line `- counted once per commit over N commits with a subject`. Kept the `term(n)` format unchanged rather than inlining percentages, so nothing downstream that reads the existing format breaks; the denominator on its own line makes the counts readable without format churn. references/reporting.md does not pin the line format, so no doc change was needed.

Tests (TDD, 4 new, all red before the fix):
- test_message_counted_once_per_commit: 1 commit / 50 files -> count 1.
- test_wide_commits_do_not_outrank_frequent_terms: pins the REORDERING, not just the scaling. One 10-file 'sweep' commit vs three 1-file 'disbursement' commits; per-row ranks sweep first (10 v 3), per-commit ranks disbursement first (3 v 1).
- test_ranking_agrees_with_commit_cloud: replicates commit_cloud's dedupe-by-rev algorithm over digest's tokens/stopwords and asserts both counts AND order match, so the digest and the word cloud on the same report page can never contradict each other again.
- test_denominator_is_reported.
Existing test_stopwords_dropped was given explicit `rev` values (it previously relied on rows with no rev).

Verified end-to-end on this repo: 977 parse rows -> 17 commits; digest reports refactoring(5) and `jq | sort -u | grep -ci refactoring` confirms 5 unique commits.

All 8 python suites OK, ruff check clean, make build green. Note: `ruff format --check` flags digest.py and digest_test.py, but it does so at HEAD too (pre-existing long lines); I left those alone and only kept my own new code format-clean.
