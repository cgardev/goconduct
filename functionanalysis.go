package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

func (analyzer *analyzer) inspectFunctions(
	modulePath string,
	sourcePaths []string,
) ([]functionDeclaration, []functionReference, error) {
	fileSet := token.NewFileSet()
	loadedPackages, err := packages.Load(&packages.Config{
		Mode:  packages.LoadSyntax | packages.NeedForTest,
		Dir:   analyzer.repositoryRoot,
		Fset:  fileSet,
		Tests: true,
	}, functionPackageQueries(sourcePaths)...)
	if err != nil {
		return nil, nil, fmt.Errorf("load Go type information: %w", err)
	}
	slices.SortFunc(loadedPackages, func(first, second *packages.Package) int {
		return strings.Compare(first.ID, second.ID)
	})

	selectedPaths := absolutePathSet(sourcePaths)
	inspectedPaths := make(stringSet)
	var declarations []functionDeclaration
	var references []functionReference
	for _, loadedPackage := range loadedPackages {
		packageDeclarations, packageReferences, inspectError := analyzer.inspectLoadedPackage(
			modulePath,
			loadedPackage,
			fileSet,
			selectedPaths,
			inspectedPaths,
		)
		if inspectError != nil {
			return nil, nil, inspectError
		}
		declarations = append(declarations, packageDeclarations...)
		references = append(references, packageReferences...)
	}
	return declarations, references, nil
}

func functionPackageQueries(sourcePaths []string) []string {
	filesByDirectory := make(map[string]string)
	for _, sourcePath := range sourcePaths {
		directory := filepath.Dir(sourcePath)
		current, exists := filesByDirectory[directory]
		if !exists || sourcePath < current {
			filesByDirectory[directory] = sourcePath
		}
	}
	directories := sortedMapKeys(filesByDirectory)
	queries := make([]string, 0, len(directories))
	for _, directory := range directories {
		queries = append(queries, "file="+filesByDirectory[directory])
	}
	return queries
}

func absolutePathSet(paths []string) stringSet {
	result := make(stringSet, len(paths))
	for _, sourcePath := range paths {
		result.add(filepath.Clean(sourcePath))
	}
	return result
}

func (analyzer *analyzer) inspectLoadedPackage(
	modulePath string,
	loadedPackage *packages.Package,
	fileSet *token.FileSet,
	selectedPaths stringSet,
	inspectedPaths stringSet,
) ([]functionDeclaration, []functionReference, error) {
	if loadedPackage.TypesInfo == nil || strings.HasSuffix(loadedPackage.ID, ".test") {
		return nil, nil, nil
	}
	var declarations []functionDeclaration
	var references []functionReference
	for _, file := range loadedPackage.Syntax {
		absolutePath := filepath.Clean(physicalSourcePosition(fileSet, file.Pos()).Filename)
		if !selectedPaths.contains(absolutePath) || inspectedPaths.contains(absolutePath) {
			continue
		}
		if !functionPackageOwnsFile(loadedPackage, absolutePath) {
			continue
		}
		inspectedPaths.add(absolutePath)
		fileDeclarations, fileReferences, err := analyzer.inspectFunctionFile(
			modulePath,
			loadedPackage,
			fileSet,
			file,
			selectedPaths,
		)
		if err != nil {
			return nil, nil, err
		}
		declarations = append(declarations, fileDeclarations...)
		references = append(references, fileReferences...)
	}
	return declarations, references, nil
}

func functionPackageOwnsFile(loadedPackage *packages.Package, absolutePath string) bool {
	isTest := strings.HasSuffix(absolutePath, "_test.go")
	if isTest {
		return loadedPackage.ForTest != ""
	}
	return loadedPackage.ForTest == ""
}

func (analyzer *analyzer) inspectFunctionFile(
	modulePath string,
	loadedPackage *packages.Package,
	fileSet *token.FileSet,
	file *ast.File,
	selectedPaths stringSet,
) ([]functionDeclaration, []functionReference, error) {
	declarations, err := analyzer.functionDefinitions(
		modulePath,
		loadedPackage,
		fileSet,
		file,
		selectedPaths,
	)
	if err != nil {
		return nil, nil, err
	}
	declarationsByPosition := declarationsAtPositions(declarations)
	var references []functionReference
	for _, sourceDeclaration := range file.Decls {
		switch declaration := sourceDeclaration.(type) {
		case *ast.FuncDecl:
			source, exists := declarationsByPosition[declaration.Name.Pos()]
			if !exists || declaration.Body == nil {
				continue
			}
			targets, calls, inspectError := analyzer.callsInNode(
				modulePath,
				loadedPackage,
				fileSet,
				declaration.Body,
				source,
				selectedPaths,
			)
			if inspectError != nil {
				return nil, nil, inspectError
			}
			declarations = append(declarations, targets...)
			references = append(references, calls...)
		case *ast.GenDecl:
			if declaration.Tok != token.VAR {
				continue
			}
			packageDeclarations, packageCalls, inspectError := analyzer.inspectPackageInitializers(
				modulePath,
				loadedPackage,
				fileSet,
				declaration,
				selectedPaths,
			)
			if inspectError != nil {
				return nil, nil, inspectError
			}
			declarations = append(declarations, packageDeclarations...)
			references = append(references, packageCalls...)
		}
	}
	return declarations, references, nil
}

