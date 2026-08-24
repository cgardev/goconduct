package architecture

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/cgardev/goconduct/pkg/failure"
)

func TestFunctionAnalysis_StopCanceledPackageLoad(t *testing.T) {
	t.Run("Scenario: The analysis context ends before Go package loading", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var analysisContext context.Context
		var sourcePath string
		var analysisError error

		t.Run("Given a Go source file and a canceled analysis context", func(step *testing.T) {
			repositoryRoot := t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/context\n\ngo 1.26\n")
			writeFixtureFile(step, repositoryRoot, "sample.go", "package sample\n\nfunc Run() {}\n")
			sourcePath = filepath.Join(repositoryRoot, "sample.go")
			sourceAnalyzer = &analyzer{repositoryRoot: repositoryRoot}
			var cancel context.CancelFunc
			analysisContext, cancel = context.WithCancel(t.Context())
			cancel()
		})

		t.Run("When the analyzer loads Go package type information", func(*testing.T) {
			_, _, analysisError = sourceAnalyzer.loadTypedPackages(
				analysisContext,
				[]string{sourcePath},
			)
		})

		t.Run("Then package loading returns the context cancellation", func(t *testing.T) {
			if !errors.Is(analysisError, context.Canceled) {
				t.Fatalf("package load error is %v, want context.Canceled", analysisError)
			}
			if errors.Is(analysisError, failure.ErrUnavailable) {
				t.Fatalf("package load error is %v, do not want ErrUnavailable", analysisError)
			}
		})
	})
}

func TestFunctionAnalysis_CalculateResolvedCallMetrics(t *testing.T) {
	t.Run("Scenario: Production and test functions call package functions and methods", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var graph Graph
		var analysisError error

		t.Run(
			"Given a repository with direct, generic, method, interface, and initializer calls",
			func(step *testing.T) {
				repositoryRoot := newFunctionAnalysisFixture(t)
				var err error
				sourceAnalyzer, err = newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
				if err != nil {
					step.Fatalf("newAnalyzer fails: %v", err)
				}
			},
		)

		t.Run("When the analyzer calculates the function dependency graph", func(*testing.T) {
			graph, analysisError = sourceAnalyzer.analyze(t.Context())
		})

		if !t.Run("Then the analysis resolves production and test function calls", func(t *testing.T) {
			if analysisError != nil {
				t.Fatalf("function analysis fails: %v", analysisError)
			}
			if graph.Summary.Functions == 0 || graph.Summary.FunctionCallSites != 10 {
				t.Fatalf("unexpected function summary: %+v", graph.Summary)
			}
			if graph.Summary.CrossComponentFunctionCallSites != 8 {
				t.Errorf(
					"cross-component call sites are %d, want 8",
					graph.Summary.CrossComponentFunctionCallSites,
				)
			}
		}) {
			return
		}

		t.Run("And the most called method has unique caller and call-site counts", func(t *testing.T) {
			record := functionWithIdentifier(
				t,
				graph,
				"internal/library/telemetry.Recorder.Record",
			)
			if record.AfferentCoupling != 1 || record.IncomingCallSites != 2 ||
				record.TestAfferentCoupling != 1 || record.TestIncomingCallSites != 1 {
				t.Errorf("unexpected Record metrics: %+v", record)
			}
			if record.CrossComponentCallerFunctions != 1 || record.Instability != 0 {
				t.Errorf("unexpected Record coupling: %+v", record)
			}
		})

		t.Run("And outgoing coupling includes package, method, generic, and interface targets", func(t *testing.T) {
			execute := functionWithIdentifier(t, graph, "internal/module/orders.Execute")
			if execute.EfferentCoupling != 3 || execute.OutgoingCallSites != 4 {
				t.Errorf("unexpected Execute metrics: %+v", execute)
			}
			useWriter := functionWithIdentifier(t, graph, "internal/module/orders.UseWriter")
			if useWriter.EfferentCoupling != 1 {
				t.Errorf("unexpected UseWriter metrics: %+v", useWriter)
			}
			functionWithIdentifier(t, graph, "internal/library/telemetry.Writer.Write")
		})

		t.Run("And each component relationship retains imports and exact function call sites", func(t *testing.T) {
			relationship := relationshipBetween(
				t,
				graph,
				"internal/module/orders",
				"internal/library/telemetry",
			)
			if relationship.ProductionFunctionCallSites != 6 || relationship.TestFunctionCallSites != 2 {
				t.Errorf("unexpected relationship call metrics: %+v", relationship)
			}
			if len(relationship.ImportSites) != 2 || relationship.ImportSites[0].Line == 0 {
				t.Errorf("unexpected import sites: %+v", relationship.ImportSites)
			}
			call := functionCallBetween(
				t,
				graph,
				"internal/module/orders.Execute",
				"internal/library/telemetry.Recorder.Record",
				false,
			)
			if call.Calls != 2 || len(call.CallSites) != 2 ||
				call.CallSites[0].Path != "internal/module/orders/orders.go" {
				t.Errorf("unexpected resolved call sites: %+v", call)
			}
		})

		t.Run("And the graph sorts functions and calls by stable identifiers", func(t *testing.T) {
			functionIdentifiers := make([]string, 0, len(graph.Functions))
			for _, function := range graph.Functions {
				functionIdentifiers = append(functionIdentifiers, function.Identifier)
			}
			if !slices.IsSorted(functionIdentifiers) {
				t.Errorf("function identifiers are not sorted: %v", functionIdentifiers)
			}
		})
	})
}

