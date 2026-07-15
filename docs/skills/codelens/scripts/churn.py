# /// script
# requires-python = ">=3.12"
# dependencies = ["matplotlib"]
# ///
# matplotlib's stubs type methods with **kwargs: Unknown, so every Axes/Figure call
# reads as partially unknown under strict; that third-party-origin rule is off here.
# pyright: reportUnknownMemberType=false
"""Churn time series (added vs deleted) or summary tiles from codelens output.

  churn:   codelens absolute-churn -> date, added, deleted, commits
  summary: codelens summary        -> statistic, value

Usage:
  uv run scripts/churn.py --churn churn.json -o churn.svg
  uv run scripts/churn.py --summary summary.json -o summary.svg
Exit codes: 0 ok; 2 usage; 3 empty.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any, cast


def rows(path: str) -> list[dict[str, Any]]:
    doc: Any = json.loads(Path(path).read_text(encoding="utf-8"))
    data = cast("dict[str, Any]", doc).get("rows") if isinstance(doc, dict) else doc
    if not data:
        print("churn.py: empty result", file=sys.stderr)
        raise SystemExit(3)
    return cast("list[dict[str, Any]]", data)


def plot_churn(path: str, out: str) -> None:
    r = rows(path)
    dates = [x["date"] for x in r]
    added = [x["added"] for x in r]
    deleted = [-x["deleted"] for x in r]

    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    fig, ax = plt.subplots(figsize=(11, 4.5))
    x = range(len(dates))
    ax.bar(x, added, color="#4b9e5f", label="added")
    ax.bar(x, deleted, color="#d1495b", label="deleted")
    ax.axhline(0, color="#888", linewidth=0.8)
    step = max(1, len(dates) // 12)
    ax.set_xticks(list(x)[::step])
    ax.set_xticklabels(dates[::step], rotation=45, ha="right")
    ax.set_ylabel("lines")
    ax.set_title("Code churn over time")
    ax.legend()
    fig.tight_layout()
    fig.savefig(out)
    print(f"wrote {out} ({len(dates)} periods)", file=sys.stderr)


def plot_summary(path: str, out: str) -> None:
    r = rows(path)
    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    fig, axes = plt.subplots(1, len(r), figsize=(2.6 * len(r), 2.2))
    if len(r) == 1:
        axes = [axes]
    for ax, row in zip(axes, r):
        ax.axis("off")
        ax.text(0.5, 0.62, f"{row['value']:,}", ha="center", fontsize=26, weight="bold")
        ax.text(0.5, 0.22, row["statistic"], ha="center", fontsize=11, color="#666")
    fig.tight_layout()
    fig.savefig(out)
    print(f"wrote {out} ({len(r)} tiles)", file=sys.stderr)


def main() -> None:
    ap = argparse.ArgumentParser(description="Churn time series or summary tiles.")
    g = ap.add_mutually_exclusive_group(required=True)
    g.add_argument("--churn", help="absolute-churn JSON")
    g.add_argument("--summary", help="summary JSON")
    ap.add_argument("-o", "--out", required=True)
    args = ap.parse_args()
    if args.churn:
        plot_churn(args.churn, args.out)
    else:
        plot_summary(args.summary, args.out)


if __name__ == "__main__":
    main()
