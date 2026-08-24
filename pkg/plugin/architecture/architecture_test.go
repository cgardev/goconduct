package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestLayerArchitecture_KeepCoreFilesWithinImportBoundaries(t *testing.T) {
	testCases := []struct {
		file           string
		allowedImports []string
	}{
		{
			file: "domain.go",
			allowedImports: []string{
				"github.com/cgardev/goconduct/pkg/report",
			},
		},
		{
			file: "calculation.go",
			allowedImports: []string{
				"github.com/cgardev/goconduct/internal/architecture",
				"github.com/cgardev/goconduct/internal/calculation",
				"sort",
			},
		},
		{
			file: "application.go",
			allowedImports: []string{
				"context",
				"github.com/cgardev/goconduct/internal/application",
			},
		},
		{
			file: "classification.go",
			allowedImports: []string{
				"fmt",
				"github.com/cgardev/goconduct/pkg/failure",
				"slices",
				"strings",
			},
		},
		{
			file: "functioncalculation.go",
			allowedImports: []string{
				"github.com/cgardev/goconduct/internal/calculation",
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
				"fmt",
				"github.com/cgardev/goconduct/pkg/failure",
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
					testSourcePath(testCase.file),
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

func repositoryRoot() string {
	return filepath.Join("..", "..", "..")
}

func testSourcePath(name string) string {
	if strings.HasPrefix(name, "internal/") || strings.HasPrefix(name, "cmd/") {
		return filepath.Join(repositoryRoot(), filepath.FromSlash(name))
	}
	return name
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
