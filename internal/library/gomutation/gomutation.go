// Package gomutation discovers the expressions of a Go file that one mutation
// can change, and applies one of those changes to the source text.
//
// Mutation testing measures whether a test suite detects a change of behavior.
// The analysis changes one expression, runs the suite, and reads the result. A
// suite that fails detects the change and kills the mutation. A suite that
// passes does not describe that behavior, so the mutation survives.
//
// A mutation over a line that no test reaches proves nothing, so the caller
// filters the sites by coverage before it runs them.
package gomutation

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"

	"github.com/cgardev/goconduct/internal/library/gocomplexity"
	"github.com/cgardev/goconduct/pkg/failure"
)

// Category names the kind of change one mutation makes.
type Category string

const (
	// CategoryArithmetic changes an arithmetic operator.
	CategoryArithmetic Category = "arithmetic"
	// CategoryComparison changes the boundary of an order comparison.
	CategoryComparison Category = "comparison"
	// CategoryEquality inverts an equality test.
	CategoryEquality Category = "equality"
	// CategoryBoolean inverts a boolean constant.
	CategoryBoolean Category = "boolean"
	// CategoryLogical exchanges the two logical operators.
	CategoryLogical Category = "logical"
	// CategoryConstant exchanges the constants zero and one.
	CategoryConstant Category = "constant"
)

// Site is one expression that a mutation can change.
type Site struct {
	Index    int
	Line     int
	Column   int
	Original string
	Mutant   string
	Category Category
	Function string

	startOffset int
	endOffset   int
}

// Describe reports the change of one site as source text.
func (site Site) Describe() string {
	return site.Original + " -> " + site.Mutant
}

// Apply returns the source with the expression of one site changed.
// The offsets come from the parse of that same source, so the caller passes
// the unchanged text of the file the site belongs to.
func (site Site) Apply(source string) string {
	if site.startOffset < 0 || site.endOffset > len(source) || site.startOffset > site.endOffset {
		return source
	}
	return source[:site.startOffset] + site.Mutant + source[site.endOffset:]
}

// Sites reads every mutation site of one Go file, ordered by position.
func Sites(path string) ([]Site, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, failure.Unavailable(fmt.Sprintf("read Go source %q", path), err)
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, failure.DataIntegrity(fmt.Sprintf("parse Go source %q", path), err)
	}
	functions, err := gocomplexity.Functions(path)
	if err != nil {
		return nil, err
	}
	sites := make([]Site, 0)
	ast.Inspect(file, func(node ast.Node) bool {
		sites = append(sites, nodeSites(fileSet, node)...)
		return true
	})
	slices.SortStableFunc(sites, comparePosition)
	for index := range sites {
		sites[index].Index = index
		sites[index].Function = functionAt(functions, sites[index].Line)
	}
	return sites, nil
}

// nodeSites reads the mutation sites of one syntax node.
func nodeSites(fileSet *token.FileSet, node ast.Node) []Site {
	switch typed := node.(type) {
	case *ast.BasicLit:
		return literalSites(fileSet, typed)
	case *ast.Ident:
		return identifierSites(fileSet, typed)
	case *ast.BinaryExpr:
		return operatorSites(fileSet, typed)
	}
	return nil
}

// literalSites exchanges the constants zero and one.
// No other integer literal has a change that keeps the source valid and small.
func literalSites(fileSet *token.FileSet, literal *ast.BasicLit) []Site {
	if literal.Kind != token.INT {
		return nil
	}
	mutants := map[string]string{"0": "1", "1": "0"}
	mutant, mutable := mutants[literal.Value]
	if !mutable {
		return nil
	}
	return []Site{newSite(fileSet, literal.Pos(), literal.End(), literal.Value, mutant, CategoryConstant)}
}

// identifierSites inverts the boolean constants.
func identifierSites(fileSet *token.FileSet, identifier *ast.Ident) []Site {
	mutants := map[string]string{"true": "false", "false": "true"}
	mutant, mutable := mutants[identifier.Name]
	if !mutable {
		return nil
	}
	return []Site{newSite(
		fileSet, identifier.Pos(), identifier.End(), identifier.Name, mutant, CategoryBoolean,
	)}
}

// operatorMutants maps each mutable operator to its change and its category.
var operatorMutants = map[token.Token]struct {
	mutant   string
	category Category
}{
	token.ADD:  {"-", CategoryArithmetic},
	token.SUB:  {"+", CategoryArithmetic},
	token.MUL:  {"/", CategoryArithmetic},
	token.GTR:  {">=", CategoryComparison},
	token.GEQ:  {">", CategoryComparison},
	token.LSS:  {"<=", CategoryComparison},
	token.LEQ:  {"<", CategoryComparison},
	token.EQL:  {"!=", CategoryEquality},
	token.NEQ:  {"==", CategoryEquality},
	token.LAND: {"||", CategoryLogical},
	token.LOR:  {"&&", CategoryLogical},
}

// operatorSites changes one binary operator.
// A comparison moves its boundary, and an equality or a logical operator
// inverts, so every change alters the behavior of a correct program.
func operatorSites(fileSet *token.FileSet, expression *ast.BinaryExpr) []Site {
	change, mutable := operatorMutants[expression.Op]
	if !mutable {
		return nil
	}
	original := expression.Op.String()
	end := expression.OpPos + token.Pos(len(original))
	return []Site{newSite(fileSet, expression.OpPos, end, original, change.mutant, change.category)}
}

func newSite(
	fileSet *token.FileSet,
	start token.Pos,
	end token.Pos,
	original string,
	mutant string,
	category Category,
) Site {
	position := fileSet.Position(start)
	return Site{
		Line: position.Line, Column: position.Column,
		startOffset: position.Offset, endOffset: fileSet.Position(end).Offset,
		Original: original, Mutant: mutant, Category: category,
	}
}

func comparePosition(left, right Site) int {
	if left.Line != right.Line {
		return left.Line - right.Line
	}
	return left.Column - right.Column
}

// functionAt names the function that holds one line.
func functionAt(functions []gocomplexity.Function, line int) string {
	for _, function := range functions {
		if line >= function.StartLine && line <= function.EndLine {
			return function.Name
		}
	}
	return ""
}
