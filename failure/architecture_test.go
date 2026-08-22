package failure

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// exemptDirectories create errors outside this taxonomy for a documented reason.
// This package declares the categories. The Connect translator builds sanitized
// transport errors, so those messages never carry a category.
var exemptDirectories = []string{
	"failure",
	filepath.Join("internal", "library", "connecterror"),
}

// skippedDirectories hold generated code or sources of another language.
var skippedDirectories = []string{
	"docs",
	"target",
	"web",
	"node_modules",
	filepath.Join("internal", "protogen"),
}

func TestErrorPolicy_ClassifyEveryProductionFailure(t *testing.T) {
	root := ".."
	walkError := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skipDirectory(relative) {
				return filepath.SkipDir
			}
			return nil
		}
		if !productionGoFile(entry.Name()) {
			return nil
		}
		inspectErrorFactories(t, path, relative)
		return nil
	})
	if walkError != nil {
		t.Fatalf("walk repository: %v", walkError)
	}
}

func skipDirectory(relative string) bool {
	if relative == "." {
		return false
	}
	if strings.HasPrefix(filepath.Base(relative), ".") {
		return true
	}
	for _, skipped := range skippedDirectories {
		if relative == skipped || strings.HasPrefix(relative, skipped+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func productionGoFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func inspectErrorFactories(t *testing.T, path string, relative string) {
	t.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", relative, err)
	}
	exempt := exemptDirectory(filepath.Dir(relative))
	ast.Inspect(file, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		name, isFactory := errorFactoryName(call)
		if !isFactory {
			return true
		}
		line := fileSet.Position(call.Pos()).Line
		if name == "errors.New" && !exempt {
			t.Errorf("%s:%d creates an unclassified error with errors.New", relative, line)
		}
		if name == "fmt.Errorf" && !exempt && !wrapsCause(call) {
			t.Errorf("%s:%d creates an unclassified error with fmt.Errorf", relative, line)
		}
		return true
	})
}

func exemptDirectory(directory string) bool {
	for _, exempt := range exemptDirectories {
		if directory == exempt ||
			strings.HasPrefix(directory, exempt+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func errorFactoryName(call *ast.CallExpr) (string, bool) {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return "", false
	}
	pkg, isIdentifier := selector.X.(*ast.Ident)
	if !isIdentifier {
		return "", false
	}
	name := pkg.Name + "." + selector.Sel.Name
	return name, name == "errors.New" || name == "fmt.Errorf"
}

// wrapsCause reports whether one fmt.Errorf call preserves its cause with %w.
func wrapsCause(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	literal, isLiteral := call.Args[0].(*ast.BasicLit)
	if !isLiteral || literal.Kind != token.STRING {
		return false
	}
	format, err := strconv.Unquote(literal.Value)
	if err != nil {
		return false
	}
	return strings.Contains(format, "%w")
}
