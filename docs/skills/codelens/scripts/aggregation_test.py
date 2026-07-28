# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Tests for aggregation.py, the shared aggregation guard.

The load-bearing cases are the two negatives: combine_for_value REFUSES an
intensive column, and both helpers PASS THROUGH when no roles are available.
The first is the guard's entire point; the second is the graceful degradation
that makes an optional --schema acceptable.

Run: `uv run aggregation_test.py` from the scripts directory.
"""

from __future__ import annotations

import json
import sys
import tempfile
import unittest
import warnings
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).parent))

import aggregation as agg  # noqa: E402

# Column -> role as roles_from_schema would build it for `coupling`.
COUPLING_ROLES = {
    "entity": "dimension",
    "coupled": "dimension",
    "degree": "intensive",
    "average_revs": "additive",
}

CHURN_ROWS: list[dict[str, Any]] = [
    {"date": "2026-01-01", "added": 10, "deleted": 4, "commits": 2},
    {"date": "2026-01-02", "added": 5, "deleted": 1, "commits": 1},
]

CHURN_ROLES = {
    "date": "dimension",
    "added": "additive",
    "deleted": "additive",
    "commits": "additive",
}

PAIR_ROWS: list[dict[str, Any]] = [
    {"author": "ada", "peer": "bob", "strength": 60},
    {"author": "ada", "peer": "cyd", "strength": 36},
]

PAIR_ROLES = {"author": "dimension", "peer": "dimension", "strength": "intensive"}


def pair_keys(row: dict[str, Any]) -> tuple[str, str]:
    return (str(row["author"]), str(row["peer"]))


def schema_doc(**extra: Any) -> dict[str, Any]:
    """A `codelens schema --command coupling` document, trimmed to the keys the
    guard reads."""
    doc: dict[str, Any] = {
        "schema_version": 1,
        "ok": True,
        "command": "coupling",
        "row_schema": [
            {"name": "entity", "type": "string", "semantic": "filepath"},
            {"name": "coupled", "type": "string", "semantic": "filepath"},
            {"name": "degree", "type": "int", "semantic": "percentage"},
            {"name": "average_revs", "type": "int", "semantic": "count"},
        ],
        "aggregation_roles": {
            "filepath": "dimension",
            "percentage": "intensive",
            "count": "additive",
        },
    }
    doc.update(extra)
    return doc


def write_schema(doc: object) -> str:
    tmp = tempfile.NamedTemporaryFile(  # noqa: SIM115
        "w", suffix=".json", delete=False, encoding="utf-8"
    )
    with tmp:
        tmp.write(doc if isinstance(doc, str) else json.dumps(doc))
    return tmp.name


class CombineForValueCase(unittest.TestCase):
    def test_sums_an_additive_column(self) -> None:
        self.assertEqual(agg.combine_for_value(CHURN_ROWS, "added", CHURN_ROLES), 15)
        self.assertEqual(agg.combine_for_value(CHURN_ROWS, "deleted", CHURN_ROLES), 5)

    def test_refuses_an_intensive_column(self) -> None:
        """The guard's entire point: a sum of percentages is not a value."""
        rows = [{"degree": 78}, {"degree": 62}]
        with self.assertRaises(agg.AggregationError) as caught:
            agg.combine_for_value(rows, "degree", COUPLING_ROLES)
        msg = str(caught.exception)
        self.assertIn("degree", msg)
        self.assertIn("intensive", msg)
        self.assertIn("combine_for_rank", msg)

    def test_refuses_a_dimension_column(self) -> None:
        rows = [{"entity": "a.go"}, {"entity": "b.go"}]
        with self.assertRaises(agg.AggregationError):
            agg.combine_for_value(rows, "entity", COUPLING_ROLES)

    def test_passes_through_without_roles(self) -> None:
        """Absent --schema there is nothing to check against, so an intensive
        column sums with no error and no warning."""
        rows = [{"degree": 78}, {"degree": 62}]
        with warnings.catch_warnings():
            warnings.simplefilter("error")
            self.assertEqual(agg.combine_for_value(rows, "degree", {}), 140)

    def test_passes_through_for_a_column_absent_from_roles(self) -> None:
        rows = [{"size": 3}, {"size": 4}]
        self.assertEqual(agg.combine_for_value(rows, "size", COUPLING_ROLES), 7)

    def test_a_missing_column_contributes_zero(self) -> None:
        rows: list[dict[str, Any]] = [{"added": 10}, {}]
        self.assertEqual(agg.combine_for_value(rows, "added", CHURN_ROLES), 10)

    def test_empty_rows_total_zero(self) -> None:
        self.assertEqual(agg.combine_for_value([], "added", CHURN_ROLES), 0)

    def test_a_non_numeric_value_is_an_aggregation_error(self) -> None:
        rows: list[dict[str, Any]] = [{"added": "many"}]
        with self.assertRaises(agg.AggregationError) as caught:
            agg.combine_for_value(rows, "added", CHURN_ROLES)
        self.assertIn("added", str(caught.exception))


