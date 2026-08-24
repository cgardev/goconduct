package calculation

import (
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestLayerArchitecture_KeepCalculationPure(t *testing.T) {
	t.Run("Scenario: The calculation package has a closed dependency set", func(t *testing.T) {
		var imports []string
		var inspectError error

		t.Run("Given the pure model dependencies are permitted", func(*testing.T) {
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

		if !t.Run("Then the test reads all production files", func(t *testing.T) {
			if inspectError != nil {
				t.Fatalf("inspect calculation imports: %v", inspectError)
			}
		}) {
			return
		}

		t.Run("And the package imports only the permitted pure dependencies", func(t *testing.T) {
			permittedImports := []string{
				"cmp",
				"github.com/cgardev/goconduct/internal/architecture",
				"github.com/cgardev/goconduct/pkg/report",
				"math",
				"slices",
				"sort",
				"strconv",
				"strings",
			}
			if !slices.Equal(imports, permittedImports) {
				t.Errorf("calculation imports %v, want only %v", imports, permittedImports)
			}
		})
	})
}
