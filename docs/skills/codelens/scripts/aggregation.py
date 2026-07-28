# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Aggregation guard shared by the codelens skill's scripts.

codelens classifies every semantic in its vocabulary with an aggregation role
(`additive`, `intensive`, `dimension`, `identifier`) and publishes the map as
`aggregation_roles` in `codelens schema` output. This module is the consumer:
it turns that classification into a check at the sites that actually combine
rows.

The API carries the CALLER'S INTENT rather than adding a fifth role, because
ordinal summation is legal for every numeric measure, intensive included:

  combine_for_value(rows, col, roles)        -> total; refuses an intensive column
  combine_for_rank(rows, col, roles, keys)   -> per-key total; permits any measure,
                                                and the result is ordinal ONLY

So summing `degree` (a percentage) to size a node or to pick a top-N is legal
and says so at the call site, while summing it into a reported total is refused.
A `dimension` or `identifier` column is refused by both: it is not a measure.

Usage (from a script in this directory; `roles` is empty when --schema is absent):

    import aggregation

    roles = aggregation.roles_from_schema(args.schema) if args.schema else {}
    total = aggregation.combine_for_value(rows, "added", roles)

ENFORCEMENT IS AVAILABLE, NOT GUARANTEED. The roles come from an OPTIONAL
`--schema FILE`; absent it there is nothing to check against and both helpers
pass values through with no error and no warning. An invocation without
`--schema` is therefore unchecked, and no correctness claim about it follows
from this module.

Failures raise AggregationError; callers map it to their own usage exit code
(2 across these scripts) rather than this module owning an exit convention.
"""

from __future__ import annotations

import json
from collections import defaultdict
from collections.abc import Callable, Iterable
from pathlib import Path
from typing import Any, cast

ADDITIVE = "additive"
INTENSIVE = "intensive"

# Column -> aggregation role, as roles_from_schema builds it. Keyed by column
# rather than by semantic so a caller never repeats the join.
Roles = dict[str, str]

Row = dict[str, Any]
KeyFn = Callable[[Row], Iterable[str]]


class AggregationError(ValueError):
    """An illegal combination, or unusable role input."""


def roles_from_schema(path: str) -> Roles:
    """Column -> aggregation role from `codelens schema --command CMD` output.

    Joins `row_schema[].semantic` against the document's `aggregation_roles`
    map. Every declared column is covered, flag-gated ones included, because
    `schema` describes the command rather than one run.

    A schema document carrying no `aggregation_roles` (an older codelens)
    yields no roles, which the helpers treat as the unchecked pass-through
    case. A document with no `row_schema` at all is not schema output and is an
    error.
    """
    try:
        doc: Any = json.loads(Path(path).read_text(encoding="utf-8"))
    except OSError as e:
        raise AggregationError(f"cannot read schema file {path}: {e}") from e
    except json.JSONDecodeError as e:
        raise AggregationError(f"invalid JSON in schema file {path}: {e}") from e

    row_schema = (
        cast("dict[str, Any]", doc).get("row_schema") if isinstance(doc, dict) else None
    )
    if not isinstance(row_schema, list):
        raise AggregationError(
            f"schema file {path} carries no row_schema; not codelens schema output"
        )
    by_semantic = cast("dict[str, Any]", doc).get("aggregation_roles")
    if not isinstance(by_semantic, dict):
        return {}

    semantic_role = cast("dict[str, Any]", by_semantic)
    roles: Roles = {}
    for column in cast("list[Any]", row_schema):
        if not isinstance(column, dict):
            continue
        name = cast("dict[str, Any]", column).get("name")
        semantic = cast("dict[str, Any]", column).get("semantic")
        role = semantic_role.get(semantic)
        # An unknown semantic leaves the column unclassified rather than
        # guessing: an unclassified column passes through, a wrong role would
        # refuse a legal call.
        if isinstance(name, str) and isinstance(role, str):
            roles[name] = role
    return roles


def combine_for_value(rows: list[Row], col: str, roles: Roles) -> float:
    """Total of `col` over `rows`, as a REPORTABLE value.

    Refuses an intensive column (a sum of levels or proportions is not a
    meaningful quantity) and a non-measure column. A row missing `col`
    contributes 0.
    """
    _require_role(col, roles, {ADDITIVE}, ordinal=False)
    return sum(_number(row, col) for row in rows)


def combine_for_rank(
    rows: list[Row], col: str, roles: Roles, keys: KeyFn
) -> dict[str, float]:
    """Per-key total of `col`, usable ONLY as a ranking key.

    `keys(row)` yields the keys a row contributes to, once per contribution: a
    symmetric pair row credits both of its endpoints, and a self-pair credits
    the same key twice, which is what a weighted degree centrality means.

    Any measure is permitted, intensive included, because the result orders
    keys and its magnitude is never reported. A row missing `col` contributes 0.
    """
    _require_role(col, roles, {ADDITIVE, INTENSIVE}, ordinal=True)
    totals: dict[str, float] = defaultdict(float)
    for row in rows:
        value = _number(row, col)
        for key in keys(row):
            totals[key] += value
    return dict(totals)


def _require_role(col: str, roles: Roles, allowed: set[str], ordinal: bool) -> None:
    role = roles.get(col)
    if role is None or role in allowed:
        return
    if role == INTENSIVE:
        raise AggregationError(
            f"cannot total column {col!r}: it is intensive, so a sum is not a "
            "meaningful value; use combine_for_rank if the total is only a "
            "ranking key"
        )
    ordinally = " even ordinally" if ordinal else ""
    raise AggregationError(
        f"cannot total column {col!r}: its role is {role!r}, not a measure, so "
        f"it cannot be aggregated{ordinally}"
    )


def _number(row: Row, col: str) -> float:
    value = row.get(col, 0)
    try:
        return float(cast("Any", value))
    except (TypeError, ValueError) as e:
        raise AggregationError(f"column {col!r} is not numeric: {value!r}") from e
