package application

import (
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"
)

func TestLayerArchitecture_KeepApplicationTransportIndependent(t *testing.T) {
	testCases := []struct {
		file           string
		allowedImports []string
	}{
		{file: "ports.go", allowedImports: []string{"context"}},
		{
			file: "usecase.go",
			allowedImports: []string{
				"context",
				"fmt",
				"github.com/cgardev/goconduct/internal/failure",
			},
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: The application file is "+testCase.file, func(t *testing.T) {
			var imports []string
			var parseError error

			t.Run("Given a closed set of standard library dependencies", func(*testing.T) {
				imports = make([]string, 0)
			})

			t.Run("When the architecture test reads the file imports", func(*testing.T) {
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

			if !t.Run("Then the parser reads the application file", func(t *testing.T) {
				if parseError != nil {
					t.Fatalf("parse %s: %v", testCase.file, parseError)
				}
			}) {
				return
			}

			t.Run("And the application file has only domain and standard library dependencies", func(t *testing.T) {
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
