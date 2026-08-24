// Package gosimilarity finds structurally duplicate Go functions and methods.
//
// Each function becomes a normalized syntax tree. The tree keeps the shape of
// the signature and of the body. It drops every identifier, every selector
// name, and every literal value. Two functions that differ only in their names
// therefore produce the same tree.
//
// One walk of that tree collects one fingerprint for the function and one for
// each nested node. The similarity of two functions is the Jaccard index of
// those two fingerprint sets:
//
//	score = shared fingerprints / all fingerprints of both functions
//
// A score of 1 means that both sets hold the same fingerprints. A lower score
// means that each function also holds structure that the other one misses.
//
// This package returns Go values only, so the caller owns the presentation.
package gosimilarity

import (
	"cmp"
	"context"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cgardev/goconduct/pkg/failure"
)

// Location is one function or one method inside one Go source file.
type Location struct {
	// File names the source file with forward slashes.
	File string
	// StartLine is the line of the func keyword.
	StartLine int
	// EndLine is the line of the closing brace of the body.
	EndLine int
}

// Candidate is one pair of functions that share normalized structure.
type Candidate struct {
	// Score is the Jaccard index of both fingerprint sets, from 0 to 1.
	Score float64
	// Left is the function that the walk found first.
	Left Location
	// Right is the function that the walk found second.
	Right Location
	// LeftNodes counts the normalized nodes of the left function.
	LeftNodes int
	// RightNodes counts the normalized nodes of the right function.
	RightNodes int
}

// Candidates reports every pair of Go functions that share normalized structure.
//
// Each path names one file or one directory, and the caller passes absolute
// paths. A directory contributes every Go file below it, test files included.
// The walk skips the .git, vendor, and target directories. It skips one of
// them also when the caller names it. All files join one comparison set, so a
// duplicate across two directories still appears.
//
// The threshold is the lowest Jaccard index of a reported pair, from 0 to 1.
// A function joins the comparison only when it holds minimumLines source lines
// and minimumNodes normalized nodes. Both minimums must be positive.
//
// The result ranks the most similar pair first and stays stable across runs.
func Candidates(
	ctx context.Context,
	paths []string,
	threshold float64,
	minimumLines int,
	minimumNodes int,
) ([]Candidate, error) {
	if err := validateLimits(threshold, minimumLines, minimumNodes); err != nil {
		return nil, err
	}
	files, err := sourceFiles(paths)
	if err != nil {
		return nil, err
	}
	functions, err := scanFiles(ctx, files, minimumLines, minimumNodes)
	if err != nil {
		return nil, err
	}
	return pairs(ctx, functions, threshold)
}

// validateLimits rejects a threshold or a minimum that no comparison accepts.
func validateLimits(threshold float64, minimumLines int, minimumNodes int) error {
	if threshold < 0 || threshold > 1 {
		return failure.Validation(fmt.Sprintf(
			"similarity threshold %.3f is outside 0 through 1",
			threshold,
		), nil)
	}
	if minimumLines <= 0 {
		return failure.Validation(fmt.Sprintf(
			"minimum source lines %d is not positive",
			minimumLines,
		), nil)
	}
	if minimumNodes <= 0 {
		return failure.Validation(fmt.Sprintf(
			"minimum normalized nodes %d is not positive",
			minimumNodes,
		), nil)
	}
	return nil
}

// skippedDirectories hold version control data, vendored dependencies, or build
// output. A duplicate inside them belongs to another author or to a generator.
var skippedDirectories = []string{".git", "target", "vendor"}

// sourceFiles lists the Go files of every selected path.
// It sorts the list and removes duplicates, so one file joins one comparison
// set exactly once even when two paths name it.
func sourceFiles(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, failure.Validation("the analysis path list is empty", nil)
	}
	files := make([]string, 0, len(paths))
	for _, path := range paths {
		found, err := filesUnder(path)
		if err != nil {
			return nil, err
		}
		files = append(files, found...)
	}
	slices.Sort(files)
	return slices.Compact(files), nil
}

