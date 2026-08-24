package gocomplexity

import (
	"errors"
	"go/ast"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/cgardev/goconduct/pkg/failure"
)

func writeSource(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.go")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write Go source: %v", err)
	}
	return path
}

func TestCyclomaticCountsEveryDecisionPoint(t *testing.T) {
	testCases := []struct {
		name   string
		body   string
		want   int
		reason string
	}{
		{name: "a function with no branch", body: "return 1", want: 1},
		{name: "one if statement", body: "if v > 0 {\n\treturn 1\n}\nreturn 0", want: 2},
		{name: "one for statement", body: "for i := 0; i < v; i++ {\n}\nreturn 0", want: 2},
		{name: "one range statement", body: "for range []int{1} {\n}\nreturn 0", want: 2},
		{
			name: "two case clauses",
			body: "switch v {\ncase 1:\n\treturn 1\ncase 2:\n\treturn 2\n}\nreturn 0",
			want: 3,
		},
		{name: "one logical and", body: "if v > 0 && v < 9 {\n\treturn 1\n}\nreturn 0", want: 3},
		{name: "one logical or", body: "if v > 0 || v < 9 {\n\treturn 1\n}\nreturn 0", want: 3},
		{name: "a nested if statement", body: "if v > 0 {\n\tif v > 9 {\n\t\treturn 2\n\t}\n}\nreturn 0", want: 3},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			path := writeSource(t, "package sample\n\nfunc Sample(v int) int {\n\t"+testCase.body+"\n}\n")

			functions, err := Functions(path)
			if err != nil {
				t.Fatalf("read functions: %v", err)
			}

			if len(functions) != 1 {
				t.Fatalf("the file declares %d functions, want one", len(functions))
			}
			if functions[0].Complexity != testCase.want {
				t.Errorf("complexity is %d, want %d", functions[0].Complexity, testCase.want)
			}
		})
	}
}

func TestCyclomaticCountsOneCommunicationClause(t *testing.T) {
	path := writeSource(t, "package sample\n\nfunc Sample(c chan int) int {\n"+
		"\tselect {\n\tcase v := <-c:\n\t\treturn v\n\t}\n}\n")

	functions, err := Functions(path)
	if err != nil {
		t.Fatalf("read functions: %v", err)
	}

	if functions[0].Complexity != 2 {
		t.Errorf("complexity is %d, want 2", functions[0].Complexity)
	}
}

func TestFunctionsNameEveryReceiverShape(t *testing.T) {
	path := writeSource(t, `package sample

type Widget struct{}

type Generic[T any] struct{}

func (w Widget) Value() int { return 1 }

func (w *Widget) Pointer() int { return 1 }

func (g Generic[T]) Instance() int { return 1 }

func Plain() int { return 1 }

func Declared()
`)

	functions, err := Functions(path)
	if err != nil {
		t.Fatalf("read functions: %v", err)
	}

	names := make([]string, 0, len(functions))
	for _, function := range functions {
		names = append(names, function.Name)
	}
	want := []string{"Widget.Value", "Widget.Pointer", "Generic.Instance", "Plain"}
	if len(names) != len(want) {
		t.Fatalf("the file declares %v, want %v", names, want)
	}
	for index, name := range want {
		if names[index] != name {
			t.Errorf("function %d is %q, want %q", index, names[index], name)
		}
	}
}

func TestFunctionsReportsThePackageAndTheLineRange(t *testing.T) {
	path := writeSource(t, "package sample\n\nfunc Sample() int {\n\treturn 1\n}\n")

	functions, err := Functions(path)
	if err != nil {
		t.Fatalf("read functions: %v", err)
	}

	if functions[0].Package != "sample" {
		t.Errorf("package is %q, want sample", functions[0].Package)
	}
	if functions[0].StartLine != 3 || functions[0].EndLine != 5 {
		t.Errorf("line range is %d-%d, want 3-5", functions[0].StartLine, functions[0].EndLine)
	}
}

func TestFunctionsClassifiesUnreadableSource(t *testing.T) {
	path := writeSource(t, "package sample\n\nfunc Broken( {\n")

	_, err := Functions(path)

	if !errors.Is(err, failure.ErrDataIntegrity) {
		t.Errorf("read error is %v, want a data integrity failure", err)
	}
}

func TestScoreAppliesTheChangeRiskFormula(t *testing.T) {
	testCases := []struct {
		name       string
		complexity int
		coverage   float64
		want       float64
	}{
		{name: "a fully covered function scores its complexity", complexity: 12, coverage: 100, want: 12},
		{name: "a simple covered function", complexity: 1, coverage: 100, want: 1},
		{name: "an untested simple function", complexity: 1, coverage: 0, want: 2},
		{name: "an untested complex function", complexity: 12, coverage: 0, want: 156},
		{name: "a partly covered function", complexity: 6, coverage: 50, want: 10.5},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			score := Score(testCase.complexity, testCase.coverage)

			if math.Abs(score-testCase.want) > 1e-9 {
				t.Errorf("score is %v, want %v", score, testCase.want)
			}
		})
	}
}

func TestScoreGrowsWithTheUncoveredFraction(t *testing.T) {
	previous := Score(10, 100)
	for _, coverage := range []float64{90, 70, 50, 20, 0} {
		score := Score(10, coverage)
		if score <= previous {
			t.Fatalf("coverage %v scores %v, which does not exceed %v", coverage, score, previous)
		}
		previous = score
	}
}

func TestReceiverNameReadsEveryTypeExpression(t *testing.T) {
	identifier := &ast.Ident{Name: "Widget"}
	testCases := []struct {
		name       string
		expression ast.Expr
		want       string
	}{
		{name: "a plain type", expression: identifier, want: "Widget"},
		{name: "a pointer type", expression: &ast.StarExpr{X: identifier}, want: "Widget"},
		{
			name:       "a generic instance",
			expression: &ast.IndexExpr{X: identifier, Index: &ast.Ident{Name: "T"}},
			want:       "Widget",
		},
		{
			name: "a generic instance with two parameters",
			expression: &ast.IndexListExpr{
				X:       identifier,
				Indices: []ast.Expr{&ast.Ident{Name: "A"}, &ast.Ident{Name: "B"}},
			},
			want: "Widget",
		},
		{
			name:       "a qualified type",
			expression: &ast.SelectorExpr{X: &ast.Ident{Name: "shop"}, Sel: &ast.Ident{Name: "Widget"}},
			want:       "shop.Widget",
		},
		{
			name:       "a parenthesized type",
			expression: &ast.ParenExpr{X: &ast.StarExpr{X: identifier}},
			want:       "Widget",
		},
		{
			name:       "a type expression with no readable name",
			expression: &ast.ArrayType{Elt: identifier},
			want:       unknownReceiver,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if name := receiverName(testCase.expression); name != testCase.want {
				t.Errorf("receiver name is %q, want %q", name, testCase.want)
			}
		})
	}
}

func TestFunctionsNamesAGenericMethodWithTwoParameters(t *testing.T) {
	path := writeSource(t, `package sample

type Pair[A any, B any] struct{}

func (p Pair[A, B]) Value() int { return 1 }
`)

	functions, err := Functions(path)
	if err != nil {
		t.Fatalf("read functions: %v", err)
	}

	if functions[0].Name != "Pair.Value" {
		t.Errorf("function name is %q, want Pair.Value", functions[0].Name)
	}
}
