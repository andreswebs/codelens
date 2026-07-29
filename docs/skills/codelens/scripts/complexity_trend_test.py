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

    def head(self) -> str:
        r = subprocess.run(
            ["git", "-C", str(self.root), "rev-parse", "HEAD"],
            check=True,
            capture_output=True,
            text=True,
        )
        return r.stdout.strip()

    def commit(self, path: str, body: str) -> None:
        """Write `body` to `path` (creating dirs) and commit it."""
        fp = self.root / path
        fp.parent.mkdir(parents=True, exist_ok=True)
        fp.write_text(body, encoding="utf-8")
        self._git("add", "-A")
        self._git("commit", "-q", "-m", f"touch {path}")

    def delete(self, path: str) -> None:
        self._git("rm", "-q", path)
        self._git("commit", "-q", "-m", f"delete {path}")

    def rename(self, old: str, new: str) -> None:
        (self.root / new).parent.mkdir(parents=True, exist_ok=True)
        self._git("mv", old, new)
        self._git("commit", "-q", "-m", f"rename {old} -> {new}")


def run_trend(repo: Path, file: str, *extra: str) -> tuple[int, str, bool, int | None]:
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
                *extra,
            ],
            capture_output=True,
            text=True,
        )
        m = re.search(r"\((\d+) revisions", proc.stderr)
        revs = int(m.group(1)) if m else None
        return proc.returncode, proc.stderr, out.is_file(), revs


def reported_paths(stderr: str) -> list[str]:
    """The paths the series was actually read at, per the `wrote ...` line."""
    m = re.search(r"paths: ([^)]+)\)", stderr)
    return m.group(1).split(", ") if m else []


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

    def test_series_spans_both_sides_of_a_directory_rename(self) -> None:
        # The regression case: a bulk `src` -> `adminSrc` directory move. Every
        # revision resolves, and the series is read at both names.
        old = "Keeper/src/Infra/Mod.cs"
        new = "Keeper/adminSrc/Infra/Mod.cs"
        with tempfile.TemporaryDirectory() as d:
            repo = TrendRepo(Path(d))
            repo.commit(old, "a\n  b\n")
            repo.commit("Keeper/src/Infra/Other.cs", "x\n")
            repo.commit(old, "a\n  b\n    c\n")
            repo.rename("Keeper/src", "Keeper/adminSrc")
            repo.commit(new, "a\n  b\n    c\n      d\n")
            rc, stderr, out_exists, revs = run_trend(Path(d), new)
        self.assertEqual(rc, EXIT_OK, msg=stderr)
        self.assertTrue(out_exists, msg=stderr)
        self.assertEqual(revs, 4, msg=stderr)
        self.assertNotIn("skipped", stderr)
        self.assertEqual(reported_paths(stderr), [old, new], msg=stderr)

    def test_unresolvable_revision_is_skipped_not_fatal(self) -> None:
        # A delete-then-readd history lists a commit in which the file does not
        # exist at any path it ever carried. That one revision must cost one data
        # point, never the whole figure.
        with tempfile.TemporaryDirectory() as d:
            repo = TrendRepo(Path(d))
            repo.commit("f.py", "def f():\n    pass\n")
            repo.delete("f.py")
            repo.commit("f.py", "def f():\n    if True:\n        pass\n")
            rc, stderr, out_exists, revs = run_trend(Path(d), "f.py")
        self.assertEqual(rc, EXIT_OK, msg=stderr)
        self.assertTrue(out_exists, msg=stderr)
        self.assertEqual(revs, 2, msg=stderr)
        self.assertIn("skipped 1 of 3 revisions", stderr)

    def test_every_revision_unresolvable_is_fatal(self) -> None:
        # Nothing to plot is still a real error: the range holds only the commit
        # that deleted the file.
        with tempfile.TemporaryDirectory() as d:
            repo = TrendRepo(Path(d))
            repo.commit("f.py", "def f():\n    pass\n")
            first = repo.head()
            repo.delete("f.py")
            rc, stderr, out_exists, _revs = run_trend(
                Path(d), "f.py", "--start", first, "--end", repo.head()
            )
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)
        self.assertFalse(out_exists, msg=stderr)
        self.assertIn("could be resolved", stderr)

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
