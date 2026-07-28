# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Emit a Flint ChartAssemblyInput from a codelens result envelope.

Reads a codelens table envelope on stdin and writes a self-contained Flint
(flint-chart) ChartAssemblyInput on stdout: rows inlined into data.values,
per-column semantic_types derived from the envelope's semantics map, and a
chart_spec chosen by a generic semantics-driven policy plus per-analysis
overrides. Emits a spec only; rendering is downstream (flint_render.ts or the
flint-chart MCP server). See docs/specs/003-flint-adapter/plan.md.

  codelens coupling < git.log | uv run scripts/flint_spec.py -o coupling.flint.json

Optional: --schema FILE (codelens schema --command CMD output) populates
field_display_names from column descriptions; absent, raw column names are
used. --chart-type NAME overrides the default chart; --backend picks
vegalite (default) or echarts.

Exit codes: 0 ok; 2 usage/rejected envelope; 3 empty payload.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any, NoReturn

EXIT_USAGE = 2
EXIT_EMPTY = 3

# Aggregation roles for the closed semantic vocabulary
# (docs/specs/004-aggregation-roles): how a column may be combined. The
# channel policy is written over these roles, not over semantic names.
AGGREGATION_ROLES = {
    "count": "additive",
    "loc": "additive",
    "percentage": "intensive",
    "ratio": "intensive",
    "duration_months": "intensive",
    "filepath": "dimension",
    "person": "dimension",
    "date": "dimension",
    "label": "dimension",
    "flag": "dimension",
    "commit_id": "identifier",
    "text": "identifier",
}

# codelens semantic -> Flint semantic_types annotation (plan 3.2).
#
# percentage and ratio land on the same Flint type but are annotated
# ASYMMETRICALLY, and that is load-bearing, not an oversight: intrinsicDomain
# is a GATE, not an override. Flint checks only its PRESENCE before running
# fractional detection on the data, so annotating a percentage column whose
# surviving values are all <= 1 renders 1 (one percent) as "100%". Leaving
# percentage bare prevents that; ratio keeps the annotation because there the
# x100 transform is exactly what is wanted (0.34 -> 34%).
#
# ratio maps to Percentage rather than Number although Flint's own dropped-type
# table recommends Number for a ratio: Number is additive and summing ownership
# ratios is meaningless, whereas Percentage is intensive. The registry wins.
SEMANTIC_TYPES: dict[str, str | dict[str, Any]] = {
    "filepath": "Name",
    "person": "Name",
    "text": "Name",
    "label": "Category",
    "flag": "Boolean",
    "commit_id": "ID",
    "date": "Date",
    "count": "Count",
    "loc": {"semanticType": "Quantity", "unit": "lines"},
    "duration_months": {"semanticType": "Duration", "unit": "months"},
    "percentage": "Percentage",
    "ratio": {"semanticType": "Percentage", "intrinsicDomain": [0, 1]},
}

# Declared channel sets per backend and chartType, read from Flint's
# vlGetTemplateChannels / ecGetTemplateChannels at flint-chart@0.4.0. This is
# a COPY of Flint data that no unit test here can verify (F12): Flint silently
# discards an encoding channel a template does not declare, so every emitted
# channel is checked against this table and an unknown one is a hard error.
# flint_render.ts re-checks authoritatively at render time; on a version-pin
# upgrade, re-read the lists (upgrade checklist item 6).
CHANNELS: dict[str, dict[str, frozenset[str]]] = {
    "vegalite": {
        "Bar Chart": frozenset({"x", "y", "color", "opacity", "column", "row"}),
        "Stacked Bar Chart": frozenset({"x", "y", "color", "column", "row"}),
        "Histogram": frozenset({"x", "color", "column", "row"}),
        "Scatter Plot": frozenset(
            {"x", "y", "color", "size", "shape", "opacity", "column", "row"}
        ),
        "KPI Card": frozenset({"metric", "value", "goal"}),
        "Lollipop Chart": frozenset({"x", "y", "color", "column", "row"}),
        "Heatmap": frozenset({"x", "y", "color", "column", "row"}),
        "Line Chart": frozenset(
            {"x", "y", "color", "strokeDash", "detail", "opacity", "column", "row"}
        ),
    },
    "echarts": {
        "Network Graph": frozenset({"x", "y", "size"}),
        "Heatmap": frozenset({"x", "y", "color", "column", "row"}),
    },
}

DIMENSION = "dimension"
IDENTIFIER = "identifier"

Encodings = dict[str, str | list[str]]

