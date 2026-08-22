package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestLayerArchitecture_KeepProjectImportsInsideTool(t *testing.T) {
	t.Run("Scenario: The dependency graph source remains isolated from shared project code", func(t *testing.T) {
		var forbiddenImports []string
		var inspectError error

		t.Run("Given the dependency graph source tree", func(*testing.T) {
			forbiddenImports = make([]string, 0)
		})

		t.Run("When the test inspects each Go import", func(*testing.T) {
			const projectImportPrefix = "digginginsights.com/v3/"
			const toolImportPrefix = "digginginsights.com/v3/internal/devtool/dependencygraph/"
			inspectError = filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkError error) error {
				if walkError != nil {
					return walkError
				}
				if entry.IsDir() || filepath.Ext(path) != ".go" {
					return nil
				}
				file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
				if err != nil {
					return err
				}
				for _, specification := range file.Imports {
					importPath, err := strconv.Unquote(specification.Path.Value)
					if err != nil {
						return err
					}
					if strings.HasPrefix(importPath, projectImportPrefix) &&
						!strings.HasPrefix(importPath, toolImportPrefix) {
						forbiddenImports = append(
							forbiddenImports,
							filepath.ToSlash(path)+": "+importPath,
						)
					}
				}
				return nil
			})
			slices.Sort(forbiddenImports)
		})

		if !t.Run("Then the test can inspect the complete source tree", func(t *testing.T) {
			if inspectError != nil {
				t.Fatalf("inspect dependency graph imports: %v", inspectError)
			}
		}) {
			return
		}

		t.Run("And no source file imports another project module", func(t *testing.T) {
			if len(forbiddenImports) != 0 {
				t.Fatalf("imports outside dependency graph: %v", forbiddenImports)
			}
		})
	})
}

func TestLayerArchitecture_KeepCoreFilesWithinImportBoundaries(t *testing.T) {
	testCases := []struct {
		file           string
		allowedImports []string
	}{
		{
			file: "domain.go",
			allowedImports: []string{
				"digginginsights.com/v3/internal/devtool/dependencygraph/internal/architecture",
				"digginginsights.com/v3/internal/devtool/dependencygraph/internal/report",
			},
		},
		{
			file: "calculation.go",
			allowedImports: []string{
				"digginginsights.com/v3/internal/devtool/dependencygraph/internal/architecture",
				"digginginsights.com/v3/internal/devtool/dependencygraph/internal/calculation",
				"sort",
			},
		},
		{
			file: "application.go",
			allowedImports: []string{
				"context",
				"digginginsights.com/v3/internal/devtool/dependencygraph/internal/application",
			},
		},
		{
			file:           "classification.go",
			allowedImports: []string{"fmt", "slices", "strings"},
		},
		{
			file: "functioncalculation.go",
			allowedImports: []string{
				"digginginsights.com/v3/internal/devtool/dependencygraph/internal/calculation",
			},
		},
		{file: "internal/architecture/domain.go", allowedImports: []string{}},
		{file: "internal/calculation/metrics.go", allowedImports: []string{"math"}},
		{
			file:           "internal/architecture/rules.go",
			allowedImports: []string{"cmp", "slices", "strings"},
		},
		{
			file:           "internal/application/ports.go",
			allowedImports: []string{"context"},
		},
		{
			file: "internal/application/usecase.go",
			allowedImports: []string{
				"context",
				"digginginsights.com/v3/internal/devtool/dependencygraph/internal/failure",
				"fmt",
			},
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: The pure layer file is "+testCase.file, func(t *testing.T) {
			var imports []string
			var parseError error

			t.Run("Given the file has a closed import set", func(t *testing.T) {
				imports = make([]string, 0)
			})

			t.Run("When the test inspects the layer imports", func(t *testing.T) {
				file, err := parser.ParseFile(
					token.NewFileSet(),
					testCase.file,
					nil,
					parser.ImportsOnly,
				)
				if err != nil {
					parseError = err
					return
				}
				for _, specification := range file.Imports {
					path, err := strconv.Unquote(specification.Path.Value)
					if err != nil {
						parseError = err
						return
					}
					imports = append(imports, path)
				}
				slices.Sort(imports)
			})

			if !t.Run("Then the parser can parse the source file", func(t *testing.T) {
				if parseError != nil {
					t.Fatalf("parse %s: %v", testCase.file, parseError)
				}
			}) {
				return
			}

			t.Run("And the file has only its permitted dependencies", func(t *testing.T) {
				if !slices.Equal(imports, testCase.allowedImports) {
					t.Errorf(
						"%s imports %v, want only %v",
						testCase.file,
						imports,
						testCase.allowedImports,
					)
				}
			})
		})
	}
}

func TestLayerArchitecture_RejectRepositoryRootsOutsideConfiguration(t *testing.T) {
	t.Run("Scenario: The test inspects core analysis files for repository-specific roots", func(t *testing.T) {
		var payloads map[string]string
		var readError error

		t.Run("Given analyzer, classification, and calculation source files", func(t *testing.T) {
			payloads = make(map[string]string)
		})

		t.Run("When the test reads their source text", func(t *testing.T) {
			for _, file := range []string{
				"analyzer.go",
				"classification.go",
				"calculation.go",
			} {
				payload, err := os.ReadFile(file)
				if err != nil {
					readError = err
					return
				}
				payloads[file] = string(payload)
			}
		})

		if !t.Run("Then the test can inspect every core file", func(t *testing.T) {
			if readError != nil {
				t.Fatalf("read architecture source: %v", readError)
			}
		}) {
			return
		}

		t.Run("And only configuration owns the default repository layout", func(t *testing.T) {
			for file, payload := range payloads {
				for _, fixedRoot := range []string{
					`"cmd/`,
					`"internal/module/`,
					`"internal/library/`,
					`"internal/devtool/`,
				} {
					if strings.Contains(payload, fixedRoot) {
						t.Errorf("%s fixes repository layout %s outside configuration", file, fixedRoot)
					}
				}
			}
		})
	})
}

func TestLayerArchitecture_KeepRuntimeAdaptersBehindGraphPorts(t *testing.T) {
	t.Run("Scenario: The test inspects each runtime adapter", func(t *testing.T) {
		var payloads map[string]string
		var readError error

		t.Run("Given the runtime adapter source files", func(t *testing.T) {
			payloads = make(map[string]string)
		})

		t.Run("When the test reads the adapter source", func(t *testing.T) {
			for _, file := range []string{
				"assets.go",
				"cache.go",
				"events.go",
				"graphapi.go",
				"lifecycle.go",
				"monitor.go",
				"runtime.go",
				"server.go",
			} {
				payload, err := os.ReadFile(file)
				if err != nil {
					readError = err
					return
				}
				payloads[file] = string(payload)
			}
		})

		if !t.Run("Then the test can inspect each runtime adapter", func(t *testing.T) {
			if readError != nil {
				t.Fatalf("read runtime adapter source: %v", readError)
			}
		}) {
			return
		}

		t.Run("And each adapter depends on graph ports only", func(t *testing.T) {
			for file, payload := range payloads {
				forbiddenDependencies := []string{"*analyzer", ".analyzer"}
				if file != "monitor.go" {
					forbiddenDependencies = append(forbiddenDependencies, "*graphMonitor")
				}
				for _, forbidden := range forbiddenDependencies {
					if strings.Contains(payload, forbidden) {
						t.Errorf("%s uses the concrete graph source %q", file, forbidden)
					}
				}
			}
		})
	})
}