func TestFunctionAnalysis_BuildPackageQueries(t *testing.T) {
	t.Run("Scenario: Source paths contain repeated package directories", func(t *testing.T) {
		var sourcePaths []string
		var queries []string
		var pathSet stringSet

		t.Run("Given unsorted production and test files", func(*testing.T) {
			sourcePaths = []string{
				filepath.Join("repository", "second", "z.go"),
				filepath.Join("repository", "first", "a_test.go"),
				filepath.Join("repository", "first", "b.go"),
				filepath.Join("repository", "first", "a.go"),
			}
		})

		t.Run("When the analyzer builds package queries and the selected path set", func(*testing.T) {
			queries = functionPackageQueries(sourcePaths)
			pathSet = absolutePathSet([]string{filepath.Join("repository", "first", "..", "first", "a.go")})
		})

		t.Run("Then one sorted query selects the first file in each directory", func(t *testing.T) {
			want := []string{
				"file=" + filepath.Join("repository", "first", "a.go"),
				"file=" + filepath.Join("repository", "second", "z.go"),
			}
			if !slices.Equal(queries, want) {
				t.Errorf("package queries are %v, want %v", queries, want)
			}
			if len(pathSet) != 1 || !pathSet.contains(filepath.Join("repository", "first", "a.go")) {
				t.Errorf("selected path set is %v", pathSet)
			}
		})
	})
}

func TestFunctionAnalysis_ClassifyIdentifiersAndPaths(t *testing.T) {
	t.Run("Scenario: Functions use root, nested, external, and receiver identifiers", func(t *testing.T) {
		var repositoryRoot string
		var sourceAnalyzer analyzer
		var nestedPath string
		var nestedLocal bool
		var rootPath string
		var rootLocal bool
		var externalPath string
		var externalLocal bool
		var insidePath string
		var inside bool
		var parentPath string
		var parentInside bool
		var siblingPath string
		var siblingInside bool
		var emptyInside bool
		var freeReceiver string
		var namedReceiver string
		var basicReceiver string

		t.Run("Given a repository root and free, named, and basic receiver types", func(t *testing.T) {
			repositoryRoot = filepath.Join(t.TempDir(), "repository")
			sourceAnalyzer = analyzer{repositoryRoot: repositoryRoot}
			packageValue := types.NewPackage("example.com/functions/internal/library/data", "data")
			namedType := types.NewNamed(
				types.NewTypeName(token.NoPos, packageValue, "Reader", nil),
				types.NewStruct(nil, nil),
				nil,
			)
			freeFunction := newFunctionWithReceiver(packageValue, "Free", nil)
			namedFunction := newFunctionWithReceiver(
				packageValue,
				"Read",
				types.NewPointer(namedType),
			)
			basicFunction := newFunctionWithReceiver(
				packageValue,
				"ReadNumber",
				types.NewPointer(types.Typ[types.Int]),
			)
			freeReceiver = functionReceiver(freeFunction)
			namedReceiver = functionReceiver(namedFunction)
			basicReceiver = functionReceiver(basicFunction)
		})

		t.Run("When package paths and source paths are normalized", func(*testing.T) {
			nestedPath, nestedLocal = localPackagePath(
				"example.com/functions",
				"example.com/functions/internal/library/data",
			)
			rootPath, rootLocal = localPackagePath("example.com/functions", "example.com/functions")
			externalPath, externalLocal = localPackagePath(
				"example.com/functions",
				"example.com/functionsextension/data",
			)
			insidePath, inside = sourceAnalyzer.relativeSourcePath(
				filepath.Join(repositoryRoot, "internal", "library", "data", "read.go"),
			)
			parentPath, parentInside = sourceAnalyzer.relativeSourcePath(filepath.Dir(repositoryRoot))
			siblingPath, siblingInside = sourceAnalyzer.relativeSourcePath(
				filepath.Join(filepath.Dir(repositoryRoot), "outside.go"),
			)
			_, emptyInside = sourceAnalyzer.relativeSourcePath("")
		})

		t.Run("Then local and external package paths remain distinct", func(t *testing.T) {
			if nestedPath != "internal/library/data" || !nestedLocal || rootPath != "." || !rootLocal {
				t.Errorf(
					"local package results are nested=(%q,%t) root=(%q,%t)",
					nestedPath,
					nestedLocal,
					rootPath,
					rootLocal,
				)
			}
			if externalPath != "" || externalLocal {
				t.Errorf("external package result is (%q,%t)", externalPath, externalLocal)
			}
		})

		t.Run("And source paths cannot leave the repository", func(t *testing.T) {
			if insidePath != "internal/library/data/read.go" || !inside {
				t.Errorf("inside source result is (%q,%t)", insidePath, inside)
			}
			if parentPath != "" || parentInside || siblingPath != "" || siblingInside || emptyInside {
				t.Errorf(
					"outside source results are parent=(%q,%t) sibling=(%q,%t) empty=%t",
					parentPath,
					parentInside,
					siblingPath,
					siblingInside,
					emptyInside,
				)
			}
		})

		t.Run("And function identifiers include receiver and initializer identity", func(t *testing.T) {
			if freeReceiver != "" || namedReceiver != "Reader" || basicReceiver != "int" {
				t.Errorf(
					"receivers are free=%q named=%q basic=%q",
					freeReceiver,
					namedReceiver,
					basicReceiver,
				)
			}
			if functionIdentifier("data", "Reader", "Read", "read.go", 4) != "data.Reader.Read" {
				t.Error("method identifier does not contain its receiver")
			}
			if functionIdentifier("data", "", "init", "read.go", 4) != "data.init@read.go:4" {
				t.Error("initializer identifier does not contain its source position")
			}
			if packageName(nil) != "" || packageName(types.NewPackage("example.com/data", "data")) != "data" {
				t.Error("package name normalization is incorrect")
			}
		})
	})
}