# The analyses whose best chart is not what the generic policy produces
# (plan 3.4); everything else takes the generic default. Notes on the rows
# that look arbitrary without their compile-verified history (3.4.1):
# - summary: KPI Card's channels are metric/value/goal, not x/y; metric/value
#   yields a vconcat of one card per row, x/y silently yielded a layer spec.
# - authors: entity is deliberately UNBOUND. detail is not a Scatter Plot
#   channel (it was silently dropped), addTooltips does not restore it, and
#   color/shape/size would each ask for hundreds of values. The finding is
#   the shape of the cloud, not which point is which file.
# - absolute-churn: the y array is Flint's documented reshape; it folds to a
#   series key/value pair, auto-binds color, and keeps added-vs-deleted, the
#   signal the churn analysis exists to show.
OVERRIDES: dict[str, tuple[str, str, Encodings]] = {
    "code-age": ("vegalite", "Histogram", {"x": "age_months"}),
    "summary": (
        "vegalite",
        "KPI Card",
        {"metric": "statistic", "value": "value"},
    ),
    "authors": ("vegalite", "Scatter Plot", {"x": "n_revs", "y": "n_authors"}),
    "coupling": (
        "echarts",
        "Network Graph",
        {"x": "entity", "y": "coupled", "size": "degree"},
    ),
    "communication": (
        "echarts",
        "Network Graph",
        {"x": "author", "y": "peer", "size": "strength"},
    ),
    "absolute-churn": (
        "vegalite",
        "Stacked Bar Chart",
        {"x": "date", "y": ["added", "deleted"]},
    ),
}


def semantic_of(semantics: dict[str, str], column: str) -> str:
    sem = semantics[column]
    if sem not in AGGREGATION_ROLES:
        die(
            f"unknown semantic {sem!r} for column {column!r}; "
            "the vocabulary is closed, update the maps in this script",
            EXIT_USAGE,
        )
    return sem


def measure_rank(semantic: str) -> int:
    """Primary-measure precedence (plan 3.3): an intensive proportion is the
    analysis's headline result, then loc, then duration_months, then count.
    A count is almost always a supporting quantity (total_revs, commits,
    average_revs). Ties keep envelope column order, which also keeps a
    component ahead of its own total_* column."""
    if AGGREGATION_ROLES[semantic] == "intensive":
        # duration_months is intensive but a magnitude, not a proportion;
        # it ranks below loc.
        return 0 if semantic != "duration_months" else 2
    return 1 if semantic == "loc" else 3


def partition(
    semantics: dict[str, str],
) -> tuple[list[str], list[str]]:
    """Split the envelope's columns into dimensions and measures, dropping
    identifiers (F9: text and commit_id are never bound to a channel).
    Measures come back strongest-first; dimensions keep envelope order except
    that a date dimension moves first, because it always takes the x axis."""
    dims: list[str] = []
    measures: list[str] = []
    for column in semantics:
        role = AGGREGATION_ROLES[semantic_of(semantics, column)]
        if role == IDENTIFIER:
            continue
        (dims if role == DIMENSION else measures).append(column)
    dims.sort(key=lambda c: semantics[c] != "date")
    measures.sort(key=lambda c: measure_rank(semantics[c]))
    return dims, measures


def is_edge_table(semantics: dict[str, str], dims: list[str]) -> bool:
    return len(dims) >= 2 and semantics[dims[0]] == semantics[dims[1]]


def bind(
    channels: frozenset[str],
    semantics: dict[str, str],
    dims: list[str],
    measures: list[str],
) -> Encodings:
    """Generic channel policy (plan 3.3), adapted to the chosen template's
    declared channels. Binds ONE measure plus dimensions, so a component and
    its own total_* column are never stacked together (the total-mixed-with-
    components hazard)."""
    enc: Encodings = {}
    if "metric" in channels and "value" in channels:
        if not dims or not measures:
            die("KPI Card needs one dimension and one measure", EXIT_USAGE)
        return {"metric": dims[0], "value": measures[0]}
    if is_edge_table(semantics, dims):
        enc["x"], enc["y"] = dims[0], dims[1]
        if measures:
            enc["size" if "size" in channels else "color"] = measures[0]
        return enc
    if "y" not in channels:
        # A distribution template (Histogram): one measure, no value axis.
        if not measures:
            die("a distribution chart needs a measure column", EXIT_USAGE)
        return {"x": measures[0]}
    if dims:
        enc["x"] = dims[0]
        if measures:
            enc["y"] = measures[0]
        if len(dims) > 1 and "color" in channels:
            enc["color"] = dims[1]
    elif len(measures) >= 2:
        enc["x"], enc["y"] = measures[0], measures[1]
    elif measures:
        enc["x"] = measures[0]
    else:
        die("no plottable columns survive the identifier exclusion", EXIT_USAGE)
    # loc prefers size over the value axis when the template offers one; here
    # that means an unbound loc measure fills a free size channel.
    if "size" in channels and "size" not in enc:
        bound = set(enc.values())
        loc_left = [m for m in measures if m not in bound and semantics[m] == "loc"]
        if loc_left:
            enc["size"] = loc_left[0]
    return enc


