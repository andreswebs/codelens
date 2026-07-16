# /// script
# requires-python = ">=3.12"
# dependencies = ["matplotlib", "squarify"]
# ///
# matplotlib types methods with **kwargs: Unknown and squarify ships no stubs, so
# both read as partially unknown under strict; those third-party rules are off here.
# pyright: reportUnknownMemberType=false, reportMissingTypeStubs=false
"""Static enclosure-family treemap from codelens output (no D3, embeddable SVG).

The static counterpart of enclosure.py for a report: same weight column selects the
map, same flags, same structure-first node set, so the treemap and the interactive
circle map agree.
  hotspot map    --weights revisions.json  --weight-col n_revs
  knowledge map  --weights main-dev.json    --weight-col main_dev --categorical
  code-age map   --weights code-age.json    --weight-col age_months --invert

Rectangle area is tokei LOC (from --structure) or the weight value (degraded).
Colour is a heat scale over the numeric weight (respecting --invert) or one colour
per category (--categorical; a file with no recorded value renders neutral grey).

Inputs are JSON files (or '-' for stdin on --weights). Outputs SVG or PNG (the -o
extension picks the format) and, with --json-out, the drawn leaves.

Exit codes: 0 ok; 2 usage/bad input; 3 empty result.

See references/interpretation.md for how to read the result.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any, Callable, NoReturn, cast

EXIT_USAGE = 2
EXIT_EMPTY = 3

MAX_GLOB_LEN = 1000

# A single file occupying more than this share of total mapped LOC visually
# dominates the treemap, drowning the real code. The map is left unaltered (area
# stays true LOC) and the offender named so it can be added to --exclude and the
# run repeated. Only meaningful when area is tokei LOC (--structure); without it,
# area is the weight and a dominant file is the real signal, not noise.
DOMINATION_THRESHOLD = 0.10
DOMINATION_TOP = 5

# Sentinel category for a file present in the structure but absent from the
# categorical weights (no recorded author in the window); drawn neutral grey.
UNOWNED_CATEGORY = "(unowned)"
UNOWNED_COLOR = "#d9d9d9"


def die(msg: str, code: int) -> NoReturn:
    print(f"treemap.py: {msg}", file=sys.stderr)
    raise SystemExit(code)


def warn_domination(leaves: dict[str, dict[str, Any]]) -> None:
    """Warn on stderr for each file whose LOC exceeds DOMINATION_THRESHOLD of the
    total mapped LOC, most-dominant first, capped at DOMINATION_TOP. Computed on the
    given (post-exclude) node set, so re-running after an --exclude surfaces the next
    offender. Never alters the map or the exit code."""
    total = sum(int(leaf["size"]) for leaf in leaves.values())
    if total <= 0:
        return
    offenders = sorted(
        (
            (path, loc)
            for path, leaf in leaves.items()
            if (loc := int(leaf["size"])) > DOMINATION_THRESHOLD * total
        ),
        key=lambda kv: kv[1],
        reverse=True,
    )
    for path, loc in offenders[:DOMINATION_TOP]:
        pct = round(loc / total * 100)
        print(f"dominant: {path} {pct}% ({loc} LOC)", file=sys.stderr)


def glob_to_regex(pattern: str) -> str:
    """Translate a gitignore-style glob to an anchored regex, matching the codelens
    Go side (doublestar) and enclosure.py so one glob set behaves identically."""
    i, n = 0, len(pattern)
    out = ["(?s:"]
    while i < n:
        c = pattern[i]
        if c == "*":
            i += 1
            if i < n and pattern[i] == "*":  # ** crosses separators
                i += 1
                if i < n and pattern[i] == "/":  # **/ matches zero or more dirs
                    out.append("(?:.*/)?")
                    i += 1
                else:
                    out.append(".*")
            else:
                out.append("[^/]*")
        elif c == "?":
            out.append("[^/]")
            i += 1
        elif c == "[":
            j = i + 1
            if j < n and pattern[j] in ("!", "^"):
                j += 1
            if j < n and pattern[j] == "]":
                j += 1
            while j < n and pattern[j] != "]":
                j += 1
            if j >= n:
                raise ValueError(f"unterminated character class in {pattern!r}")
            body = pattern[i + 1 : j]
            i = j + 1
            if body.startswith(("!", "^")):
                body = "^" + body[1:]
            out.append("[" + body.replace("\\", "\\\\") + "]")
        else:
            out.append(re.escape(c))
            i += 1
    out.append(r")\Z")
    return "".join(out)


def compile_globs(patterns: list[str]) -> list[re.Pattern[str]]:
    """Validate and compile globs; a malformed glob is a usage error (exit 2)."""
    compiled: list[re.Pattern[str]] = []
    for p in patterns:
        if not p:
            die("empty --include/--exclude glob", EXIT_USAGE)
        if len(p) > MAX_GLOB_LEN:
            die(f"glob exceeds {MAX_GLOB_LEN} chars", EXIT_USAGE)
        try:
            compiled.append(re.compile(glob_to_regex(p)))
        except (ValueError, re.error) as e:
            die(f"invalid glob {p!r}: {e}", EXIT_USAGE)
    return compiled


def make_path_filter(includes: list[str], excludes: list[str]) -> Callable[[str], bool]:
    """Exclude-after-include predicate over normalized paths."""
    inc = compile_globs(includes)
    exc = compile_globs(excludes)

    def keep(path: str) -> bool:
        if inc and not any(r.match(path) for r in inc):
            return False
        return not any(r.match(path) for r in exc)

    return keep


def norm_path(p: str) -> str:
    p = p.strip()
    while p.startswith("./"):
        p = p[2:]
    return p


def load_json(path: str) -> Any:
    text = sys.stdin.read() if path == "-" else Path(path).read_text(encoding="utf-8")
    try:
        return json.loads(text)
    except json.JSONDecodeError as e:
        die(f"invalid JSON in {path}: {e}", EXIT_USAGE)


def read_weights(doc: Any, col: str) -> dict[str, Any]:
    """Return path -> raw weight value from a codelens envelope (or bare rows)."""
    raw = cast("dict[str, Any]", doc).get("rows") if isinstance(doc, dict) else doc
    if not isinstance(raw, list):
        die("weights JSON has no 'rows' array", EXIT_USAGE)
    out: dict[str, Any] = {}
    for r in cast("list[dict[str, Any]]", raw):
        entity = r.get("entity")
        if entity is None or col not in r:
            continue
        out[norm_path(entity)] = r[col]
    return out


def read_structure(doc: Any) -> dict[str, int]:
    """Return path -> lines of code from a `tokei --output json` document."""
    sizes: dict[str, int] = {}
    if not isinstance(doc, dict):
        return sizes
    for lang, info in cast("dict[str, Any]", doc).items():
        if lang == "Total" or not isinstance(info, dict):
            continue
        reports = cast("dict[str, Any]", info).get("reports", [])
        if not isinstance(reports, list):
            continue
        for report in cast("list[dict[str, Any]]", reports):
            name = norm_path(report["name"])
            sizes[name] = report.get("stats", {}).get("code", 0)
    return sizes


def build_leaves(args: argparse.Namespace) -> dict[str, dict[str, Any]]:
    """Build the flat `path -> leaf` node set exactly as enclosure.py does, so the
    static treemap and the interactive circle map draw the same files."""
    weights = read_weights(load_json(args.weights), args.weight_col)
    if not weights:
        die(f"no rows carried column {args.weight_col!r}", EXIT_EMPTY)

    def strip_prefix(m: dict[str, Any]) -> dict[str, Any]:
        if not args.path_prefix:
            return m
        pre = norm_path(args.path_prefix).rstrip("/") + "/"
        return {(k[len(pre) :] if k.startswith(pre) else k): v for k, v in m.items()}

    weights = strip_prefix(weights)

    keep = make_path_filter(args.include or [], args.exclude or [])
    weights = {p: v for p, v in weights.items() if keep(p)}

    sizes: dict[str, int] = {}
    if args.structure:
        sizes = strip_prefix(read_structure(load_json(args.structure)))
        if not sizes:
            die("tokei structure has no files", EXIT_EMPTY)
        sizes = {p: v for p, v in sizes.items() if keep(p)}

    if not (sizes if args.structure else weights):
        die("no files remain after --include/--exclude filtering", EXIT_EMPTY)

    if args.categorical:
        if sizes:  # full mode: node set = structure, area = LOC, category joined
            return {
                p: {"size": loc, "category": weights.get(p, UNOWNED_CATEGORY)}
                for p, loc in sizes.items()
            }
        return {p: {"size": 1, "category": v} for p, v in weights.items()}

    numeric = {p: float(v) for p, v in weights.items()}
    hi, lo = max(numeric.values()), min(numeric.values())
    span = (hi - lo) or 1.0

    def norm(v: float) -> float:
        frac = (v - lo) / span
        return 1.0 - frac if args.invert else frac

    if sizes:  # full mode: node set = structure; a file absent from the weights is cold
        return {
            p: {
                "size": loc,
                "weight": round(norm(numeric[p]), 4) if p in numeric else 0.0,
            }
            for p, loc in sizes.items()
        }
    return {
        p: {"size": max(v, 1), "weight": round(norm(v), 4)} for p, v in numeric.items()
    }


def draw(
    leaves: dict[str, dict[str, Any]], categorical: bool, top: int, title: str, out: str
) -> int:
    """Render the top-N leaves (by area) as a squarify treemap. Returns the count
    drawn. Non-positive areas are dropped (squarify needs positive sizes)."""
    items = [(p, leaf) for p, leaf in leaves.items() if leaf["size"] > 0]
    items.sort(key=lambda kv: kv[1]["size"], reverse=True)
    items = items[:top]
    if not items:
        die("no files with positive size to draw", EXIT_EMPTY)

    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    import squarify
    from matplotlib.cm import ScalarMappable
    from matplotlib.colors import Normalize
    from matplotlib.patches import Rectangle

    sizes = [leaf["size"] for _, leaf in items]
    # Label only the largest rectangles to avoid clutter on the small ones.
    labels = [p.split("/")[-1] if i < 15 else "" for i, (p, _) in enumerate(items)]

    fig, ax = plt.subplots(figsize=(12, 7))
    if categorical:
        cats = sorted(
            {
                leaf["category"]
                for _, leaf in items
                if leaf["category"] != UNOWNED_CATEGORY
            }
        )
        cmap = plt.get_cmap("tab20")
        # matplotlib color specs are a str | RGBA-tuple union, so type the map as
        # Any at this boundary (the sentinel is a hex string, the rest are tuples).
        color_of: dict[str, Any] = {c: cmap(i % 20) for i, c in enumerate(cats)}
        color_of[UNOWNED_CATEGORY] = UNOWNED_COLOR
        colors = [color_of[leaf["category"]] for _, leaf in items]
        squarify.plot(
            sizes=sizes,
            label=labels,
            color=colors,
            ax=ax,
            pad=True,
            text_kwargs={"fontsize": 6},
            bar_kwargs={"edgecolor": "white", "linewidth": 0.5},
        )
        legend_cats = cats[:12] + (
            [UNOWNED_CATEGORY]
            if any(leaf["category"] == UNOWNED_CATEGORY for _, leaf in items)
            else []
        )
        handles = [Rectangle((0, 0), 1, 1, color=color_of[c]) for c in legend_cats]
        ax.legend(
            handles,
            legend_cats,
            loc="center left",
            bbox_to_anchor=(1.0, 0.5),
            fontsize=7,
            frameon=False,
        )
    else:
        cmap = plt.get_cmap("YlOrRd")
        colors = [cmap(float(leaf["weight"])) for _, leaf in items]
        squarify.plot(
            sizes=sizes,
            label=labels,
            color=colors,
            ax=ax,
            pad=True,
            text_kwargs={"fontsize": 6},
            bar_kwargs={"edgecolor": "white", "linewidth": 0.5},
        )
        sm = ScalarMappable(norm=Normalize(0.0, 1.0), cmap=cmap)
        fig.colorbar(sm, ax=ax, fraction=0.04, pad=0.02, label="change (hot = red)")

    ax.set_title(title)
    ax.axis("off")
    fig.tight_layout()
    fig.savefig(out)
    return len(items)


def main() -> None:
    ap = argparse.ArgumentParser(description="Static enclosure-family treemap.")
    ap.add_argument(
        "--weights", required=True, help="codelens analysis JSON, or '-' for stdin"
    )
    ap.add_argument(
        "--weight-col", default="n_revs", help="row column used as the weight"
    )
    ap.add_argument(
        "--structure", help="tokei --output json (size source); omit to degrade"
    )
    ap.add_argument(
        "--categorical",
        action="store_true",
        help="weight is a category (knowledge map)",
    )
    ap.add_argument(
        "--invert",
        action="store_true",
        help="invert numeric weight (low = hot, e.g. age)",
    )
    ap.add_argument(
        "--path-prefix", default="", help="strip this prefix from tokei/weight paths"
    )
    ap.add_argument(
        "--include",
        action="append",
        metavar="GLOB",
        help="keep only paths matching GLOB (gitignore-style, repeatable; exclude wins)",
    )
    ap.add_argument(
        "--exclude",
        action="append",
        metavar="GLOB",
        help="drop paths matching GLOB (gitignore-style, repeatable; applied after --include)",
    )
    ap.add_argument(
        "--top", type=int, default=50, help="draw the N largest files (default 50)"
    )
    ap.add_argument("--title", default="Hotspot treemap (area = size, colour = weight)")
    ap.add_argument(
        "-o", "--out", required=True, help="output SVG/PNG (extension picks format)"
    )
    ap.add_argument(
        "--json-out", help="also write the drawn leaves as JSON (for tests)"
    )
    args = ap.parse_args()

    leaves = build_leaves(args)

    # Area is tokei LOC only when --structure is given; warn when a single file
    # dominates that area (see references/operating.md, the reference-data recipe).
    if args.structure:
        warn_domination(leaves)

    drawn = draw(leaves, args.categorical, args.top, args.title, args.out)

    if args.json_out:
        top_items = sorted(
            ((p, leaf) for p, leaf in leaves.items() if leaf["size"] > 0),
            key=lambda kv: kv[1]["size"],
            reverse=True,
        )[: args.top]
        Path(args.json_out).write_text(
            json.dumps({p: leaf for p, leaf in top_items}, indent=2), encoding="utf-8"
        )

    print(f"wrote {args.out} ({drawn} files)", file=sys.stderr)


if __name__ == "__main__":
    main()
