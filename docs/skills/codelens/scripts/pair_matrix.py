# /// script
# requires-python = ">=3.12"
# dependencies = ["matplotlib"]
# ///
# matplotlib types methods with **kwargs: Unknown, so calls read as partially
# unknown under strict; that third-party-origin rule is off here.
# pyright: reportUnknownMemberType=false
"""Static adjacency-matrix heatmap for a symmetric pair analysis (no D3).

The static counterpart of the coupling graph and the communication network, for a
report. Generic over the pair columns, so one script serves both:
  coupling        --a-col entity --b-col coupled --weight-col degree
  communication   --a-col author --b-col peer    --weight-col strength

Picks the top-N entities by total involvement and renders their pairwise weights as
an annotated, symmetric heatmap. Reads a codelens envelope (or bare rows) as a JSON
file or '-' for stdin. Outputs SVG or PNG (the -o extension picks the format) and,
with --json-out, the ordered entities and the matrix.

For the communication network, pass a --note carrying the guardrail framing
(coordination risk, aggregate to teams; never a performance ranking). See
references/interpretation.md.

Exit codes: 0 ok; 2 usage/bad input; 3 empty result.
"""

from __future__ import annotations

import argparse
import json
import sys
from collections import defaultdict
from pathlib import Path
from typing import Any, NoReturn, cast


def die(msg: str, code: int) -> NoReturn:
    print(f"pair_matrix.py: {msg}", file=sys.stderr)
    raise SystemExit(code)


def load_rows(path: str) -> list[dict[str, Any]]:
    text = sys.stdin.read() if path == "-" else Path(path).read_text(encoding="utf-8")
    try:
        doc: Any = json.loads(text)
    except json.JSONDecodeError as e:
        die(f"invalid JSON in {path}: {e}", 2)
    data = cast("dict[str, Any]", doc).get("rows") if isinstance(doc, dict) else doc
    if not isinstance(data, list):
        die("no 'rows' array in input", 2)
    return cast("list[dict[str, Any]]", data)


def build_matrix(
    rows: list[dict[str, Any]], a_col: str, b_col: str, w_col: str, top: int
) -> tuple[list[str], list[list[float]]]:
    """Return (ordered entities, symmetric weight matrix) for the top-N entities by
    total involvement. Entities tie-break by name for determinism."""
    weight: dict[tuple[str, str], float] = {}
    involvement: dict[str, float] = defaultdict(float)
    for r in rows:
        a, b = r.get(a_col), r.get(b_col)
        if a is None or b is None or w_col not in r:
            continue
        w = float(r[w_col])
        weight[(a, b)] = w
        involvement[a] += w
        involvement[b] += w
    if not involvement:
        die(f"no rows carried columns {a_col!r}, {b_col!r}, {w_col!r}", 3)

    ordered = sorted(involvement, key=lambda e: (-involvement[e], e))[:top]
    idx = {e: i for i, e in enumerate(ordered)}
    n = len(ordered)
    matrix = [[0.0] * n for _ in range(n)]
    for (a, b), w in weight.items():
        if a in idx and b in idx:
            i, j = idx[a], idx[b]
            matrix[i][j] = w
            matrix[j][i] = w  # symmetric
    return ordered, matrix


def draw(
    entities: list[str], matrix: list[list[float]], title: str, note: str, out: str
) -> None:
    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    labels = [e.split("/")[-1] for e in entities]
    n = len(entities)
    fig, ax = plt.subplots(figsize=(max(6, 0.5 * n + 2), max(5, 0.5 * n + 1.5)))
    im = ax.imshow(matrix, cmap="YlOrRd", vmin=0)
    ax.set_xticks(range(n))
    ax.set_yticks(range(n))
    ax.set_xticklabels(labels, rotation=45, ha="right", fontsize=7)
    ax.set_yticklabels(labels, fontsize=7)
    # Annotate non-zero cells so exact strengths are legible.
    for i in range(n):
        for j in range(n):
            v = matrix[i][j]
            if v:
                ax.text(
                    j, i, f"{v:g}", ha="center", va="center", fontsize=6, color="#222"
                )
    ax.set_title(title)
    fig.colorbar(im, ax=ax, fraction=0.045, pad=0.04, label="strength")
    if note:
        fig.text(0.5, 0.01, note, ha="center", fontsize=7, color="#666")
    fig.tight_layout(rect=(0, 0.03, 1, 1) if note else None)
    fig.savefig(out)


def main() -> None:
    ap = argparse.ArgumentParser(
        description="Static symmetric pair matrix (coupling/communication)."
    )
    ap.add_argument(
        "--pairs", required=True, help="codelens pair JSON, or '-' for stdin"
    )
    ap.add_argument(
        "--a-col", required=True, help="first entity column (e.g. entity, author)"
    )
    ap.add_argument(
        "--b-col", required=True, help="second entity column (e.g. coupled, peer)"
    )
    ap.add_argument(
        "--weight-col", required=True, help="pair weight column (e.g. degree, strength)"
    )
    ap.add_argument(
        "--top",
        type=int,
        default=30,
        help="show the N most-involved entities (default 30)",
    )
    ap.add_argument("--title", default="Pair matrix")
    ap.add_argument(
        "--note",
        default="",
        help="footnote (e.g. coordination-risk framing for communication)",
    )
    ap.add_argument(
        "-o", "--out", required=True, help="output SVG/PNG (extension picks format)"
    )
    ap.add_argument(
        "--json-out", help="also write {entities, matrix} as JSON (for tests)"
    )
    args = ap.parse_args()

    entities, matrix = build_matrix(
        load_rows(args.pairs), args.a_col, args.b_col, args.weight_col, args.top
    )
    draw(entities, matrix, args.title, args.note, args.out)

    if args.json_out:
        Path(args.json_out).write_text(
            json.dumps({"entities": entities, "matrix": matrix}, indent=2),
            encoding="utf-8",
        )

    pair_count = sum(
        1
        for i in range(len(entities))
        for j in range(i + 1, len(entities))
        if matrix[i][j]
    )
    print(
        f"wrote {args.out} ({len(entities)} entities, {pair_count} pairs)",
        file=sys.stderr,
    )


if __name__ == "__main__":
    main()