func TestFunctionAnalysis_InspectTypedPackage(t *testing.T) {
	t.Run("Scenario: One package contains functions, methods, initializers, and unresolved calls", func(
		t *testing.T,
	) {
		var sourceAnalyzer *analyzer
		var loadedPackage *packages.Package
		var fileSet *token.FileSet
		var file *ast.File
		var sourcePath string
		var declarations []functionDeclaration
		var references []functionReference
		var inspectError error
		var secondDeclarations []functionDeclaration
		var secondReferences []functionReference
		var secondInspectError error
		var boundaryDeclarations []functionDeclaration
		var boundaryError error

		t.Run(
			"Given a type-checked package with generic, method, interface, and variable calls",
			func(t *testing.T) {
				sourceAnalyzer, loadedPackage, fileSet, file, sourcePath = newTypedFunctionFixture(t)
			},
		)

		t.Run("When the analyzer inspects the loaded package twice", func(*testing.T) {
			selectedPaths := absolutePathSet([]string{sourcePath})
			inspectedPaths := make(stringSet)
			declarations, references, inspectError = sourceAnalyzer.inspectLoadedPackage(
				"example.com/functions",
				loadedPackage,
				fileSet,
				selectedPaths,
				inspectedPaths,
			)
			secondDeclarations, secondReferences, secondInspectError = sourceAnalyzer.inspectLoadedPackage(
				"example.com/functions",
				loadedPackage,
				fileSet,
				selectedPaths,
				inspectedPaths,
			)
			startIdentifier := &ast.Ident{NamePos: file.Pos(), Name: "StartBoundary"}
			endIdentifier := &ast.Ident{NamePos: file.End(), Name: "EndBoundary"}
			boundaryPackage := *loadedPackage
			boundaryPackage.TypesInfo = &types.Info{Defs: map[*ast.Ident]types.Object{
				startIdentifier: newFunctionWithReceiver(loadedPackage.Types, "StartBoundary", nil),
				endIdentifier:   newFunctionWithReceiver(loadedPackage.Types, "EndBoundary", nil),
			}}
			boundaryDeclarations, boundaryError = sourceAnalyzer.functionDefinitions(
				"example.com/functions",
				&boundaryPackage,
				fileSet,
				file,
				selectedPaths,
			)
		})

		if !t.Run("Then each resolved call has a declaration and exact source position", func(t *testing.T) {
			if inspectError != nil {
				t.Fatalf("inspect typed package: %v", inspectError)
			}
			if len(references) != 6 {
				t.Fatalf("reference count is %d, want 6: %+v", len(references), references)
			}
			wantTargets := map[int]string{
				16: "internal/library/sample.Generic",
				21: "internal/library/sample.Pair",
				23: "internal/library/sample.Worker.Work",
				24: "internal/library/sample.Writer.Write",
				26: "internal/library/sample.Generic",
				31: "internal/library/sample.Generic",
			}
			wantColumns := map[int]int{16: 6, 21: 6, 23: 2, 24: 2, 26: 11, 31: 15}
			for _, reference := range references {
				if reference.source == "" || reference.target == "" ||
					reference.site.Path != "internal/library/sample/sample.go" ||
					reference.target != wantTargets[reference.site.Line] ||
					reference.site.Column != wantColumns[reference.site.Line] {
					t.Errorf("resolved reference is incomplete: %+v", reference)
				}
			}
		}) {
			return
		}

		t.Run("And synthetic and declared functions keep their identity", func(t *testing.T) {
			if len(declarations) != 14 {
				t.Errorf("declaration count is %d, want 14", len(declarations))
			}
			initializer := declarationWithName(t, declarations, "<package-init>")
			if !initializer.synthetic || !initializer.inAnalysisScope || initializer.test ||
				initializer.line == 0 || !strings.Contains(initializer.identifier, initializer.relativePath) {
				t.Errorf("package initializer is incorrect: %+v", initializer)
			}
			method := declarationWithName(t, declarations, "Worker.Work")
			if method.receiver != "Worker" || !method.method || method.name != "Worker.Work" {
				t.Errorf("method declaration is incorrect: %+v", method)
			}
			initializerFunction := declarationWithName(t, declarations, "init")
			if !strings.Contains(initializerFunction.identifier, "init@internal/library/sample/sample.go:") {
				t.Errorf("initializer function identifier is incorrect: %+v", initializerFunction)
			}
			for _, declaration := range declarations {
				if declaration.identifier == "" {
					t.Error("a declaration has an empty identifier")
				}
			}
		})

		t.Run("And a selected file is not inspected more than once", func(t *testing.T) {
			if secondInspectError != nil {
				t.Fatalf("inspect selected package twice: %v", secondInspectError)
			}
			if len(secondDeclarations) != 0 || len(secondReferences) != 0 {
				t.Errorf(
					"second inspection returned %d declarations and %d references",
					len(secondDeclarations),
					len(secondReferences),
				)
			}
			if !functionPackageOwnsFile(loadedPackage, sourcePath) {
				t.Error("the production package does not own its production file")
			}
			if functionPackageOwnsFile(loadedPackage, strings.TrimSuffix(sourcePath, ".go")+"_test.go") {
				t.Error("the production package owns a test file")
			}
			loadedPackage.ForTest = loadedPackage.PkgPath
			if functionPackageOwnsFile(loadedPackage, sourcePath) ||
				!functionPackageOwnsFile(loadedPackage, strings.TrimSuffix(sourcePath, ".go")+"_test.go") {
				t.Error("the test package owns an incorrect file")
			}
		})

		t.Run("And definition positions include both physical file boundaries", func(t *testing.T) {
			if boundaryError != nil {
				t.Fatalf("inspect boundary definitions: %v", boundaryError)
			}
			if len(boundaryDeclarations) != 2 {
				t.Fatalf("boundary declaration count is %d, want 2", len(boundaryDeclarations))
			}
			positions := []int{
				boundaryDeclarations[0].sourcePosition,
				boundaryDeclarations[1].sourcePosition,
			}
			slices.Sort(positions)
			want := []int{int(file.Pos()), int(file.End())}
			if !slices.Equal(positions, want) {
				t.Errorf("boundary positions are %v, want %v", positions, want)
			}
		})

		t.Run("And packages without type data, selected files, or package ownership are omitted", func(
			t *testing.T,
		) {
			emptyPackages := []*packages.Package{
				{ID: loadedPackage.ID, TypesInfo: nil},
				{
					ID:        loadedPackage.ID + ".test",
					TypesInfo: loadedPackage.TypesInfo,
					Syntax:    []*ast.File{file},
				},
			}
			for _, emptyPackage := range emptyPackages {
				foundDeclarations, foundReferences, err := sourceAnalyzer.inspectLoadedPackage(
					"example.com/functions",
					emptyPackage,
					fileSet,
					absolutePathSet([]string{sourcePath}),
					make(stringSet),
				)
				if err != nil || len(foundDeclarations) != 0 || len(foundReferences) != 0 {
					t.Errorf(
						"empty package returned data: declarations=%v references=%v error=%v",
						foundDeclarations,
						foundReferences,
						err,
					)
				}
			}
			loadedPackage.ID = loadedPackage.PkgPath
			foundDeclarations, foundReferences, err := sourceAnalyzer.inspectLoadedPackage(
				"example.com/functions",
				loadedPackage,
				fileSet,
				make(stringSet),
				make(stringSet),
			)
			if err != nil || len(foundDeclarations) != 0 || len(foundReferences) != 0 {
				t.Errorf(
					"unselected file returned data: declarations=%v references=%v error=%v",
					foundDeclarations,
					foundReferences,
					err,
				)
			}
		})
	})
}

