package main

import (
	"go/parser"
	"go/token"
	"slices"
	"strconv"
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
