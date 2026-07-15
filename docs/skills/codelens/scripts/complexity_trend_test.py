# /// script
# requires-python = ">=3.12"
# dependencies = ["matplotlib"]
# ///
"""Behavioral tests for complexity_trend.py, exercised through its output.

complexity_trend.py reads a live git repo, so a real temporary git repo built by
`git init` + commits + `git mv` is the honest test double; we never mock `git`.
Each test drives the script as a subprocess against a fresh fixture repo and
asserts on its observable behavior: exit code, the `-o` file's existence, and the
trailing `wrote ... (N revisions)` count on stderr. Never on matplotlib internals.

Run: `uv run complexity_trend_test.py` or `python3 -m unittest complexity_trend_test`
from the scripts directory.
"""

from __future__ import annotations

import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).with_name("complexity_trend.py")

EXIT_OK = 0
EXIT_USAGE = 2
EXIT_NO_HISTORY = 3


class TrendRepo:
    """A throwaway git repo for one test case."""

    def __init__(self, root: Path) -> None:
        self.root = root
        self._git("init", "-q")
        self._git("config", "user.email", "test@example.com")
        self._git("config", "user.name", "Test")

    def _git(self, *args: str) -> None:
        subprocess.run(
            ["git", "-C", str(self.root), *args], check=True, capture_output=True
        )

    def commit(self, path: str, body: str) -> None:
        """Write `body` to `path` (creating dirs) and commit it."""
        fp = self.root / path
        fp.parent.mkdir(parents=True, exist_ok=True)
        fp.write_text(body, encoding="utf-8")
        self._git("add", "-A")
        self._git("commit", "-q", "-m", f"touch {path}")

    def rename(self, old: str, new: str) -> None:
        (self.root / new).parent.mkdir(parents=True, exist_ok=True)
        self._git("mv", old, new)
        self._git("commit", "-q", "-m", f"rename {old} -> {new}")


def run_trend(repo: Path, file: str) -> tuple[int, str, bool, int | None]:
    """Run the script; return (rc, stderr, out_exists, revisions_reported)."""
    with tempfile.TemporaryDirectory() as d:
        out = Path(d) / "trend.svg"
        proc = subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                "--repo",
                str(repo),
                "--file",
                file,
                "-o",
                str(out),
            ],
            capture_output=True,
            text=True,
        )
        m = re.search(r"\((\d+) revisions\)", proc.stderr)
        revs = int(m.group(1)) if m else None
        return proc.returncode, proc.stderr, out.is_file(), revs


class TestTrend(unittest.TestCase):
    def test_trend_no_rename(self) -> None:
        # Tracer bullet: a file created and modified in place across 3 commits.
        with tempfile.TemporaryDirectory() as d:
            repo = TrendRepo(Path(d))
            repo.commit("a.py", "def f():\n    pass\n")
            repo.commit("a.py", "def f():\n    if True:\n        pass\n")
            repo.commit("a.py", "def f():\n    if True:\n        return 1\n")
            rc, stderr, out_exists, revs = run_trend(Path(d), "a.py")
        self.assertEqual(rc, EXIT_OK, msg=stderr)
        self.assertTrue(out_exists, msg=stderr)
        self.assertEqual(revs, 3, msg=stderr)

    def test_trend_across_one_rename(self) -> None:
        # Reproduction of the bug: history spans a rename. Every revision must be
        # counted, including the two carried under the old name.
        with tempfile.TemporaryDirectory() as d:
            repo = TrendRepo(Path(d))
            repo.commit("a.py", "def f():\n    pass\n")
            repo.commit("a.py", "def f():\n    if True:\n        pass\n")
            repo.rename("a.py", "b.py")
            repo.commit("b.py", "def f():\n    if True:\n        return 1\n")
            rc, stderr, out_exists, revs = run_trend(Path(d), "b.py")
        self.assertEqual(rc, EXIT_OK, msg=stderr)
        self.assertTrue(out_exists, msg=stderr)
        self.assertEqual(revs, 4, msg=stderr)

    def test_trend_across_two_renames(self) -> None:
        # a -> b -> c: every revision across both renames is counted.
        with tempfile.TemporaryDirectory() as d:
            repo = TrendRepo(Path(d))
            repo.commit("a.py", "def f():\n    pass\n")
            repo.rename("a.py", "b.py")
            repo.commit("b.py", "def f():\n    if True:\n        pass\n")
            repo.rename("b.py", "c.py")
            repo.commit("c.py", "def f():\n    if True:\n        return 1\n")
            rc, stderr, out_exists, revs = run_trend(Path(d), "c.py")
        self.assertEqual(rc, EXIT_OK, msg=stderr)
        self.assertTrue(out_exists, msg=stderr)
        self.assertEqual(revs, 5, msg=stderr)

    def test_trend_missing_file(self) -> None:
        # No history for the file preserves the exit-3 no-history contract.
        with tempfile.TemporaryDirectory() as d:
            repo = TrendRepo(Path(d))
            repo.commit("a.py", "x\n")
            rc, stderr, out_exists, _revs = run_trend(Path(d), "does/not/exist.py")
        self.assertEqual(rc, EXIT_NO_HISTORY, msg=stderr)
        self.assertFalse(out_exists, msg=stderr)
        self.assertIn("no history", stderr)


if __name__ == "__main__":
    unittest.main()
