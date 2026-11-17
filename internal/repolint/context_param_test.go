// Package repolint holds repo-wide source checks that no linter in the
// toolchain covers. It has no non-test code.
package repolint

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestNoBlankOrUnnamedContextParameter enforces CONTEXT.md's "Context
// parameter" rule: non-test code never declares a context.Context parameter
// that is blank (_) or unnamed. revive's unused-parameter and
// context-as-argument rules catch a named-but-unused ctx and a misplaced one,
// but flag neither of these.
func TestNoBlankOrUnnamedContextParameter(t *testing.T) {
	root := filepath.Join("..", "..")
	var offenders []string
	for _, dir := range []string{"cmd", "internal"} {
		files, err := sourceFiles(filepath.Join(root, dir))
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
		for _, path := range files {
			found, err := blankContextParams(path)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			offenders = append(offenders, found...)
		}
	}
	for _, o := range offenders {
		t.Errorf("%s: context.Context parameter is blank or unnamed; drop it unless the function reads it or passes it on", o)
	}
}

// sourceFiles lists the non-test .go files under dir, skipping testdata.
func sourceFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// blankContextParams returns "file:line" for every function declaration,
// method declaration or function literal in path with a blank or unnamed
// context.Context parameter.
func blankContextParams(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	aliases := contextImportNames(file)
	if len(aliases) == 0 {
		return nil, nil
	}
	var found []string
	ast.Inspect(file, func(n ast.Node) bool {
		ft := funcTypeOf(n)
		if ft == nil {
			return true
		}
		for _, field := range ft.Params.List {
			if isBlankContextField(field, aliases) {
				found = append(found, fmt.Sprintf("%s:%d", path, fset.Position(field.Pos()).Line))
			}
		}
		return true
	})
	return found, nil
}

// funcTypeOf returns the signature of a function declaration or literal, and
// nil for any other node.
func funcTypeOf(n ast.Node) *ast.FuncType {
	switch fn := n.(type) {
	case *ast.FuncDecl:
		return fn.Type
	case *ast.FuncLit:
		return fn.Type
	}
	return nil
}

// contextImportNames returns the local names under which file imports the
// standard "context" package.
func contextImportNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, spec := range file.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil || p != "context" {
			continue
		}
		name := "context"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		names[name] = true
	}
	return names
}

// isBlankContextField reports whether field is a context.Context parameter
// that is unnamed or named _.
func isBlankContextField(field *ast.Field, aliases map[string]bool) bool {
	sel, ok := field.Type.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Context" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || !aliases[pkg.Name] {
		return false
	}
	if len(field.Names) == 0 {
		return true
	}
	for _, name := range field.Names {
		if name.Name == "_" {
			return true
		}
	}
	return false
}
