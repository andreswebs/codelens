# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Assemble a sequenced findings report from codelens output (plain markdown).

The report is one self-contained markdown document: the crime-scene investigation
walked in the book's order, figures embedded inline as SVG, and the agent's reading
of each analysis slotted into a fixed sequence. The script pins the structure,
order, embedding, and the misuse guardrails; the agent supplies the prose via a
findings file, so the reading stays a matter of judgment.

There is no MARP and no slide deck: this is a report meant to be read (rendered via
pandoc or any HTML markdown pipeline). Inline `<svg>` renders there; note that
GitHub's markdown sanitizer strips inline SVG, so the report targets HTML/pandoc,
not GitHub preview.

Inputs (all optional except -o; at least one source must be present):
  --findings FILE     agent-authored findings markdown (see references/reporting.md)
  --figures-dir DIR   directory of degraded SVGs (conventional names, see below)
  --summary FILE      codelens summary JSON, for the situational-awareness tiles
Output: -o report.md

Figure filename convention in --figures-dir (a missing file just omits its picture):
  summary.svg hotspots.svg complexity.svg coupling.svg knowledge.svg fractal.svg
  network.svg age.svg churn.svg cloud.svg

Exit codes: 0 ok; 2 usage/bad input; 3 nothing to assemble.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any, NoReturn, cast

EXIT_USAGE = 2
EXIT_EMPTY = 3

# Guardrails the report must always carry for the social analyses (they describe
# code and coordination risk, never individual performance).
SOCIAL_DISCLAIMER = (
    "> These views describe code and coordination risk, not individual "
    "performance. They are not a productivity ranking."
)
TEAM_NOTE = (
    "_Aggregate authors to teams (`--team-map`) before reading this as a Conway "
    "signal; an inter-team tie is a potential coordination bottleneck, not proof._"
)
HEURISTIC_NOTE = "_Heuristic only - a conversation starter, not a hard finding._"

PLACEHOLDER = "_No finding provided for this section._"

# The fixed investigative sequence (book funnel, business-first). Each section maps
# a findings key to a report section, an optional figure stem, and flags for the
# guardrails the script injects unconditionally.
SECTIONS: list[dict[str, Any]] = [
    {"key": "executive_summary", "title": "Executive summary", "fig": "summary"},
    {"key": "hotspots", "title": "Hotspots - where the risk is", "fig": "hotspots"},
    {
        "key": "complexity_trend",
        "title": "Complexity trend - is it getting worse?",
        "fig": "complexity",
    },
    {
        "key": "coupling",
        "title": "Change coupling - why changes ripple",
        "fig": "coupling",
    },
    {
        "key": "knowledge",
        "title": "Knowledge & ownership",
        "fig": "knowledge",
        "social": True,
    },
    {"key": "fractal", "title": "Fragmentation", "fig": "fractal", "social": True},
    {
        "key": "communication",
        "title": "Communication & Conway alignment",
        "fig": "network",
        "social": True,
        "team_note": True,
    },
    {"key": "code_age", "title": "Code age - stabilization", "fig": "age"},
    {"key": "churn", "title": "Churn - the macro trend", "fig": "churn"},
    {
        "key": "word_cloud",
        "title": "Commit vocabulary",
        "fig": "cloud",
        "heuristic": True,
    },
    {"key": "risk_choices", "title": "Recommended actions", "fig": None},
]


def die(msg: str, code: int) -> NoReturn:
    print(f"report.py: {msg}", file=sys.stderr)
    raise SystemExit(code)


def parse_findings(text: str) -> dict[str, str]:
    """Parse the findings markdown into `key -> body`. The `# H1` line is the sole
    report title (key 'title'); a `## title` block is ignored so the title can never
    be overridden or blanked. Each other `## key` line starts a section whose body is
    the text up to the next `## ` or EOF. Keys are normalized (lowercased,
    spaces/hyphens -> underscores) to the reserved vocabulary in references/reporting.md."""
    out: dict[str, list[str]] = {}
    current: str | None = None
    for line in text.splitlines():
        if line.startswith("## "):
            key = line[3:].strip().lower().replace("-", "_").replace(" ", "_")
            if key == "title":
                # The title comes only from the `# H1` line; drop a `## title` block.
                current = None
                continue
            current = key
            out.setdefault(current, [])
        elif line.startswith("# ") and current is None:
            out.setdefault("title", []).append(line[2:].strip())
        elif current is not None:
            out[current].append(line)
    return {k: "\n".join(v).strip() for k, v in out.items()}


