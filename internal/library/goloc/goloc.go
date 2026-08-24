// Package goloc measures physical lines and declared functions in one Go file.
package goloc

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"strings"

	"github.com/cgardev/goconduct/pkg/failure"
)

// Lines contains one exclusive physical-line classification.
type Lines struct {
	Total   int
	Code    int
	Comment int
	Blank   int
}

// Function contains one declared Go function or method and its line counts.
type Function struct {
	Name      string
	StartLine int
	EndLine   int
	HasBody   bool
	Lines     Lines
}

// File contains the source measurements of one Go file.
type File struct {
	Package           string
	Header            string
	StandardGenerated bool
	Lines             Lines
	Functions         []Function
}

type lineContent struct {
	code    bool
	comment bool
}

// Analyze parses and measures one Go source file.
func Analyze(path string) (File, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return File{}, failure.Unavailable(fmt.Sprintf("read Go source %q", path), err)
	}
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(
		fileSet,
		path,
		source,
		parser.ParseComments|parser.SkipObjectResolution,
	)
	if err != nil {
		return File{}, failure.DataIntegrity(fmt.Sprintf("parse Go source %q", path), err)
	}
	contents := classifyLines(path, source)
	result := File{
		Package:           parsed.Name.Name,
		Header:            sourceHeader(fileSet, parsed, source),
		StandardGenerated: ast.IsGenerated(parsed),
		Lines:             summarizeLines(contents, 1, len(contents)),
		Functions:         declaredFunctions(fileSet, parsed, contents),
	}
	return result, nil
}

func classifyLines(path string, source []byte) []lineContent {
	contents := make([]lineContent, physicalLineCount(source))
	fileSet := token.NewFileSet()
	file := fileSet.AddFile(path, fileSet.Base(), len(source))
	var lexer scanner.Scanner
	lexer.Init(file, source, nil, scanner.ScanComments)
	for {
		position, item, literal := lexer.Scan()
		if item == token.EOF {
			break
		}
		if item == token.SEMICOLON {
			if literal == "\n" {
				continue
			}
		}
		startLine := file.Position(position).Line
		endLine := startLine + strings.Count(literal, "\n")
		if literal == "" {
			endLine = startLine
		}
		for line := startLine; line <= endLine && line <= len(contents); line++ {
			if item == token.COMMENT {
				contents[line-1].comment = true
				continue
			}
			contents[line-1].code = true
		}
	}
	return contents
}

func physicalLineCount(source []byte) int {
	if len(source) == 0 {
		return 0
	}
	lines := bytes.Count(source, []byte{'\n'})
	if source[len(source)-1] != '\n' {
		lines++
	}
	return lines
}

func summarizeLines(contents []lineContent, startLine, endLine int) Lines {
	if startLine < 1 || endLine < startLine || len(contents) == 0 {
		return Lines{}
	}
	endLine = min(endLine, len(contents))
	result := Lines{Total: endLine - startLine + 1}
	for _, content := range contents[startLine-1 : endLine] {
		switch {
		case content.code:
			result.Code++
		case content.comment:
			result.Comment++
		default:
			result.Blank++
		}
	}
	return result
}

func sourceHeader(fileSet *token.FileSet, parsed *ast.File, source []byte) string {
	file := fileSet.File(parsed.Package)
	if file == nil {
		return ""
	}
	offset := file.Offset(parsed.Package)
	return strings.ReplaceAll(string(source[:offset]), "\r\n", "\n")
}

func declaredFunctions(
	fileSet *token.FileSet,
	parsed *ast.File,
	contents []lineContent,
) []Function {
	functions := make([]Function, 0, len(parsed.Decls))
	for _, declaration := range parsed.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction {
			continue
		}
		startLine := fileSet.Position(function.Pos()).Line
		endLine := fileSet.Position(function.End()).Line
		functions = append(functions, Function{
			Name:      functionName(function),
			StartLine: startLine,
			EndLine:   endLine,
			HasBody:   function.Body != nil,
			Lines:     summarizeLines(contents, startLine, endLine),
		})
	}
	return functions
}

func functionName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return function.Name.Name
	}
	return receiverName(function.Recv.List[0].Type) + "." + function.Name.Name
}

func receiverName(expression ast.Expr) string {
	expression = ast.Unparen(expression)
	if identifier, isIdentifier := expression.(*ast.Ident); isIdentifier {
		return identifier.Name
	}
	return wrappedReceiverName(expression)
}

func wrappedReceiverName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.StarExpr:
		return receiverName(typed.X)
	case *ast.IndexExpr:
		return receiverName(typed.X)
	case *ast.IndexListExpr:
		return receiverName(typed.X)
	case *ast.SelectorExpr:
		return receiverName(typed.X) + "." + typed.Sel.Name
	default:
		return "?"
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-23T19:50:41Z","module_hash":"be07ff55490bef263d8f858a4b08adb9a69e5b17bab2b0277a5a777e524c0928","functions":[{"id":"func/Analyze","name":"Analyze","line":49,"end_line":73,"hash":"3f40186577d820c97bf7aafa4a0c99601c74c82d7f40acc6cec781bf806a7328"},{"id":"func/classifyLines","name":"classifyLines","line":75,"end_line":105,"hash":"aeb1a426e60d61135943ba4ea111c44b9b3199549eb188c95dfbd781b7a61f9e"},{"id":"func/physicalLineCount","name":"physicalLineCount","line":107,"end_line":116,"hash":"6b869eff9159572d29048f74e7580e6f80bc5d313a600a149662743a66a1afe4"},{"id":"func/summarizeLines","name":"summarizeLines","line":118,"end_line":135,"hash":"6e94fe67596acf3e335e231e23b1ff403367798f276e6f6dc35d01b58f1fec57"},{"id":"func/sourceHeader","name":"sourceHeader","line":137,"end_line":144,"hash":"4adedd5a6aff8ef4bc3c1e395a4ebbc1555ef32aa26f429bc0ddb4a5d97a4ec9"},{"id":"func/declaredFunctions","name":"declaredFunctions","line":146,"end_line":168,"hash":"dcf700802923493ab8f6d304e0ea42fa746c96b7d34393adc86a409b47a6f72f"},{"id":"func/functionName","name":"functionName","line":170,"end_line":175,"hash":"fb41dd412705f87fad1397314e87767b5e014835f800cc6ed104e648ab809fdf"},{"id":"func/receiverName","name":"receiverName","line":177,"end_line":183,"hash":"29c14ee03e798d83bff56e3bcda0b7e9d13deebb5922f705cccfe60f6128c60f"},{"id":"func/wrappedReceiverName","name":"wrappedReceiverName","line":185,"end_line":198,"hash":"64115cea364a901f43ca7654463f2f0329427f1bff5b649819d3f6a24216fc2d"}]}
// mutate4go-manifest-end
