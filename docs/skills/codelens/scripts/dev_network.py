# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Communication / developer network (interactive HTML) from codelens output.

Consumes `codelens communication` -> author, peer, shared, average, strength. A
Conway litmus test: nodes are authors (or teams, when the log was collected with
`--team-map`), links are shared work weighted by strength. Node size is the total
strength of a node's ties. Dense local clusters are healthy; a hairball of
inter-cluster links is a coordination bottleneck.

Usage:
  uv run scripts/dev_network.py --communication comm.json [--min-strength 25] -o network.html
Exit codes: 0 ok; 2 usage; 3 empty.
"""

from __future__ import annotations

import argparse
import json
import sys
from collections import defaultdict
from pathlib import Path
from typing import Any, NoReturn, cast

D3_CDN = '<script src="https://cdn.jsdelivr.net/npm/d3@7"></script>'


def die(msg: str, code: int) -> NoReturn:
    print(f"dev_network.py: {msg}", file=sys.stderr)
    raise SystemExit(code)


def rows(path: str) -> list[dict[str, Any]]:
    doc: Any = json.loads(Path(path).read_text(encoding="utf-8"))
    data = cast("dict[str, Any]", doc).get("rows") if isinstance(doc, dict) else doc
    if not isinstance(data, list):
        die(f"{path}: no rows array", 2)
    return cast("list[dict[str, Any]]", data)


def script_json(data: dict[str, Any]) -> str:
    # Escape <, >, & so the blob is safe inside an inline <script>: a "</script>"
    # in any string value would otherwise close the tag and break the page. These
    # chars only ever appear inside JSON string values, and \uXXXX is a valid JS
    # string escape. ensure_ascii already escapes U+2028/U+2029.
    blob = json.dumps(data, separators=(",", ":"))
    return blob.replace("<", "\\u003c").replace(">", "\\u003e").replace("&", "\\u0026")


def render(template: Path, data: dict[str, Any], title: str, subtitle: str) -> str:
    # Static values first, data last, so nothing rescans the substituted blob.
    return (
        template.read_text(encoding="utf-8")
        .replace("{{TITLE}}", title)
        .replace("{{SUBTITLE}}", subtitle)
        .replace("{{D3}}", D3_CDN)
        .replace("{{DATA}}", script_json(data))
    )


def main() -> None:
    ap = argparse.ArgumentParser(
        description="Developer/communication network (interactive HTML)."
    )
    ap.add_argument(
        "--communication", required=True, help="codelens communication JSON"
    )
    ap.add_argument(
        "--min-strength", type=float, default=0.0, help="drop links below this strength"
    )
    ap.add_argument("--template", default=None)
    ap.add_argument("-o", "--out", required=True)
    args = ap.parse_args()

    links: list[dict[str, Any]] = []
    strength_total: dict[str, float] = defaultdict(float)
    for row in rows(args.communication):
        s = float(row.get("strength", 0))
        if s < args.min_strength:
            continue
        a, b = row["author"], row["peer"]
        links.append({"source": a, "target": b, "value": s})
        strength_total[a] += s
        strength_total[b] += s
    if not links:
        die("no links above --min-strength", 3)

    nodes = [
        {"id": nid, "val": round(v, 1)} for nid, v in sorted(strength_total.items())
    ]

    tpl = (
        Path(args.template)
        if args.template
        else (
            Path(__file__).parent.parent
            / "assets"
            / "templates"
            / "force-network.html.jinja"
        )
    )
    if not tpl.is_file():
        die(f"template not found: {tpl}", 2)

    html = render(
        tpl,
        {"nodes": nodes, "links": links},
        "Communication network",
        "node = author/team, edge = shared work (strength); drag to explore",
    )
    Path(args.out).write_text(html, encoding="utf-8")
    print(f"wrote {args.out} ({len(nodes)} people, {len(links)} ties)", file=sys.stderr)


if __name__ == "__main__":
    main()
