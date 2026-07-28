# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Behavioral tests for flint_spec.py, through its emitted JSON.

flint_spec.py is stdlib-only; its observable output is the ChartAssemblyInput
JSON on stdout (or -o), the trailing summary on stderr, and the exit code.
Tests run it as a subprocess and assert on those. The channel-table tests
import the module and assert SELF-CONSISTENCY only: parity with Flint is
deliberately not asserted here (no Deno test lane); it is checked at render
time and by the upgrade checklist.

The six override fixtures are recorded expected specs; each was compiled
against flint-chart@0.4.0 under Deno when this suite was written, which is the
check that catches a channel a template silently discards.

Run: `uv run flint_spec_test.py` from the scripts directory.
"""

from __future__ import annotations

import json
import subprocess
import sys
import unittest
from pathlib import Path
from typing import Any

SCRIPT = Path(__file__).with_name("flint_spec.py")

EXIT_USAGE = 2
EXIT_EMPTY = 3


def envelope(
    analysis: str,
    semantics: dict[str, str],
    rows: list[dict[str, Any]],
    **extra: Any,
) -> dict[str, Any]:
    doc: dict[str, Any] = {
        "schema_version": 1,
        "ok": True,
        "analysis": analysis,
        "shape": "table",
        "semantics": semantics,
        "row_count": len(rows),
        "rows": rows,
    }
    doc.update(extra)
    return doc


class FlintSpecCase(unittest.TestCase):
    def run_spec(
        self, stdin_text: str, *args: str
    ) -> tuple[int, dict[str, Any] | None, str]:
        proc = subprocess.run(
            [sys.executable, str(SCRIPT), *args],
            input=stdin_text,
            capture_output=True,
            text=True,
        )
        spec = json.loads(proc.stdout) if proc.returncode == 0 else None
        return proc.returncode, spec, proc.stderr

    def run_doc(
        self, doc: dict[str, Any], *args: str
    ) -> tuple[int, dict[str, Any] | None, str]:
        return self.run_spec(json.dumps(doc), *args)


class TestRejection(FlintSpecCase):
    def test_invalid_json_exits_2(self) -> None:
        rc, _, stderr = self.run_spec("not json")
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)
        self.assertIn("JSON", stderr)

    def test_bare_rows_array_exits_2(self) -> None:
        rc, _, stderr = self.run_spec(json.dumps([{"entity": "a", "n_revs": 1}]))
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)

    def test_error_envelope_exits_2(self) -> None:
        doc = {"schema_version": 1, "ok": False, "error": {"code": "empty_log"}}
        rc, _, stderr = self.run_doc(doc)
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)
        self.assertIn("error envelope", stderr)

    def test_text_shape_exits_2(self) -> None:
        doc = {"schema_version": 1, "ok": True, "shape": "text"}
        rc, _, stderr = self.run_doc(doc)
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)

    def test_schema_output_exits_2(self) -> None:
        doc = {
            "schema_version": 1,
            "ok": True,
            "command": "coupling",
            "shape": "table",
            "row_schema": [{"name": "entity", "semantic": "filepath"}],
        }
        rc, _, stderr = self.run_doc(doc)
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)
        self.assertIn("schema", stderr)

    def test_schema_list_output_exits_2(self) -> None:
        doc = {"schema_version": 1, "ok": True, "commands": []}
        rc, _, stderr = self.run_doc(doc)
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)

    def test_parse_exits_2(self) -> None:
        doc = envelope(
            "parse",
            {"entity": "filepath", "rev": "commit_id"},
            [{"entity": "a.go", "rev": "abc"}],
        )
        rc, _, stderr = self.run_doc(doc)
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)
        self.assertIn("parse", stderr)

    def test_empty_rows_exit_3(self) -> None:
        doc = envelope("revisions", {"entity": "filepath", "n_revs": "count"}, [])
        rc, _, stderr = self.run_doc(doc)
        self.assertEqual(rc, EXIT_EMPTY, msg=stderr)


class TestGenericPolicy(FlintSpecCase):
    def test_one_dimension_is_a_bar_chart(self) -> None:
        doc = envelope(
            "revisions",
            {"entity": "filepath", "n_revs": "count"},
            [{"entity": "a.go", "n_revs": 7}, {"entity": "b.go", "n_revs": 3}],
        )
        rc, spec, stderr = self.run_doc(doc)
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(spec["data"]["values"], doc["rows"])
        self.assertEqual(spec["semantic_types"], {"entity": "Name", "n_revs": "Count"})
        self.assertEqual(spec["chart_spec"]["chartType"], "Bar Chart")
        self.assertEqual(
            spec["chart_spec"]["encodings"],
            {"x": {"field": "entity"}, "y": {"field": "n_revs"}},
        )
        self.assertIn("Bar Chart", stderr)
        self.assertIn("vegalite", stderr)

    def test_intensive_measure_outranks_additive_and_second_dim_colors(self) -> None:
        doc = envelope(
            "main-developer",
            {
                "entity": "filepath",
                "main_dev": "person",
                "added": "loc",
                "total_added": "loc",
                "ownership": "ratio",
            },
            [
                {
                    "entity": "a.go",
                    "main_dev": "alice",
                    "added": 10,
                    "total_added": 12,
                    "ownership": 0.83,
                }
            ],
        )
        rc, spec, stderr = self.run_doc(doc)
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(spec["chart_spec"]["chartType"], "Bar Chart")
        self.assertEqual(
            spec["chart_spec"]["encodings"],
            {
                "x": {"field": "entity"},
                "y": {"field": "ownership"},
                "color": {"field": "main_dev"},
            },
        )

    def test_edge_table_is_a_network_graph(self) -> None:
        # Not an override: a synthetic analysis with two same-semantic
        # dimensions must reach Network Graph through the generic rule.
        doc = envelope(
            "future-pairs",
            {"src": "person", "dst": "person", "shared": "count"},
            [{"src": "alice", "dst": "bob", "shared": 4}],
        )
        rc, spec, stderr = self.run_doc(doc)
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(spec["chart_spec"]["chartType"], "Network Graph")
        self.assertEqual(
            spec["chart_spec"]["encodings"],
            {
                "x": {"field": "src"},
                "y": {"field": "dst"},
                "size": {"field": "shared"},
            },
        )
        self.assertIn("echarts", stderr)

    def test_date_dimension_takes_the_x_axis(self) -> None:
        doc = envelope(
            "future-timeline",
            {"kind": "label", "date": "date", "commits": "count"},
            [{"kind": "fix", "date": "2026-01-02", "commits": 3}],
        )
        rc, spec, stderr = self.run_doc(doc)
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(
            spec["chart_spec"]["encodings"],
            {
                "x": {"field": "date"},
                "y": {"field": "commits"},
                "color": {"field": "kind"},
            },
        )

    def test_unknown_semantic_exits_2(self) -> None:
        doc = envelope(
            "future-metric",
            {"entity": "filepath", "score": "grade"},
            [{"entity": "a.go", "score": "A"}],
        )
        rc, _, stderr = self.run_doc(doc)
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)
        self.assertIn("grade", stderr)


# One small envelope per override row (plan 3.4). The expected specs below are
# recorded fixtures: each compiled under deno run npm:flint-chart@0.4.0 with
# every encoding channel surviving into the assembled output.
CODE_AGE = envelope(
    "code-age",
    {"entity": "filepath", "age_months": "duration_months"},
    [
        {"entity": "a.go", "age_months": 14},
        {"entity": "b.go", "age_months": 2},
        {"entity": "c.go", "age_months": 2},
    ],
)
SUMMARY = envelope(
    "summary",
    {"statistic": "label", "value": "count"},
    [
        {"statistic": "number-of-commits", "value": 12},
        {"statistic": "number-of-entities", "value": 5},
    ],
)
AUTHORS = envelope(
    "authors",
    {"entity": "filepath", "n_authors": "count", "n_revs": "count"},
    [
        {"entity": "a.go", "n_authors": 3, "n_revs": 20},
        {"entity": "b.go", "n_authors": 1, "n_revs": 4},
    ],
)
COUPLING = envelope(
    "coupling",
    {
        "entity": "filepath",
        "coupled": "filepath",
        "degree": "percentage",
        "average_revs": "count",
    },
    [
        {"entity": "a.go", "coupled": "b.go", "degree": 78, "average_revs": 44},
        {"entity": "a.go", "coupled": "c.go", "degree": 62, "average_revs": 45},
    ],
)
COMMUNICATION = envelope(
    "communication",
    {
        "author": "person",
        "peer": "person",
        "shared": "count",
        "average": "count",
        "strength": "percentage",
    },
    [{"author": "alice", "peer": "bob", "shared": 8, "average": 12, "strength": 66}],
)
ABSOLUTE_CHURN = envelope(
    "absolute-churn",
    {"date": "date", "added": "loc", "deleted": "loc", "commits": "count"},
    [
        {"date": "2026-01-02", "added": 120, "deleted": 30, "commits": 4},
        {"date": "2026-01-03", "added": 20, "deleted": 90, "commits": 2},
    ],
)


def expected_spec(
    doc: dict[str, Any],
    semantic_types: dict[str, Any],
    chart_type: str,
    encodings: dict[str, Any],
) -> dict[str, Any]:
    return {
        "data": {"values": doc["rows"]},
        "semantic_types": semantic_types,
        "chart_spec": {"chartType": chart_type, "encodings": encodings},
    }


class TestOverrideFixtures(FlintSpecCase):
    def assert_spec(
        self, doc: dict[str, Any], expected: dict[str, Any], backend: str
    ) -> None:
        rc, spec, stderr = self.run_doc(doc)
        self.assertEqual(rc, 0, msg=stderr)
        self.assertEqual(spec, expected)
        self.assertIn(backend, stderr)

    def test_code_age_histogram(self) -> None:
        self.assert_spec(
            CODE_AGE,
            expected_spec(
                CODE_AGE,
                {
                    "entity": "Name",
                    "age_months": {"semanticType": "Duration", "unit": "months"},
                },
                "Histogram",
                {"x": {"field": "age_months"}},
            ),
            "vegalite",
        )

    def test_summary_kpi_card(self) -> None:
        self.assert_spec(
            SUMMARY,
            expected_spec(
                SUMMARY,
                {"statistic": "Category", "value": "Count"},
                "KPI Card",
                {"metric": {"field": "statistic"}, "value": {"field": "value"}},
            ),
            "vegalite",
        )

    def test_authors_scatter_leaves_entity_unbound(self) -> None:
        self.assert_spec(
            AUTHORS,
            expected_spec(
                AUTHORS,
                {"entity": "Name", "n_authors": "Count", "n_revs": "Count"},
                "Scatter Plot",
                {"x": {"field": "n_revs"}, "y": {"field": "n_authors"}},
            ),
            "vegalite",
        )

    def test_coupling_network_graph(self) -> None:
        self.assert_spec(
            COUPLING,
            expected_spec(
                COUPLING,
                {
                    "entity": "Name",
                    "coupled": "Name",
                    "degree": "Percentage",
                    "average_revs": "Count",
                },
                "Network Graph",
                {
                    "x": {"field": "entity"},
                    "y": {"field": "coupled"},
                    "size": {"field": "degree"},
                },
            ),
            "echarts",
        )

    def test_communication_network_graph(self) -> None:
        self.assert_spec(
            COMMUNICATION,
            expected_spec(
                COMMUNICATION,
                {
                    "author": "Name",
                    "peer": "Name",
                    "shared": "Count",
                    "average": "Count",
                    "strength": "Percentage",
                },
                "Network Graph",
                {
                    "x": {"field": "author"},
                    "y": {"field": "peer"},
                    "size": {"field": "strength"},
                },
            ),
            "echarts",
        )

    def test_absolute_churn_stacked_bar_folds_added_deleted(self) -> None:
        self.assert_spec(
            ABSOLUTE_CHURN,
            expected_spec(
                ABSOLUTE_CHURN,
                {
                    "date": "Date",
                    "added": {"semanticType": "Quantity", "unit": "lines"},
                    "deleted": {"semanticType": "Quantity", "unit": "lines"},
                    "commits": "Count",
                },
                "Stacked Bar Chart",
                # The y array is the documented reshape and must stay BARE:
                # the {"field": [...]} object form throws at assembly.
                {"x": {"field": "date"}, "y": ["added", "deleted"]},
            ),
            "vegalite",
        )


class TestSemanticAnnotationRegression(FlintSpecCase):
    """percentage bare, ratio annotated: the asymmetry is load-bearing
    (plan 3.2.1). Annotating percentage causes a 100x rendering error on
    filtered tables; ratio needs intrinsicDomain to render 0.34 as 34%."""

    def test_percentage_is_a_bare_string(self) -> None:
        rc, spec, stderr = self.run_doc(COUPLING)
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(spec["semantic_types"]["degree"], "Percentage")

    def test_ratio_carries_intrinsic_domain(self) -> None:
        doc = envelope(
            "fragmentation",
            {"entity": "filepath", "fractal_value": "ratio", "total_revs": "count"},
            [{"entity": "a.go", "fractal_value": 0.42, "total_revs": 9}],
        )
        rc, spec, stderr = self.run_doc(doc)
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(
            spec["semantic_types"]["fractal_value"],
            {"semanticType": "Percentage", "intrinsicDomain": [0, 1]},
        )


ENTITY_OWNERSHIP = envelope(
    "entity-ownership",
    {"entity": "filepath", "author": "person", "added": "loc", "deleted": "loc"},
    [
        {"entity": "a.go", "author": "alice", "added": 30, "deleted": 4},
        {"entity": "a.go", "author": "bob", "added": 10, "deleted": 2},
    ],
)
ENTITY_EFFORT = envelope(
    "entity-effort",
    {
        "entity": "filepath",
        "author": "person",
        "author_revs": "count",
        "total_revs": "count",
    },
    [{"entity": "a.go", "author": "alice", "author_revs": 3, "total_revs": 5}],
)


class TestChartTypeFlag(FlintSpecCase):
    def test_lollipop_for_a_ranked_list(self) -> None:
        doc = envelope(
            "revisions",
            {"entity": "filepath", "n_revs": "count"},
            [{"entity": "a.go", "n_revs": 7}],
        )
        rc, spec, stderr = self.run_doc(doc, "--chart-type", "Lollipop Chart")
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(spec["chart_spec"]["chartType"], "Lollipop Chart")
        self.assertEqual(
            spec["chart_spec"]["encodings"],
            {"x": {"field": "entity"}, "y": {"field": "n_revs"}},
        )
        self.assertIn("vegalite", stderr)

    def test_coupling_heatmap_prefers_vegalite(self) -> None:
        rc, spec, stderr = self.run_doc(COUPLING, "--chart-type", "Heatmap")
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(
            spec["chart_spec"]["encodings"],
            {
                "x": {"field": "entity"},
                "y": {"field": "coupled"},
                "color": {"field": "degree"},
            },
        )
        self.assertIn("vegalite", stderr)

    def test_coupling_heatmap_on_echarts(self) -> None:
        rc, spec, stderr = self.run_doc(
            COUPLING, "--chart-type", "Heatmap", "--backend", "echarts"
        )
        self.assertEqual(rc, 0, msg=stderr)
        self.assertIn("echarts", stderr)

    def test_ownership_stacked_bar_normalizes(self) -> None:
        rc, spec, stderr = self.run_doc(
            ENTITY_OWNERSHIP, "--chart-type", "Stacked Bar Chart"
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(
            spec["chart_spec"]["encodings"],
            {
                "x": {"field": "entity"},
                "y": {"field": "added"},
                "color": {"field": "author"},
            },
        )
        self.assertEqual(
            spec["chart_spec"]["chartProperties"], {"stackMode": "normalize"}
        )

    def test_effort_stacked_bar_binds_component_not_total(self) -> None:
        # author_revs plus total_revs is Flint's total-mixed-with-components
        # hazard: only the component may reach a stacked channel.
        rc, spec, stderr = self.run_doc(
            ENTITY_EFFORT, "--chart-type", "Stacked Bar Chart"
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(spec["chart_spec"]["encodings"]["y"], {"field": "author_revs"})
        self.assertNotIn("total_revs", json.dumps(spec["chart_spec"]["encodings"]))

    def test_churn_stacked_bar_keeps_the_override_fold(self) -> None:
        rc, spec, stderr = self.run_doc(
            ABSOLUTE_CHURN, "--chart-type", "Stacked Bar Chart"
        )
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(spec["chart_spec"]["encodings"]["y"], ["added", "deleted"])

    def test_unknown_chart_type_lists_valid_names(self) -> None:
        rc, _, stderr = self.run_doc(COUPLING, "--chart-type", "Pie Chart")
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)
        self.assertIn("Network Graph", stderr)

    def test_unfitting_channels_exit_2(self) -> None:
        # F12 layer 1: an edge-table binding wants x, y and a weight channel;
        # Histogram declares no y, so the spec must be refused, not emitted
        # for Flint to silently mis-assemble.
        rc, _, stderr = self.run_doc(COUPLING, "--chart-type", "Histogram")
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)
        self.assertIn("y", stderr)
        self.assertIn("Histogram", stderr)


class TestBackendFlag(FlintSpecCase):
    def test_vegalite_edge_table_falls_back_to_heatmap(self) -> None:
        rc, spec, stderr = self.run_doc(COUPLING, "--backend", "vegalite")
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(spec["chart_spec"]["chartType"], "Heatmap")
        self.assertIn("vegalite", stderr)

    def test_echarts_without_chart_type_on_plain_table_exits_2(self) -> None:
        doc = envelope(
            "revisions",
            {"entity": "filepath", "n_revs": "count"},
            [{"entity": "a.go", "n_revs": 7}],
        )
        rc, _, stderr = self.run_doc(doc, "--backend", "echarts")
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)
        self.assertIn("--chart-type", stderr)


# Column inventory of the 17 table-shaped analyses (plan 1.2): every one must
# yield a valid spec via the generic policy plus the 6 overrides.
ALL_ANALYSES: dict[str, dict[str, str]] = {
    "revisions": {"entity": "filepath", "n_revs": "count"},
    "code-age": {"entity": "filepath", "age_months": "duration_months"},
    "sum-of-coupling": {"entity": "filepath", "soc": "count"},
    "coupling": {
        "entity": "filepath",
        "coupled": "filepath",
        "degree": "percentage",
        "average_revs": "count",
        "first_entity_revisions": "count",
        "second_entity_revisions": "count",
        "shared_revisions": "count",
    },
    "communication": {
        "author": "person",
        "peer": "person",
        "shared": "count",
        "average": "count",
        "strength": "percentage",
    },
    "absolute-churn": {
        "date": "date",
        "added": "loc",
        "deleted": "loc",
        "commits": "count",
    },
    "author-churn": {
        "author": "person",
        "added": "loc",
        "deleted": "loc",
        "commits": "count",
    },
    "entity-churn": {
        "entity": "filepath",
        "added": "loc",
        "deleted": "loc",
        "commits": "count",
    },
    "entity-ownership": {
        "entity": "filepath",
        "author": "person",
        "added": "loc",
        "deleted": "loc",
    },
    "main-developer": {
        "entity": "filepath",
        "main_dev": "person",
        "added": "loc",
        "total_added": "loc",
        "ownership": "ratio",
    },
    "entity-effort": {
        "entity": "filepath",
        "author": "person",
        "author_revs": "count",
        "total_revs": "count",
    },
    "fragmentation": {
        "entity": "filepath",
        "fractal_value": "ratio",
        "total_revs": "count",
    },
    "summary": {"statistic": "label", "value": "count"},
    "messages": {"entity": "filepath", "matches": "count"},
    "authors": {"entity": "filepath", "n_authors": "count", "n_revs": "count"},
    "refactoring-main-developer": {
        "entity": "filepath",
        "main_dev": "person",
        "removed": "loc",
        "total_removed": "loc",
        "ownership": "ratio",
    },
    "main-developer-by-revisions": {
        "entity": "filepath",
        "main_dev": "person",
        "added": "count",
        "total_added": "count",
        "ownership": "ratio",
    },
}

SAMPLE_VALUES: dict[str, Any] = {
    "filepath": "src/a.go",
    "person": "alice",
    "date": "2026-01-02",
    "label": "number-of-commits",
    "count": 7,
    "loc": 120,
    "percentage": 62,
    "ratio": 0.62,
    "duration_months": 14,
}


class TestAllTableAnalyses(FlintSpecCase):
    def test_every_analysis_emits_a_spec(self) -> None:
        for analysis, semantics in ALL_ANALYSES.items():
            with self.subTest(analysis=analysis):
                row = {col: SAMPLE_VALUES[sem] for col, sem in semantics.items()}
                rc, spec, stderr = self.run_doc(envelope(analysis, semantics, [row]))
                self.assertEqual(rc, 0, msg=stderr)
                assert spec is not None
                self.assertIn("chartType", spec["chart_spec"])
                self.assertTrue(spec["chart_spec"]["encodings"])
                self.assertEqual(set(spec["semantic_types"]), set(semantics))


class TestSchemaFlag(FlintSpecCase):
    SCHEMA = {
        "schema_version": 1,
        "ok": True,
        "command": "revisions",
        "row_schema": [
            {"name": "entity", "type": "string", "desc": "module path"},
            {
                "name": "n_revs",
                "type": "int",
                "desc": "number of distinct revisions",
            },
            {"name": "unrelated", "type": "int", "desc": "not in the envelope"},
        ],
    }
    DOC = envelope(
        "revisions",
        {"entity": "filepath", "n_revs": "count"},
        [{"entity": "a.go", "n_revs": 7}],
    )

    def test_schema_populates_field_display_names(self) -> None:
        import tempfile

        with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as f:
            json.dump(self.SCHEMA, f)
        rc, spec, stderr = self.run_doc(self.DOC, "--schema", f.name)
        Path(f.name).unlink()
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(
            spec["field_display_names"],
            {"entity": "module path", "n_revs": "number of distinct revisions"},
        )

    def test_absent_schema_degrades_to_raw_names(self) -> None:
        rc, spec, stderr = self.run_doc(self.DOC)
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertNotIn("field_display_names", spec)

    def test_unreadable_schema_exits_2(self) -> None:
        rc, _, stderr = self.run_doc(self.DOC, "--schema", "/nonexistent.json")
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)

    def test_malformed_schema_exits_2(self) -> None:
        import tempfile

        with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as f:
            f.write("{no row_schema}")
        rc, _, stderr = self.run_doc(self.DOC, "--schema", f.name)
        Path(f.name).unlink()
        self.assertEqual(rc, EXIT_USAGE, msg=stderr)


class TestSizeAndOut(FlintSpecCase):
    DOC = envelope(
        "revisions",
        {"entity": "filepath", "n_revs": "count"},
        [{"entity": "a.go", "n_revs": 7}],
    )

    def test_width_and_height_set_base_size(self) -> None:
        rc, spec, stderr = self.run_doc(self.DOC, "--width", "640", "--height", "480")
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(spec["chart_spec"]["baseSize"], {"width": 640, "height": 480})

    def test_one_dimension_defaults_the_other(self) -> None:
        rc, spec, stderr = self.run_doc(self.DOC, "--width", "640")
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertEqual(spec["chart_spec"]["baseSize"], {"width": 640, "height": 320})

    def test_no_size_flags_omit_base_size(self) -> None:
        rc, spec, stderr = self.run_doc(self.DOC)
        self.assertEqual(rc, 0, msg=stderr)
        assert spec is not None
        self.assertNotIn("baseSize", spec["chart_spec"])

    def test_out_writes_file_and_keeps_stdout_quiet(self) -> None:
        import tempfile

        with tempfile.TemporaryDirectory() as d:
            out = Path(d) / "spec.json"
            proc = subprocess.run(
                [sys.executable, str(SCRIPT), "-o", str(out)],
                input=json.dumps(self.DOC),
                capture_output=True,
                text=True,
            )
            self.assertEqual(proc.returncode, 0, msg=proc.stderr)
            self.assertEqual(proc.stdout, "")
            spec = json.loads(out.read_text(encoding="utf-8"))
            self.assertEqual(spec["chart_spec"]["chartType"], "Bar Chart")


class TestChannelTableSelfConsistency(unittest.TestCase):
    """Parity with Flint is deliberately NOT asserted (no Deno test lane);
    these assert only that the script agrees with itself (F12/Q11)."""

    def setUp(self) -> None:
        sys.path.insert(0, str(SCRIPT.parent))
        import flint_spec

        self.mod = flint_spec

    def tearDown(self) -> None:
        sys.path.remove(str(SCRIPT.parent))

    def test_every_override_chart_type_has_a_channel_entry(self) -> None:
        for analysis, (backend, chart_type, enc) in self.mod.OVERRIDES.items():
            with self.subTest(analysis=analysis):
                self.assertIn(chart_type, self.mod.CHANNELS[backend])
                self.assertLessEqual(set(enc), self.mod.CHANNELS[backend][chart_type])

    def test_vocabulary_maps_agree_on_the_twelve_semantics(self) -> None:
        self.assertEqual(set(self.mod.SEMANTIC_TYPES), set(self.mod.AGGREGATION_ROLES))
        self.assertEqual(len(self.mod.SEMANTIC_TYPES), 12)


if __name__ == "__main__":
    unittest.main()