class CombineForRankCase(unittest.TestCase):
    def test_permits_an_intensive_column(self) -> None:
        """Ordinal summation is legal for every measure, intensive included."""
        totals = agg.combine_for_rank(PAIR_ROWS, "strength", PAIR_ROLES, pair_keys)
        self.assertEqual(totals, {"ada": 96, "bob": 60, "cyd": 36})

    def test_refuses_a_dimension_column(self) -> None:
        """Not a measure: unrankable even ordinally."""
        rows = [{"entity": "a.go", "coupled": "b.go"}]
        with self.assertRaises(agg.AggregationError) as caught:
            agg.combine_for_rank(
                rows,
                "entity",
                COUPLING_ROLES,
                lambda r: (str(r["entity"]), str(r["coupled"])),
            )
        self.assertIn("entity", str(caught.exception))

    def test_permits_an_additive_column(self) -> None:
        totals = agg.combine_for_rank(PAIR_ROWS, "strength", CHURN_ROLES, pair_keys)
        self.assertEqual(totals, {"ada": 96, "bob": 60, "cyd": 36})

    def test_counts_a_repeated_key_once_per_occurrence(self) -> None:
        """A self-tie adds its weight twice, as the hand-written loops did."""
        rows: list[dict[str, Any]] = [{"author": "ada", "peer": "ada", "strength": 30}]
        totals = agg.combine_for_rank(rows, "strength", PAIR_ROLES, pair_keys)
        self.assertEqual(totals, {"ada": 60})

    def test_passes_through_without_roles(self) -> None:
        with warnings.catch_warnings():
            warnings.simplefilter("error")
            totals = agg.combine_for_rank(PAIR_ROWS, "strength", {}, pair_keys)
        self.assertEqual(totals, {"ada": 96, "bob": 60, "cyd": 36})

    def test_empty_rows_give_no_keys(self) -> None:
        self.assertEqual(
            agg.combine_for_rank([], "strength", PAIR_ROLES, pair_keys), {}
        )


class RolesFromSchemaCase(unittest.TestCase):
    def test_joins_row_schema_semantics_against_the_role_map(self) -> None:
        self.assertEqual(
            agg.roles_from_schema(write_schema(schema_doc())), COUPLING_ROLES
        )

    def test_covers_flag_gated_columns(self) -> None:
        """`schema` declares every column, gated ones included, so the join must
        carry them too."""
        doc = schema_doc()
        doc["row_schema"].append(
            {"name": "shared_revisions", "type": "int", "semantic": "count"}
        )
        roles = agg.roles_from_schema(write_schema(doc))
        self.assertEqual(roles["shared_revisions"], "additive")

    def test_no_role_map_degrades_to_no_roles(self) -> None:
        """An older codelens without aggregation_roles must not break the guard."""
        doc = schema_doc()
        del doc["aggregation_roles"]
        self.assertEqual(agg.roles_from_schema(write_schema(doc)), {})

    def test_an_unknown_semantic_is_skipped_rather_than_guessed(self) -> None:
        doc = schema_doc()
        doc["row_schema"].append(
            {"name": "novel", "type": "string", "semantic": "not_a_semantic"}
        )
        self.assertNotIn("novel", agg.roles_from_schema(write_schema(doc)))

    def test_a_document_without_row_schema_is_an_error(self) -> None:
        with self.assertRaises(agg.AggregationError) as caught:
            agg.roles_from_schema(write_schema({"schema_version": 1, "ok": True}))
        self.assertIn("row_schema", str(caught.exception))

    def test_invalid_json_is_an_error(self) -> None:
        with self.assertRaises(agg.AggregationError):
            agg.roles_from_schema(write_schema("{not json"))

    def test_a_missing_file_is_an_error(self) -> None:
        with self.assertRaises(agg.AggregationError):
            agg.roles_from_schema(str(Path(tempfile.gettempdir()) / "no-such-schema"))

    def test_the_real_binarys_output_shape_round_trips(self) -> None:
        """Guards against the join drifting from the wire format: every column
        the schema declares resolves to a role."""
        doc = schema_doc()
        roles = agg.roles_from_schema(write_schema(doc))
        self.assertEqual(sorted(roles), sorted(c["name"] for c in doc["row_schema"]))


if __name__ == "__main__":
    unittest.main()