// filesUnder lists the Go files of one path.
// A path that names another kind of file contributes nothing.
func filesUnder(path string) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, failure.Validation("an analysis path is empty", nil)
	}
	information, err := os.Stat(path)
	if err != nil {
		return nil, failure.Validation(fmt.Sprintf("inspect analysis path %q", path), err)
	}
	if !information.IsDir() {
		if !goFile(path) {
			return nil, nil
		}
		return []string{filepath.ToSlash(path)}, nil
	}
	files := make([]string, 0)
	walkError := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if entry.IsDir() {
			if slices.Contains(skippedDirectories, entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if goFile(current) {
			files = append(files, filepath.ToSlash(current))
		}
		return nil
	})
	if walkError != nil {
		return nil, failure.Unavailable(fmt.Sprintf("walk analysis path %q", path), walkError)
	}
	return files, nil
}

func goFile(path string) bool {
	return strings.HasSuffix(path, ".go")
}

// function is one analyzed function with its normalized fingerprints.
type function struct {
	location     Location
	nodes        int
	fingerprints []fingerprint
}

// scanFiles reads every file of the comparison set.
// The scan stops when the caller cancels the context, because one repository
// holds thousands of files.
func scanFiles(
	ctx context.Context,
	files []string,
	minimumLines int,
	minimumNodes int,
) ([]function, error) {
	functions := make([]function, 0, len(files))
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		found, err := functionsIn(file, minimumLines, minimumNodes)
		if err != nil {
			return nil, err
		}
		functions = append(functions, found...)
	}
	return functions, nil
}

// functionsIn reads every function and method that one Go file declares.
// It skips a declaration without a body, because that body lives elsewhere.
// It also skips a function that misses one of the two minimums.
func functionsIn(path string, minimumLines int, minimumNodes int) ([]function, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, failure.DataIntegrity(fmt.Sprintf("parse Go source %q", path), err)
	}
	functions := make([]function, 0, len(file.Decls))
	for _, declaration := range file.Decls {
		declared, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction || declared.Body == nil {
			continue
		}
		location := Location{
			File:      filepath.ToSlash(path),
			StartLine: fileSet.Position(declared.Pos()).Line,
			EndLine:   fileSet.Position(declared.End()).Line,
		}
		if location.EndLine-location.StartLine+1 < minimumLines {
			continue
		}
		root := normalizeFunction(declared)
		nodes := countShapes(root)
		if nodes < minimumNodes {
			continue
		}
		functions = append(functions, function{
			location:     location,
			nodes:        nodes,
			fingerprints: fingerprints(root),
		})
	}
	return functions, nil
}

// pairs compares every distinct pair of functions once.
// The Jaccard index is symmetric, so the second half of the matrix repeats the
// first half. The comparison stops when the caller cancels the context.
func pairs(ctx context.Context, functions []function, threshold float64) ([]Candidate, error) {
	candidates := make([]Candidate, 0)
	for index, left := range functions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, right := range functions[index+1:] {
			score := similarity(left.fingerprints, right.fingerprints)
			if score < threshold {
				continue
			}
			candidates = append(candidates, Candidate{
				Score:      score,
				Left:       left.location,
				Right:      right.location,
				LeftNodes:  left.nodes,
				RightNodes: right.nodes,
			})
		}
	}
	slices.SortStableFunc(candidates, compareCandidate)
	return candidates, nil
}

// compareCandidate ranks the most similar pair first.
// Two pairs with one score follow their source coordinates, so a reader finds
// the same order in every run.
func compareCandidate(left Candidate, right Candidate) int {
	if comparison := cmp.Compare(right.Score, left.Score); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(left.Left.File, right.Left.File); comparison != 0 {
		return comparison
	}
	if comparison := cmp.Compare(left.Left.StartLine, right.Left.StartLine); comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(left.Right.File, right.Right.File); comparison != 0 {
		return comparison
	}
	return cmp.Compare(left.Right.StartLine, right.Right.StartLine)
}