func TestFunctionAnalysis_RejectUnclassifiedCalls(t *testing.T) {
	t.Run("Scenario: Function objects and calls cross the configured analysis boundary", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var loadedPackage *packages.Package
		var fileSet *token.FileSet
		var file *ast.File
		var sourcePath string
		var runFunction *types.Func
		var workFunction *types.Func
		var runDeclaration functionDeclaration
		var workDeclaration functionDeclaration
		var packageDeclaration functionDeclaration
		var outsideScopeDeclaration functionDeclaration
		var runClassified bool
		var workClassified bool
		var packageClassified bool
		var outsideScopeClassified bool
		var runDeclarationError error
		var workDeclarationError error
		var packageDeclarationError error
		var outsideScopeError error
		var invalidPatternError error
		var invalidPatternClassified bool
		var omittedCount int
		var callError error
		var unclassifiedCalls []functionReference
		var nestedUnresolvedCalls []functionReference
		var nestedExternalCalls []functionReference
		var unclassifiedCallError error
		var nestedUnresolvedCallError error
		var nestedExternalCallError error

		t.Run("Given local, external, ignored, and unclassified function objects", func(t *testing.T) {
			sourceAnalyzer, loadedPackage, fileSet, file, sourcePath = newTypedFunctionFixture(t)
			runFunction = typedFunctionWithName(t, loadedPackage, "Run", "")
			workFunction = typedFunctionWithName(t, loadedPackage, "Work", "Worker")
		})

		t.Run("When the analyzer classifies each function object and call", func(*testing.T) {
			selectedPaths := absolutePathSet([]string{sourcePath})
			runDeclaration, runClassified, runDeclarationError = sourceAnalyzer.functionDeclarationFromObject(
				"example.com/functions",
				runFunction,
				fileSet,
				selectedPaths,
			)
			workDeclaration, workClassified, workDeclarationError = sourceAnalyzer.functionDeclarationFromObject(
				"example.com/functions",
				workFunction,
				fileSet,
				selectedPaths,
			)
			outsideScopeDeclaration, outsideScopeClassified, outsideScopeError = sourceAnalyzer.functionDeclarationFromObject(
				"example.com/functions",
				runFunction,
				fileSet,
				make(stringSet),
			)
			packageFunction := types.NewFunc(
				token.NoPos,
				types.NewPackage("example.com/functions/internal/library/data", "data"),
				"Read",
				types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false),
			)
			packageDeclaration, packageClassified, packageDeclarationError = sourceAnalyzer.functionDeclarationFromObject(
				"example.com/functions",
				packageFunction,
				fileSet,
				selectedPaths,
			)
			omittedFunctions := []*types.Func{
				types.NewFunc(
					token.NoPos,
					nil,
					"Builtin",
					types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false),
				),
				types.NewFunc(
					token.NoPos,
					types.NewPackage("example.net/external", "external"),
					"Call",
					types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false),
				),
				types.NewFunc(
					token.NoPos,
					types.NewPackage("example.com/functions/misc/tool", "tool"),
					"Call",
					types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false),
				),
			}
			for _, omittedFunction := range omittedFunctions {
				_, classified, err := sourceAnalyzer.functionDeclarationFromObject(
					"example.com/functions",
					omittedFunction,
					fileSet,
					selectedPaths,
				)
				if err == nil && !classified {
					omittedCount++
				}
			}
			ignoredAnalyzer := *sourceAnalyzer
			ignoredAnalyzer.ignoredPaths = ignoredPathMatcher{patterns: []string{"sample"}}
			_, ignored, ignoredError := ignoredAnalyzer.functionDeclarationFromObject(
				"example.com/functions",
				runFunction,
				fileSet,
				selectedPaths,
			)
			if ignoredError == nil && !ignored {
				omittedCount++
			}
			invalidAnalyzer := *sourceAnalyzer
			invalidAnalyzer.ignoredPaths = ignoredPathMatcher{patterns: []string{"["}}
			_, invalidPatternClassified, invalidPatternError = invalidAnalyzer.functionDeclarationFromObject(
				"example.com/functions",
				runFunction,
				fileSet,
				selectedPaths,
			)
			runBody := functionBodyWithName(t, file, "Run")
			_, _, callError = invalidAnalyzer.callsInNode(
				"example.com/functions",
				loadedPackage,
				fileSet,
				runBody,
				runDeclaration,
				selectedPaths,
			)
			_, unclassifiedCalls, unclassifiedCallError = sourceAnalyzer.callsInNode(
				"example.net/different",
				loadedPackage,
				fileSet,
				runBody,
				runDeclaration,
				selectedPaths,
			)
			localPackage := types.NewPackage(
				"example.com/functions/internal/library/sample",
				"sample",
			)
			localTarget := newFunctionWithReceiver(localPackage, "Nested", nil)
			externalTarget := newFunctionWithReceiver(
				types.NewPackage("example.net/external", "external"),
				"Outer",
				nil,
			)
			unresolvedOuter := ast.NewIdent("callback")
			unresolvedInner := ast.NewIdent("Nested")
			unresolvedNode := &ast.CallExpr{
				Fun: unresolvedOuter,
				Args: []ast.Expr{
					&ast.CallExpr{Fun: unresolvedInner},
				},
			}
			unresolvedPackage := &packages.Package{TypesInfo: &types.Info{
				Uses: map[*ast.Ident]types.Object{unresolvedInner: localTarget},
			}}
			_, nestedUnresolvedCalls, nestedUnresolvedCallError = sourceAnalyzer.callsInNode(
				"example.com/functions",
				unresolvedPackage,
				token.NewFileSet(),
				unresolvedNode,
				runDeclaration,
				selectedPaths,
			)
			externalOuter := ast.NewIdent("Outer")
			externalInner := ast.NewIdent("Nested")
			externalNode := &ast.CallExpr{
				Fun: externalOuter,
				Args: []ast.Expr{
					&ast.CallExpr{Fun: externalInner},
				},
			}
			externalCallPackage := &packages.Package{TypesInfo: &types.Info{
				Uses: map[*ast.Ident]types.Object{
					externalOuter: externalTarget,
					externalInner: localTarget,
				},
			}}
			_, nestedExternalCalls, nestedExternalCallError = sourceAnalyzer.callsInNode(
				"example.com/functions",
				externalCallPackage,
				token.NewFileSet(),
				externalNode,
				runDeclaration,
				selectedPaths,
			)
		})

		t.Run("Then local functions retain source, receiver, scope, and export data", func(t *testing.T) {
			for name, err := range map[string]error{
				"run declaration":           runDeclarationError,
				"work declaration":          workDeclarationError,
				"package declaration":       packageDeclarationError,
				"outside-scope declaration": outsideScopeError,
			} {
				if err != nil {
					t.Fatalf("classify %s: %v", name, err)
				}
			}
			if !runClassified || !workClassified || !packageClassified || !outsideScopeClassified {
				t.Errorf(
					"unexpected classifications: run=%t work=%t package=%t outside=%t",
					runClassified,
					workClassified,
					packageClassified,
					outsideScopeClassified,
				)
			}
			if runDeclaration.name != "Run" || runDeclaration.method || !runDeclaration.exported ||
				!runDeclaration.inAnalysisScope || runDeclaration.relativePath != "internal/library/sample/sample.go" {
				t.Errorf("free function declaration is incorrect: %+v", runDeclaration)
			}
			if workDeclaration.name != "Worker.Work" || workDeclaration.receiver != "Worker" ||
				!workDeclaration.method || !workDeclaration.inAnalysisScope {
				t.Errorf("method declaration is incorrect: %+v", workDeclaration)
			}
			if outsideScopeDeclaration.inAnalysisScope {
				t.Errorf("unselected function is in the analysis scope: %+v", outsideScopeDeclaration)
			}
			if packageDeclaration.relativePath != "" || packageDeclaration.component != "internal/library/data" ||
				packageDeclaration.inAnalysisScope {
				t.Errorf("package-only declaration is incorrect: %+v", packageDeclaration)
			}
		})

		t.Run("And external, ignored, unclassified, and invalid targets are omitted", func(t *testing.T) {
			for name, err := range map[string]error{
				"unclassified call":      unclassifiedCallError,
				"nested unresolved call": nestedUnresolvedCallError,
				"nested external call":   nestedExternalCallError,
			} {
				if err != nil {
					t.Fatalf("inspect %s: %v", name, err)
				}
			}
			if omittedCount != 4 {
				t.Errorf("omitted function count is %d, want 4", omittedCount)
			}
			if invalidPatternError == nil || invalidPatternClassified || callError == nil {
				t.Errorf(
					"invalid patterns returned declaration error %v and call error %v",
					invalidPatternError,
					callError,
				)
			}
			if len(unclassifiedCalls) != 0 {
				t.Errorf("unclassified calls are %v, want no calls", unclassifiedCalls)
			}
			for name, calls := range map[string][]functionReference{
				"unresolved outer call": nestedUnresolvedCalls,
				"external outer call":   nestedExternalCalls,
			} {
				if len(calls) != 1 || calls[0].target != "internal/library/sample.Nested" {
					t.Errorf("%s has nested calls %v", name, calls)
				}
			}
		})
	})
}

