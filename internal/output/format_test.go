package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/output"
)

// nonEmptyLines splits s on newlines and drops a trailing empty line, so line
// counts are not thrown off by a final record terminator.
func nonEmptyLines(s string) []string {
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

// fmtRow is a fixed row shape for format tests: it mirrors the authors
// analysis's row so column names and json keys are realistic.
type fmtRow struct {
	Entity   string `json:"entity"`
	NAuthors int    `json:"n_authors"`
	NRevs    int    `json:"n_revs"`
}

// fmtColumns is the schema column order for fmtRow, as snake_case json keys.
var fmtColumns = []string{"entity", "n_authors", "n_revs"}

// fmtResult builds a fixed two-row result for format tests.
func fmtResult() output.Result {
	return output.Result{
		SchemaVersion: output.SchemaVersion,
		OK:            true,
		Analysis:      "authors",
		RowCount:      2,
		Rows: []fmtRow{
			{Entity: "A.go", NAuthors: 3, NRevs: 10},
			{Entity: "B.go", NAuthors: 2, NRevs: 7},
		},
	}
}

func TestFormat_JSON_Default(t *testing.T) {
	var buf bytes.Buffer
	if err := output.Emit(&buf, "json", fmtResult(), fmtColumns, ""); err != nil {
		t.Fatalf("Emit json: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		`"schema_version":1`,
		`"ok":true`,
		`"analysis":"authors"`,
		`"entity":"A.go"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("json output missing %q\ngot: %s", want, got)
		}
	}
}

func TestFormat_NDJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := output.Emit(&buf, "ndjson", fmtResult(), fmtColumns, ""); err != nil {
		t.Fatalf("Emit ndjson: %v", err)
	}

	lines := nonEmptyLines(buf.String())
	if len(lines) != 2 {
		t.Fatalf("ndjson line count = %d, want 2\ngot: %s", len(lines), buf.String())
	}

	for i, line := range lines {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("ndjson line %d is not valid JSON: %v\nline: %s", i, err, line)
		}
		if _, ok := row["entity"]; !ok {
			t.Errorf("ndjson line %d missing entity: %s", i, line)
		}
		if _, ok := row["schema_version"]; ok {
			t.Errorf("ndjson line %d should carry no envelope keys: %s", i, line)
		}
	}

	if !strings.Contains(lines[0], `"entity":"A.go"`) {
		t.Errorf("ndjson first row should be A.go (log order preserved): %s", lines[0])
	}
}

func TestFormat_CSV_Header_Kebab(t *testing.T) {
	var buf bytes.Buffer
	if err := output.Emit(&buf, "csv", fmtResult(), fmtColumns, ""); err != nil {
		t.Fatalf("Emit csv: %v", err)
	}

	lines := nonEmptyLines(buf.String())
	if len(lines) != 3 {
		t.Fatalf("csv line count = %d, want 3 (header + 2 rows)\ngot: %s", len(lines), buf.String())
	}

	if lines[0] != "entity,n-authors,n-revs" {
		t.Errorf("csv header = %q, want %q", lines[0], "entity,n-authors,n-revs")
	}
	if lines[1] != "A.go,3,10" {
		t.Errorf("csv row 1 = %q, want %q", lines[1], "A.go,3,10")
	}
	if lines[2] != "B.go,2,7" {
		t.Errorf("csv row 2 = %q, want %q", lines[2], "B.go,2,7")
	}
}

func TestFormat_Table_Aligned(t *testing.T) {
	var buf bytes.Buffer
	if err := output.Emit(&buf, "table", fmtResult(), fmtColumns, ""); err != nil {
		t.Fatalf("Emit table: %v", err)
	}

	out := buf.String()
	lines := nonEmptyLines(out)
	if len(lines) != 3 {
		t.Fatalf("table line count = %d, want 3 (header + 2 rows)\ngot: %s", len(lines), out)
	}

	for _, col := range fmtColumns {
		if !strings.Contains(lines[0], col) {
			t.Errorf("table header missing column %q\nheader: %q", col, lines[0])
		}
	}
	for _, val := range []string{"A.go", "10", "B.go", "7"} {
		if !strings.Contains(out, val) {
			t.Errorf("table output missing value %q\ngot: %s", val, out)
		}
	}

	// Columns are padded to a common width, so the entity column of the two
	// data rows starts at the same offset.
	off1 := strings.Index(lines[1], "3")
	off2 := strings.Index(lines[2], "2")
	if off1 <= 0 || off1 != off2 {
		t.Errorf("table columns are not aligned: n_authors offsets %d vs %d\n%s", off1, off2, out)
	}
}

func TestFormat_Fields_JSONOnly(t *testing.T) {
	// json honours --fields: rows keep only entity.
	var jsonBuf bytes.Buffer
	if err := output.Emit(&jsonBuf, "json", fmtResult(), fmtColumns, "rows.entity"); err != nil {
		t.Fatalf("Emit json with fields: %v", err)
	}
	if strings.Contains(jsonBuf.String(), "n_authors") {
		t.Errorf("json with --fields rows.entity should drop n_authors\ngot: %s", jsonBuf.String())
	}
	if !strings.Contains(jsonBuf.String(), `"entity":"A.go"`) {
		t.Errorf("json with --fields rows.entity should keep entity\ngot: %s", jsonBuf.String())
	}

	// csv ignores --fields: the full kebab header is still emitted.
	var csvBuf bytes.Buffer
	if err := output.Emit(&csvBuf, "csv", fmtResult(), fmtColumns, "rows.entity"); err != nil {
		t.Fatalf("Emit csv with fields: %v", err)
	}
	if got := nonEmptyLines(csvBuf.String())[0]; got != "entity,n-authors,n-revs" {
		t.Errorf("csv should ignore --fields, header = %q, want full header", got)
	}
}

func TestFormat_Rows_AllFormats(t *testing.T) {
	// --rows truncation happens before Emit; a capped result carries only the
	// surviving rows. Every format must emit exactly those rows.
	capped := fmtResult()
	capped.Rows = []fmtRow{{Entity: "A.go", NAuthors: 3, NRevs: 10}}
	capped.RowCount = 1
	capped.TotalCount = 2
	capped.Truncated = true

	cases := []struct {
		format   string
		wantRows int // data rows, excluding any header
		header   bool
	}{
		{"ndjson", 1, false},
		{"csv", 1, true},
		{"table", 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			var buf bytes.Buffer
			if err := output.Emit(&buf, tc.format, capped, fmtColumns, ""); err != nil {
				t.Fatalf("Emit %s: %v", tc.format, err)
			}
			lines := nonEmptyLines(buf.String())
			want := tc.wantRows
			if tc.header {
				want++
			}
			if len(lines) != want {
				t.Errorf("%s emitted %d lines, want %d\ngot: %s", tc.format, len(lines), want, buf.String())
			}
		})
	}
}
