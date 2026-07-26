package command

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// urfaveImportPath is the CLI framework the tool is deliberately confined to
// internal/command. ADR 0004's no-leak rule keeps it there: the framework may be
// used inside this package but must not appear in an exported identifier, so
// replacing it is a local change.
const urfaveImportPath = "github.com/urfave/cli/v3"

// commandDir is this package's path relative to the module root, used to exempt
// it from the "no other package imports urfave" walk.
const commandDir = "internal/command"

// TestURFave_ImportedOnlyByCommand walks every non-test Go file in the module
// and fails if any file outside internal/command imports the CLI framework. This
// is the cheap half of ADR 0004's no-leak rule (an import grep as a test).
func TestURFave_ImportedOnlyByCommand(t *testing.T) {
	moduleRoot := filepath.Join("..", "..")

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
			if strings.Trim(imp.Path.Value, `"`) != urfaveImportPath {
				continue
			}
			rel, _ := filepath.Rel(moduleRoot, path)
			if !strings.HasPrefix(filepath.ToSlash(rel), commandDir+"/") {
				t.Errorf("%s imports %q; the CLI framework must stay inside %s", rel, urfaveImportPath, commandDir)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking module: %v", err)
	}
}

// TestURFave_NoLeakInExportedSignatures is the load-bearing half of the no-leak
// rule: no exported identifier of internal/command may name a urfave type. It
// parses this package's non-test files and checks every exported function
// signature (receiver, parameters, results), struct field, and type declaration
// for a selector into the urfave import. An ast-level check over these
// declarations is sufficient; a full type-resolution pass would be
// over-engineered for a rule that only needs to catch a framework type surfacing
// in an exported signature.
func TestURFave_NoLeakInExportedSignatures(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		names := urfaveLocalNames(f)
		if len(names) == 0 {
			continue
		}
		for _, decl := range f.Decls {
			checkDecl(t, path, decl, names)
		}
	}
}

// urfaveLocalNames returns the local identifiers the urfave import is bound to in
// f (its package name, or an explicit alias), so a leaked reference is
// recognized regardless of how the file names the import.
func urfaveLocalNames(f *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) != urfaveImportPath {
			continue
		}
		if imp.Name != nil {
			names[imp.Name.Name] = true
		} else {
			names["cli"] = true // the v3 package's declared name
		}
	}
	return names
}

// checkDecl reports a urfave reference in an exported function signature, struct
// field, or type declaration in decl.
func checkDecl(t *testing.T, path string, decl ast.Decl, names map[string]bool) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if !d.Name.IsExported() {
			return
		}
		if d.Recv != nil {
			flagLeak(t, path, "method receiver of "+d.Name.Name, d.Recv, names)
		}
		if d.Type.Params != nil {
			flagLeak(t, path, "parameters of "+d.Name.Name, d.Type.Params, names)
		}
		if d.Type.Results != nil {
			flagLeak(t, path, "results of "+d.Name.Name, d.Type.Results, names)
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || !ts.Name.IsExported() {
				continue
			}
			flagLeak(t, path, "type "+ts.Name.Name, ts.Type, names)
		}
	}
}

// flagLeak fails when node contains a selector into a urfave import (e.g.
// cli.Command), naming where the leak is.
func flagLeak(t *testing.T, path, where string, node ast.Node, names map[string]bool) {
	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && names[id.Name] {
			t.Errorf("%s: %s names urfave type %s.%s in an exported signature", path, where, id.Name, sel.Sel.Name)
		}
		return true
	})
}
