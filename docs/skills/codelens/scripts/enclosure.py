# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Build a zoomable circle-packing (enclosure) map from codelens output.

Generalized over the enclosure family: the weight column selects the map.
  hotspot map    --weights revisions.json  --weight-col n_revs
  knowledge map  --weights main-dev.json    --weight-col main_dev --categorical
  code-age map   --weights code-age.json    --weight-col age_months --invert

Structure (node set + circle radius) comes from `tokei --output json` when given;
otherwise it degrades to the weight source, sizing circles by the weight value.

Inputs are JSON files (or '-' for stdin on --weights). Outputs an HTML file and,
with --json-out, the intermediate hierarchy. Stdlib only.

Exit codes: 0 ok; 2 usage/bad input; 3 empty result.

See references/enclosure.md for the full data contract.
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

# Sentinel category for a file present in the tokei structure but absent from the
# categorical weights (e.g. a file with no recorded author in the window). The
# template renders it in neutral grey, distinct from every real category color.
UNOWNED_CATEGORY = "(unowned)"


def die(msg: str, code: int) -> NoReturn:
    print(f"enclosure.py: {msg}", file=sys.stderr)
    raise SystemExit(code)


def glob_to_regex(pattern: str) -> str:
    """Translate a gitignore-style glob to an anchored regex.

    Semantics match the codelens Go side (github.com/bmatcuk/doublestar), matched
    against the full entity path, so one shared glob set behaves identically in
    both surfaces:
      **  matches any run of characters, including path separators;
      *   matches any run of non-separator characters;
      ?   matches a single non-separator character;
      [.] a character class (leading ! or ^ negates).
    A malformed class raises ValueError so the caller can report a usage error.
    """
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
    """Build an exclude-after-include predicate over normalized paths: if any
    include is given a path must match one, then any exclude match drops it."""
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