func declarationsAtPositions(declarations []functionDeclaration) map[token.Pos]functionDeclaration {
	positions := make(map[token.Pos]functionDeclaration, len(declarations))
	for _, declaration := range declarations {
		positions[token.Pos(declaration.sourcePosition)] = declaration
	}
	return positions
}

func (analyzer *analyzer) functionDefinitions(
	modulePath string,
	loadedPackage *packages.Package,
	fileSet *token.FileSet,
	file *ast.File,
	selectedPaths stringSet,
) ([]functionDeclaration, error) {
	declarations := make([]functionDeclaration, 0)
	for identifier, object := range loadedPackage.TypesInfo.Defs {
		function, ok := object.(*types.Func)
		if !ok || identifier.Pos() < file.Pos() || identifier.Pos() > file.End() {
			continue
		}
		declaration, classified, err := analyzer.functionDeclarationFromObject(
			modulePath,
			function,
			fileSet,
			selectedPaths,
		)
		if err != nil {
			return nil, err
		}
		if classified {
			declaration.sourcePosition = int(identifier.Pos())
			declarations = append(declarations, declaration)
		}
	}
	return declarations, nil
}

func (analyzer *analyzer) callsInNode(
	modulePath string,
	loadedPackage *packages.Package,
	fileSet *token.FileSet,
	node ast.Node,
	source functionDeclaration,
	selectedPaths stringSet,
) ([]functionDeclaration, []functionReference, error) {
	var declarations []functionDeclaration
	var references []functionReference
	for current := range ast.Preorder(node) {
		call, ok := current.(*ast.CallExpr)
		if !ok {
			continue
		}
		targetObject := calledFunction(loadedPackage.TypesInfo, call.Fun)
		if targetObject == nil {
			continue
		}
		target, classified, err := analyzer.functionDeclarationFromObject(
			modulePath,
			targetObject,
			fileSet,
			selectedPaths,
		)
		if err != nil {
			return declarations, references, err
		}
		if !classified {
			continue
		}
		position := physicalSourcePosition(fileSet, call.Pos())
		declarations = append(declarations, target)
		references = append(references, functionReference{
			source: source.identifier,
			target: target.identifier,
			test:   source.test,
			site: CallSite{
				Path:   source.relativePath,
				Line:   position.Line,
				Column: position.Column,
			},
		})
	}
	return declarations, references, nil
}

func calledFunction(typeInformation *types.Info, expression ast.Expr) *types.Func {
	switch function := unwrapFunctionExpression(expression).(type) {
	case *ast.Ident:
		object, _ := typeInformation.ObjectOf(function).(*types.Func)
		return object
	case *ast.SelectorExpr:
		selection := typeInformation.Selections[function]
		if selection != nil {
			object, _ := selection.Obj().(*types.Func)
			return object
		}
		object, _ := typeInformation.ObjectOf(function.Sel).(*types.Func)
		return object
	default:
		return nil
	}
}

func unwrapFunctionExpression(expression ast.Expr) ast.Expr {
	for {
		switch current := expression.(type) {
		case *ast.ParenExpr:
			expression = current.X
		case *ast.IndexExpr:
			expression = current.X
		case *ast.IndexListExpr:
			expression = current.X
		default:
			return expression
		}
	}
}

func (analyzer *analyzer) inspectPackageInitializers(
	modulePath string,
	loadedPackage *packages.Package,
	fileSet *token.FileSet,
	declaration *ast.GenDecl,
	selectedPaths stringSet,
) ([]functionDeclaration, []functionReference, error) {
	source, classified := analyzer.packageInitializerDeclaration(
		modulePath,
		loadedPackage,
		fileSet,
		declaration,
	)
	if !classified {
		return nil, nil, nil
	}
	targets, calls, err := analyzer.callsInNode(
		modulePath,
		loadedPackage,
		fileSet,
		declaration,
		source,
		selectedPaths,
	)
	if err != nil || len(calls) == 0 {
		return targets, calls, err
	}
	return append([]functionDeclaration{source}, targets...), calls, nil
}

