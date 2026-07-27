package command

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// authorsFixtureLog reads the shared authors log fixture as a string.
func authorsFixtureLog(t *testing.T) string {
	t.Helper()
	in, err := os.ReadFile(authorsFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(in)
}

// resultEnvelope mirrors the fields of a success envelope the semantics and
// transforms tests inspect. semantics is always present; transforms is a pointer
// so its absence (a pass-through pipeline) is distinguishable from an empty object.
type resultEnvelope struct {
	Analysis   string            `json:"analysis"`
	Shape      string            `json:"shape"`
	Semantics  map[string]string `json:"semantics"`
	Transforms map[string]any    `json:"transforms"`
	RowCount   int               `json:"row_count"`
}

// runResult runs the CLI with stdin and returns the parsed success envelope,
// failing the test on a non-zero exit or an unparseable stdout.
func runResult(t *testing.T, stdin string, args ...string) resultEnvelope {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, Deps{In: strings.NewReader(stdin), Out: &stdout, Err: &stderr})
	if code != 0 {
		t.Fatalf("%v exit = %d, want 0; stderr:\n%s", args, code, stderr.String())
	}
	var env resultEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("%v: stdout is not a result envelope: %v\n%s", args, err, stdout.String())
	}
	return env
}

// TestSemantics_AlwaysPresent pins that every analysis envelope carries a
// non-nil semantics map, even for an empty result (semantics track flags, not
// data). rawSemanticsPresent additionally confirms the key marshals as {} rather
// than being absent when empty.
func TestSemantics_AlwaysPresent(t *testing.T) {
	env := runResult(t, authorsFixtureLog(t), "authors")
	if len(env.Semantics) == 0 {
		t.Fatalf("semantics = %v, want a populated map", env.Semantics)
	}
	if env.Semantics["entity"] != "filepath" {
		t.Errorf("entity semantic = %q, want filepath", env.Semantics["entity"])
	}
}

// TestSemantics_CouplingFlagGating pins D15: coupling emits 4 entries without
// --verbose and 7 with it.
func TestSemantics_CouplingFlagGating(t *testing.T) {
	// A log where a.go and b.go co-change enough to survive the default thresholds.
	log := weakCouplingLog(8, 2)

	plain := runResult(t, log, "coupling")
	if len(plain.Semantics) != 4 {
		t.Errorf("plain coupling semantics = %v, want 4 entries", plain.Semantics)
	}
	for _, gated := range []string{"first_entity_revisions", "second_entity_revisions", "shared_revisions"} {
		if _, ok := plain.Semantics[gated]; ok {
			t.Errorf("plain coupling lists gated column %q; want absent", gated)
		}
	}

	verbose := runResult(t, log, "coupling", "--verbose")
	if len(verbose.Semantics) != 7 {
		t.Errorf("verbose coupling semantics = %v, want 7 entries", verbose.Semantics)
	}
	if verbose.Semantics["shared_revisions"] != "count" {
		t.Errorf("shared_revisions semantic = %q, want count", verbose.Semantics["shared_revisions"])
	}
}

// TestSemantics_ParseTracksFlagsNotData pins that parse declares all 8 columns
// even on a log with no numstat, where loc_added/loc_deleted are absent from
// every row: semantics track flags, not data.
func TestSemantics_ParseTracksFlagsNotData(t *testing.T) {
	// A record with a subject but no numstat lines: loc metrics never appear.
	const noNumstat = "--a1--2024-01-01--Alice--c1\n\n"
	env := runResult(t, noNumstat, "parse")
	for _, col := range []string{"entity", "rev", "date", "author", "message", "loc_added", "loc_deleted", "binary"} {
		if _, ok := env.Semantics[col]; !ok {
			t.Errorf("parse semantics missing %q; want all 8 columns regardless of data", col)
		}
	}
}

