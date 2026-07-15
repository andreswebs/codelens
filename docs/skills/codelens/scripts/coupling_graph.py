# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Change-coupling graph (interactive HTML) from codelens output.

Consumes `codelens coupling` -> entity, coupled, degree, average_revs. Optionally
`sum-of-coupling` (entity, soc) for node size. Nodes are files; edges are files
that change together, weighted by coupling degree. Node color is the top-level
directory, so an edge between two colors flags cross-boundary coupling (decay).

Usage:
  uv run scripts/coupling_graph.py --coupling coupling.json [--soc soc.json] \
      [--min-degree 30] -o coupling.html
Exit codes: 0 ok; 2 usage; 3 empty.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any, NoReturn, cast

D3_CDN = '<script src="https://cdn.jsdelivr.net/npm/d3@7"></script>'


def die(msg: str, code: int) -> NoReturn:
    print(f"coupling_graph.py: {msg}", file=sys.stderr)
    raise SystemExit(code)


def rows(path: str) -> list[dict[str, Any]]:
    doc: Any = json.loads(Path(path).read_text(encoding="utf-8"))
    data = cast("dict[str, Any]", doc).get("rows") if isinstance(doc, dict) else doc
    if not isinstance(data, list):
        die(f"{path}: no rows array", 2)
    return cast("list[dict[str, Any]]", data)


def component(path: str) -> str:
    return path.split("/")[0] if "/" in path else path


def script_json(data: dict[str, Any]) -> str:
    # Escape <, >, & so the blob is safe inside an inline <script>: a "</script>"
    # in any string value (node ids are file paths) would otherwise close the tag
    # and break the page. These chars only ever appear inside JSON string values,
    # and \uXXXX is a valid JS string escape. ensure_ascii escapes U+2028/U+2029.
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
    ap = argparse.ArgumentParser(description="Change-coupling graph (interactive HTML).")
    ap.add_argument("--coupling", required=True, help="codelens coupling JSON")
    ap.add_argument("--soc", help="codelens sum-of-coupling JSON (node size)")
    ap.add_argument("--min-degree", type=float, default=0.0, help="drop edges below this coupling degree")
    ap.add_argument("--template", default=None)
    ap.add_argument("-o", "--out", required=True)
    args = ap.parse_args()

    links: list[dict[str, Any]] = []
    node_ids: set[str] = set()
    for row in rows(args.coupling):
        if float(row.get("degree", 0)) < args.min_degree:
            continue
        a, b = row["entity"], row["coupled"]
        links.append({"source": a, "target": b, "value": row.get("degree", 1)})
        node_ids.update((a, b))
    if not links:
        die("no edges above --min-degree", 3)

    soc = {r["entity"]: r["soc"] for r in rows(args.soc)} if args.soc else {}
    nodes = [
        {"id": nid, "label": nid.split("/")[-1], "group": component(nid), "val": soc.get(nid, 1)}
        for nid in sorted(node_ids)
    ]

    tpl = Path(args.template) if args.template else (
        Path(__file__).parent.parent / "assets" / "templates" / "force-network.html.jinja"
    )
    if not tpl.is_file():
        die(f"template not found: {tpl}", 2)

    html = render(tpl, {"nodes": nodes, "links": links},
                  "Change coupling", "node = file (color = folder), edge = co-change; drag to explore")
    Path(args.out).write_text(html, encoding="utf-8")
    print(f"wrote {args.out} ({len(nodes)} files, {len(links)} couplings)", file=sys.stderr)


if __name__ == "__main__":
    main()