func (analyzer *analyzer) packageInitializerDeclaration(
	modulePath string,
	loadedPackage *packages.Package,
	fileSet *token.FileSet,
	declaration *ast.GenDecl,
) (functionDeclaration, bool) {
	packagePath, local := localPackagePath(modulePath, loadedPackage.PkgPath)
	if !local {
		return functionDeclaration{}, false
	}
	position := physicalSourcePosition(fileSet, declaration.Pos())
	relativePath, insideRepository := analyzer.relativeSourcePath(position.Filename)
	if !insideRepository {
		return functionDeclaration{}, false
	}
	component, classified := analyzer.classifier.classify(relativePath)
	if !classified {
		return functionDeclaration{}, false
	}
	return functionDeclaration{
		identifier:      packagePath + ".<package-init>@" + relativePath,
		name:            "<package-init>",
		packagePath:     packagePath,
		component:       component.identifier,
		relativePath:    relativePath,
		line:            position.Line,
		test:            strings.HasSuffix(relativePath, "_test.go"),
		synthetic:       true,
		inAnalysisScope: true,
	}, true
}

func (analyzer *analyzer) functionDeclarationFromObject(
	modulePath string,
	function *types.Func,
	fileSet *token.FileSet,
	selectedPaths stringSet,
) (functionDeclaration, bool, error) {
	function = function.Origin()
	if function.Pkg() == nil {
		return functionDeclaration{}, false, nil
	}
	packagePath, local := localPackagePath(modulePath, function.Pkg().Path())
	if !local {
		return functionDeclaration{}, false, nil
	}
	position := physicalSourcePosition(fileSet, function.Pos())
	relativePath, insideRepository := analyzer.relativeSourcePath(position.Filename)
	classificationPath := packagePath
	if insideRepository {
		classificationPath = relativePath
	}
	ignored, err := analyzer.ignoredPaths.matches(classificationPath)
	if err != nil {
		return functionDeclaration{}, false, err
	}
	if ignored {
		return functionDeclaration{}, false, nil
	}
	component, classified := analyzer.classifier.classify(classificationPath)
	if !classified {
		return functionDeclaration{}, false, nil
	}
	receiver := functionReceiver(function)
	identifier := functionIdentifier(packagePath, receiver, function.Name(), relativePath, position.Line)
	name := function.Name()
	if receiver != "" {
		name = receiver + "." + name
	}
	absolutePath := filepath.Clean(position.Filename)
	return functionDeclaration{
		identifier:      identifier,
		name:            name,
		packagePath:     packagePath,
		component:       component.identifier,
		relativePath:    relativePath,
		line:            position.Line,
		receiver:        receiver,
		method:          receiver != "",
		exported:        function.Exported(),
		test:            strings.HasSuffix(relativePath, "_test.go"),
		inAnalysisScope: selectedPaths.contains(absolutePath),
	}, true, nil
}

func localPackagePath(modulePath, packagePath string) (string, bool) {
	if packagePath == modulePath {
		return ".", true
	}
	prefix := modulePath + "/"
	if !strings.HasPrefix(packagePath, prefix) {
		return "", false
	}
	return strings.TrimPrefix(packagePath, prefix), true
}

func functionReceiver(function *types.Func) string {
	signature, ok := function.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return ""
	}
	receiverType := types.Unalias(signature.Recv().Type())
	if pointer, ok := receiverType.(*types.Pointer); ok {
		receiverType = types.Unalias(pointer.Elem())
	}
	if named, ok := receiverType.(*types.Named); ok {
		return named.Obj().Name()
	}
	return strings.TrimPrefix(types.TypeString(receiverType, packageName), "*")
}

func packageName(packageValue *types.Package) string {
	if packageValue == nil {
		return ""
	}
	return packageValue.Name()
}

func functionIdentifier(packagePath, receiver, name, relativePath string, line int) string {
	identifier := packagePath + "."
	if receiver != "" {
		identifier += receiver + "."
	}
	identifier += name
	if name == "init" {
		identifier += "@" + relativePath + ":" + strconv.Itoa(line)
	}
	return identifier
}

func (analyzer *analyzer) relativeSourcePath(sourcePath string) (string, bool) {
	if sourcePath == "" {
		return "", false
	}
	relativePath, err := filepath.Rel(analyzer.repositoryRoot, sourcePath)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(relativePath), true
}

