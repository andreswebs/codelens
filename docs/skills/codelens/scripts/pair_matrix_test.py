# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Behavioral tests for pair_matrix.py, through its observable output.

pair_matrix.py's observable outputs are the ordered entities and the symmetric
matrix (via --json-out), the trailing `wrote ... (N entities, M pairs)` count on
stderr, and the image file. Tests run the script (via `uv run`, so matplotlib
resolves) and assert on those, never on matplotlib internals.

Run: `uv run pair_matrix_test.py` from the scripts directory.
"""

from __future__ import annotations

import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from typing import Any, cast

SCRIPT = Path(__file__).with_name("pair_matrix.py")

EXIT_EMPTY = 3


def _doc(rows: list[dict[str, Any]]) -> dict[str, Any]:
    return {
        "schema_version": 1,
        "ok": True,
        "analysis": "test",
        "row_count": len(rows),
        "rows": rows,
    }


class PairMatrixCase(unittest.TestCase):
    def run_pm(
        self,
        rows: list[dict[str, Any]],
        *,
        a: str,
        b: str,
        w: str,
        extra: list[str] | None = None,
    ) -> tuple[int, str, dict[str, Any] | None, bool]:
        with tempfile.TemporaryDirectory() as d:
            dp = Path(d)
            pairs = dp / "pairs.json"
            pairs.write_text(json.dumps(_doc(rows)), encoding="utf-8")
            out_img = dp / "out.svg"
            json_out = dp / "m.json"
            argv = [
                "uv",
                "run",
                str(SCRIPT),
                "--pairs",
                str(pairs),
                "--a-col",
                a,
                "--b-col",
                b,
                "--weight-col",
                w,
                "-o",
                str(out_img),
                "--json-out",
                str(json_out),
            ]
            if extra:
                argv += extra
            proc = subprocess.run(argv, capture_output=True, text=True)
            data = (
                cast("dict[str, Any]", json.loads(json_out.read_text(encoding="utf-8")))
                if json_out.is_file()
                else None
            )
            return (
                proc.returncode,
                proc.stderr,
                data,
                out_img.is_file() and out_img.stat().st_size > 0,
            )


class TestCoupling(PairMatrixCase):
    def test_symmetric_and_values(self) -> None:
        rows = [
            {"entity": "a.cs", "coupled": "b.cs", "degree": 80},
            {"entity": "a.cs", "coupled": "c.cs", "degree": 40},
        ]
        rc, stderr, data, exists = self.run_pm(
            rows, a="entity", b="coupled", w="degree"
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert data is not None
        ents = cast("list[str]", data["entities"])
        m = cast("list[list[float]]", data["matrix"])
        self.assertIn("a.cs", ents)
        ia, ib = ents.index("a.cs"), ents.index("b.cs")
        self.assertEqual(m[ia][ib], 80)
        self.assertEqual(m[ib][ia], 80)  # symmetric
        self.assertTrue(exists)
        self.assertIn("entities", stderr)

    def test_top_limits_entities(self) -> None:
        rows = [
            {"entity": "a", "coupled": "b", "degree": 90},  # a,b most involved
            {"entity": "a", "coupled": "c", "degree": 10},
            {"entity": "d", "coupled": "e", "degree": 5},
        ]
        rc, stderr, data, _ = self.run_pm(
            rows, a="entity", b="coupled", w="degree", extra=["--top", "2"]
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert data is not None
        self.assertEqual(set(data["entities"]), {"a", "b"})


class TestCommunication(PairMatrixCase):
    def test_communication_columns_and_note(self) -> None:
        rows = [
            {"author": "alice", "peer": "bob", "strength": 41},
            {"author": "alice", "peer": "carol", "strength": 12},
        ]
        rc, stderr, data, exists = self.run_pm(
            rows,
            a="author",
            b="peer",
            w="strength",
            extra=["--note", "coordination risk, not performance"],
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert data is not None
        self.assertIn("alice", data["entities"])
        self.assertTrue(exists)


class TestEmpty(PairMatrixCase):
    def test_empty_rows_exit_3(self) -> None:
        rc, stderr, _data, _ = self.run_pm([], a="entity", b="coupled", w="degree")
        self.assertEqual(rc, EXIT_EMPTY, msg=stderr)


class TestAggregationGuard(PairMatrixCase):
    """Involvement is a ranking key, so --weight-col may be intensive; it may not
    be a non-measure. --schema is what makes either check live."""

    ROWS = [
        {"entity": "a/one.go", "coupled": "a/two.go", "degree": 78},
        {"entity": "a/one.go", "coupled": "b/three.go", "degree": 62},
    ]

    SCHEMA: dict[str, Any] = {
        "row_schema": [
            {"name": "entity", "semantic": "filepath"},
            {"name": "coupled", "semantic": "filepath"},
            {"name": "degree", "semantic": "percentage"},
        ],
        "aggregation_roles": {"filepath": "dimension", "percentage": "intensive"},
    }

    def run_with_schema(
        self, w: str, schema: object
    ) -> tuple[int, str, dict[str, Any] | None, bool]:
        with tempfile.TemporaryDirectory() as d:
            path = Path(d) / "schema.json"
            path.write_text(json.dumps(schema), encoding="utf-8")
            return self.run_pm(
                self.ROWS, a="entity", b="coupled", w=w, extra=["--schema", str(path)]
            )

    def test_an_intensive_weight_is_permitted_and_unchanged(self) -> None:
        rc, stderr, guarded, _ = self.run_with_schema("degree", self.SCHEMA)
        self.assertEqual(rc, 0, msg=stderr)
        _rc, _stderr, plain, _ = self.run_pm(
            self.ROWS, a="entity", b="coupled", w="degree"
        )
        self.assertEqual(guarded, plain)

    def test_a_dimension_weight_is_refused(self) -> None:
        """A filepath cannot be ranked even ordinally, and the guard says so
        instead of failing on a float() conversion."""
        rc, stderr, _data, _ = self.run_with_schema("entity", self.SCHEMA)
        self.assertEqual(rc, 2, msg=stderr)
        self.assertIn("entity", stderr)
        self.assertIn("dimension", stderr)

    def test_an_unusable_schema_file_is_a_usage_error(self) -> None:
        rc, stderr, _data, _ = self.run_with_schema("degree", {"not": "schema output"})
        self.assertEqual(rc, 2, msg=stderr)
        self.assertIn("row_schema", stderr)


if __name__ == "__main__":
    unittest.main()