func TestFunctionAnalysis_InspectInitializerBoundaries(t *testing.T) {
	t.Run("Scenario: Package variables can contain calls or plain values", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var loadedPackage *packages.Package
		var fileSet *token.FileSet
		var file *ast.File
		var sourcePath string
		var initializer functionDeclaration
		var initializerClassified bool
		var externalClassified bool
		var noPositionClassified bool
		var unclassifiedPathClassified bool
		var plainDeclarations []functionDeclaration
		var plainCalls []functionReference
		var plainInspectionError error
		var callError error

		t.Run("Given one called initializer and one initializer without calls", func(t *testing.T) {
			sourceAnalyzer, loadedPackage, fileSet, file, sourcePath = newTypedFunctionFixture(t)
		})

		t.Run("When the analyzer inspects each initializer boundary", func(step *testing.T) {
			calledVariable := variableDeclarationWithName(t, file, "Default")
			plainVariable := variableDeclarationWithName(t, file, "Plain")
			initializer, initializerClassified = sourceAnalyzer.packageInitializerDeclaration(
				"example.com/functions",
				loadedPackage,
				fileSet,
				calledVariable,
			)
			externalPackage := *loadedPackage
			externalPackage.PkgPath = "example.net/external"
			_, externalClassified = sourceAnalyzer.packageInitializerDeclaration(
				"example.com/functions",
				&externalPackage,
				fileSet,
				calledVariable,
			)
			_, noPositionClassified = sourceAnalyzer.packageInitializerDeclaration(
				"example.com/functions",
				loadedPackage,
				token.NewFileSet(),
				&ast.GenDecl{Tok: token.VAR},
			)
			unclassifiedFileSet := token.NewFileSet()
			unclassifiedFile, err := parser.ParseFile(
				unclassifiedFileSet,
				filepath.Join(sourceAnalyzer.repositoryRoot, "misc.go"),
				"package sample\nvar Value = 1\n",
				0,
			)
			if err != nil {
				step.Fatalf("parse unclassified initializer: %v", err)
			}
			unclassifiedVariable := variableDeclarationWithName(step, unclassifiedFile, "Value")
			_, unclassifiedPathClassified = sourceAnalyzer.packageInitializerDeclaration(
				"example.com/functions",
				loadedPackage,
				unclassifiedFileSet,
				unclassifiedVariable,
			)
			plainDeclarations, plainCalls, plainInspectionError = sourceAnalyzer.inspectPackageInitializers(
				"example.com/functions",
				loadedPackage,
				fileSet,
				plainVariable,
				absolutePathSet([]string{sourcePath}),
			)
			invalidAnalyzer := *sourceAnalyzer
			invalidAnalyzer.ignoredPaths = ignoredPathMatcher{patterns: []string{"["}}
			_, _, callError = invalidAnalyzer.inspectPackageInitializers(
				"example.com/functions",
				loadedPackage,
				fileSet,
				calledVariable,
				absolutePathSet([]string{sourcePath}),
			)
		})

		t.Run("Then only the local initializer with a resolved call is synthetic", func(t *testing.T) {
			if plainInspectionError != nil {
				t.Fatalf("inspect plain initializer: %v", plainInspectionError)
			}
			if !initializerClassified || initializer.name != "<package-init>" ||
				!initializer.synthetic || !initializer.inAnalysisScope || initializer.test {
				t.Errorf("called initializer is incorrect: %+v", initializer)
			}
			if externalClassified || noPositionClassified || unclassifiedPathClassified {
				t.Errorf(
					"invalid initializer classifications are external=%t no-position=%t path=%t",
					externalClassified,
					noPositionClassified,
					unclassifiedPathClassified,
				)
			}
			if len(plainDeclarations) != 0 || len(plainCalls) != 0 {
				t.Errorf("plain initializer returned declarations=%v calls=%v", plainDeclarations, plainCalls)
			}
			if callError == nil {
				t.Error("invalid ignored-path pattern did not stop initializer inspection")
			}
		})
	})
}

