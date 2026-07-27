package output_test

import (
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/output"
)

func TestResult_JSONShape(t *testing.T) {
	r := output.Result{
		SchemaVersion: output.SchemaVersion,
		OK:            true,
		Analysis:      "authors",
		Shape:         "table",
		RowCount:      0,
		Payload:       []any{},
	}

	got := marshalString(t, r)

	for _, want := range []string{
		`"schema_version":1`,
		`"ok":true`,
		`"shape":"table"`,
		`"rows":[]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("marshaled Result missing %q\ngot: %s", want, got)
		}
	}
	for _, absent := range []string{`"total_count"`, `"truncated"`, `"params"`} {
		if strings.Contains(got, absent) {
			t.Errorf("marshaled Result should omit %q when zero/false/nil\ngot: %s", absent, got)
		}
	}
}

// TestResult_KeyOrder pins the exact key order on raw bytes: order is the golden
// contract (ADR 0007), not merely key presence. The envelope must lead with the
// metadata and end with the payload key so `head` on a large result shows the
// descriptive fields.
func TestResult_KeyOrder(t *testing.T) {
	r := output.Result{
		SchemaVersion: output.SchemaVersion,
		OK:            true,
		Analysis:      "authors",
		Shape:         "table",
		Params:        map[string]any{"min_revs": 5},
		RowCount:      1,
		Payload:       []any{map[string]any{"entity": "a.go"}},
	}

	got := marshalString(t, r)

	const wantPrefix = `{"schema_version":1,"ok":true,"analysis":"authors","shape":"table",`
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("key order prefix mismatch\n got: %s\nwant prefix: %s", got, wantPrefix)
	}
	if !strings.HasSuffix(got, `"rows":[{"entity":"a.go"}]}`) {
		t.Errorf("payload key must be last\ngot: %s", got)
	}
	// params precedes row_count, which precedes the payload.
	if got := indexOrder(got, `"params"`, `"row_count"`, `"rows"`); got != true {
		t.Errorf("expected params < row_count < rows in key order\ngot: false")
	}
}

// TestResult_SemanticsAlwaysPresent pins that semantics marshals as {} rather
// than null or an absent key when the map is nil, so a consumer reads it
// unconditionally, and lands immediately after shape.
func TestResult_SemanticsAlwaysPresent(t *testing.T) {
	r := output.Result{SchemaVersion: 1, OK: true, Analysis: "authors", Shape: "table", Payload: []any{}}
	got := marshalString(t, r)
	if !strings.Contains(got, `"shape":"table","semantics":{}`) {
		t.Errorf("nil semantics must marshal as {} right after shape\ngot: %s", got)
	}
	if strings.Contains(got, `"transforms"`) {
		t.Errorf("nil transforms must be omitted\ngot: %s", got)
	}
}

// TestResult_TransformsWhenPresent pins that a populated transforms record is
// written between semantics and params.
func TestResult_TransformsWhenPresent(t *testing.T) {
	r := output.Result{
		SchemaVersion: 1, OK: true, Analysis: "authors", Shape: "table",
		Semantics:  map[string]string{"entity": "label"},
		Transforms: map[string]any{"group": true},
		Payload:    []any{},
	}
	got := marshalString(t, r)
	if !indexOrder(got, `"semantics"`, `"transforms"`, `"rows"`) {
		t.Errorf("expected semantics < transforms < rows\ngot: %s", got)
	}
}

// indexOrder reports whether the given substrings appear in strictly increasing
// index order in s.
func indexOrder(s string, subs ...string) bool {
	last := -1
	for _, sub := range subs {
		i := strings.Index(s, sub)
		if i < 0 || i <= last {
			return false
		}
		last = i
	}
	return true
}

func TestResult_TruncatedShape(t *testing.T) {
	r := output.Result{
		SchemaVersion: output.SchemaVersion,
		OK:            true,
		Analysis:      "revisions",
		Shape:         "table",
		RowCount:      10,
		TotalCount:    812,
		Truncated:     true,
		Payload:       []any{},
	}

	got := marshalString(t, r)

	for _, want := range []string{`"total_count":812`, `"truncated":true`} {
		if !strings.Contains(got, want) {
			t.Errorf("marshaled Result missing %q\ngot: %s", want, got)
		}
	}
	// total_count precedes truncated precedes rows.
	if !indexOrder(got, `"row_count"`, `"total_count"`, `"truncated"`, `"rows"`) {
		t.Errorf("truncation metadata out of contract order\ngot: %s", got)
	}
}
