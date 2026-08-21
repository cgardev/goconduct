package query

import (
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestLayerArchitecture_KeepQueryTransportIndependent(t *testing.T) {
	t.Run("Scenario: The query package has a closed dependency set", func(t *testing.T) {
		var imports []string
		var inspectError error

		t.Run("Given the permitted pure query dependencies", func(*testing.T) {
			imports = make([]string, 0)
		})

		t.Run("When the test inspects each production file", func(*testing.T) {
			entries, err := os.ReadDir(".")
			if err != nil {
				inspectError = err
				return
			}
			uniqueImports := make(map[string]struct{})
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
					strings.HasSuffix(entry.Name(), "_test.go") {
					continue
				}
				file, err := parser.ParseFile(
					token.NewFileSet(),
					entry.Name(),
					nil,
					parser.ImportsOnly,
				)
				if err != nil {
					inspectError = err
					return
				}
				for _, specification := range file.Imports {
					path, err := strconv.Unquote(specification.Path.Value)
					if err != nil {
						inspectError = err
						return
					}
					uniqueImports[path] = struct{}{}
				}
			}
			for path := range uniqueImports {
				imports = append(imports, path)
			}
			slices.Sort(imports)
		})

		if !t.Run("Then the test reads all query files", func(t *testing.T) {
			if inspectError != nil {
				t.Fatalf("inspect query imports: %v", inspectError)
			}
		}) {
			return
		}

		t.Run("And the query package has no transport dependency", func(t *testing.T) {
			permitted := []string{
				"cmp",
				"digginginsights.com/v3/internal/devtool/dependencygraph/internal/architecture",
				"digginginsights.com/v3/internal/devtool/dependencygraph/internal/report",
				"errors",
				"fmt",
				"slices",
				"strings",
			}
			if !slices.Equal(imports, permitted) {
				t.Errorf("query imports %v, want only %v", imports, permitted)
			}
		})
	})
}
