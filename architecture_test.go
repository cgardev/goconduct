package main

import (
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestLayerArchitecture_KeepPureFilesIndependent(t *testing.T) {
	testCases := []struct {
		file           string
		allowedImports []string
	}{
		{file: "domain.go", allowedImports: []string{}},
		{
			file:           "calculation.go",
			allowedImports: []string{"cmp", "math", "slices", "sort", "strings"},
		},
		{
			file:           "classification.go",
			allowedImports: []string{"fmt", "slices", "strings"},
		},
		{
			file:           "query.go",
			allowedImports: []string{"cmp", "errors", "fmt", "slices", "strings"},
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: The pure layer file is "+testCase.file, func(t *testing.T) {
			var imports []string
			var parseError error

			t.Run("Given the layer has a closed standard-library import set", func(t *testing.T) {
				imports = make([]string, 0)
			})

			t.Run("When the layer imports are inspected", func(t *testing.T) {
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

			if !t.Run("Then the source file can be parsed", func(t *testing.T) {
				if parseError != nil {
					t.Fatalf("parse %s: %v", testCase.file, parseError)
				}
			}) {
				return
			}

			t.Run("And the layer has no infrastructure or transport dependency", func(t *testing.T) {
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

func TestLayerArchitecture_KeepRepositoryLayoutInConfiguration(t *testing.T) {
	t.Run("Scenario: Core analysis files are inspected for repository-specific roots", func(t *testing.T) {
		var payloads map[string]string
		var readError error

		t.Run("Given analyzer, classification, and calculation source files", func(t *testing.T) {
			payloads = make(map[string]string)
		})

		t.Run("When their source text is read", func(t *testing.T) {
			for _, file := range []string{
				"analyzer.go",
				"classification.go",
				"calculation.go",
				"query.go",
			} {
				payload, err := os.ReadFile(file)
				if err != nil {
					readError = err
					return
				}
				payloads[file] = string(payload)
			}
		})

		if !t.Run("Then every core file can be inspected", func(t *testing.T) {
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
