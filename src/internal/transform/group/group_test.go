package group_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
	"github.com/andreswebs/codelens/internal/transform/group"
)

func mustParse(t *testing.T, spec, format string) []group.Spec {
	t.Helper()
	specs, err := group.Parse(strings.NewReader(spec), format)
	if err != nil {
		t.Fatalf("Parse(%q, %q) returned error: %v", spec, format, err)
	}
	return specs
}

func TestParse_Text_PrefixAnchor(t *testing.T) {
	specs := mustParse(t, "src/Features/Core => Core", "text")
	if len(specs) != 1 {
		t.Fatalf("got %d specs, want 1", len(specs))
	}
	if got, want := specs[0].Pattern.String(), "^src/Features/Core/"; got != want {
		t.Errorf("pattern = %q, want %q", got, want)
	}
	if got, want := specs[0].Name, "Core"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
	if !specs[0].Pattern.MatchString("src/Features/Core/x.cs") {
		t.Error("expected match on src/Features/Core/x.cs")
	}
	if specs[0].Pattern.MatchString("other/Features/Core/x.cs") {
		t.Error("did not expect match on other/Features/Core/x.cs")
	}
	if specs[0].Pattern.MatchString("src/Features/Core") {
		t.Error("prefix anchor must require the trailing slash")
	}
}

func TestParse_Text_RegexVerbatim(t *testing.T) {
	specs := mustParse(t, `^src/.*Tests\.cs$ => Tests`, "text")
	if got, want := specs[0].Pattern.String(), `^src/.*Tests\.cs$`; got != want {
		t.Errorf("pattern = %q, want verbatim %q", got, want)
	}
	if !specs[0].Pattern.MatchString("src/foo/BarTests.cs") {
		t.Error("expected match on src/foo/BarTests.cs")
	}
	if specs[0].Pattern.MatchString("src/foo/Bar.cs") {
		t.Error("did not expect match on src/foo/Bar.cs")
	}
}

func TestParse_Text_TrimsAndSkipsBlankLines(t *testing.T) {
	specs := mustParse(t, "\n  src/a   =>   A  \n\n^b$ => B\n", "text")
	if len(specs) != 2 {
		t.Fatalf("got %d specs, want 2", len(specs))
	}
	if got, want := specs[0].Pattern.String(), "^src/a/"; got != want {
		t.Errorf("pattern[0] = %q, want %q", got, want)
	}
	if got, want := specs[0].Name, "A"; got != want {
		t.Errorf("name[0] = %q, want %q", got, want)
	}
}

func TestParse_JSON(t *testing.T) {
	json := `[{"pattern":"src/a","name":"A"},{"pattern":"^b$","name":"B"}]`
	specs := mustParse(t, json, "json")
	if len(specs) != 2 {
		t.Fatalf("got %d specs, want 2", len(specs))
	}
	if got, want := specs[0].Pattern.String(), "^src/a/"; got != want {
		t.Errorf("pattern[0] = %q, want anchored %q", got, want)
	}
	if got, want := specs[1].Pattern.String(), "^b$"; got != want {
		t.Errorf("pattern[1] = %q, want verbatim %q", got, want)
	}
	if got, want := specs[1].Name, "B"; got != want {
		t.Errorf("name[1] = %q, want %q", got, want)
	}
}

func TestApply_FirstMatchWins(t *testing.T) {
	specs := mustParse(t, "src/Features/Core => Core\nsrc/Features => Features\n", "text")
	mods := []model.Modification{{Entity: "src/Features/Core/x.cs", Rev: "a"}}
	out := group.Apply(mods, specs)
	if len(out) != 1 {
		t.Fatalf("got %d mods, want 1", len(out))
	}
	if got, want := out[0].Entity, "Core"; got != want {
		t.Errorf("entity = %q, want first-match %q", got, want)
	}
}

func TestApply_DropsUnmatched(t *testing.T) {
	specs := mustParse(t, "src/a => A", "text")
	mods := []model.Modification{
		{Entity: "src/a/x.go", Rev: "1"},
		{Entity: "other/y.go", Rev: "2"},
		{Entity: "src/a/z.go", Rev: "3"},
	}
	out := group.Apply(mods, specs)
	if len(out) != 2 {
		t.Fatalf("got %d mods, want 2 (unmatched dropped)", len(out))
	}
	for _, m := range out {
		if m.Entity != "A" {
			t.Errorf("surviving entity = %q, want A", m.Entity)
		}
	}
	if out[0].Rev != "1" || out[1].Rev != "3" {
		t.Errorf("order not preserved: got revs %q,%q want 1,3", out[0].Rev, out[1].Rev)
	}
}