def die(msg: str, code: int) -> NoReturn:
    print(f"flint_spec.py: {msg}", file=sys.stderr)
    raise SystemExit(code)


def load_envelope() -> dict[str, Any]:
    """Read and gate the envelope per plan 3.1: accept shape "table" with a
    non-empty payload; reject text output, error envelopes, schema output,
    and the parse event dump."""
    try:
        doc: Any = json.loads(sys.stdin.read())
    except json.JSONDecodeError as e:
        die(f"invalid JSON on stdin: {e}", EXIT_USAGE)
    if not isinstance(doc, dict):
        die("input is not a codelens envelope (expected a JSON object)", EXIT_USAGE)
    if doc.get("ok") is False:
        die("input is an error envelope, not a result", EXIT_USAGE)
    if "command" in doc or "commands" in doc:
        die("input is codelens schema output, not a result envelope", EXIT_USAGE)
    if doc.get("shape") != "table":
        die(f'unsupported shape {doc.get("shape")!r}; only "table"', EXIT_USAGE)
    if doc.get("analysis") == "parse":
        die("parse is an event dump, not an analysis; not charted", EXIT_USAGE)
    semantics = doc.get("semantics")
    if not isinstance(semantics, dict) or not semantics:
        die("envelope carries no semantics map", EXIT_USAGE)
    rows = doc.get("rows")
    if not isinstance(rows, list) or not rows:
        die("empty payload: no rows to chart", EXIT_EMPTY)
    return doc


def encoded_fields(enc: Encodings) -> set[str]:
    fields: set[str] = set()
    for value in enc.values():
        fields.update(value if isinstance(value, list) else (value,))
    return fields


def default_chart(
    semantics: dict[str, str],
    dims: list[str],
    measures: list[str],
    backend_arg: str | None,
) -> tuple[str, str]:
    """Default (backend, chartType) from the generic policy. Vega-Lite is the
    default backend (F7); echarts is selected only by the template that needs
    it (Network Graph for an edge table)."""
    if is_edge_table(semantics, dims):
        # Heatmap is the documented cross-backend alternative to the
        # ECharts-only Network Graph.
        return (
            ("vegalite", "Heatmap")
            if backend_arg == "vegalite"
            else (
                "echarts",
                "Network Graph",
            )
        )
    if backend_arg == "echarts":
        die(
            "no default echarts chart for this table; pass --chart-type "
            f"(valid for echarts: {', '.join(sorted(CHANNELS['echarts']))})",
            EXIT_USAGE,
        )
    if not dims and len(measures) >= 2:
        return "vegalite", "Scatter Plot"
    if not dims and len(measures) == 1:
        return "vegalite", "Histogram"
    return "vegalite", "Bar Chart"


def resolve_backend(chart_type: str, backend_arg: str | None) -> str:
    """Backend for an explicit --chart-type: the caller's, if it declares the
    template, else the first backend that does, Vega-Lite preferred (F7)."""
    candidates = [backend_arg] if backend_arg else ["vegalite", "echarts"]
    for backend in candidates:
        if chart_type in CHANNELS[backend]:
            return backend
    valid = "; ".join(
        f"{backend}: {', '.join(sorted(CHANNELS[backend]))}"
        for backend in (candidates if backend_arg else CHANNELS)
    )
    die(f"unknown chart type {chart_type!r} (valid {valid})", EXIT_USAGE)


def validate_channels(backend: str, chart_type: str, enc: Encodings) -> None:
    """F12 layer 1: Flint silently discards an undeclared channel, so an
    encoding key outside the template's declared set is a hard error here."""
    declared = CHANNELS[backend][chart_type]
    unknown = sorted(set(enc) - declared)
    if unknown:
        die(
            f"channel(s) {', '.join(unknown)} not declared by {backend} "
            f"{chart_type!r} (declared: {', '.join(sorted(declared))})",
            EXIT_USAGE,
        )