def read_summary(path: str) -> list[tuple[str, Any]]:
    """Return (label, value) tiles from a codelens summary envelope (or bare rows).
    'number-of-commits' -> 'commits' so the tiles read as plain nouns."""
    doc: Any = json.loads(Path(path).read_text(encoding="utf-8"))
    rows = cast("dict[str, Any]", doc).get("rows") if isinstance(doc, dict) else doc
    if not isinstance(rows, list):
        die("summary JSON has no 'rows' array", EXIT_USAGE)
    tiles: list[tuple[str, Any]] = []
    for r in cast("list[dict[str, Any]]", rows):
        stat = str(r.get("statistic", "")).removeprefix("number-of-").replace("-", " ")
        tiles.append((stat, r.get("value")))
    return tiles


def inline_svg(figures_dir: Path | None, stem: str) -> str | None:
    """Return the file's `<svg>...</svg>` (XML prolog/doctype stripped) so it embeds
    self-contained, or None when the figure is absent."""
    if figures_dir is None:
        return None
    path = figures_dir / f"{stem}.svg"
    if not path.is_file():
        return None
    text = path.read_text(encoding="utf-8")
    start = text.find("<svg")
    if start < 0:
        return None
    return text[start:].rstrip()


def render_tiles(tiles: list[tuple[str, Any]]) -> str:
    def fmt(v: Any) -> str:
        return f"{v:,}" if isinstance(v, int) else str(v)

    lines = ["| metric | value |", "| --- | --- |"]
    lines += [f"| {label} | {fmt(value)} |" for label, value in tiles]
    return "\n".join(lines)


def assemble(
    findings: dict[str, str], figures_dir: Path | None, tiles: list[tuple[str, Any]]
) -> str:
    title = findings.get("title") or "Codebase evolutionary analysis"
    parts: list[str] = [f"# {title}", ""]
    window = findings.get("window")
    if window:
        parts += [f"_Analysis window: {window}_", ""]

    for i, sec in enumerate(SECTIONS, start=1):
        parts.append(f"## {i}. {sec['title']}")
        parts.append("")
        if sec.get("social"):
            parts += [SOCIAL_DISCLAIMER, ""]

        # Situational-awareness tiles live in the executive summary section.
        if sec["key"] == "executive_summary" and tiles:
            parts += [render_tiles(tiles), ""]

        prose = findings.get(sec["key"], "").strip()
        parts += [prose or PLACEHOLDER, ""]

        if sec.get("team_note"):
            parts += [TEAM_NOTE, ""]
        if sec.get("heuristic"):
            parts += [HEURISTIC_NOTE, ""]

        fig = sec.get("fig")
        svg = inline_svg(figures_dir, fig) if fig else None
        if svg:
            parts += [svg, "", f"*Figure {i}. {sec['title']}.*", ""]

    return "\n".join(parts).rstrip() + "\n"


def main() -> None:
    ap = argparse.ArgumentParser(
        description="Assemble a sequenced findings report (plain markdown)."
    )
    ap.add_argument("--findings", help="agent-authored findings markdown")
    ap.add_argument("--figures-dir", help="directory of degraded SVG figures")
    ap.add_argument(
        "--summary", help="codelens summary JSON (situational-awareness tiles)"
    )
    ap.add_argument("-o", "--out", required=True, help="output report.md")
    args = ap.parse_args()

    if not (args.findings or args.figures_dir or args.summary):
        die(
            "nothing to assemble: pass --findings, --figures-dir, or --summary",
            EXIT_EMPTY,
        )

    findings: dict[str, str] = {}
    if args.findings:
        findings = parse_findings(Path(args.findings).read_text(encoding="utf-8"))

    figures_dir: Path | None = None
    if args.figures_dir:
        figures_dir = Path(args.figures_dir)
        if not figures_dir.is_dir():
            die(f"--figures-dir not a directory: {figures_dir}", EXIT_USAGE)

    tiles = read_summary(args.summary) if args.summary else []

    report = assemble(findings, figures_dir, tiles)
    Path(args.out).write_text(report, encoding="utf-8")
    n_figs = sum(
        1 for s in SECTIONS if s.get("fig") and inline_svg(figures_dir, s["fig"])
    )
    print(
        f"wrote {args.out} ({len(SECTIONS)} sections, {n_figs} figures)",
        file=sys.stderr,
    )


if __name__ == "__main__":
    main()
