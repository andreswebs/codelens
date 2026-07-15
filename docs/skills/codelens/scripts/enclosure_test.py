# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Behavioral tests for enclosure.py, exercised through its observable output.

enclosure.py is stdlib-only and its only observable outputs are the intermediate
hierarchy (via --json-out) and the trailing `wrote ... (N files)` count on stderr.
Every test runs the script as a subprocess and asserts on those, never on
internals.

Run: `uv run enclosure_test.py` or `python3 -m unittest enclosure_test` from the
scripts directory.
"""

from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from typing import cast

SCRIPT = Path(__file__).with_name("enclosure.py")

EXIT_USAGE = 2
EXIT_EMPTY = 3


def _weights_doc(col: str, values: dict[str, object]) -> dict[str, object]:
    """A minimal codelens envelope with one weight column per entity."""
    return {
        "schema_version": 1,
        "ok": True,
        "analysis": "test",
        "row_count": len(values),
        "rows": [{"entity": p, col: v} for p, v in values.items()],
    }


def _structure_doc(files: dict[str, int]) -> dict[str, object]:
    """A minimal `tokei --output json` document: one language, one report each."""
    return {
        "Go": {
            "reports": [
                {"name": p, "stats": {"code": loc, "comments": 0, "blanks": 0}}
                for p, loc in files.items()
            ]
        },
        "Total": {"code": sum(files.values())},
    }


def _leaves_by_path(tree: dict[str, object]) -> dict[str, dict[str, object]]:
    """Flatten the D3 hierarchy back to `path -> leaf` (leaf keys minus name)."""
    out: dict[str, dict[str, object]] = {}

    def walk(node: dict[str, object], prefix: str) -> None:
        children = node.get("children")
        if isinstance(children, list):
            for child in cast("list[dict[str, object]]", children):
                name = cast("str", child["name"])
                p = f"{prefix}/{name}" if prefix else name
                walk(child, p)
        else:
            out[prefix] = {k: v for k, v in node.items() if k != "name"}

    walk(tree, "")
    return out


class EnclosureCase(unittest.TestCase):
    def run_enclosure(
        self,
        *,
        weights: dict[str, object],
        weight_col: str = "n_revs",
        structure: dict[str, int] | None = None,
        extra: list[str] | None = None,
    ) -> tuple[int, str, dict[str, object] | None]:
        """Run enclosure.py against temp inputs; return (rc, stderr, hierarchy)."""
        with tempfile.TemporaryDirectory() as d:
            dp = Path(d)
            wpath = dp / "weights.json"
            wpath.write_text(
                json.dumps(_weights_doc(weight_col, weights)), encoding="utf-8"
            )
            out_html = dp / "out.html"
            json_out = dp / "tree.json"
            argv = [
                sys.executable,
                str(SCRIPT),
                "--weights",
                str(wpath),
                "--weight-col",
                weight_col,
                "-o",
                str(out_html),
                "--json-out",
                str(json_out),
            ]
            if structure is not None:
                spath = dp / "structure.json"
                spath.write_text(json.dumps(_structure_doc(structure)), encoding="utf-8")
                argv += ["--structure", str(spath)]
            if extra:
                argv += extra
            proc = subprocess.run(argv, capture_output=True, text=True)
            tree = (
                json.loads(json_out.read_text(encoding="utf-8"))
                if json_out.is_file()
                else None
            )
            return proc.returncode, proc.stderr, tree


class TestCategoricalUsesStructure(EnclosureCase):
    def test_categorical_uses_structure_when_given(self) -> None:
        rc, _stderr, tree = self.run_enclosure(
            weights={"src/a.go": "alice", "src/b.go": "bob"},
            weight_col="main_dev",
            structure={"src/a.go": 100, "src/b.go": 200, "src/c.go": 300},
            extra=["--categorical"],
        )
        self.assertEqual(rc, 0)
        assert tree is not None
        leaves = _leaves_by_path(tree)
        self.assertEqual(set(leaves), {"src/a.go", "src/b.go", "src/c.go"})
        self.assertEqual(leaves["src/a.go"]["size"], 100)
        self.assertEqual(leaves["src/b.go"]["size"], 200)
        self.assertEqual(leaves["src/c.go"]["size"], 300)

    def test_categorical_neutral_sentinel(self) -> None:
        rc, _stderr, tree = self.run_enclosure(
            weights={"src/a.go": "alice"},
            weight_col="main_dev",
            structure={"src/a.go": 100, "src/c.go": 300},
            extra=["--categorical"],
        )
        self.assertEqual(rc, 0)
        assert tree is not None
        leaves = _leaves_by_path(tree)
        self.assertEqual(leaves["src/a.go"]["category"], "alice")
        self.assertEqual(leaves["src/c.go"]["category"], "(unowned)")


class TestSizeMode(EnclosureCase):
    def test_size_mode_nodeset_is_structure(self) -> None:
        rc, _stderr, tree = self.run_enclosure(
            weights={"src/a.go": 10, "src/b.go": 20},
            structure={"src/a.go": 100, "src/b.go": 200, "src/c.go": 300},
        )
        self.assertEqual(rc, 0)
        assert tree is not None
        leaves = _leaves_by_path(tree)
        self.assertEqual(set(leaves), {"src/a.go", "src/b.go", "src/c.go"})
        self.assertEqual(leaves["src/c.go"]["size"], 300)
        self.assertEqual(leaves["src/c.go"]["weight"], 0.0)

    def test_invert_missing_is_cold_not_hot(self) -> None:
        # code-age map: --invert makes low age hot. A file absent from the age
        # weights has no recorded change and must render cold (0.0), never hot.
        rc, _stderr, tree = self.run_enclosure(
            weights={"src/a.go": 2, "src/b.go": 40},
            weight_col="age_months",
            structure={"src/a.go": 100, "src/b.go": 200, "src/c.go": 300},
            extra=["--invert"],
        )
        self.assertEqual(rc, 0)
        assert tree is not None
        leaves = _leaves_by_path(tree)
        self.assertEqual(leaves["src/c.go"]["weight"], 0.0)
        self.assertEqual(leaves["src/a.go"]["weight"], 1.0)  # youngest -> hottest


class TestDegradedMode(EnclosureCase):
    def test_degraded_numeric_unchanged(self) -> None:
        rc, _stderr, tree = self.run_enclosure(
            weights={"src/a.go": 3, "src/b.go": 9},
        )
        self.assertEqual(rc, 0)
        assert tree is not None
        leaves = _leaves_by_path(tree)
        self.assertEqual(set(leaves), {"src/a.go", "src/b.go"})
        self.assertEqual(leaves["src/a.go"]["size"], 3)
        self.assertEqual(leaves["src/b.go"]["weight"], 1.0)

    def test_degraded_categorical_unchanged(self) -> None:
        rc, _stderr, tree = self.run_enclosure(
            weights={"src/a.go": "alice", "src/b.go": "bob"},
            weight_col="main_dev",
            extra=["--categorical"],
        )
        self.assertEqual(rc, 0)
        assert tree is not None
        leaves = _leaves_by_path(tree)
        self.assertEqual(set(leaves), {"src/a.go", "src/b.go"})
        self.assertEqual(leaves["src/a.go"]["size"], 1)
        self.assertEqual(leaves["src/a.go"]["category"], "alice")


class TestCountMatchesNodeSet(EnclosureCase):
    def test_count_matches_nodeset(self) -> None:
        structure = {"src/a.go": 100, "src/b.go": 200, "src/c.go": 300}
        for extra in ([], ["--categorical"], ["--invert"]):
            col = "main_dev" if "--categorical" in extra else "n_revs"
            wval: object = "alice" if "--categorical" in extra else 5
            rc, stderr, tree = self.run_enclosure(
                weights={"src/a.go": wval, "src/b.go": wval},
                weight_col=col,
                structure=structure,
                extra=extra,
            )
            self.assertEqual(rc, 0, msg=f"extra={extra}")
            assert tree is not None
            leaves = _leaves_by_path(tree)
            self.assertIn(f"({len(leaves)} files)", stderr, msg=f"extra={extra}")
            self.assertEqual(len(leaves), 3, msg=f"extra={extra}")


class TestPathFilter(EnclosureCase):
    def test_exclude_filters_structure_and_weights(self) -> None:
        # A Migrations/ file present in both the structure and the weights is
        # excluded from both, so it is in neither the node set nor the leaves and
        # the wrote-count drops accordingly.
        rc, stderr, tree = self.run_enclosure(
            weights={"src/a.go": 5, "src/Migrations/0001.go": 9},
            structure={"src/a.go": 100, "src/Migrations/0001.go": 200},
            extra=["--exclude", "**/Migrations/**"],
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert tree is not None
        leaves = _leaves_by_path(tree)
        self.assertEqual(set(leaves), {"src/a.go"})
        self.assertIn("(1 files)", stderr)

    def test_include_then_exclude(self) -> None:
        # include **/*.cs then exclude **/*.Designer.cs: the .cs file survives,
        # the .Designer.cs file is excluded, and a .dart file is never included.
        rc, stderr, tree = self.run_enclosure(
            weights={
                "src/Page.cs": "alice",
                "src/Page.Designer.cs": "bob",
                "src/app.g.dart": "carol",
            },
            weight_col="main_dev",
            structure={
                "src/Page.cs": 100,
                "src/Page.Designer.cs": 50,
                "src/app.g.dart": 30,
            },
            extra=["--categorical", "--include", "**/*.cs", "--exclude", "**/*.Designer.cs"],
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert tree is not None
        leaves = _leaves_by_path(tree)
        self.assertEqual(set(leaves), {"src/Page.cs"})

    def test_exclude_everything_is_empty(self) -> None:
        rc, stderr, _tree = self.run_enclosure(
            weights={"src/a.go": 5},
            structure={"src/a.go": 100},
            extra=["--exclude", "**"],
        )
        self.assertEqual(rc, EXIT_EMPTY, msg=stderr)

    def test_bad_glob_is_usage_error(self) -> None:
        rc, stderr, _tree = self.run_enclosure(
            weights={"src/a.go": 5},
            extra=["--exclude", "a[b"],
        )
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)
        self.assertIn("invalid glob", stderr)


if __name__ == "__main__":
    unittest.main()