// similarity reports the Jaccard index of two sorted fingerprint sets.
// One merge walk counts the shared fingerprints, and the union is the sum of
// both sizes without them. Every tree holds its function node, so no set is
// empty and the division always has a divisor.
func similarity(left []fingerprint, right []fingerprint) float64 {
	shared := 0
	leftIndex := 0
	rightIndex := 0
	for leftIndex < len(left) && rightIndex < len(right) {
		comparison := compareFingerprint(left[leftIndex], right[rightIndex])
		if comparison < 0 {
			leftIndex++
			continue
		}
		if comparison > 0 {
			rightIndex++
			continue
		}
		shared++
		leftIndex++
		rightIndex++
	}
	return float64(shared) / float64(len(left)+len(right)-shared)
}

// shape is one normalized syntax node.
// The tag names the syntax role and the children keep the order of the source.
type shape struct {
	tag      string
	children []shape
}

// newShape creates one normalized node from its tag and its children.
func newShape(tag string, children ...shape) shape {
	return shape{tag: tag, children: children}
}

// normalizeFunction shapes one declaration from its signature and its body.
// A method also keeps its receiver, so a method and a plain function never
// share the root fingerprint. Every nested fingerprint still matches.
func normalizeFunction(declaration *ast.FuncDecl) shape {
	children := []shape{
		normalizeFieldList("params", declaration.Type.Params),
		normalizeFieldList("results", declaration.Type.Results),
	}
	if declaration.Recv != nil {
		children = append(children, normalizeFieldList("receiver", declaration.Recv))
	}
	return newShape("func", append(children, normalize(declaration.Body))...)
}

// normalizeFieldList shapes one parameter, result, receiver, or field group.
// One field declares several names, so each name adds one child. The number of
// children then states how many values the list declares.
func normalizeFieldList(tag string, fields *ast.FieldList) shape {
	if fields == nil {
		return newShape(tag)
	}
	children := make([]shape, 0, len(fields.List))
	for _, field := range fields.List {
		for range max(len(field.Names), 1) {
			children = append(children, newShape("field", normalize(field.Type)))
		}
	}
	return newShape(tag, children...)
}

// normalize shapes one syntax node.
// An absent clause keeps its place, so the structure still states that the
// syntax offers that clause.
func normalize(node ast.Node) shape {
	if node == nil {
		return newShape("nil")
	}
	if statement, isStatement := node.(ast.Stmt); isStatement {
		return normalizeStatement(statement)
	}
	if expression, isExpression := node.(ast.Expr); isExpression {
		return normalizeExpression(expression)
	}
	return newShape(fmt.Sprintf("%T", node))
}

// normalizeStatement shapes one statement and keeps its control structure.
// The operator of an assignment, of a branch, and of an increment stays in the
// tag, because it changes what the statement does.
func normalizeStatement(statement ast.Stmt) shape {
	switch typed := statement.(type) {
	case *ast.BlockStmt:
		return newShape("block", normalizeEach(typed.List)...)
	case *ast.IfStmt:
		return newShape(
			"if",
			normalize(typed.Init),
			normalize(typed.Cond),
			normalize(typed.Body),
			normalize(typed.Else),
		)
	case *ast.ForStmt:
		return newShape(
			"for",
			normalize(typed.Init),
			normalize(typed.Cond),
			normalize(typed.Post),
			normalize(typed.Body),
		)
	case *ast.RangeStmt:
		return newShape("range", normalize(typed.X), normalize(typed.Body))
	case *ast.SwitchStmt:
		return newShape("switch", normalize(typed.Init), normalize(typed.Tag), normalize(typed.Body))
	case *ast.TypeSwitchStmt:
		return newShape(
			"type-switch",
			normalize(typed.Init),
			normalize(typed.Assign),
			normalize(typed.Body),
		)
	case *ast.SelectStmt:
		return newShape("select", normalize(typed.Body))
	case *ast.CaseClause:
		return newShape(
			"case",
			newShape("case-list", normalizeEach(typed.List)...),
			newShape("case-body", normalizeEach(typed.Body)...),
		)
	case *ast.CommClause:
		return newShape(
			"comm",
			normalize(typed.Comm),
			newShape("comm-body", normalizeEach(typed.Body)...),
		)
	case *ast.AssignStmt:
		return newShape(
			"assign/"+typed.Tok.String(),
			newShape("lhs", normalizeEach(typed.Lhs)...),
			newShape("rhs", normalizeEach(typed.Rhs)...),
		)
	case *ast.DeclStmt:
		return newShape("decl", normalizeDeclaration(typed.Decl))
	case *ast.ExprStmt:
		return newShape("expr-stmt", normalize(typed.X))
	case *ast.ReturnStmt:
		return newShape("return", normalizeEach(typed.Results)...)
	case *ast.BranchStmt:
		return newShape("branch/" + typed.Tok.String())
	case *ast.GoStmt:
		return newShape("go", normalize(typed.Call))
	case *ast.DeferStmt:
		return newShape("defer", normalize(typed.Call))
	case *ast.SendStmt:
		return newShape("send", normalize(typed.Chan), normalize(typed.Value))
	case *ast.IncDecStmt:
		return newShape("incdec/"+typed.Tok.String(), normalize(typed.X))
	case *ast.LabeledStmt:
		return newShape("label", normalize(typed.Stmt))
	case *ast.EmptyStmt:
		return newShape("empty")
	default:
		return newShape(fmt.Sprintf("%T", statement))
	}
}

