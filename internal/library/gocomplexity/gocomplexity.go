// Package gocomplexity measures the change risk of one Go function.
//
// Cyclomatic complexity counts the decision points of a function and adds one,
// as Thomas McCabe defines it. A function with no branch has a complexity of 1.
//
// The Change Risk Anti-Pattern score combines that complexity with statement
// coverage, as Alberto Savoia and Bob Evans define it:
//
//	CRAP(f) = CC(f)^2 * (1 - coverage(f))^3 + CC(f)
//
// The cube of the uncovered fraction makes an untested complex function grow
// much faster than a tested one. A fully covered function scores its own
// complexity, so coverage alone never hides complexity.
package gocomplexity

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/cgardev/goconduct/failure"
)

// Function is one declared Go function or method with its measured complexity.
type Function struct {
	Name       string
	Package    string
	StartLine  int
	EndLine    int
	Complexity int
}

// Functions reads every function and method that one Go file declares.
// It skips a declaration without a body, because that body lives elsewhere.
func Functions(path string) ([]Function, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, failure.DataIntegrity(fmt.Sprintf("parse Go source %q", path), err)
	}
	functions := make([]Function, 0, len(file.Decls))
	for _, declaration := range file.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction || function.Body == nil {
			continue
		}
		functions = append(functions, Function{
			Name:       functionName(function),
			Package:    file.Name.Name,
			StartLine:  fileSet.Position(function.Pos()).Line,
			EndLine:    fileSet.Position(function.End()).Line,
			Complexity: Cyclomatic(function),
		})
	}
	return functions, nil
}

// Cyclomatic counts the decision points of one function body and adds one.
// A case clause, a communication clause, and each of the operators "&&" and
// "||" open one more path through the function.
func Cyclomatic(function *ast.FuncDecl) int {
	complexity := 1
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if typed.Op == token.LAND || typed.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return complexity
}

// Score reports the Change Risk Anti-Pattern score of one function.
// The coverage is a percentage between 0 and 100.
func Score(complexity int, coveragePercent float64) float64 {
	count := float64(complexity)
	uncovered := 1 - coveragePercent/100
	return count*count*uncovered*uncovered*uncovered + count
}

// functionName names one method after its receiver type.
func functionName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return function.Name.Name
	}
	return receiverName(function.Recv.List[0].Type) + "." + function.Name.Name
}

// receiverName reads the type name of one method receiver.
// A receiver can be a pointer, a generic instance, or a qualified type.
func receiverName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return receiverName(typed.X)
	case *ast.IndexExpr:
		return receiverName(typed.X)
	case *ast.IndexListExpr:
		return receiverName(typed.X)
	case *ast.SelectorExpr:
		return receiverName(typed.X) + "." + typed.Sel.Name
	case *ast.ParenExpr:
		return receiverName(typed.X)
	default:
		return unknownReceiver
	}
}

// unknownReceiver names a receiver that the Go syntax permits but that no
// readable name describes. The original tool printed a syntax tree here, which
// made the report unreadable.
const unknownReceiver = "?"
