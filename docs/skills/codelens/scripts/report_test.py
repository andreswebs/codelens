# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Behavioral tests for report.py, exercised through the emitted markdown.

report.py is stdlib-only; its observable output is the report.md it writes. Tests
run it as a subprocess and assert on the markdown: section order, inline-SVG
embedding, findings slotting, the always-present guardrails, placeholders for
missing findings, and the empty-input exit code.

Run: `uv run report_test.py` from the scripts directory.
"""

from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).with_name("report.py")

EXIT_EMPTY = 3

FINDINGS = """\
# keeper-core evolutionary analysis
## window
last 12 months
## executive_summary
The disbursement services carry the churn.
## hotspots
EmployeeStatusService.cs is the real offender.
## coupling
Config files change in lockstep.
## risk_choices
Mitigate the disbursement hotspot first.
"""

SVG = '<?xml version="1.0"?>\n<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "x.dtd">\n<svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/></svg>\n'


class ReportCase(unittest.TestCase):
    def run_report(
        self,
        *,
        findings: str | None = None,
        figures: dict[str, str] | None = None,
        summary: list[dict[str, object]] | None = None,
        omit_all: bool = False,
    ) -> tuple[int, str, str]:
        """Run report.py; return (rc, stderr, report_markdown)."""
        with tempfile.TemporaryDirectory() as d:
            dp = Path(d)
            out = dp / "report.md"
            argv = [sys.executable, str(SCRIPT), "-o", str(out)]
            if findings is not None:
                fp = dp / "findings.md"
                fp.write_text(findings, encoding="utf-8")
                argv += ["--findings", str(fp)]
            if figures is not None:
                fdir = dp / "figs"
                fdir.mkdir()
                for stem, svg in figures.items():
                    (fdir / f"{stem}.svg").write_text(svg, encoding="utf-8")
                argv += ["--figures-dir", str(fdir)]
            if summary is not None:
                sp = dp / "summary.json"
                sp.write_text(json.dumps({"rows": summary}), encoding="utf-8")
                argv += ["--summary", str(sp)]
            proc = subprocess.run(argv, capture_output=True, text=True)
            md = out.read_text(encoding="utf-8") if out.is_file() else ""
            return proc.returncode, proc.stderr, md


class TestStructure(ReportCase):
    def test_sections_in_order(self) -> None:
        rc, stderr, md = self.run_report(findings=FINDINGS)
        self.assertEqual(rc, 0, msg=stderr)
        self.assertTrue(md.startswith("# keeper-core evolutionary analysis"))
        # The eleven fixed sections appear, in order.
        order = [
            "## 1. Executive summary",
            "## 2. Hotspots",
            "## 3. Complexity trend",
            "## 4. Change coupling",
            "## 5. Knowledge & ownership",
            "## 6. Fragmentation",
            "## 7. Communication & Conway",
            "## 8. Code age",
            "## 9. Churn",
            "## 10. Commit vocabulary",
            "## 11. Recommended actions",
        ]
        positions = [md.find(h) for h in order]
        self.assertNotIn(-1, positions, msg="a section heading is missing")
        self.assertEqual(positions, sorted(positions), msg="sections out of order")

    def test_window_and_findings_slotted(self) -> None:
        _rc, _stderr, md = self.run_report(findings=FINDINGS)
        self.assertIn("last 12 months", md)
        # The hotspots prose lands under the hotspots heading, not elsewhere.
        after = md.split("## 2. Hotspots", 1)[1]
        self.assertIn("EmployeeStatusService.cs is the real offender.", after)


class TestFigures(ReportCase):
    def test_inline_svg_no_external_ref(self) -> None:
        rc, stderr, md = self.run_report(findings=FINDINGS, figures={"hotspots": SVG})
        self.assertEqual(rc, 0, msg=stderr)
        self.assertIn("<svg", md)  # embedded inline
        self.assertNotIn("<?xml", md)  # prolog stripped
        self.assertNotIn("![", md)  # no external image reference
        self.assertIn("1 figures", stderr)


class TestGuardrails(ReportCase):
    def test_guardrails_always_present(self) -> None:
        # Even with no findings at all, the social disclaimer and the word-cloud
        # heuristic label are emitted.
        _rc, _stderr, md = self.run_report(findings="")
        self.assertIn("not a productivity ranking", md)
        self.assertIn("Heuristic only", md)
        self.assertIn("Aggregate authors to teams", md)


class TestPlaceholders(ReportCase):
    def test_missing_findings_placeholder(self) -> None:
        rc, _stderr, md = self.run_report(findings=FINDINGS)
        self.assertEqual(rc, 0)
        # No 'fractal' block was provided -> its section shows the placeholder.
        after = md.split("## 6. Fragmentation", 1)[1].split("## 7.", 1)[0]
        self.assertIn("No finding provided", after)


class TestSummaryTiles(ReportCase):
    def test_summary_tiles_rendered(self) -> None:
        rc, stderr, md = self.run_report(
            findings=FINDINGS,
            summary=[
                {"statistic": "number-of-commits", "value": 9752},
                {"statistic": "number-of-authors", "value": 34},
            ],
        )
        self.assertEqual(rc, 0, msg=stderr)
        self.assertIn("| commits | 9,752 |", md)
        self.assertIn("| authors | 34 |", md)


class TestTitle(ReportCase):
    def test_h1_is_sole_title_and_title_block_ignored(self) -> None:
        # A `## title` block must not override or blank the `# H1` title.
        findings = "# Real Title\n## title\nBogus Override\n## hotspots\nX is hot.\n"
        rc, stderr, md = self.run_report(findings=findings)
        self.assertEqual(rc, 0, msg=stderr)
        self.assertTrue(md.startswith("# Real Title"))
        self.assertNotIn("Bogus Override", md)
        self.assertNotIn("Codebase evolutionary analysis", md)  # generic fallback

    def test_title_block_without_h1_falls_back_not_uses_block(self) -> None:
        # With only a `## title` block and no H1, the block is ignored and the
        # generic fallback title is used (the block never becomes the title).
        rc, stderr, md = self.run_report(findings="## title\nShould Be Ignored\n")
        self.assertEqual(rc, 0, msg=stderr)
        self.assertNotIn("Should Be Ignored", md)
        self.assertTrue(md.startswith("# Codebase evolutionary analysis"))


class TestEmpty(ReportCase):
    def test_no_inputs_exit_3(self) -> None:
        rc, stderr, _md = self.run_report()
        self.assertEqual(rc, EXIT_EMPTY, msg=stderr)


if __name__ == "__main__":
    unittest.main()
