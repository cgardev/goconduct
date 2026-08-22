package gomutation

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cgardev/goconduct/failure"
)

func writeSource(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.go")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write Go source: %v", err)
	}
	return path
}

func TestSitesCoverEveryMutationCategory(t *testing.T) {
	testCases := []struct {
		name       string
		expression string
		original   string
		mutant     string
		category   Category
	}{
		{name: "an addition", expression: "a + b", original: "+", mutant: "-", category: CategoryArithmetic},
		{name: "a subtraction", expression: "a - b", original: "-", mutant: "+", category: CategoryArithmetic},
		{name: "a product", expression: "a * b", original: "*", mutant: "/", category: CategoryArithmetic},
		{name: "a greater test", expression: "boolInt(a > b)", original: ">", mutant: ">=", category: CategoryComparison},
		{name: "a greater or equal test", expression: "boolInt(a >= b)", original: ">=", mutant: ">", category: CategoryComparison},
		{name: "a lower test", expression: "boolInt(a < b)", original: "<", mutant: "<=", category: CategoryComparison},
		{name: "a lower or equal test", expression: "boolInt(a <= b)", original: "<=", mutant: "<", category: CategoryComparison},
		{name: "an equality", expression: "boolInt(a == b)", original: "==", mutant: "!=", category: CategoryEquality},
		{name: "an inequality", expression: "boolInt(a != b)", original: "!=", mutant: "==", category: CategoryEquality},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			path := writeSource(t, "package sample\n\nfunc boolInt(v bool) int { return 0 }\n\n"+
				"func Sample(a int, b int) int {\n\treturn "+testCase.expression+"\n}\n")

			sites, err := Sites(path)
			if err != nil {
				t.Fatalf("read mutation sites: %v", err)
			}

			found := false
			for _, site := range sites {
				if site.Original != testCase.original {
					continue
				}
				found = true
				if site.Mutant != testCase.mutant {
					t.Errorf("mutant is %q, want %q", site.Mutant, testCase.mutant)
				}
				if site.Category != testCase.category {
					t.Errorf("category is %q, want %q", site.Category, testCase.category)
				}
			}
			if !found {
				t.Fatalf("no site changes %q: %+v", testCase.original, sites)
			}
		})
	}
}

func TestSitesInvertTheBooleanAndLogicalOperators(t *testing.T) {
	path := writeSource(t, "package sample\n\nfunc Sample(a bool, b bool) bool {\n"+
		"\tif a && b {\n\t\treturn true\n\t}\n\tif a || b {\n\t\treturn false\n\t}\n\treturn true\n}\n")

	sites, err := Sites(path)
	if err != nil {
		t.Fatalf("read mutation sites: %v", err)
	}

	changes := make(map[string]string, len(sites))
	for _, site := range sites {
		changes[site.Original] = site.Mutant
	}
	want := map[string]string{"&&": "||", "||": "&&", "true": "false", "false": "true"}
	for original, mutant := range want {
		if changes[original] != mutant {
			t.Errorf("%q changes to %q, want %q", original, changes[original], mutant)
		}
	}
}

func TestSitesExchangeTheConstantsZeroAndOne(t *testing.T) {
	path := writeSource(t, "package sample\n\nfunc Sample() int {\n\treturn 0 + 1 + 42\n}\n")

	sites, err := Sites(path)
	if err != nil {
		t.Fatalf("read mutation sites: %v", err)
	}

	constants := make([]string, 0)
	for _, site := range sites {
		if site.Category == CategoryConstant {
			constants = append(constants, site.Original+"->"+site.Mutant)
		}
	}
	if len(constants) != 2 {
		t.Fatalf("the source holds two mutable constants, the analysis reports %v", constants)
	}
}