func physicalSourcePosition(fileSet *token.FileSet, position token.Pos) token.Position {
	sourceFile := fileSet.File(position)
	if sourceFile == nil {
		return token.Position{}
	}
	offset := sourceFile.Offset(position)
	lineOffsets := sourceFile.Lines()
	lineIndex := sort.Search(len(lineOffsets), func(index int) bool {
		return lineOffsets[index] > offset
	}) - 1
	line := lineIndex + 1
	return token.Position{
		Filename: sourceFile.Name(),
		Offset:   offset,
		Line:     line,
		Column:   offset - lineOffsets[lineIndex] + 1,
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T10:40:04Z","module_hash":"4bf15b4203e5f4914bf8b06993da34fc2131d9f9ed3fc50f6d3ca50f688daf67","functions":[{"id":"func/analyzer.inspectFunctions","name":"analyzer.inspectFunctions","line":17,"end_line":54,"hash":"b920c358367eccf41a365b290d4216b4bb2299c573f7f93b839bbdbcf59dddf7"},{"id":"func/functionPackageQueries","name":"functionPackageQueries","line":56,"end_line":71,"hash":"a5e077580e8cc89a2c8d11318f560e84e4e9605e1692e2515b4ebcd98d35ff82"},{"id":"func/absolutePathSet","name":"absolutePathSet","line":73,"end_line":79,"hash":"b435ed48604e786de8925ffe2bc3a2cd435473c75657266a9679dbe0020c3562"},{"id":"func/analyzer.inspectLoadedPackage","name":"analyzer.inspectLoadedPackage","line":81,"end_line":116,"hash":"d366008304ae01d70f89884108d1b2c1599960b02cc1d50dbcfb67f80384ca3a"},{"id":"func/functionPackageOwnsFile","name":"functionPackageOwnsFile","line":118,"end_line":124,"hash":"8baf30bbd99f9916dba41d589951b282364d5b216c0f30fd32914fd65d6e028b"},{"id":"func/analyzer.inspectFunctionFile","name":"analyzer.inspectFunctionFile","line":126,"end_line":184,"hash":"a71f8a2c0116a0a11c867e69dc46e3c53bb6f8c376290393b6b2ee31b0d7d05d"},{"id":"func/declarationsAtPositions","name":"declarationsAtPositions","line":186,"end_line":192,"hash":"72cc26a02c26f6f509017f78da9358d9cae0eb201b32d364982e8ebcf30c6296"},{"id":"func/analyzer.functionDefinitions","name":"analyzer.functionDefinitions","line":194,"end_line":222,"hash":"31fd1c6dfc2f21e5c15fc25ce20d43c211c151f05876b9033680e69e60c28f17"},{"id":"func/analyzer.callsInNode","name":"analyzer.callsInNode","line":224,"end_line":269,"hash":"95c4517435c35d03b65c584979704f6415072d9d36456ee46a39597f18d4c3fc"},{"id":"func/calledFunction","name":"calledFunction","line":271,"end_line":287,"hash":"aa1a6907c04f3e9d8bac582fa0f10eacf8802b09c89cfa9f3d0150792cfdcf82"},{"id":"func/unwrapFunctionExpression","name":"unwrapFunctionExpression","line":289,"end_line":302,"hash":"c0b6788b63d8de47f7d47d55b4a65262b9572626614aa42c2c6d9bd8d59bd94f"},{"id":"func/analyzer.inspectPackageInitializers","name":"analyzer.inspectPackageInitializers","line":304,"end_line":332,"hash":"9597ecb40da71afd4fa33f2d264a9eab9e27eda9aadf29dce749596a7b9184f8"},{"id":"func/analyzer.packageInitializerDeclaration","name":"analyzer.packageInitializerDeclaration","line":334,"end_line":364,"hash":"2734967e9120fdf911b6c921da4bf4def560bc1daf21ef742a88b28a597a1aae"},{"id":"func/analyzer.functionDeclarationFromObject","name":"analyzer.functionDeclarationFromObject","line":366,"end_line":417,"hash":"dc66a15af5f8ff8ee10babc00ab586e2ace32f28a23837cabbae9c7236d2d973"},{"id":"func/localPackagePath","name":"localPackagePath","line":419,"end_line":428,"hash":"23f0e4b39a37fd495ec4760eb7489a64840108d33326439d68fd828061264c4d"},{"id":"func/functionReceiver","name":"functionReceiver","line":430,"end_line":443,"hash":"c0f4b9b7cbc8c76f5da4d93c6c48d63263959b8146a7bd8d20a596296cef4057"},{"id":"func/packageName","name":"packageName","line":445,"end_line":450,"hash":"c4b4868b0d37a96ecacd72d16c66fd07cee089c3234149b6322b216b42b11d9b"},{"id":"func/functionIdentifier","name":"functionIdentifier","line":452,"end_line":462,"hash":"9006cdce7a6490a864a659c72b02d0c6814552dffe83fbfc48dba713dd22e79c"},{"id":"func/analyzer.relativeSourcePath","name":"analyzer.relativeSourcePath","line":464,"end_line":473,"hash":"ac46a771087acaf040416e5e3f4348ec54c990a6a041d1d66d8516be99b01f75"},{"id":"func/physicalSourcePosition","name":"physicalSourcePosition","line":475,"end_line":492,"hash":"63c57b04c08a73f2dfe3a683ba2656e702fa1f3c2b2c6064546e8e459dd23004"}]}
// mutate4go-manifest-end
