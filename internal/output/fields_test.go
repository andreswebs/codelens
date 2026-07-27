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
// projection over a slice held in Result.Payload.
type projRow struct {
	Entity string `json:"entity"`
	Degree int    `json:"degree"`
}

// declaredCols is the column-name list a coupling-like command would declare;
// it seeds the --fields valid-path set independently of the data.
var declaredCols = []string{"entity", "degree"}

// sampleResult is a fully populated envelope for projection tests.
func sampleResult() output.Result {
	return output.Result{
		SchemaVersion: output.SchemaVersion,
		OK:            true,
		Analysis:      "coupling",
		Shape:         "table",
		Params:        map[string]any{"min_coupling": 30},
		RowCount:      2,
		Payload: []projRow{
			{Entity: "A.go", Degree: 78},
			{Entity: "B.go", Degree: 62},
		},
	}
}

// emptyResult is a valid table envelope whose payload has no rows, so the valid
// path set can only come from the declared columns (the D7a fix).
func emptyResult() output.Result {
	return output.Result{
		SchemaVersion: output.SchemaVersion,
		OK:            true,
		Analysis:      "authors",
		Shape:         "table",
		RowCount:      0,
		Payload:       []projRow{},
	}
}

func TestValidateFields_Empty(t *testing.T) {
	got, err := output.ValidateFields("", sampleResult(), declaredCols)
	if err != nil {
		t.Fatalf("ValidateFields(\"\"): unexpected error %v", err)
	}
	if got != nil {
		t.Errorf("ValidateFields(\"\") = %v, want nil", got)
	}
}

func TestValidateFields_TopLevel(t *testing.T) {
	got, err := output.ValidateFields("rows", sampleResult(), declaredCols)
	if err != nil {
		t.Fatalf("ValidateFields(\"rows\"): unexpected error %v", err)
	}
	if len(got) != 1 || got[0] != "rows" {
		t.Errorf("ValidateFields(\"rows\") = %v, want [rows]", got)
	}
}

func TestValidateFields_Nested(t *testing.T) {
	if _, err := output.ValidateFields("rows.entity", sampleResult(), declaredCols); err != nil {
		t.Errorf("ValidateFields(\"rows.entity\"): unexpected error %v", err)
	}
}

// TestValidateFields_EmptyPayload is the critical D7a regression: a declared
// payload path stays valid even when the payload has zero rows, because the
// valid-path set comes from the schema's columns, not from the data.
func TestValidateFields_EmptyPayload(t *testing.T) {
	got, err := output.ValidateFields("rows.entity", emptyResult(), declaredCols)
	if err != nil {
		t.Fatalf("ValidateFields(\"rows.entity\") on empty payload: unexpected error %v", err)
	}
	if len(got) != 1 || got[0] != "rows.entity" {
		t.Errorf("ValidateFields = %v, want [rows.entity]", got)
	}
}

// TestValidateFields_Shape asserts the self-describing shape key is a valid
// projection target.
func TestValidateFields_Shape(t *testing.T) {
	if _, err := output.ValidateFields("shape", sampleResult(), declaredCols); err != nil {
		t.Errorf("ValidateFields(\"shape\"): unexpected error %v", err)
	}
}

// TestValidateFields_ParamsWildcard keeps the map-typed wildcard behaviour.
func TestValidateFields_ParamsWildcard(t *testing.T) {
	if _, err := output.ValidateFields("params.anything", sampleResult(), declaredCols); err != nil {
		t.Errorf("ValidateFields(\"params.anything\"): unexpected error %v", err)
	}
}

func TestValidateFields_Invalid(t *testing.T) {
	_, err := output.ValidateFields("rows.bogus", sampleResult(), declaredCols)
	if err == nil {
		t.Fatal("ValidateFields(\"rows.bogus\"): want error, got nil")
	}

	var coded terr.Coded
	if !errors.As(err, &coded) {
		t.Fatalf("error %v is not terr.Coded", err)
	}
	if coded.ExitCode() != 64 {
		t.Errorf("exit code = %d, want 64", coded.ExitCode())
	}
	if coded.Code() != "invalid_field" {
		t.Errorf("code = %q, want %q", coded.Code(), "invalid_field")
	}
	if !strings.Contains(err.Error(), "rows.entity") {
		t.Errorf("message should list valid paths (e.g. rows.entity), got: %s", err.Error())
	}
}

func TestProjectFields_KeepsSchemaOKAndShape(t *testing.T) {
	data, err := json.Marshal(sampleResult())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	out, err := output.ProjectFields(data, []string{"rows.entity"}, "rows")
	if err != nil {
		t.Fatalf("ProjectFields: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal projected: %v\ngot: %s", err, out)
	}
	for _, key := range []string{"schema_version", "ok", "shape"} {
		if _, ok := m[key]; !ok {
			t.Errorf("projected output dropped %s: %s", key, out)
		}
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

	out, err := output.ProjectFields(data, []string{"rows.entity"}, "rows")
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

// semanticResult is a fully populated envelope carrying a semantics map and a
// transforms record, for exercising the D6 projection rules.
func semanticResult() output.Result {
	r := sampleResult()
	r.Semantics = map[string]string{"entity": "filepath", "degree": "percentage"}
	r.Transforms = map[string]any{"group": true}
	return r
}

// TestProjectFields_FiltersSemantics pins D6: a projection to rows.entity keeps
// only the entity semantic, retains transforms (it justifies an adjusted
// semantic), and keeps shape.
func TestProjectFields_FiltersSemantics(t *testing.T) {
	data, err := json.Marshal(semanticResult())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out, err := output.ProjectFields(data, []string{"rows.entity"}, "rows")
	if err != nil {
		t.Fatalf("ProjectFields: %v", err)
	}

	var m struct {
		Shape      string            `json:"shape"`
		Semantics  map[string]string `json:"semantics"`
		Transforms map[string]any    `json:"transforms"`
	}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal projected: %v\ngot: %s", err, out)
	}
	if len(m.Semantics) != 1 || m.Semantics["entity"] != "filepath" {
		t.Errorf("semantics = %v, want exactly {entity: filepath}", m.Semantics)
	}
	if m.Shape != "table" {
		t.Errorf("shape = %q, want table", m.Shape)
	}
	if m.Transforms["group"] != true {
		t.Errorf("transforms = %v, want group retained", m.Transforms)
	}
}

// TestProjectFields_WholePayloadKeepsAllSemantics pins that requesting the whole
// payload (--fields rows) keeps every semantic.
func TestProjectFields_WholePayloadKeepsAllSemantics(t *testing.T) {
	data, err := json.Marshal(semanticResult())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out, err := output.ProjectFields(data, []string{"rows"}, "rows")
	if err != nil {
		t.Fatalf("ProjectFields: %v", err)
	}
	var m struct {
		Semantics map[string]string `json:"semantics"`
	}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(m.Semantics) != 2 {
		t.Errorf("semantics = %v, want the full 2-entry map", m.Semantics)
	}
}

func TestEmitProjected_EmptyEqualsEmitJSON(t *testing.T) {
	env := sampleResult()

	var projected, plain bytes.Buffer
	if err := output.EmitProjected(&projected, env, "", declaredCols); err != nil {
		t.Fatalf("EmitProjected: %v", err)
	}
	if err := output.EmitJSON(&plain, env); err != nil {
		t.Fatalf("EmitJSON: %v", err)
	}

	if !bytes.Equal(projected.Bytes(), plain.Bytes()) {
		t.Errorf("EmitProjected(\"\") = %q, want EmitJSON = %q", projected.String(), plain.String())
	}
}