func TestFunctionAnalysis_UnwrapFunctionExpressions(t *testing.T) {
	t.Run("Scenario: Generic call expressions contain nested syntax nodes", func(t *testing.T) {
		var identifier *ast.Ident
		var nested ast.Expr
		var literal *ast.BasicLit
		var unwrappedNested ast.Expr
		var unwrappedLiteral ast.Expr

		t.Run("Given parenthesized single-index and multi-index expressions", func(*testing.T) {
			identifier = ast.NewIdent("Target")
			nested = &ast.ParenExpr{X: &ast.IndexListExpr{
				X: &ast.IndexExpr{X: identifier, Index: ast.NewIdent("First")},
				Indices: []ast.Expr{
					ast.NewIdent("Second"),
					ast.NewIdent("Third"),
				},
			}}
			literal = &ast.BasicLit{Kind: token.INT, Value: "1"}
		})

		t.Run("When both expressions are unwrapped", func(*testing.T) {
			unwrappedNested = unwrapFunctionExpression(nested)
			unwrappedLiteral = unwrapFunctionExpression(literal)
		})

		t.Run("Then only wrapper nodes are removed", func(t *testing.T) {
			if unwrappedNested != identifier {
				t.Errorf("nested expression resolves to %T, want the identifier", unwrappedNested)
			}
			if unwrappedLiteral != literal {
				t.Errorf("literal expression resolves to %T, want the original literal", unwrappedLiteral)
			}
		})
	})
}