// normalizeExpression shapes one value expression.
// A type expression is also an expression, so an unmatched node continues to
// the type shapes.
func normalizeExpression(expression ast.Expr) shape {
	switch typed := expression.(type) {
	case *ast.BinaryExpr:
		return newShape("binary/"+typed.Op.String(), normalize(typed.X), normalize(typed.Y))
	case *ast.UnaryExpr:
		return newShape("unary/"+typed.Op.String(), normalize(typed.X))
	case *ast.CallExpr:
		return newShape("call", prepend(normalizeCallee(typed.Fun), normalizeEach(typed.Args))...)
	case *ast.SelectorExpr:
		return newShape("selector", normalize(typed.X), newShape("member"))
	case *ast.IndexExpr:
		return newShape("index", normalize(typed.X), normalize(typed.Index))
	case *ast.IndexListExpr:
		return newShape("index-list", prepend(normalize(typed.X), normalizeEach(typed.Indices))...)
	case *ast.SliceExpr:
		return newShape(
			"slice",
			normalize(typed.X),
			normalize(typed.Low),
			normalize(typed.High),
			normalize(typed.Max),
		)
	case *ast.StarExpr:
		return newShape("star", normalize(typed.X))
	case *ast.ParenExpr:
		return newShape("paren", normalize(typed.X))
	case *ast.CompositeLit:
		return newShape("composite", prepend(normalize(typed.Type), normalizeEach(typed.Elts))...)
	case *ast.KeyValueExpr:
		return newShape("key-value", normalize(typed.Key), normalize(typed.Value))
	case *ast.FuncLit:
		return newShape(
			"func-lit",
			normalizeFieldList("params", typed.Type.Params),
			normalizeFieldList("results", typed.Type.Results),
			normalize(typed.Body),
		)
	case *ast.TypeAssertExpr:
		return newShape("type-assert", normalize(typed.X), normalize(typed.Type))
	case *ast.Ident:
		// Go declares true, false, and nil as identifiers, so they arrive here
		// with every variable and type name. The comparison keeps their names,
		// because a test against nil states a different intent from a test
		// against another value. The published description of the method says
		// that every identifier loses its name, so this rule follows the
		// reference implementation rather than that description.
		if typed.Name == "true" || typed.Name == "false" || typed.Name == "nil" {
			return newShape("literal/" + typed.Name)
		}
		return newShape("ident")
	case *ast.BasicLit:
		// The kind of a literal states its type, which is structure. The written
		// value states data, which the comparison drops.
		return newShape("literal/" + typed.Kind.String())
	}
	return normalizeType(expression)
}