func TestApply_RemapsEntity_OtherFieldsIntact(t *testing.T) {
	specs := mustParse(t, "src/a => A", "text")
	mods := []model.Modification{{
		Entity: "src/a/x.go", Rev: "abc", Date: "2024-01-02",
		Author: "alice", Message: "msg", LocAdded: 3, LocDeleted: 1, HasLoc: true,
	}}
	out := group.Apply(mods, specs)
	got := out[0]
	if got.Entity != "A" {
		t.Errorf("entity = %q, want A", got.Entity)
	}
	want := model.Modification{
		Entity: "A", Rev: "abc", Date: "2024-01-02",
		Author: "alice", Message: "msg", LocAdded: 3, LocDeleted: 1, HasLoc: true,
	}
	if got != want {
		t.Errorf("modification = %+v, want %+v", got, want)
	}
}

func TestApply_NoSpecsDropsEverything(t *testing.T) {
	mods := []model.Modification{{Entity: "src/a/x.go"}}
	out := group.Apply(mods, nil)
	if len(out) != 0 {
		t.Fatalf("got %d mods, want 0 when no specs match", len(out))
	}
}

func assertInvalidGroup(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var coded terr.Coded
	if !errors.As(err, &coded) {
		t.Fatalf("error %v is not a terr.Coded", err)
	}
	if coded.Code() != "invalid_group" {
		t.Errorf("code = %q, want invalid_group", coded.Code())
	}
	if coded.ExitCode() != 2 {
		t.Errorf("exit = %d, want 2", coded.ExitCode())
	}
}

func TestParse_InvalidRegex(t *testing.T) {
	_, err := group.Parse(strings.NewReader(`^([a-z => Bad`), "text")
	assertInvalidGroup(t, err)
}

func TestParse_OversizePattern(t *testing.T) {
	huge := "^" + strings.Repeat("a", 2000)
	_, err := group.Parse(strings.NewReader(huge+" => Big"), "text")
	assertInvalidGroup(t, err)
}

func TestParse_MissingSeparator(t *testing.T) {
	_, err := group.Parse(strings.NewReader("src/a Core"), "text")
	assertInvalidGroup(t, err)
}

func TestParse_MalformedJSON(t *testing.T) {
	_, err := group.Parse(strings.NewReader(`{not valid`), "json")
	assertInvalidGroup(t, err)
}

func TestParse_UnknownFormat(t *testing.T) {
	_, err := group.Parse(strings.NewReader("src/a => A"), "yaml")
	assertInvalidGroup(t, err)
}

func TestApply_PortedLayerDefs(t *testing.T) {
	cases := []struct {
		file   string
		entity string
		want   string // "" means dropped
	}{
		{"text-layers-definition.txt", "src/Features/Core/x.cs", "Core"},
		{"text-layers-definition.txt", "src/Features/UI/y.cs", "Features"},
		{"text-layers-definition.txt", "other/z.cs", ""},
		{"regex-layers-definition.txt", "src/foo/BarTests.cs", "Tests"},
		{"regex-layers-definition.txt", "src/foo/Baz.cs", "Src"},
		{"regex-layers-definition.txt", "docs/readme.md", ""},
		{"regex-and-text-layers-definition.txt", "README.md", "Docs"},
		{"regex-and-text-layers-definition.txt", "src/main.go", "Source"},
		{"regex-and-text-layers-definition.txt", "other.txt", ""},
	}
	for _, tc := range cases {
		t.Run(tc.file+"/"+tc.entity, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			specs := mustParse(t, string(data), "text")
			out := group.Apply([]model.Modification{{Entity: tc.entity}}, specs)
			if tc.want == "" {
				if len(out) != 0 {
					t.Fatalf("entity %q: got %q, want dropped", tc.entity, out[0].Entity)
				}
				return
			}
			if len(out) != 1 {
				t.Fatalf("entity %q: got %d mods, want 1", tc.entity, len(out))
			}
			if out[0].Entity != tc.want {
				t.Errorf("entity %q -> %q, want %q", tc.entity, out[0].Entity, tc.want)
			}
		})
	}
}