func newFunctionWithReceiver(
	packageValue *types.Package,
	name string,
	receiverType types.Type,
) *types.Func {
	var receiver *types.Var
	if receiverType != nil {
		receiver = types.NewVar(token.NoPos, packageValue, "receiver", receiverType)
	}
	signature := types.NewSignatureType(
		receiver,
		nil,
		nil,
		types.NewTuple(),
		types.NewTuple(),
		false,
	)
	return types.NewFunc(token.NoPos, packageValue, name, signature)
}

func newTypedFunctionFixture(
	t *testing.T,
) (*analyzer, *packages.Package, *token.FileSet, *ast.File, string) {
	t.Helper()
	repositoryRoot := t.TempDir()
	writeFixtureFile(t, repositoryRoot, "go.mod", "module example.com/functions\n\ngo 1.26\n")
	sourcePath := filepath.Join(repositoryRoot, "internal/library/sample/sample.go")
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/library/sample/sample.go",
		`package sample

type Worker struct{}

func (Worker) Work() {}

type Writer interface {
	Write()
}

func Generic[Value any](value Value) Value { return value }
func Pair[First, Second any](first First, _ Second) First { return first }
func External()

func init() {
	_ = ((Generic[int]))(1)
}

//line virtual.go:200
func Run(writer Writer) {
	_ = Pair[int, string](1, "value")
	worker := Worker{}
	worker.Work()
	writer.Write()
	callback := func(_ int) {}
	callback(Generic[int](3))
	_ = len([]int{})
}

//line virtual.go:400
var Default = Generic[int](2)
var Plain = 1
const Fixed = 2
`,
	)
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, sourcePath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse typed function fixture: %v", err)
	}
	typeInformation := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Instances:  make(map[*ast.Ident]types.Instance),
	}
	packagePath := "example.com/functions/internal/library/sample"
	checkedPackage, err := (&types.Config{}).Check(
		packagePath,
		fileSet,
		[]*ast.File{file},
		typeInformation,
	)
	if err != nil {
		t.Fatalf("check typed function fixture: %v", err)
	}
	sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
	if err != nil {
		t.Fatalf("build typed function analyzer: %v", err)
	}
	loadedPackage := &packages.Package{
		ID:        packagePath,
		Name:      checkedPackage.Name(),
		PkgPath:   packagePath,
		Types:     checkedPackage,
		TypesInfo: typeInformation,
		Syntax:    []*ast.File{file},
	}
	return sourceAnalyzer, loadedPackage, fileSet, file, sourcePath
}

