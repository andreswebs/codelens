package command

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoLoggingImports pins the ADR 0005 (local) silent posture at the module
// level: no non-test Go file may import "log" or "log/slog". The absence of any
// logging package is the conformance evidence for the silent posture, so a
// re-introduced logger must trip a test rather than pass review unnoticed.
// "runtime/debug" (used by internal/version) is a distinct import path and is
// intentionally not matched.
func TestNoLoggingImports(t *testing.T) {
	moduleRoot := filepath.Join("..", "..")
	forbidden := map[string]bool{"log": true, "log/slog": true}

	fset := token.NewFileSet()
	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if forbidden[p] {
				t.Errorf("%s imports %q; the silent posture forbids logging packages", path, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking module: %v", err)
	}
}
