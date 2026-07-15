# /// script
# requires-python = ">=3.12"
# dependencies = ["matplotlib", "squarify"]
# ///
# matplotlib types methods with **kwargs: Unknown and squarify ships no stubs, so
# both read as partially unknown under strict; those third-party rules are off here.
# pyright: reportUnknownMemberType=false, reportMissingTypeStubs=false
"""Fractal figures: developer effort per module, from codelens output.

Consumes `codelens entity-effort` -> entity, author, author_revs, total_revs. Each
module is a panel; sub-rectangles are per-author with area proportional to that
author's revision share, one color per developer. Reads ownership patterns:
single developer, balanced (high main-dev ownership predicts fewer defects), or
many minor contributors (defect risk).

Usage:
  uv run scripts/fractal.py --effort effort.json [--top 16] -o fractal.svg
Exit codes: 0 ok; 2 usage; 3 empty.
"""

from __future__ import annotations

import argparse
import json
import math
import sys
from collections import defaultdict
from pathlib import Path
from typing import Any, NoReturn, cast


def die(msg: str, code: int) -> NoReturn:
    print(f"fractal.py: {msg}", file=sys.stderr)
    raise SystemExit(code)


def rows(path: str) -> list[dict[str, Any]]:
    doc: Any = json.loads(Path(path).read_text(encoding="utf-8"))
    data = cast("dict[str, Any]", doc).get("rows") if isinstance(doc, dict) else doc
    if not data:
        die("empty result", 3)
    return cast("list[dict[str, Any]]", data)


def main() -> None:
    ap = argparse.ArgumentParser(description="Fractal figures of developer effort.")
    ap.add_argument("--effort", required=True, help="codelens entity-effort JSON")
    ap.add_argument(
        "--top",
        type=int,
        default=16,
        help="show the N entities with the most total revs",
    )
    ap.add_argument("-o", "--out", required=True)
    args = ap.parse_args()

    # entity -> {author -> author_revs}; and entity -> total_revs
    per_entity: dict[str, dict[str, int]] = defaultdict(dict)
    totals: dict[str, int] = {}
    authors: set[str] = set()
    for r in rows(args.effort):
        e, a = r["entity"], r["author"]
        per_entity[e][a] = r["author_revs"]
        totals[e] = r["total_revs"]
        authors.add(a)

    top = sorted(per_entity, key=lambda e: totals.get(e, 0), reverse=True)[: args.top]
    if not top:
        die("no entities to plot", 3)

    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    import squarify

    cmap = plt.get_cmap("tab20")
    color_of = {a: cmap(i % 20) for i, a in enumerate(sorted(authors))}

    cols = min(4, len(top))
    rows_n = math.ceil(len(top) / cols)
    fig, axes = plt.subplots(rows_n, cols, figsize=(3 * cols, 3 * rows_n))
    axes = [axes] if len(top) == 1 else list(axes.flat)

    for ax, entity in zip(axes, top):
        shares = sorted(per_entity[entity].items(), key=lambda kv: kv[1], reverse=True)
        sizes = [v for _, v in shares]
        colors = [color_of[a] for a, _ in shares]
        squarify.plot(
            sizes=sizes,
            color=colors,
            ax=ax,
            pad=True,
            bar_kwargs={"edgecolor": "white", "linewidth": 0.5},
        )
        ax.set_title(
            f"{entity.split('/')[-1]}\n{len(shares)} devs, {totals[entity]} revs",
            fontsize=8,
        )
        ax.axis("off")
    for ax in axes[len(top) :]:
        ax.axis("off")

    fig.suptitle("Developer effort per module (area = revision share)", fontsize=12)
    fig.tight_layout()
    fig.savefig(args.out)
    print(
        f"wrote {args.out} ({len(top)} modules, {len(authors)} authors)",
        file=sys.stderr,
    )


if __name__ == "__main__":
    main()