func declarationWithName(
	t *testing.T,
	declarations []functionDeclaration,
	name string,
) functionDeclaration {
	t.Helper()
	for _, declaration := range declarations {
		if declaration.name == name {
			return declaration
		}
	}
	t.Fatalf("declarations contain no function named %q", name)
	return functionDeclaration{}
}

func typedFunctionWithName(
	t *testing.T,
	loadedPackage *packages.Package,
	name string,
	receiver string,
) *types.Func {
	t.Helper()
	for _, object := range loadedPackage.TypesInfo.Defs {
		function, ok := object.(*types.Func)
		if ok && function.Name() == name && functionReceiver(function) == receiver {
			return function
		}
	}
	t.Fatalf("typed package contains no function %s.%s", receiver, name)
	return nil
}

func functionBodyWithName(t *testing.T, file *ast.File, name string) *ast.BlockStmt {
	t.Helper()
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == name {
			return function.Body
		}
	}
	t.Fatalf("syntax file contains no function %q", name)
	return nil
}

func variableDeclarationWithName(t *testing.T, file *ast.File, name string) *ast.GenDecl {
	t.Helper()
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.VAR {
			continue
		}
		for _, specification := range general.Specs {
			value, ok := specification.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, identifier := range value.Names {
				if identifier.Name == name {
					return general
				}
			}
		}
	}
	t.Fatalf("syntax file contains no package variable %q", name)
	return nil
}

func newFunctionAnalysisFixture(t *testing.T) string {
	t.Helper()
	repositoryRoot := t.TempDir()
	writeFixtureFile(t, repositoryRoot, "go.mod", "module example.com/functions\n\ngo 1.26\n")
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/library/telemetry/telemetry.go",
		`package telemetry

type Recorder struct{}

func NewRecorder() *Recorder { return &Recorder{} }
func (recorder *Recorder) Record() {}
func Transform[Value any](value Value) Value { return value }

type Writer interface {
	Write()
}
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/module/orders/orders.go",
		`package orders

import "example.com/functions/internal/library/telemetry"

var defaultRecorder = telemetry.NewRecorder()

func Execute() {
	recorder := telemetry.NewRecorder()
	recorder.Record()
	recorder.Record()
	telemetry.Transform[int](1)
}

func UseWriter(writer telemetry.Writer) {
	writer.Write()
}

func callExecute() {
	Execute()
}
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/module/orders/orders_test.go",
		`package orders

import (
	"testing"

	"example.com/functions/internal/library/telemetry"
)

func TestExecute(t *testing.T) {
	Execute()
	telemetry.NewRecorder().Record()
}
`,
	)
	return repositoryRoot
}

func functionWithIdentifier(t *testing.T, graph Graph, identifier string) Function {
	t.Helper()
	for _, function := range graph.Functions {
		if function.Identifier == identifier {
			return function
		}
	}
	t.Fatalf("the graph contains no function %q", identifier)
	return Function{}
}

func functionCallBetween(
	t *testing.T,
	graph Graph,
	source string,
	target string,
	testOnly bool,
) FunctionCall {
	t.Helper()
	for _, call := range graph.FunctionCalls {
		if call.Source == source && call.Target == target && call.TestOnly == testOnly {
			return call
		}
	}
	t.Fatalf("the graph contains no function call from %q to %q", source, target)
	return FunctionCall{}
}