func TestSitesIgnoreALiteralWithNoUsefulChange(t *testing.T) {
	path := writeSource(t, "package sample\n\nfunc Sample() string {\n\treturn \"text\"\n}\n")

	sites, err := Sites(path)
	if err != nil {
		t.Fatalf("read mutation sites: %v", err)
	}

	if len(sites) != 0 {
		t.Errorf("a string literal reports %+v, want no site", sites)
	}
}

func TestSitesOrderByPositionAndNumberFromZero(t *testing.T) {
	path := writeSource(t, "package sample\n\nfunc Sample(a int, b int) int {\n"+
		"\tc := a + b\n\td := a * b\n\treturn c - d\n}\n")

	sites, err := Sites(path)
	if err != nil {
		t.Fatalf("read mutation sites: %v", err)
	}

	if len(sites) != 3 {
		t.Fatalf("the source holds three operators, the analysis reports %d", len(sites))
	}
	for index, site := range sites {
		if site.Index != index {
			t.Errorf("site %d carries index %d", index, site.Index)
		}
		if index > 0 && comparePosition(sites[index-1], site) > 0 {
			t.Errorf("site %d comes before site %d", index, index-1)
		}
	}
}

func TestSitesNameTheFunctionThatHoldsTheExpression(t *testing.T) {
	path := writeSource(t, "package sample\n\ntype Widget struct{}\n\n"+
		"func (w Widget) Method(a int) int {\n\treturn a + 1\n}\n\n"+
		"func Plain(a int) int {\n\treturn a * 2\n}\n")

	sites, err := Sites(path)
	if err != nil {
		t.Fatalf("read mutation sites: %v", err)
	}

	names := make(map[string]string, len(sites))
	for _, site := range sites {
		names[site.Original] = site.Function
	}
	if names["+"] != "Widget.Method" {
		t.Errorf("the addition belongs to %q, want Widget.Method", names["+"])
	}
	if names["*"] != "Plain" {
		t.Errorf("the product belongs to %q, want Plain", names["*"])
	}
}

func TestSitesLeaveTheFunctionEmptyOutsideEveryBody(t *testing.T) {
	path := writeSource(t, "package sample\n\nvar Total = 1 + 2\n")

	sites, err := Sites(path)
	if err != nil {
		t.Fatalf("read mutation sites: %v", err)
	}

	if len(sites) == 0 {
		t.Fatal("a package level expression reports no site")
	}
	if sites[0].Function != "" {
		t.Errorf("a package level site belongs to %q, want no function", sites[0].Function)
	}
}

func TestApplyChangesOnlyTheExpressionOfTheSite(t *testing.T) {
	source := "package sample\n\nfunc Sample(a int, b int) int {\n\treturn a + b\n}\n"
	path := writeSource(t, source)
	sites, err := Sites(path)
	if err != nil {
		t.Fatalf("read mutation sites: %v", err)
	}

	mutated := sites[0].Apply(source)

	want := "package sample\n\nfunc Sample(a int, b int) int {\n\treturn a - b\n}\n"
	if mutated != want {
		t.Errorf("the mutated source is %q, want %q", mutated, want)
	}
}

func TestApplyKeepsSourceThatDoesNotHoldTheSite(t *testing.T) {
	site := Site{startOffset: 10, endOffset: 20, Mutant: "-"}

	if mutated := site.Apply("short"); mutated != "short" {
		t.Errorf("the analysis changed unrelated source into %q", mutated)
	}
}

func TestDescribeReportsTheChange(t *testing.T) {
	site := Site{Original: ">", Mutant: ">="}

	if description := site.Describe(); description != "> -> >=" {
		t.Errorf("description is %q", description)
	}
}

func TestSitesClassifyAnUnreadableFile(t *testing.T) {
	_, err := Sites(filepath.Join(t.TempDir(), "absent.go"))

	if !errors.Is(err, failure.ErrUnavailable) {
		t.Errorf("read error is %v, want an unavailable failure", err)
	}
}

