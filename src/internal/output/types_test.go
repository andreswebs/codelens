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
		RowCount:      0,
		Rows:          []any{},
	}

	got := marshalString(t, r)

	for _, want := range []string{
		`"schema_version":1`,
		`"ok":true`,
		`"rows":[]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("marshaled Result missing %q\ngot: %s", want, got)
		}
	}
	for _, absent := range []string{`"total_count"`, `"truncated"`} {
		if strings.Contains(got, absent) {
			t.Errorf("marshaled Result should omit %q when zero/false\ngot: %s", absent, got)
		}
	}
}

func TestResult_TruncatedShape(t *testing.T) {
	r := output.Result{
		SchemaVersion: output.SchemaVersion,
		OK:            true,
		Analysis:      "revisions",
		RowCount:      10,
		TotalCount:    812,
		Truncated:     true,
		Rows:          []any{},
	}

	got := marshalString(t, r)

	for _, want := range []string{`"total_count":812`, `"truncated":true`} {
		if !strings.Contains(got, want) {
			t.Errorf("marshaled Result missing %q\ngot: %s", want, got)
		}
	}
}
