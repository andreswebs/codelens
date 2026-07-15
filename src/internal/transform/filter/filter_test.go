package filter_test

import (
	"errors"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
	"github.com/andreswebs/codelens/internal/transform/filter"
)

func mustCompile(t *testing.T, includes, excludes []string) filter.Spec {
	t.Helper()
	spec, err := filter.Compile(includes, excludes)
	if err != nil {
		t.Fatalf("Compile(%v, %v) returned error: %v", includes, excludes, err)
	}
	return spec
}

func entities(mods []model.Modification) []string {
	out := make([]string, len(mods))
	for i, m := range mods {
		out[i] = m.Entity
	}
	return out
}

// TestFilter_ExcludeDropsMatches asserts a single exclude glob drops matching
// entities and keeps the rest.
func TestFilter_ExcludeDropsMatches(t *testing.T) {
	spec := mustCompile(t, nil, []string{"**/Migrations/**"})
	mods := []model.Modification{
		{Entity: "a.cs", Rev: "1"},
		{Entity: "x/Migrations/m.cs", Rev: "2"},
	}
	out := filter.Apply(mods, spec)
	if got, want := entities(out), []string{"a.cs"}; !equal(got, want) {
		t.Errorf("entities = %v, want %v", got, want)
	}
}

// TestFilter_IncludeThenExclude asserts exclude-after-include precedence: an
// entity must match an include, and a later exclude still drops it; an entity
// matching no include is dropped even without an exclude.
func TestFilter_IncludeThenExclude(t *testing.T) {
	spec := mustCompile(t, []string{"**/*.cs"}, []string{"**/*.Designer.cs"})
	mods := []model.Modification{
		{Entity: "src/Page.cs", Rev: "1"},
		{Entity: "src/Page.Designer.cs", Rev: "2"},
		{Entity: "src/app.g.dart", Rev: "3"},
	}
	out := filter.Apply(mods, spec)
	if got, want := entities(out), []string{"src/Page.cs"}; !equal(got, want) {
		t.Errorf("entities = %v, want %v (.Designer.cs excluded, .dart not included)", got, want)
	}
}

// TestFilter_NoGlobsKeepsAll asserts an empty spec is a no-op passthrough.
func TestFilter_NoGlobsKeepsAll(t *testing.T) {
	spec := mustCompile(t, nil, nil)
	if !spec.IsZero() {
		t.Error("empty spec should report IsZero")
	}
	mods := []model.Modification{{Entity: "a.cs"}, {Entity: "b.dart"}}
	out := filter.Apply(mods, spec)
	if got, want := entities(out), []string{"a.cs", "b.dart"}; !equal(got, want) {
		t.Errorf("entities = %v, want all %v", got, want)
	}
}

// TestFilter_BadGlob asserts an uncompilable glob is a coded usage error
// (invalid_glob, exit 2), from either the include or the exclude set.
func TestFilter_BadGlob(t *testing.T) {
	if _, err := filter.Compile([]string{"a[b"}, nil); !isInvalidGlob(err) {
		t.Errorf("include bad glob: err = %v, want invalid_glob/exit 2", err)
	}
	if _, err := filter.Compile(nil, []string{"a[b"}); !isInvalidGlob(err) {
		t.Errorf("exclude bad glob: err = %v, want invalid_glob/exit 2", err)
	}
}

// TestFilter_EmptyGlob rejects an empty pattern string as a usage error.
func TestFilter_EmptyGlob(t *testing.T) {
	if _, err := filter.Compile([]string{""}, nil); !isInvalidGlob(err) {
		t.Errorf("empty glob: err = %v, want invalid_glob/exit 2", err)
	}
}

// TestFilter_OversizeGlob rejects a pattern beyond the length guard.
func TestFilter_OversizeGlob(t *testing.T) {
	huge := make([]byte, 2000)
	for i := range huge {
		huge[i] = 'a'
	}
	if _, err := filter.Compile(nil, []string{string(huge)}); !isInvalidGlob(err) {
		t.Errorf("oversize glob: err = %v, want invalid_glob/exit 2", err)
	}
}

// TestFilter_MultipleIncludes asserts an entity survives if it matches any one
// of several include globs.
func TestFilter_MultipleIncludes(t *testing.T) {
	spec := mustCompile(t, []string{"**/*.go", "**/*.cs"}, nil)
	mods := []model.Modification{
		{Entity: "src/a.go"},
		{Entity: "src/b.cs"},
		{Entity: "src/c.dart"},
	}
	out := filter.Apply(mods, spec)
	if got, want := entities(out), []string{"src/a.go", "src/b.cs"}; !equal(got, want) {
		t.Errorf("entities = %v, want %v", got, want)
	}
}

// TestFilter_DoesNotMutateInput asserts Apply returns a fresh slice and leaves
// the caller's slice untouched.
func TestFilter_DoesNotMutateInput(t *testing.T) {
	spec := mustCompile(t, nil, []string{"**/*.dart"})
	mods := []model.Modification{{Entity: "a.go"}, {Entity: "b.dart"}}
	_ = filter.Apply(mods, spec)
	if len(mods) != 2 || mods[0].Entity != "a.go" || mods[1].Entity != "b.dart" {
		t.Errorf("input mutated: %+v", mods)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func isInvalidGlob(err error) bool {
	if err == nil {
		return false
	}
	var coded terr.Coded
	if !errors.As(err, &coded) {
		return false
	}
	return coded.Code() == "invalid_glob" && coded.ExitCode() == 2
}
