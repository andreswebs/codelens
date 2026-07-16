# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Behavioral tests for treemap.py, exercised through its observable output.

treemap.py's observable outputs are the drawn leaves (via --json-out), the trailing
`wrote ... (N files)` count on stderr, and the output image file. Every test runs
the script (via `uv run`, so its matplotlib/squarify deps resolve) and asserts on
those, never on matplotlib internals. The node-set behavior mirrors enclosure.py so
the static treemap and the interactive circle map draw the same files.

Run: `uv run treemap_test.py` from the scripts directory.
"""

from __future__ import annotations

import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from typing import cast

SCRIPT = Path(__file__).with_name("treemap.py")

EXIT_USAGE = 2
EXIT_EMPTY = 3


def _weights_doc(col: str, values: dict[str, object]) -> dict[str, object]:
    return {
        "schema_version": 1,
        "ok": True,
        "analysis": "test",
        "row_count": len(values),
        "rows": [{"entity": p, col: v} for p, v in values.items()],
    }


def _structure_doc(files: dict[str, int]) -> dict[str, object]:
    return {
        "Go": {
            "reports": [{"name": p, "stats": {"code": loc}} for p, loc in files.items()]
        },
        "Total": {"code": sum(files.values())},
    }


class TreemapCase(unittest.TestCase):
    def run_treemap(
        self,
        *,
        weights: dict[str, object],
        weight_col: str = "n_revs",
        structure: dict[str, int] | None = None,
        out_ext: str = "svg",
        extra: list[str] | None = None,
    ) -> tuple[int, str, dict[str, dict[str, object]] | None, bool]:
        """Run treemap.py; return (rc, stderr, leaves|None, out_file_exists)."""
        with tempfile.TemporaryDirectory() as d:
            dp = Path(d)
            wpath = dp / "weights.json"
            wpath.write_text(
                json.dumps(_weights_doc(weight_col, weights)), encoding="utf-8"
            )
            out_img = dp / f"out.{out_ext}"
            json_out = dp / "leaves.json"
            argv = [
                "uv",
                "run",
                str(SCRIPT),
                "--weights",
                str(wpath),
                "--weight-col",
                weight_col,
                "-o",
                str(out_img),
                "--json-out",
                str(json_out),
            ]
            if structure is not None:
                spath = dp / "structure.json"
                spath.write_text(
                    json.dumps(_structure_doc(structure)), encoding="utf-8"
                )
                argv += ["--structure", str(spath)]
            if extra:
                argv += extra
            proc = subprocess.run(argv, capture_output=True, text=True)
            leaves = (
                cast(
                    "dict[str, dict[str, object]]",
                    json.loads(json_out.read_text(encoding="utf-8")),
                )
                if json_out.is_file()
                else None
            )
            return (
                proc.returncode,
                proc.stderr,
                leaves,
                out_img.is_file() and out_img.stat().st_size > 0,
            )


class TestNodeSet(TreemapCase):
    def test_size_mode_nodeset_is_structure(self) -> None:
        rc, stderr, leaves, exists = self.run_treemap(
            weights={"src/a.go": 10, "src/b.go": 20},
            structure={"src/a.go": 100, "src/b.go": 200, "src/c.go": 300},
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert leaves is not None
        self.assertEqual(set(leaves), {"src/a.go", "src/b.go", "src/c.go"})
        self.assertEqual(leaves["src/c.go"]["size"], 300)
        self.assertEqual(
            leaves["src/c.go"]["weight"], 0.0
        )  # absent from weights -> cold
        self.assertTrue(exists)
        self.assertIn("(3 files)", stderr)

    def test_invert_missing_is_cold_not_hot(self) -> None:
        rc, stderr, leaves, _ = self.run_treemap(
            weights={"src/a.go": 2, "src/b.go": 40},
            weight_col="age_months",
            structure={"src/a.go": 100, "src/b.go": 200, "src/c.go": 300},
            extra=["--invert"],
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert leaves is not None
        self.assertEqual(leaves["src/a.go"]["weight"], 1.0)  # youngest -> hottest
        self.assertEqual(leaves["src/c.go"]["weight"], 0.0)  # unchanged -> cold

    def test_categorical_uses_structure_and_sentinel(self) -> None:
        rc, stderr, leaves, _ = self.run_treemap(
            weights={"src/a.go": "alice"},
            weight_col="main_dev",
            structure={"src/a.go": 100, "src/c.go": 300},
            extra=["--categorical"],
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert leaves is not None
        self.assertEqual(leaves["src/a.go"]["category"], "alice")
        self.assertEqual(leaves["src/a.go"]["size"], 100)  # sized by tokei LOC
        self.assertEqual(leaves["src/c.go"]["category"], "(unowned)")


class TestDegraded(TreemapCase):
    def test_degraded_numeric_nodeset_is_weights(self) -> None:
        rc, stderr, leaves, _ = self.run_treemap(weights={"src/a.go": 3, "src/b.go": 9})
        self.assertEqual(rc, 0, msg=stderr)
        assert leaves is not None
        self.assertEqual(set(leaves), {"src/a.go", "src/b.go"})
        self.assertEqual(leaves["src/a.go"]["size"], 3)
        self.assertEqual(leaves["src/b.go"]["weight"], 1.0)


class TestFormatsAndTop(TreemapCase):
    def test_png_output(self) -> None:
        rc, stderr, _leaves, exists = self.run_treemap(
            weights={"src/a.go": 3, "src/b.go": 9}, out_ext="png"
        )
        self.assertEqual(rc, 0, msg=stderr)
        self.assertTrue(exists)

    def test_top_limits_drawn(self) -> None:
        rc, stderr, leaves, _ = self.run_treemap(
            weights={"a": 1, "b": 2, "c": 3, "d": 4},
            extra=["--top", "2"],
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert leaves is not None
        self.assertEqual(
            set(leaves), {"c", "d"}
        )  # two largest by size (weight-as-size degraded)
        self.assertIn("(2 files)", stderr)


class TestFilter(TreemapCase):
    def test_exclude_filters_both(self) -> None:
        rc, stderr, leaves, _ = self.run_treemap(
            weights={"src/a.go": 5, "src/Migrations/0001.go": 9},
            structure={"src/a.go": 100, "src/Migrations/0001.go": 200},
            extra=["--exclude", "**/Migrations/**"],
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert leaves is not None
        self.assertEqual(set(leaves), {"src/a.go"})

    def test_exclude_everything_is_empty(self) -> None:
        rc, stderr, _leaves, _ = self.run_treemap(
            weights={"src/a.go": 5},
            structure={"src/a.go": 100},
            extra=["--exclude", "**"],
        )
        self.assertEqual(rc, EXIT_EMPTY, msg=stderr)

    def test_bad_glob_is_usage_error(self) -> None:
        rc, stderr, _leaves, _ = self.run_treemap(
            weights={"src/a.go": 5}, extra=["--exclude", "a[b"]
        )
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)
        self.assertIn("invalid glob", stderr)


class TestDomination(TreemapCase):
    def test_dominant_file_warns_with_pct_and_loc(self) -> None:
        rc, stderr, _leaves, _e = self.run_treemap(
            weights={"src/a.go": 5},
            structure={"data/big.json": 900, "src/a.go": 50, "src/b.go": 50},
        )
        self.assertEqual(rc, 0, msg=stderr)
        self.assertIn("dominant: data/big.json 90% (900 LOC)", stderr)
        self.assertNotIn("dominant: src/a.go", stderr)

    def test_no_file_over_threshold_no_warning(self) -> None:
        files = {f"src/f{i}.go": 100 for i in range(12)}  # each ~8.3% of total
        rc, stderr, _leaves, _e = self.run_treemap(
            weights={"src/f0.go": 5}, structure=files
        )
        self.assertEqual(rc, 0, msg=stderr)
        self.assertNotIn("dominant:", stderr)

    def test_no_structure_never_warns(self) -> None:
        # Degraded mode: area is the weight, so a file dominating by weight is the
        # real signal, not noise; the warning is gated on --structure.
        rc, stderr, _leaves, _e = self.run_treemap(weights={"big": 1000, "a": 1})
        self.assertEqual(rc, 0, msg=stderr)
        self.assertNotIn("dominant:", stderr)

    def test_categorical_mode_warns(self) -> None:
        rc, stderr, _leaves, _e = self.run_treemap(
            weights={"src/a.go": "alice"},
            weight_col="main_dev",
            structure={"data/big.json": 900, "src/a.go": 100},
            extra=["--categorical"],
        )
        self.assertEqual(rc, 0, msg=stderr)
        self.assertIn("dominant: data/big.json 90% (900 LOC)", stderr)

    def test_offenders_capped_at_five(self) -> None:
        files = {f"big{i}.json": 150 for i in range(6)}  # six files at 15% each
        files["small.go"] = 100  # exactly 10%, not over the threshold
        rc, stderr, _leaves, _e = self.run_treemap(
            weights={"small.go": 5}, structure=files
        )
        self.assertEqual(rc, 0, msg=stderr)
        self.assertEqual(stderr.count("dominant:"), 5)
        self.assertNotIn("small.go", stderr)  # 10% is not > 10%

    def test_excluded_file_not_counted_toward_total(self) -> None:
        files = {"data/gen.json": 900}
        files.update({f"src/f{i}.go": 10 for i in range(10)})
        rc, stderr, _leaves, _e = self.run_treemap(
            weights={"src/f0.go": 5},
            structure=files,
            extra=["--exclude", "data/gen.json"],
        )
        self.assertEqual(rc, 0, msg=stderr)
        # After dropping gen.json the total is 100 and each file is exactly 10%,
        # so nothing triggers and the excluded file is never named.
        self.assertNotIn("dominant:", stderr)
        self.assertNotIn("gen.json", stderr)


if __name__ == "__main__":
    unittest.main()
