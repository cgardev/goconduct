package report

import (
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestReportModel_KeepPackagePure(t *testing.T) {
	t.Run("Scenario: The report model has only a pure domain dependency", func(t *testing.T) {
		var imports []string
		var parseError error

		t.Run("Given the closed set of permitted imports", func(*testing.T) {
			imports = make([]string, 0)
		})

		t.Run("When the architecture test reads the package imports", func(*testing.T) {
			entries, err := os.ReadDir(".")
			if err != nil {
				parseError = err
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
					parseError = err
					return
				}
				for _, specification := range file.Imports {
					path, err := strconv.Unquote(specification.Path.Value)
					if err != nil {
						parseError = err
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

		if !t.Run("Then the parser reads the report package", func(t *testing.T) {
			if parseError != nil {
				t.Fatalf("read report package imports: %v", parseError)
			}
		}) {
			return
		}

		t.Run("And the report model imports only the pure architecture model", func(t *testing.T) {
			permitted := []string{
				"digginginsights.com/v3/internal/devtool/dependencygraph/internal/architecture",
			}
			if !slices.Equal(imports, permitted) {
				t.Errorf("model.go imports %v, want only %v", imports, permitted)
			}
		})
	})
}