def build_tree(
    root_name: str,
    leaves: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    """Nest flat `path -> {size, weight, ...}` leaves into a D3 hierarchy."""
    root: dict[str, Any] = {"name": root_name, "children": []}
    index: dict[str, dict[str, Any]] = {"": root}
    for path, leaf in sorted(leaves.items()):
        segments = path.split("/")
        parent = root
        prefix = ""
        for seg in segments[:-1]:
            prefix = f"{prefix}/{seg}" if prefix else seg
            node: dict[str, Any] | None = index.get(prefix)
            if node is None:
                node = {"name": seg, "children": []}
                parent["children"].append(node)
                index[prefix] = node
            parent = node
        parent["children"].append({"name": segments[-1], **leaf})
    return root


D3_CDN = '<script src="https://cdn.jsdelivr.net/npm/d3@7"></script>'


def render_html(template: str, data: dict[str, Any], vendor_d3: Path | None) -> str:
    # D3 loads from a CDN by default; --vendor-d3 inlines a local bundle if given.
    if vendor_d3 and vendor_d3.is_file():
        d3_tag = f"<script>{vendor_d3.read_text(encoding='utf-8')}</script>"
    else:
        d3_tag = D3_CDN
    # Escape <, >, & so the blob is safe inside an inline <script>: a "</script>"
    # in any string value (node ids are file paths) would otherwise close the tag
    # and break the page. These chars only ever appear inside JSON string values,
    # and \uXXXX is a valid JS string escape. ensure_ascii escapes U+2028/U+2029.
    blob = json.dumps(data, separators=(",", ":"))
    blob = blob.replace("<", "\\u003c").replace(">", "\\u003e").replace("&", "\\u0026")
    return template.replace("{{D3}}", d3_tag).replace("{{DATA}}", blob)


def main() -> None:
    ap = argparse.ArgumentParser(description="Build an enclosure (circle-packing) map.")
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
    ap.add_argument("--root-name", default="root")
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
        "--template",
        default=None,
        help="HTML template (default: assets/templates/circle-packing.html.jinja)",
    )
    ap.add_argument(
        "--vendor-d3", default=None, help="path to a vendored d3 bundle to inline"
    )
    ap.add_argument("-o", "--out", required=True, help="output HTML file")
    ap.add_argument("--json-out", help="also write the intermediate hierarchy JSON")
    args = ap.parse_args()

    weights = read_weights(load_json(args.weights), args.weight_col)
    if not weights:
        die(f"no rows carried column {args.weight_col!r}", EXIT_EMPTY)

    def strip_prefix(m: dict[str, Any]) -> dict[str, Any]:
        if not args.path_prefix:
            return m
        pre = norm_path(args.path_prefix).rstrip("/") + "/"
        return {(k[len(pre) :] if k.startswith(pre) else k): v for k, v in m.items()}

    weights = strip_prefix(weights)

    # Filter both the weights and (below) the tokei structure with one shared
    # glob set, before build_tree, so an excluded generated file is drawn on no
    # map: codelens cannot see the external structure, so the filter must be
    # reapplied here to the same node set.
    keep = make_path_filter(args.include or [], args.exclude or [])
    weights = {p: v for p, v in weights.items() if keep(p)}

    # When --structure is given, the tokei node set is authoritative for every
    # mode, so all three maps (hotspot, code-age, knowledge) draw the same files.
    # Without it, every mode degrades to the weights as the node set.
    sizes: dict[str, int] = {}
    if args.structure:
        sizes = strip_prefix(read_structure(load_json(args.structure)))
        if not sizes:
            die("tokei structure has no files", EXIT_EMPTY)
        sizes = {p: v for p, v in sizes.items() if keep(p)}

    # The effective node set is the structure (full mode) or the weights
    # (degraded mode); filtering can empty it, which is an empty result (exit 3).
    if not (sizes if args.structure else weights):
        die("no files remain after --include/--exclude filtering", EXIT_EMPTY)

    if args.categorical:
        if sizes:  # full mode: node set = structure, radius = LOC, category joined
            leaves = {
                p: {"size": loc, "category": weights.get(p, UNOWNED_CATEGORY)}
                for p, loc in sizes.items()
            }
        else:  # degraded mode: node set = weights, uniform size
            leaves = {p: {"size": 1, "category": v} for p, v in weights.items()}
    else:
        numeric = {p: float(v) for p, v in weights.items()}
        hi = max(numeric.values())
        lo = min(numeric.values())
        span = (hi - lo) or 1.0

        def norm(v: float) -> float:
            frac = (v - lo) / span
            return 1.0 - frac if args.invert else frac

        if sizes:  # full mode: node set = structure
            # A file absent from the weights renders at the neutral (cold) end,
            # not lo-as-if-present, so --invert maps never draw unchanged files hot.
            leaves = {
                p: {
                    "size": loc,
                    "weight": round(norm(numeric[p]), 4) if p in numeric else 0.0,
                }
                for p, loc in sizes.items()
            }
        else:  # degraded mode: node set = weights
            leaves = {
                p: {"size": max(v, 1), "weight": round(norm(v), 4)}
                for p, v in numeric.items()
            }

    tree = build_tree(args.root_name, leaves)

    if args.json_out:
        Path(args.json_out).write_text(json.dumps(tree, indent=2), encoding="utf-8")

    tpl_path = (
        Path(args.template)
        if args.template
        else (
            Path(__file__).parent.parent
            / "assets"
            / "templates"
            / "circle-packing.html.jinja"
        )
    )
    if not tpl_path.is_file():
        die(f"template not found: {tpl_path}", EXIT_USAGE)

    vendor = Path(args.vendor_d3) if args.vendor_d3 else None
    html = render_html(tpl_path.read_text(encoding="utf-8"), tree, vendor)
    Path(args.out).write_text(html, encoding="utf-8")
    print(f"wrote {args.out} ({len(leaves)} files)", file=sys.stderr)


if __name__ == "__main__":
    main()
