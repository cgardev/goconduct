package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCompositionRootOwnsConcretePluginSelection(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read composition root: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		for _, imported := range importsOfFile(t, entry.Name()) {
			if strings.HasPrefix(imported, "github.com/cgardev/goconduct/plugin/") && entry.Name() != "modules.go" {
				t.Errorf("%s selects concrete plugin %q", entry.Name(), imported)
			}
		}
	}
}

func TestKernelDoesNotOwnApplicationScopeOrConcretePlugins(t *testing.T) {
	kernelDirectory := filepath.Join("..", "..", "internal", "kernel")
	entries, err := os.ReadDir(kernelDirectory)
	if err != nil {
		t.Fatalf("read kernel: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(kernelDirectory, entry.Name())
		for _, imported := range importsOfFile(t, path) {
			if strings.Contains(imported, "/internal/appmodule") || strings.HasPrefix(
				imported,
				"github.com/cgardev/goconduct/plugin/",
			) {
				t.Errorf("kernel imports application-owned package %q", imported)
			}
		}
	}
}

func TestQualityModuleDependsOnlyOnPublicPluginContracts(t *testing.T) {
	qualityDirectory := filepath.Join("internal", "module", "quality")
	entries, err := os.ReadDir(qualityDirectory)
	if err != nil {
		t.Fatalf("read quality module: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(qualityDirectory, entry.Name())
		for _, imported := range importsOfFile(t, path) {
			if strings.HasPrefix(imported, "github.com/cgardev/goconduct/plugin/") {
				t.Errorf("quality module imports concrete plugin %q", imported)
			}
		}
	}
}

func importsOfFile(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	imports := make([]string, 0, len(file.Imports))
	for _, specification := range file.Imports {
		imported, err := strconv.Unquote(specification.Path.Value)
		if err != nil {
			t.Fatalf("read import from %s: %v", path, err)
		}
		imports = append(imports, imported)
	}
	return imports
}