func TestSitesClassifyAMalformedFile(t *testing.T) {
	path := writeSource(t, "package sample\n\nfunc Broken( {\n")

	_, err := Sites(path)

	if !errors.Is(err, failure.ErrDataIntegrity) {
		t.Errorf("parse error is %v, want a data integrity failure", err)
	}
}

func TestApplyGuardsEveryOffsetBoundary(t *testing.T) {
	const source = "abcdef"
	testCases := []struct {
		name string
		site Site
		want string
	}{
		{
			name: "a site at the start of the source",
			site: Site{startOffset: 0, endOffset: 1, Mutant: "Z"},
			want: "Zbcdef",
		},
		{
			name: "a site that ends at the end of the source",
			site: Site{startOffset: 5, endOffset: 6, Mutant: "Z"},
			want: "abcdeZ",
		},
		{
			name: "a site with an empty range",
			site: Site{startOffset: 3, endOffset: 3, Mutant: "Z"},
			want: "abcZdef",
		},
		{
			name: "a site that ends past the source",
			site: Site{startOffset: 3, endOffset: 7, Mutant: "Z"},
			want: source,
		},
		{
			name: "a site with a negative start",
			site: Site{startOffset: -1, endOffset: 2, Mutant: "Z"},
			want: source,
		},
		{
			name: "a site whose start follows its end",
			site: Site{startOffset: 4, endOffset: 2, Mutant: "Z"},
			want: source,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if mutated := testCase.site.Apply(source); mutated != testCase.want {
				t.Errorf("the mutated source is %q, want %q", mutated, testCase.want)
			}
		})
	}
}

func TestComparePositionOrdersByLineThenColumn(t *testing.T) {
	testCases := []struct {
		name     string
		left     Site
		right    Site
		negative bool
		zero     bool
	}{
		{
			name: "a lower line comes first",
			left: Site{Line: 4, Column: 9}, right: Site{Line: 7, Column: 1}, negative: true,
		},
		{
			name: "a higher line comes last",
			left: Site{Line: 7, Column: 1}, right: Site{Line: 4, Column: 9},
		},
		{
			name: "a lower column comes first on the same line",
			left: Site{Line: 4, Column: 1}, right: Site{Line: 4, Column: 5}, negative: true,
		},
		{
			name: "a higher column comes last on the same line",
			left: Site{Line: 4, Column: 5}, right: Site{Line: 4, Column: 1},
		},
		{
			name: "the same position compares equal",
			left: Site{Line: 4, Column: 5}, right: Site{Line: 4, Column: 5}, zero: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			comparison := comparePosition(testCase.left, testCase.right)
			switch {
			case testCase.zero && comparison != 0:
				t.Errorf("comparison is %d, want zero", comparison)
			case testCase.negative && comparison >= 0:
				t.Errorf("comparison is %d, want a negative value", comparison)
			case !testCase.zero && !testCase.negative && comparison <= 0:
				t.Errorf("comparison is %d, want a positive value", comparison)
			}
		})
	}
}

func TestFunctionAtIncludesBothBoundaryLines(t *testing.T) {
	path := writeSource(t, "package sample\n\nfunc OneLine(a int) int { return a + 1 }\n")

	sites, err := Sites(path)
	if err != nil {
		t.Fatalf("read mutation sites: %v", err)
	}

	if len(sites) == 0 {
		t.Fatal("a one line function reports no site")
	}
	for _, site := range sites {
		if site.Function != "OneLine" {
			t.Errorf("a site on the only line of the function belongs to %q", site.Function)
		}
	}
}

func TestFunctionAtLeavesASiteOutsideEveryRangeEmpty(t *testing.T) {
	path := writeSource(t, "package sample\n\nvar Total = 1 + 2\n\nfunc Later(a int) int {\n\treturn a\n}\n")

	sites, err := Sites(path)
	if err != nil {
		t.Fatalf("read mutation sites: %v", err)
	}

	if sites[0].Function != "" {
		t.Errorf("a site before every function belongs to %q", sites[0].Function)
	}
}
