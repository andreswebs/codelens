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

Usage:
  uv run scripts/complexity_trend.py --repo . --file src/foo.go -o trend.svg
Exit codes: 0 ok; 2 usage; 3 no history for the file.
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
        print(f"complexity_trend.py: git {' '.join(args)}: {r.stderr.strip()}", file=sys.stderr)
        raise SystemExit(2)
    return r.stdout


def enumerate_revs(repo: str, file: str, rng: str) -> list[tuple[str, str, str]]:
    """Return (rev, date, path_at_rev) oldest-first across the file's whole life.

    `--follow` yields commits from before the file reached its current path, so
    each revision must be fetched at the path it carried *then*. `--name-status`
    surfaces that path per commit: a rename is `R<score>\\t<old>\\t<new>` (the new
    side is the path this commit produced), everything else is `<status>\\t<path>`.
    """
    log = git(repo, "log", "--follow", "--name-status",
              "--format=%H\t%ad", "--date=short", rng, "--", file)
    revs: list[tuple[str, str, str]] = []
    rev: str | None = None
    date = ""
    path: str | None = None
    for line in log.splitlines():
        if not line.strip():
            continue
        parts = line.split("\t")
        if len(parts) == 2 and _HASH.match(parts[0]):
            if rev is not None and path is not None:
                revs.append((rev, date, path))
            rev, date = parts
            path = None
        elif path is None:  # first status line of the current commit
            path = parts[2] if parts[0][:1] in ("R", "C") else parts[1]
    if rev is not None and path is not None:
        revs.append((rev, date, path))
    revs.reverse()  # oldest first
    return revs


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
    ap = argparse.ArgumentParser(description="Indentation-complexity trend for one file.")
    ap.add_argument("--repo", default=".")
    ap.add_argument("--file", required=True, help="repo-relative path to the hotspot")
    ap.add_argument("--start", help="oldest commit-ish (default: full history)")
    ap.add_argument("--end", default="HEAD")
    ap.add_argument("-o", "--out", required=True, help="output SVG/PNG (extension picks format)")
    args = ap.parse_args()

    rng = f"{args.start}..{args.end}" if args.start else args.end
    revs = enumerate_revs(args.repo, args.file, rng)
    if not revs:
        print(f"complexity_trend.py: no history for {args.file}", file=sys.stderr)
        raise SystemExit(3)

    dates: list[str] = []
    totals: list[float] = []
    locs: list[int] = []
    for rev, date, path in revs:
        src = git(args.repo, "show", f"{rev}:{path}")
        n, total = indentation(src)
        dates.append(date)
        totals.append(round(total, 2))
        locs.append(n)

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
    print(f"wrote {args.out} ({len(totals)} revisions)", file=sys.stderr)


if __name__ == "__main__":
    main()
