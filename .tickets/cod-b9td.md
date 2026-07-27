---
id: cod-b9td
status: closed
deps: []
links: []
created: 2026-07-27T16:23:46Z
type: bug
priority: 2
assignee: Andre Silva
tags: [codelens, skill, reporting]
---
# Bug: digest.py understates the analysis window when a commit's numstat block exceeds the sample

`digest.py` reports the analysis window by sampling only the first and last 2,000 bytes of
`git.log`. When a single commit's numstat block is larger than that sample, the tail contains no
date header at all and the reported window collapses, understating the period the report covers.

Found by running the codelens skill against this repository itself on 2026-07-27. The digest
reported:

    - window dates seen: 2026-07-27 .. 2026-07-27

The true window was 2026-07-14 to 2026-07-27, thirteen days across 7 distinct dates. Verified
against the same `git.log` (36,702 bytes):

- dates found in `text[:2000]`: `['2026-07-27']`
- dates found in `text[-2000:]`: `[]` (none)
- dates found in the whole file: `2026-07-14 .. 2026-07-27`, 7 distinct

The cause is that the log is reverse-chronological, so the tail is the OLDEST commit, and this
repository's initial commit touched hundreds of files. Its numstat block runs to many kilobytes
of `<added>\t<deleted>\t<path>` lines, none of which carry a date, so the 2,000-byte tail sample
never reaches the commit's header line. With `min()` and `max()` then computed over a single
date, the window renders as one day.

Two things make this worse than a cosmetic slip:

1. The window line is the FIRST thing in the digest and it feeds the report's framing. A reader
   or agent grounding its findings in the digest is told the analysis covers one day when it
   covers thirteen, which changes how every subsequent signal should be weighed. Thin history is
   precisely the caveat a findings report must state accurately.
2. The same digest CONTRADICTS ITSELF: its churn section lists all 7 dates correctly, because
   that section reads `abs-churn.json` rather than sampling the log.

Any repository with a large initial commit or any single large commit at the log's tail hits
this, which is most repositories. It is not specific to this one.

Skills: /python, /ruff (the script is `uv run` with PEP 723 inline metadata; `scripts/ruff.toml`
governs lint).

## Design

### Root cause

`docs/skills/codelens/scripts/digest.py:107-113`:

```python
# --- window (from git.log first/last dates) ---
gl = d / "git.log"
if gl.is_file():
    text = gl.read_text(encoding="utf-8")
    dates: list[str] = re.findall(r"\b\d{4}-\d{2}-\d{2}\b", text[:2000] + text[-2000:])
    if dates:
        w(f"- window dates seen: {min(dates)} .. {max(dates)}")
```

Two defects in one expression. The 2,000-byte end sampling is unsound because entry size is
unbounded, and the bare `\b\d{4}-\d{2}-\d{2}\b` pattern would also match a date appearing
anywhere else in the log, including inside a commit subject or a file path.

### Fix

Parse the date out of the log's ENTRY HEADER lines, across the whole file. The header format is
fixed and known: `run.bash` generates the log via `codelens print-log-command`, which emits
`--pretty=format:'--%h--%ad--%aN--%s'`, so every entry header is
`--<hash>--<YYYY-MM-DD>--<author>--<subject>` and numstat lines never start with `--`.

```python
# Entry headers are --<hash>--<date>--<author>--<subject> (see
# `codelens print-log-command`); numstat lines never start with "--". Anchoring on
# the header is what keeps a date inside a commit subject or a file path out of the
# window, and scanning every line rather than sampling the file's ends is what keeps
# a large commit from hiding the boundary: the log is reverse-chronological, so the
# oldest entry is at the tail behind however many numstat lines it carries.
HEADER_DATE = re.compile(r"^--[0-9a-f]+--(\d{4}-\d{2}-\d{2})--")
```

Then iterate lines and collect matches. Reading line by line rather than slurping also drops the
whole-file `read_text` for what can be a multi-megabyte log:

```python
dates = sorted({m.group(1) for line in gl.open(encoding="utf-8") if (m := HEADER_DATE.match(line))})
if dates:
    w(f"- window dates seen: {dates[0]} .. {dates[-1]} ({len(dates)} active dates)")
```