// normalizeType shapes one type expression.
// The length of an array and the direction of a channel stay out of the shape,
// because both state a value rather than a structure.
func normalizeType(expression ast.Expr) shape {
	switch typed := expression.(type) {
	case *ast.ArrayType:
		return newShape("array-type", normalize(typed.Elt))
	case *ast.MapType:
		return newShape("map-type", normalize(typed.Key), normalize(typed.Value))
	case *ast.StructType:
		return normalizeFieldList("struct-type", typed.Fields)
	case *ast.InterfaceType:
		return normalizeFieldList("interface-type", typed.Methods)
	case *ast.ChanType:
		return newShape("chan-type", normalize(typed.Value))
	case *ast.FuncType:
		return newShape(
			"func-type",
			normalizeFieldList("params", typed.Params),
			normalizeFieldList("results", typed.Results),
		)
	case *ast.Ellipsis:
		return newShape("ellipsis", normalize(typed.Elt))
	default:
		return newShape(fmt.Sprintf("%T", expression))
	}
}

// normalizeDeclaration shapes one declaration inside a function body.
// Only a general declaration reaches a body, and its keyword stays in the tag.
func normalizeDeclaration(declaration ast.Decl) shape {
	general, isGeneral := declaration.(*ast.GenDecl)
	if !isGeneral {
		return newShape("decl")
	}
	children := make([]shape, 0, len(general.Specs))
	for _, specification := range general.Specs {
		children = append(children, normalizeSpecification(specification))
	}
	return newShape("gen-decl/"+general.Tok.String(), children...)
}

// normalizeSpecification shapes one specification of a general declaration.
func normalizeSpecification(specification ast.Spec) shape {
	switch typed := specification.(type) {
	case *ast.ValueSpec:
		return newShape("value-spec", prepend(normalize(typed.Type), normalizeEach(typed.Values))...)
	case *ast.TypeSpec:
		return newShape("type-spec", normalize(typed.Type))
	default:
		return newShape("spec")
	}
}

// normalizeCallee shapes the called expression of one call.
// A plain call and a method call keep different tags, so the call shape stays
// after the loss of the name.
func normalizeCallee(expression ast.Expr) shape {
	switch typed := expression.(type) {
	case *ast.Ident:
		return newShape("callee")
	case *ast.SelectorExpr:
		return newShape("selector-callee", normalize(typed.X), newShape("member"))
	default:
		return normalize(expression)
	}
}

// normalizeEach shapes every node of one list and keeps the order of the source.
func normalizeEach[Node ast.Node](nodes []Node) []shape {
	shapes := make([]shape, 0, len(nodes))
	for _, node := range nodes {
		shapes = append(shapes, normalize(node))
	}
	return shapes
}

// prepend puts one shape before a list of shapes.
func prepend(first shape, rest []shape) []shape {
	return append([]shape{first}, rest...)
}

// countShapes counts the normalized nodes of one tree.
func countShapes(root shape) int {
	total := 1
	for _, child := range root.children {
		total += countShapes(child)
	}
	return total
}

// fingerprint identifies one normalized subtree.
type fingerprint [sha256.Size]byte

// fingerprints collects the fingerprint of one tree and of each nested node.
// The result is sorted and holds no duplicate, so two sets intersect with one
// merge walk. A fingerprint also has a fixed width, so one node of a large
// function needs a fixed amount of memory.
func fingerprints(root shape) []fingerprint {
	_, collected := fingerprintTree(root, nil)
	slices.SortFunc(collected, compareFingerprint)
	return slices.Compact(collected)
}

// fingerprintTree hashes one subtree and appends the fingerprint of every node
// it visits to collected. It returns the fingerprint of the subtree and the
// grown list.
//
// The message of one node holds the digest of its tag and then the fingerprint
// of each child. Every block of that message has the width of one digest, so
// one message describes one tag and one order of children only.
func fingerprintTree(current shape, collected []fingerprint) (fingerprint, []fingerprint) {
	tagDigest := sha256.Sum256([]byte(current.tag))
	message := tagDigest[:]
	for _, child := range current.children {
		var childDigest fingerprint
		childDigest, collected = fingerprintTree(child, collected)
		message = append(message, childDigest[:]...)
	}
	digest := fingerprint(sha256.Sum256(message))
	return digest, append(collected, digest)
}

// compareFingerprint orders two fingerprints by their bytes.
func compareFingerprint(left fingerprint, right fingerprint) int {
	return slices.Compare(left[:], right[:])
}
