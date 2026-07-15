package output_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/output"
	"github.com/andreswebs/codelens/internal/terr"
)

// projRow is a stand-in row type with json tags, used to exercise nested
// projection over a slice held in Result.Rows.
type projRow struct {
	Entity string `json:"entity"`
	Degree int    `json:"degree"`
}

// sampleResult is a fully populated envelope for projection tests.
func sampleResult() output.Result {
	return output.Result{
		SchemaVersion: output.SchemaVersion,
		OK:            true,
		Analysis:      "coupling",
		Params:        map[string]any{"min_coupling": 30},
		RowCount:      2,
		Rows: []projRow{
			{Entity: "A.go", Degree: 78},
			{Entity: "B.go", Degree: 62},
		},
	}
}

func TestValidateFields_Empty(t *testing.T) {
	got, err := output.ValidateFields("", output.Result{})
	if err != nil {
		t.Fatalf("ValidateFields(\"\"): unexpected error %v", err)
	}
	if got != nil {
		t.Errorf("ValidateFields(\"\") = %v, want nil", got)
	}
}

func TestValidateFields_TopLevel(t *testing.T) {
	got, err := output.ValidateFields("rows", output.Result{})
	if err != nil {
		t.Fatalf("ValidateFields(\"rows\"): unexpected error %v", err)
	}
	if len(got) != 1 || got[0] != "rows" {
		t.Errorf("ValidateFields(\"rows\") = %v, want [rows]", got)
	}
}

func TestValidateFields_Nested(t *testing.T) {
	if _, err := output.ValidateFields("rows.entity", sampleResult()); err != nil {
		t.Errorf("ValidateFields(\"rows.entity\"): unexpected error %v", err)
	}
}

func TestValidateFields_Invalid(t *testing.T) {
	_, err := output.ValidateFields("rows.bogus", sampleResult())
	if err == nil {
		t.Fatal("ValidateFields(\"rows.bogus\"): want error, got nil")
	}

	var coded terr.Coded
	if !errors.As(err, &coded) {
		t.Fatalf("error %v is not terr.Coded", err)
	}
	if coded.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", coded.ExitCode())
	}
	if coded.Code() != "invalid_field" {
		t.Errorf("code = %q, want %q", coded.Code(), "invalid_field")
	}
	if !strings.Contains(err.Error(), "rows.entity") {
		t.Errorf("message should list valid paths (e.g. rows.entity), got: %s", err.Error())
	}
}

func TestProjectFields_KeepsSchemaAndOK(t *testing.T) {
	data, err := json.Marshal(sampleResult())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	out, err := output.ProjectFields(data, []string{"rows.entity"})
	if err != nil {
		t.Fatalf("ProjectFields: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal projected: %v\ngot: %s", err, out)
	}
	if _, ok := m["schema_version"]; !ok {
		t.Errorf("projected output dropped schema_version: %s", out)
	}
	if _, ok := m["ok"]; !ok {
		t.Errorf("projected output dropped ok: %s", out)
	}
	if _, ok := m["analysis"]; ok {
		t.Errorf("projected output should not include analysis: %s", out)
	}
}

func TestProjectFields_NestedSliceRows(t *testing.T) {
	data, err := json.Marshal(sampleResult())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	out, err := output.ProjectFields(data, []string{"rows.entity"})
	if err != nil {
		t.Fatalf("ProjectFields: %v", err)
	}

	var m struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal projected: %v\ngot: %s", err, out)
	}
	if len(m.Rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(m.Rows))
	}
	for i, row := range m.Rows {
		if _, ok := row["entity"]; !ok {
			t.Errorf("row %d missing entity: %v", i, row)
		}
		if _, ok := row["degree"]; ok {
			t.Errorf("row %d should not include degree: %v", i, row)
		}
	}
}

func TestEmitProjected_EmptyEqualsEmitJSON(t *testing.T) {
	env := sampleResult()

	var projected, plain bytes.Buffer
	if err := output.EmitProjected(&projected, env, ""); err != nil {
		t.Fatalf("EmitProjected: %v", err)
	}
	if err := output.EmitJSON(&plain, env); err != nil {
		t.Fatalf("EmitJSON: %v", err)
	}

	if !bytes.Equal(projected.Bytes(), plain.Bytes()) {
		t.Errorf("EmitProjected(\"\") = %q, want EmitJSON = %q", projected.String(), plain.String())
	}
}