// TestSemantics_GroupDegradesToLabel pins the D4 asymmetry in one place: with
// --group the envelope reports entity as a label (a layer name is not a
// splittable path), while schema --command still declares filepath (the
// command's untransformed default).
func TestSemantics_GroupDegradesToLabel(t *testing.T) {
	env := runResult(t, authorsFixtureLog(t), "--group", "testdata/layers.group", "authors")
	if env.Semantics["entity"] != "label" {
		t.Errorf("grouped entity semantic = %q, want label", env.Semantics["entity"])
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"schema", "--command", "authors"}, Deps{In: strings.NewReader(""), Out: &stdout, Err: &stderr}); code != 0 {
		t.Fatalf("schema exit = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	var sc schemaCmd
	if err := json.Unmarshal(stdout.Bytes(), &sc); err != nil {
		t.Fatalf("schema not an envelope: %v", err)
	}
	for _, c := range sc.RowSchema {
		if c.Name == "entity" && c.Semantic != "filepath" {
			t.Errorf("schema entity semantic = %q, want filepath (the untransformed default)", c.Semantic)
		}
	}
}

// TestSemantics_TeamMapKeepsPerson pins the D4a negative case: --team-map
// aggregates authors into teams, but both are opaque categorical actor names, so
// author stays person (no structural affordance is lost).
func TestSemantics_TeamMapKeepsPerson(t *testing.T) {
	env := runResult(t, authorsFixtureLog(t), "--team-map", "testdata/teams.csv", "entity-ownership")
	if env.Semantics["author"] != "person" {
		t.Errorf("team-mapped author semantic = %q, want person", env.Semantics["author"])
	}
}

// TestTransforms_AbsentOnPlainRun pins that a pass-through pipeline omits the
// transforms key entirely.
func TestTransforms_AbsentOnPlainRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"authors"}, Deps{In: strings.NewReader(authorsFixtureLog(t)), Out: &stdout, Err: &stderr}); code != 0 {
		t.Fatalf("exit = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
		t.Fatalf("not a JSON object: %v", err)
	}
	if _, ok := m["transforms"]; ok {
		t.Errorf("plain run should omit transforms; got %s", stdout.String())
	}
}

// TestTransforms_RecordsActiveStages pins the exact snake_case keys a run with
// --group, --exclude, and --temporal-period records.
func TestTransforms_RecordsActiveStages(t *testing.T) {
	env := runResult(t, authorsFixtureLog(t),
		"--group", "testdata/layers.group",
		"--exclude", "**/doc/**",
		"--temporal-period", "30",
		"authors")
	if env.Transforms == nil {
		t.Fatalf("transforms absent; want group/exclude/temporal_period")
	}
	if env.Transforms["group"] != true {
		t.Errorf("transforms.group = %v, want true", env.Transforms["group"])
	}
	if _, ok := env.Transforms["exclude"]; !ok {
		t.Errorf("transforms.exclude missing: %v", env.Transforms)
	}
	if env.Transforms["temporal_period"] != float64(30) {
		t.Errorf("transforms.temporal_period = %v, want 30", env.Transforms["temporal_period"])
	}
	if _, ok := env.Transforms["team_map"]; ok {
		t.Errorf("transforms.team_map present without --team-map: %v", env.Transforms)
	}
}

// TestSemantics_FieldsProjection pins D6: --fields rows.entity narrows semantics
// to one entry, while --fields rows keeps the whole map; shape is retained in
// both.
func TestSemantics_FieldsProjection(t *testing.T) {
	one := projectedEnvelope(t, authorsFixtureLog(t), "rows.entity")
	if len(one.Semantics) != 1 || one.Semantics["entity"] != "filepath" {
		t.Errorf("--fields rows.entity semantics = %v, want exactly {entity: filepath}", one.Semantics)
	}
	if one.Shape != "table" {
		t.Errorf("--fields rows.entity dropped shape: %q", one.Shape)
	}

	all := projectedEnvelope(t, authorsFixtureLog(t), "rows")
	if len(all.Semantics) != 3 {
		t.Errorf("--fields rows semantics = %v, want the full 3-entry map", all.Semantics)
	}
}

// projectedEnvelope runs authors with a --fields projection and returns the
// parsed envelope.
func projectedEnvelope(t *testing.T, stdin, fields string) resultEnvelope {
	t.Helper()
	return runResult(t, stdin, "--fields", fields, "authors")
}
