package teammap_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
	"github.com/andreswebs/codelens/internal/transform/teammap"
)

func mustParse(t *testing.T, spec, format string) map[string]string {
	t.Helper()
	teams, err := teammap.Parse(strings.NewReader(spec), format)
	if err != nil {
		t.Fatalf("Parse(%q, %q) returned error: %v", spec, format, err)
	}
	return teams
}

func assertInvalidTeamMap(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var coded terr.Coded
	if !errors.As(err, &coded) {
		t.Fatalf("error %v is not a terr.Coded", err)
	}
	if coded.Code() != "invalid_team_map" {
		t.Errorf("code = %q, want invalid_team_map", coded.Code())
	}
	if coded.ExitCode() != 3 {
		t.Errorf("exit = %d, want 3", coded.ExitCode())
	}
}

func TestParse_CSV(t *testing.T) {
	teams := mustParse(t, "author,team\nalice,Core\nbob,Core", "csv")
	want := map[string]string{"alice": "Core", "bob": "Core"}
	if len(teams) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(teams), len(want), teams)
	}
	for k, v := range want {
		if teams[k] != v {
			t.Errorf("teams[%q] = %q, want %q", k, teams[k], v)
		}
	}
}

func TestParse_CSV_NoHeader(t *testing.T) {
	teams := mustParse(t, "alice,Core\nbob,Platform", "csv")
	if teams["alice"] != "Core" || teams["bob"] != "Platform" {
		t.Errorf("headerless CSV mapped wrong: %v", teams)
	}
}

func TestParse_DefaultFormatIsCSV(t *testing.T) {
	teams := mustParse(t, "alice,Core", "")
	if teams["alice"] != "Core" {
		t.Errorf("empty format did not default to CSV: %v", teams)
	}
}

func TestParse_JSON_Object(t *testing.T) {
	teams := mustParse(t, `{"alice":"Core","bob":"Core"}`, "json")
	if teams["alice"] != "Core" || teams["bob"] != "Core" {
		t.Errorf("JSON object mapped wrong: %v", teams)
	}
}

func TestParse_JSON_Array(t *testing.T) {
	teams := mustParse(t, `[{"author":"alice","team":"Core"},{"author":"bob","team":"Platform"}]`, "json")
	if teams["alice"] != "Core" || teams["bob"] != "Platform" {
		t.Errorf("JSON array mapped wrong: %v", teams)
	}
}

func TestApply_Remaps(t *testing.T) {
	teams := map[string]string{"alice": "Core"}
	mods := []model.Modification{{
		Entity: "src/a.go", Rev: "abc", Date: "2024-01-02",
		Author: "alice", Message: "msg", LocAdded: 3, LocDeleted: 1, HasLoc: true,
	}}
	out := teammap.Apply(mods, teams)
	if len(out) != 1 {
		t.Fatalf("got %d mods, want 1", len(out))
	}
	want := model.Modification{
		Entity: "src/a.go", Rev: "abc", Date: "2024-01-02",
		Author: "Core", Message: "msg", LocAdded: 3, LocDeleted: 1, HasLoc: true,
	}
	if out[0] != want {
		t.Errorf("modification = %+v, want %+v", out[0], want)
	}
}

func TestApply_UnmappedKept(t *testing.T) {
	teams := map[string]string{"alice": "Core"}
	mods := []model.Modification{
		{Entity: "x", Author: "alice", Rev: "1"},
		{Entity: "y", Author: "carol", Rev: "2"},
	}
	out := teammap.Apply(mods, teams)
	if len(out) != 2 {
		t.Fatalf("got %d mods, want 2", len(out))
	}
	if out[0].Author != "Core" {
		t.Errorf("mapped author = %q, want Core", out[0].Author)
	}
	if out[1].Author != "carol" {
		t.Errorf("unmapped author = %q, want carol (kept)", out[1].Author)
	}
}

func TestApply_DoesNotMutateInput(t *testing.T) {
	teams := map[string]string{"alice": "Core"}
	mods := []model.Modification{{Entity: "x", Author: "alice"}}
	_ = teammap.Apply(mods, teams)
	if mods[0].Author != "alice" {
		t.Errorf("input mutated: author = %q, want alice", mods[0].Author)
	}
}

func TestApply_NilMapKeepsAll(t *testing.T) {
	mods := []model.Modification{{Entity: "x", Author: "alice"}}
	out := teammap.Apply(mods, nil)
	if len(out) != 1 || out[0].Author != "alice" {
		t.Errorf("nil map altered authors: %+v", out)
	}
}

func TestParse_Malformed_CSV(t *testing.T) {
	_, err := teammap.Parse(strings.NewReader("alice,Core\nbob"), "csv")
	assertInvalidTeamMap(t, err)
}

func TestParse_Malformed_JSON(t *testing.T) {
	_, err := teammap.Parse(strings.NewReader("{not valid"), "json")
	assertInvalidTeamMap(t, err)
}

func TestParse_UnknownFormat(t *testing.T) {
	_, err := teammap.Parse(strings.NewReader("alice,Core"), "yaml")
	assertInvalidTeamMap(t, err)
}

func TestApply_PortedFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "team-map.csv"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	teams := mustParse(t, string(data), "csv")
	mods := []model.Modification{
		{Entity: "a", Author: "APN", Rev: "1"},
		{Entity: "b", Author: "XYZ", Rev: "2"},
		{Entity: "c", Author: "ZOP", Rev: "3"},
		{Entity: "d", Author: "QQQ", Rev: "4"},
	}
	out := teammap.Apply(mods, teams)
	wantAuthors := []string{"Blue", "Blue", "Yellow", "QQQ"}
	for i, want := range wantAuthors {
		if out[i].Author != want {
			t.Errorf("out[%d].Author = %q, want %q", i, out[i].Author, want)
		}
	}
}