Adding the distinct-date count is worth it: it distinguishes 7 active days inside a 13-day span
from 13 consecutive ones, which is exactly the density question a reader of a thin-history report
asks next. Optional, but cheap and useful.

Be tolerant of the 3-field stock git2 header (no subject), which codelens also accepts per
`docs/cli-design.md` section 5: the trailing `--` in the pattern above still matches
`--<hash>--<date>--<author>` only if a fourth field follows. Verify against a 3-field fixture and
relax the pattern to `^--[0-9a-f]+--(\d{4}-\d{2}-\d{2})--` requiring only the author separator,
which the example above already does. Confirm with a test rather than by reading.

### Alternative considered and rejected

Deriving the window from `abs-churn.json`, which the digest already loads at line 165 and which
carried all 7 dates correctly in the failing run. Rejected: `absolute-churn` only reports dates
that have numstat churn, so a commit with no line changes (a merge, a mode change, a pure rename
under `--no-renames`) would be invisible, and the window would silently narrow again for a
different reason. Parsing the log's headers is the only source that sees every commit.

### Tests

`docs/skills/codelens/scripts/digest_test.py` exists and drives the script as a subprocess.
Add:

- A fixture whose OLDEST entry carries a numstat block larger than 2,000 bytes, reproducing this
  bug exactly. Assert the reported window spans both the newest and oldest dates. This is the
  regression test and it must fail against the current implementation.
- A fixture with a date-like string in a commit subject (for example `--a1--2026-01-02--Bob--fix
  the 1999-12-31 rollover`) and assert the window is not widened by it. The current bare pattern
  would match `1999-12-31`.
- A 3-field stock git2 fixture (no subject) to confirm the header pattern still matches.
- A single-commit log: window renders with identical start and end, and the count is 1.

### Verification against the real case

```sh
bash docs/skills/codelens/scripts/run.bash --repo . --out /tmp/check --full-history
grep 'window dates' /tmp/check/digest.md
```

Must report `2026-07-14 .. 2026-07-27` for this repository, not a single day.

### Files touched

```text
docs/skills/codelens/scripts/digest.py       header-anchored, whole-file date scan; optional active-date count
docs/skills/codelens/scripts/digest_test.py  the four cases above, the first being the regression
```

Out of scope: the rest of the digest. Every other section reads analysis JSON rather than the raw
log and is correct.

## Acceptance Criteria

- The digest's window line reports the true first and last commit dates regardless of how large
  any single entry's numstat block is. Against this repository it reads
  `2026-07-14 .. 2026-07-27`, not `2026-07-27 .. 2026-07-27`.
- Dates are matched only on entry header lines, so a date inside a commit subject or a file path
  cannot widen or shift the reported window.
- The whole log is scanned; no fixed-size sampling of the file's ends remains.
- The 3-field stock git2 header (no commit subject) is still recognised.
- `digest_test.py` carries a regression test with an oversized oldest entry that fails against
  the pre-fix implementation.
- The digest no longer contradicts its own churn section on which dates the window covers.
- ruff clean under `docs/skills/codelens/scripts/ruff.toml`; `digest_test.py` passes.


## Notes

**2026-07-27T16:32:39Z**

Fixed digest.py window computation: replaced the 2,000-byte head+tail sample and bare date regex with a whole-file, header-anchored scan (^--[0-9a-f]+--(YYYY-MM-DD)--). Matches only entry-header dates so a date in a commit subject or file path can no longer widen the window, and streams line-by-line (drops the whole-file read_text). Added active-date count to the window line. Tests: 4 new TestWindow cases (oversized-oldest-entry regression, date-in-subject, 3-field stock header, single-commit) plus the existing test updated to real header format; all 14 pass, ruff clean under scripts/ruff.toml. Verified against this repo's real 36702-byte full-history log: reports '2026-07-14 .. 2026-07-27 (7 active dates)', matching the ticket. Note: make build only gates Go; run python tests via 'uv run digest_test.py' in scripts/.
