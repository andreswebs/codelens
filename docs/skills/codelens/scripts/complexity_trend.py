# /// script
# requires-python = ">=3.12"
# dependencies = ["matplotlib"]
# ///
# matplotlib's stubs type methods with **kwargs: Unknown, so every Axes/Figure call
# reads as partially unknown under strict; that third-party-origin rule is off here.
# pyright: reportUnknownMemberType=false
"""Plot the indentation-complexity trend of one file across git history.

Reads the LIVE repo (not codelens): enumerates the file's revisions with
`git log`, fetches each historical version with `git show`, and measures logical
indentation (4 spaces or 1 tab = 1 level; blank lines ignored). Emits the
oldest-first time series and a line chart of total complexity with LOC overlaid.

Shapes to read: deteriorating (rising, act), refactored (a dip), stable.

A revision whose path cannot be resolved (a deletion, an undetected copy) is
skipped and reported on stderr; only a series with no resolvable revision at all
is an error.

Usage:
  uv run scripts/complexity_trend.py --repo . --file src/foo.go -o trend.svg
Exit codes: 0 ok; 2 usage or nothing resolvable; 3 no history for the file.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

TAB = 8  # spaces-per-tab when expanding leading whitespace

_HASH = re.compile(r"\A[0-9a-f]{40}\Z")


def git(repo: str, *args: str) -> str:
    r = subprocess.run(["git", "-C", repo, *args], capture_output=True, text=True)
    if r.returncode != 0:
        print(
            f"complexity_trend.py: git {' '.join(args)}: {r.stderr.strip()}",
            file=sys.stderr,
        )
        raise SystemExit(2)
    return r.stdout


def git_try(repo: str, *args: str) -> str | None:
    """Tolerant `git`: None instead of exit on a non-zero status.

    Per-revision content fetches are best effort. A long history with copies,
    merges, deletions, and case-only renames will occasionally fail to resolve a
    path, and a miss must cost one data point rather than the whole figure.
    """
    r = subprocess.run(["git", "-C", repo, *args], capture_output=True, text=True)
    return None if r.returncode != 0 else r.stdout


def enumerate_revs(
    repo: str, file: str, rng: str
) -> tuple[list[tuple[str, str]], list[str]]:
    """Return ((rev, date) oldest-first, candidate paths newest-name-first).

    `--follow` yields commits from before the file reached its current path, so
    each revision must be read at the path it carried *then*. Pairing a
    `--name-status` line to its commit header is not reliable across bulk
    directory moves, so the status lines are mined only for the SET of names the
    file ever carried; `fetch_at` resolves each revision against that set by
    asking git per revision. Names are ordered newest-first because the log is:
    for `R<score>\\t<old>\\t<new>` the new side is the later name.
    """
    log = git(
        repo,
        "log",
        "--follow",
        "--name-status",
        "--format=%H\t%ad",
        "--date=short",
        rng,
        "--",
        file,
    )
    revs: list[tuple[str, str]] = []
    names = [file]
    for line in log.splitlines():
        if not line.strip():
            continue
        parts = line.split("\t")
        if len(parts) == 2 and _HASH.match(parts[0]):
            revs.append((parts[0], parts[1]))
        else:
            names.extend(reversed(parts[1:]))
    revs.reverse()  # oldest first
    return revs, list(dict.fromkeys(names))


def fetch_at(repo: str, rev: str, candidates: list[str]) -> tuple[str, str] | None:
    """Return (path, source) for `rev`, trying each historical name in turn."""
    for path in candidates:
        src = git_try(repo, "show", f"{rev}:{path}")
        if src is not None:
            return path, src
    return None


def indentation(source: str) -> tuple[int, float]:
    """Return (n_lines, total_complexity) for logical indentation."""
    n = 0
    total = 0.0
    for line in source.splitlines():
        stripped = line.strip()
        if not stripped:
            continue
        n += 1
        leading = line[: len(line) - len(line.lstrip())]
        spaces = leading.replace("\t", " " * TAB)
        total += len(spaces) / 4.0
    return n, total


def main() -> None:
    ap = argparse.ArgumentParser(
        description="Indentation-complexity trend for one file."
    )
    ap.add_argument("--repo", default=".")
    ap.add_argument("--file", required=True, help="repo-relative path to the hotspot")
    ap.add_argument("--start", help="oldest commit-ish (default: full history)")
    ap.add_argument("--end", default="HEAD")
    ap.add_argument(
        "-o", "--out", required=True, help="output SVG/PNG (extension picks format)"
    )
    args = ap.parse_args()

    rng = f"{args.start}..{args.end}" if args.start else args.end
    revs, candidates = enumerate_revs(args.repo, args.file, rng)
    if not revs:
        print(f"complexity_trend.py: no history for {args.file}", file=sys.stderr)
        raise SystemExit(3)

    dates: list[str] = []
    totals: list[float] = []
    locs: list[int] = []
    paths: list[str] = []
    skipped = 0
    for rev, date in revs:
        got = fetch_at(args.repo, rev, candidates)
        if got is None:
            skipped += 1
            continue
        path, src = got
        if path not in paths:
            paths.append(path)
        n, total = indentation(src)
        dates.append(date)
        totals.append(round(total, 2))
        locs.append(n)

    if not totals:
        print(
            f"complexity_trend.py: no revision of {args.file} could be resolved "
            f"({skipped} of {len(revs)} revisions failed)",
            file=sys.stderr,
        )
        raise SystemExit(2)
    if skipped:
        print(
            f"complexity_trend.py: skipped {skipped} of {len(revs)} revisions "
            "whose path could not be resolved",
            file=sys.stderr,
        )

    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    fig, ax1 = plt.subplots(figsize=(10, 4.5))
    x = range(len(totals))
    ax1.plot(x, totals, color="#d1495b", label="indentation complexity")
    ax1.set_ylabel("total complexity", color="#d1495b")
    ax1.set_xlabel(f"revisions of {args.file} (oldest -> newest)")
    ax2 = ax1.twinx()
    ax2.plot(x, locs, color="#4b6cb7", alpha=0.6, label="lines of code")
    ax2.set_ylabel("lines of code", color="#4b6cb7")
    ax1.set_title(f"Complexity trend: {Path(args.file).name}")
    fig.tight_layout()
    fig.savefig(args.out)
    print(
        f"wrote {args.out} ({len(totals)} revisions across "
        f"{len(paths)} paths: {', '.join(paths)})",
        file=sys.stderr,
    )


if __name__ == "__main__":
    main()