def display_names(schema_path: str, columns: set[str]) -> dict[str, str]:
    """Column -> desc from `codelens schema --command CMD` output, narrowed to
    the envelope's columns. Only reached when --schema was supplied; a bad
    file is a usage error, absence of the flag degrades to raw names."""
    try:
        schema: Any = json.loads(Path(schema_path).read_text(encoding="utf-8"))
    except OSError as e:
        die(f"cannot read --schema file: {e}", EXIT_USAGE)
    except json.JSONDecodeError as e:
        die(f"invalid JSON in --schema file: {e}", EXIT_USAGE)
    row_schema = schema.get("row_schema") if isinstance(schema, dict) else None
    if not isinstance(row_schema, list):
        die(
            "--schema file carries no row_schema; not codelens schema output",
            EXIT_USAGE,
        )
    return {
        col["name"]: col["desc"]
        for col in row_schema
        if isinstance(col, dict) and col.get("name") in columns and col.get("desc")
    }


def main() -> None:
    ap = argparse.ArgumentParser(
        description="Emit a Flint ChartAssemblyInput from a codelens envelope."
    )
    ap.add_argument(
        "--chart-type",
        help="chart template name, overriding the per-analysis default",
    )
    ap.add_argument(
        "--backend",
        choices=("vegalite", "echarts"),
        help="target backend (default: vegalite; echarts only when a template needs it)",
    )
    ap.add_argument(
        "--schema",
        help="codelens schema --command CMD output; populates field display "
        "names from column descriptions (optional)",
    )
    ap.add_argument("--width", type=int, help="target chart width in px")
    ap.add_argument("--height", type=int, help="target chart height in px")
    ap.add_argument("-o", "--out", help="write the spec here instead of stdout")
    args = ap.parse_args()

    for flag, value in (("--width", args.width), ("--height", args.height)):
        if value is not None and value <= 0:
            die(f"{flag} must be a positive pixel count", EXIT_USAGE)

    doc = load_envelope()
    semantics: dict[str, str] = doc["semantics"]
    rows: list[dict[str, Any]] = doc["rows"]
    analysis = str(doc.get("analysis", ""))
    dims, measures = partition(semantics)

    override = OVERRIDES.get(analysis)
    if override is not None and not (encoded_fields(override[2]) <= semantics.keys()):
        # A --fields projection dropped one of the override's columns; the
        # generic policy still produces a defensible default.
        override = None

    if args.chart_type:
        backend = resolve_backend(args.chart_type, args.backend)
        chart_type = args.chart_type
        if override is not None and override[:2] == (backend, chart_type):
            enc = override[2]
        else:
            enc = bind(CHANNELS[backend][chart_type], semantics, dims, measures)
    elif override is not None and args.backend in (None, override[0]):
        backend, chart_type, enc = override
    else:
        backend, chart_type = default_chart(semantics, dims, measures, args.backend)
        enc = bind(CHANNELS[backend][chart_type], semantics, dims, measures)
    validate_channels(backend, chart_type, enc)

    # A stacked bar split by a dimension is the part-of-whole fragmentation
    # view (ownership, effort); normalize makes the shares comparable.
    properties = (
        {"stackMode": "normalize"}
        if chart_type == "Stacked Bar Chart" and "color" in enc
        else None
    )

    spec: dict[str, Any] = {
        "data": {"values": rows},
        "semantic_types": {
            column: SEMANTIC_TYPES[semantic_of(semantics, column)]
            for column in semantics
        },
        "chart_spec": {
            "chartType": chart_type,
            # A multi-field bind (the x/y array reshape) must be the BARE
            # array: {"field": [...]} throws at assembly in flint-chart@0.4.0.
            "encodings": {
                ch: field if isinstance(field, list) else {"field": field}
                for ch, field in enc.items()
            },
        },
    }
    if properties:
        spec["chart_spec"]["chartProperties"] = properties
    if args.width or args.height:
        # Flint's documented defaults fill in the unspecified dimension.
        spec["chart_spec"]["baseSize"] = {
            "width": args.width or 400,
            "height": args.height or 320,
        }
    if args.schema:
        names = display_names(args.schema, set(semantics))
        if names:
            spec["field_display_names"] = names

    text = json.dumps(spec, indent=2)
    if args.out:
        Path(args.out).write_text(text + "\n", encoding="utf-8")
    else:
        print(text)
    print(
        f"emitted {chart_type} ({backend}) spec for {analysis or 'result'}: "
        f"{len(rows)} rows",
        file=sys.stderr,
    )


if __name__ == "__main__":
    main()
