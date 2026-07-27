# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Behavioral tests for digest.py, exercised through the emitted digest markdown.

digest.py is stdlib-only; its observable output is the digest.md it writes from a
directory of per-analysis JSON files. Tests run it as a subprocess and assert on
the markdown: the code-vs-docs hotspot split, thin-coupling note, ownership tally,
fragmentation ordering, code-age range, churn spike, commit-vocabulary stop-word
filtering, and the empty/usage exit codes.

Run: `uv run digest_test.py` from the scripts directory.
"""

from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).with_name("digest.py")

EXIT_USAGE = 2
EXIT_EMPTY = 3


class DigestCase(unittest.TestCase):
    def run_digest(
        self, files: dict[str, object], *, git_log: str | None = None
    ) -> tuple[int, str, str]:
        """Write each `name -> rows` as an envelope JSON into a temp dir, run
        digest.py, and return (rc, stderr, digest_markdown)."""
        with tempfile.TemporaryDirectory() as d:
            dp = Path(d)
            for name, rows in files.items():
                (dp / name).write_text(json.dumps({"rows": rows}), encoding="utf-8")
            if git_log is not None:
                (dp / "git.log").write_text(git_log, encoding="utf-8")
            out = dp / "digest.md"
            proc = subprocess.run(
                [sys.executable, str(SCRIPT), str(dp), "-o", str(out)],
                capture_output=True,
                text=True,
            )
            md = out.read_text(encoding="utf-8") if out.is_file() else ""
            return proc.returncode, proc.stderr, md


class TestHotspots(DigestCase):
    def test_code_and_docs_are_split(self) -> None:
        rc, stderr, md = self.run_digest(
            {
                "revisions.json": [
                    {"entity": "app/Order.php", "n_revs": 40},
                    {"entity": "README.md", "n_revs": 99},
                    {"entity": "config.yaml", "n_revs": 30},
                ]
            }
        )
        self.assertEqual(rc, 0, msg=stderr)
        code = md.split("## hotspots", 1)[1].split("## top docs/config", 1)[0]
        docs = md.split("## top docs/config", 1)[1]
        self.assertIn("app/Order.php", code)
        self.assertNotIn("app/Order.php", docs)
        # README.md tops raw revisions but is classified as docs, not a code hotspot.
        self.assertIn("README.md", docs)
        self.assertNotIn("README.md", code)


class TestCoupling(DigestCase):
    def test_empty_coupling_notes_thin_signal(self) -> None:
        _rc, _stderr, md = self.run_digest({"coupling.json": []})
        self.assertIn("none above min-degree threshold", md)


class TestOwnership(DigestCase):
    def test_single_owner_tally(self) -> None:
        _rc, _stderr, md = self.run_digest(
            {
                "main-dev.json": [
                    {"entity": "a.py", "main_dev": "alex", "ownership": 1.0},
                    {"entity": "b.py", "main_dev": "alex", "ownership": 0.5},
                    {"entity": "c.py", "main_dev": "sam", "ownership": 1.0},
                ]
            }
        )
        self.assertIn("code files with a single 100%-owner: 2/3", md)
        self.assertIn("2  alex", md)


class TestFragmentation(DigestCase):
    def test_sorted_by_fractal_desc(self) -> None:
        _rc, _stderr, md = self.run_digest(
            {
                "fragmentation.json": [
                    {"entity": "low.py", "fractal_value": 0.10, "total_revs": 5},
                    {"entity": "high.py", "fractal_value": 0.90, "total_revs": 9},
                ]
            }
        )
        block = md.split("## most fragmented", 1)[1]
        self.assertLess(block.index("high.py"), block.index("low.py"))


class TestCodeAge(DigestCase):
    def test_age_range(self) -> None:
        _rc, _stderr, md = self.run_digest(
            {
                "code-age.json": [
                    {"entity": "a.py", "age_months": 2},
                    {"entity": "b.py", "age_months": 10},
                    {"entity": "c.py", "age_months": 30},
                ]
            }
        )
        self.assertIn("min=2 median=10 max=30", md)


class TestChurn(DigestCase):
    def test_spike_and_ratio(self) -> None:
        _rc, _stderr, md = self.run_digest(
            {
                "abs-churn.json": [
                    {"date": "2026-01", "added": 100, "deleted": 50},
                    {"date": "2026-02", "added": 900, "deleted": 100},
                ]
            }
        )
        self.assertIn("biggest period: 2026-02 +900/-100", md)
        self.assertIn("add/del_ratio=6.67", md)


class TestVocabulary(DigestCase):
    def test_stopwords_dropped(self) -> None:
        _rc, _stderr, md = self.run_digest(
            {
                "summary.json": [{"statistic": "number-of-commits", "value": 2}],
                "parse.json": [
                    {"message": "fix the disbursement service"},
                    {"message": "disbursement retry disbursement"},
                ],
            }
        )
        vocab = md.split("## commit vocabulary", 1)[1]
        self.assertIn("disbursement(3)", vocab)
        self.assertNotIn("the(", vocab)  # stop word
        self.assertNotIn("fix(", vocab)  # stop word


class TestWindow(DigestCase):
    def test_window_dates_from_git_log(self) -> None:
        _rc, _stderr, md = self.run_digest(
            {"summary.json": [{"statistic": "number-of-commits", "value": 3}]},
            git_log="--a1b2c3d--2026-06-20--Bob--newest\n--e4f5a6b--2026-01-05--Bob--oldest\n",
        )
        self.assertIn("window dates seen: 2026-01-05 .. 2026-06-20", md)

    def test_oversized_oldest_entry_does_not_collapse_window(self) -> None:
        # The oldest (tail) entry carries a numstat block far larger than the old
        # 2,000-byte end sample, so a sampling implementation never reaches its
        # header date and collapses the window onto the newest date. Regression.
        numstat = "".join(f"{i}\t{i}\tsrc/file_{i:04d}.go\n" for i in range(400))
        git_log = (
            "--a1b2c3d--2026-07-27--Bob--newest change\n" + numstat + "\n"
            "--e4f5a6b--2026-07-14--Bob--initial commit\n" + numstat + "\n"
        )
        # The oldest header must fall beyond both the head and tail samples an
        # end-sampling implementation would read: newest numstat pushes it past
        # the head, its own numstat keeps it out of the tail.
        self.assertGreater(len(numstat), 2000)
        _rc, _stderr, md = self.run_digest(
            {"summary.json": [{"statistic": "number-of-commits", "value": 2}]},
            git_log=git_log,
        )
        self.assertIn("window dates seen: 2026-07-14 .. 2026-07-27", md)

    def test_date_in_subject_does_not_widen_window(self) -> None:
        git_log = (
            "--a1b2c3d--2026-01-02--Bob--fix the 1999-12-31 rollover\n"
            "1\t0\ta.go\n\n"
            "--e4f5a6b--2026-01-05--Bob--another 2099-09-09 mention\n"
            "1\t0\tb.go\n"
        )
        _rc, _stderr, md = self.run_digest(
            {"summary.json": [{"statistic": "number-of-commits", "value": 2}]},
            git_log=git_log,
        )
        self.assertIn("window dates seen: 2026-01-02 .. 2026-01-05", md)
        self.assertNotIn("1999-12-31", md)
        self.assertNotIn("2099-09-09", md)

    def test_three_field_stock_header_is_recognised(self) -> None:
        # Stock git2 header has no commit subject: --<hash>--<date>--<author>.
        git_log = "--a1b2c3d--2026-03-01--Bob\n1\t0\ta.go\n--e4f5a6b--2026-03-08--Bob\n1\t0\tb.go\n"
        _rc, _stderr, md = self.run_digest(
            {"summary.json": [{"statistic": "number-of-commits", "value": 2}]},
            git_log=git_log,
        )
        self.assertIn("window dates seen: 2026-03-01 .. 2026-03-08", md)

    def test_single_commit_window(self) -> None:
        git_log = "--a1b2c3d--2026-05-05--Bob--only commit\n1\t0\ta.go\n"
        _rc, _stderr, md = self.run_digest(
            {"summary.json": [{"statistic": "number-of-commits", "value": 1}]},
            git_log=git_log,
        )
        self.assertIn("window dates seen: 2026-05-05 .. 2026-05-05 (1 active dates)", md)


class TestExitCodes(DigestCase):
    def test_empty_dir_exit_3(self) -> None:
        with tempfile.TemporaryDirectory() as d:
            proc = subprocess.run(
                [sys.executable, str(SCRIPT), d],
                capture_output=True,
                text=True,
            )
            self.assertEqual(proc.returncode, EXIT_EMPTY, msg=proc.stderr)

    def test_missing_dir_exit_2(self) -> None:
        proc = subprocess.run(
            [sys.executable, str(SCRIPT), "/no/such/dir"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(proc.returncode, EXIT_USAGE, msg=proc.stderr)


if __name__ == "__main__":
    unittest.main()
