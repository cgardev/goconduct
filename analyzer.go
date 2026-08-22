package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type analyzer struct {
	repositoryRoot string
	analysisPaths  []string
	ignoredPaths   ignoredPathMatcher
	classifier     componentClassifier
	scope          AnalysisScope
}

var _ graphMonitorSource = (*analyzer)(nil)

type ignoredPathMatcher struct {
	patterns []string
}

func newAnalyzer(configuration AnalysisConfiguration) (*analyzer, error) {
	if strings.TrimSpace(configuration.RepositoryRoot) == "" {
		return nil, newValidationError("repository root must not be empty", nil)
	}
	absoluteRoot, err := filepath.Abs(configuration.RepositoryRoot)
	if err != nil {
		return nil, newValidationError(
			fmt.Sprintf("resolve repository root %s", configuration.RepositoryRoot),
			err,
		)
	}
	moduleFileInfo, err := os.Stat(filepath.Join(absoluteRoot, "go.mod"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, newValidationError("inspect repository module file", err)
		}
		return nil, newUnavailableError("inspect repository module file", err)
	}
	if moduleFileInfo.IsDir() {
		return nil, newValidationError("repository module file must be a file", nil)
	}
	analysisPaths, err := normalizeAnalysisPaths(configuration.Paths)
	if err != nil {
		return nil, err
	}
	ignoredPaths, err := newIgnoredPathMatcher(configuration.IgnoredPaths)
	if err != nil {
		return nil, err
	}
	componentRules := configuration.Components.domainRules()
	classifier, err := newComponentClassifier(componentRules)
	if err != nil {
		return nil, fmt.Errorf("configure component classification: %w", err)
	}
	return &analyzer{
		repositoryRoot: absoluteRoot,
		analysisPaths:  analysisPaths,
		ignoredPaths:   ignoredPaths,
		classifier:     classifier,
		scope: AnalysisScope{
			Paths:        slices.Clone(analysisPaths),
			IgnoredPaths: slices.Clone(ignoredPaths.patterns),
			Components:   cloneComponentRules(componentRules),
		},
	}, nil
}

func (analyzer *analyzer) analyze(ctx context.Context) (Graph, error) {
	if err := ctx.Err(); err != nil {
		return Graph{}, err
	}
	modulePath, err := readModulePath(analyzer.repositoryRoot)
	if err != nil {
		return Graph{}, err
	}
	paths, err := analyzer.sourcePaths(ctx)
	if err != nil {
		return Graph{}, err
	}

	components := make(map[string]*componentAccumulator)
	relationships := make(map[relationshipKey]*relationshipAccumulator)
	functionPaths := make(stringSet)
	var diagnostics []Diagnostic
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return Graph{}, err
		}
		file, err := analyzer.inspectSourceFile(modulePath, path)
		if err != nil {
			return Graph{}, err
		}
		if file == nil {
			continue
		}
		diagnostics = append(diagnostics, file.diagnostics...)
		if file.hasFunctionData {
			functionPaths.add(path)
		}
		collectComponentFile(components, *file)
		collectRelationships(components, relationships, *file)
	}

	graph := buildGraph(modulePath, components, relationships, diagnostics)
	if len(functionPaths) != 0 {
		declarations, references, functionError := analyzer.inspectFunctions(ctx, modulePath, paths)
		if functionError != nil {
			return Graph{}, functionError
		}
		graph.Functions, graph.FunctionCalls, graph.FunctionCycles = calculateFunctionGraph(
			declarations,
			references,
		)
		attachFunctionMetrics(&graph)
	}
	graph.Scope = analyzer.scope
	payload, err := json.Marshal(graph)
	if err != nil {
		return Graph{}, newInternalError("encode graph revision input", err)
	}
	digest := sha256.Sum256(payload)
	graph.Revision = hex.EncodeToString(digest[:])
	return graph, nil
}

func (analyzer *analyzer) repositoryPath() string {
	return analyzer.repositoryRoot
}

func (analyzer *analyzer) snapshot(ctx context.Context) (string, error) {
	paths, err := analyzer.sourcePaths(ctx)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	paths = append(paths, filepath.Join(analyzer.repositoryRoot, "go.mod"))
	moduleSumPath := filepath.Join(analyzer.repositoryRoot, "go.sum")
	if _, err := os.Stat(moduleSumPath); err == nil {
		paths = append(paths, moduleSumPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", newUnavailableError("inspect repository module sum", err)
	}

	var buffer []byte
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		fileInfo, err := os.Stat(path)
		if err != nil {
			return "", newUnavailableError(
				fmt.Sprintf("inspect repository source file %s", path),
				err,
			)
		}
		relativePath, err := filepath.Rel(analyzer.repositoryRoot, path)
		if err != nil {
			return "", newInternalError(
				fmt.Sprintf("resolve repository source file %s", path),
				err,
			)
		}
		buffer = append(buffer, filepath.ToSlash(relativePath)...)
		buffer = append(buffer, "\x00"...)
		buffer = strconv.AppendInt(buffer, fileInfo.Size(), 10)
		buffer = append(buffer, "\x00"...)
		buffer = strconv.AppendInt(buffer, fileInfo.ModTime().UnixNano(), 10)
		buffer = append(buffer, '\n')
	}
	digest := sha256.Sum256(buffer)
	return hex.EncodeToString(digest[:]), nil
}

func readModulePath(repositoryRoot string) (string, error) {
	path := filepath.Join(repositoryRoot, "go.mod")
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", newUnavailableError("read module file", err)
	}
	for rawLine := range strings.SplitSeq(string(payload), "\n") {
		line, _, _ := strings.Cut(rawLine, "//")
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "module" {
			continue
		}
		value, err := strconv.Unquote(fields[1])
		if err == nil {
			return value, nil
		}
		return fields[1], nil
	}
	return "", newDataIntegrityError("module declaration not found in go.mod", nil)
}

func normalizeAnalysisPaths(configuredPaths []string) ([]string, error) {
	if len(configuredPaths) == 0 {
		return nil, newValidationError(
			"analysis paths must contain at least one repository-relative path",
			nil,
		)
	}
	paths := make(stringSet)
	for _, configuredPath := range configuredPaths {
		normalized, err := normalizeRepositoryPath(configuredPath)
		if err != nil {
			return nil, fmt.Errorf("configure analysis path %q: %w", configuredPath, err)
		}
		paths.add(normalized)
	}
	return sortedSet(paths), nil
}

func normalizeRepositoryPath(configuredPath string) (string, error) {
	if configuredPath == "" {
		return "", newValidationError("path must be a non-empty relative path", nil)
	}
	if configuredPath != strings.TrimSpace(configuredPath) {
		return "", newValidationError("path must be a non-empty relative path", nil)
	}
	if filepath.IsAbs(configuredPath) {
		return "", newValidationError("path must be a non-empty relative path", nil)
	}
	cleaned := filepath.Clean(configuredPath)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", newValidationError("path must remain inside the repository root", nil)
	}
	return filepath.ToSlash(cleaned), nil
}

func newIgnoredPathMatcher(configuredPatterns []string) (ignoredPathMatcher, error) {
	patterns := make(stringSet)
	for _, configuredPattern := range configuredPatterns {
		if configuredPattern != strings.TrimSpace(configuredPattern) || strings.Contains(configuredPattern, "\\") {
			return ignoredPathMatcher{}, newValidationError(
				fmt.Sprintf(
					"ignored path pattern %q must be a non-empty relative slash path",
					configuredPattern,
				),
				nil,
			)
		}
		for segment := range strings.SplitSeq(configuredPattern, "/") {
			if segment == "" || segment == "." || segment == ".." {
				return ignoredPathMatcher{}, newValidationError(
					fmt.Sprintf(
						"ignored path pattern %q contains an invalid segment",
						configuredPattern,
					),
					nil,
				)
			}
		}
		if _, err := path.Match(configuredPattern, "validation"); err != nil {
			return ignoredPathMatcher{}, newValidationError(
				fmt.Sprintf("compile ignored path pattern %q", configuredPattern),
				err,
			)
		}
		patterns.add(configuredPattern)
	}
	return ignoredPathMatcher{patterns: sortedSet(patterns)}, nil
}

func (matcher ignoredPathMatcher) matches(relativePath string) (bool, error) {
	cleaned := strings.Trim(pathWithForwardSlashes(relativePath), "/")
	if cleaned == "" || cleaned == "." {
		return false, nil
	}
	segments := strings.Split(cleaned, "/")
	for _, pattern := range matcher.patterns {
		if !strings.Contains(pattern, "/") {
			for _, segment := range segments {
				matches, err := path.Match(pattern, segment)
				if err != nil {
					return matches, newInternalError(
						fmt.Sprintf("match ignored path pattern %q", pattern),
						err,
					)
				}
				if matches {
					return true, nil
				}
			}
			continue
		}
		for end := range segments {
			candidate := strings.Join(segments[:end+1], "/")
			matches, err := path.Match(pattern, candidate)
			if err != nil {
				return matches, newInternalError(
					fmt.Sprintf("match ignored path pattern %q", pattern),
					err,
				)
			}
			if matches {
				return true, nil
			}
		}
	}
	return false, nil
}

func (analyzer *analyzer) sourcePaths(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	paths := make(stringSet)
	for _, analysisPath := range analyzer.analysisPaths {
		if err := analyzer.collectConfiguredSourcePath(ctx, paths, analysisPath); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return sortedSet(paths), nil
}

func (analyzer *analyzer) collectConfiguredSourcePath(
	ctx context.Context,
	paths stringSet,
	analysisPath string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	absolutePath := filepath.Join(analyzer.repositoryRoot, filepath.FromSlash(analysisPath))
	fileInfo, err := os.Stat(absolutePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newValidationError(fmt.Sprintf("inspect analysis path %s", analysisPath), err)
		}
		return newUnavailableError(fmt.Sprintf("inspect analysis path %s", analysisPath), err)
	}
	if fileInfo.IsDir() {
		return analyzer.walkSourceDirectory(ctx, paths, analysisPath, absolutePath)
	}
	return analyzer.collectSourceFile(paths, analysisPath, absolutePath)
}

func (analyzer *analyzer) collectSourceFile(
	paths stringSet,
	analysisPath string,
	absolutePath string,
) error {
	if filepath.Ext(absolutePath) != ".go" {
		return newValidationError(
			fmt.Sprintf("analysis path %s must be a directory or Go source file", analysisPath),
			nil,
		)
	}
	ignored, err := analyzer.ignoredPaths.matches(analysisPath)
	if err != nil {
		return err
	}
	if !ignored {
		paths.add(absolutePath)
	}
	return nil
}

func (analyzer *analyzer) walkSourceDirectory(
	ctx context.Context,
	paths stringSet,
	analysisPath string,
	absolutePath string,
) error {
	err := filepath.WalkDir(absolutePath, func(sourcePath string, entry fs.DirEntry, walkError error) error {
		return analyzer.collectWalkedSourcePath(ctx, paths, sourcePath, entry, walkError)
	})
	if err != nil {
		return newUnavailableError(fmt.Sprintf("walk analysis path %s", analysisPath), err)
	}
	return nil
}

func (analyzer *analyzer) collectWalkedSourcePath(
	ctx context.Context,
	paths stringSet,
	sourcePath string,
	entry fs.DirEntry,
	walkError error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if walkError != nil {
		return newUnavailableError(fmt.Sprintf("visit %s", sourcePath), walkError)
	}
	relativePath, err := filepath.Rel(analyzer.repositoryRoot, sourcePath)
	if err != nil {
		return newInternalError(
			fmt.Sprintf("resolve repository source path %s", sourcePath),
			err,
		)
	}
	ignored, err := analyzer.ignoredPaths.matches(filepath.ToSlash(relativePath))
	if err != nil {
		return err
	}
	if ignored {
		if entry.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".go") {
		return nil
	}
	paths.add(sourcePath)
	return nil
}

func (analyzer *analyzer) inspectSourceFile(modulePath, sourcePath string) (*sourceFile, error) {
	relativePath, err := filepath.Rel(analyzer.repositoryRoot, sourcePath)
	if err != nil {
		return nil, newInternalError(fmt.Sprintf("resolve source path %s", sourcePath), err)
	}
	relativePath = filepath.ToSlash(relativePath)
	descriptor, classified := analyzer.classifier.classify(relativePath)
	if !classified {
		return nil, nil
	}
	payload, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, newUnavailableError(fmt.Sprintf("read source file %s", relativePath), err)
	}

	fileSet := token.NewFileSet()
	parsed, parseError := parser.ParseFile(
		fileSet,
		relativePath,
		payload,
		parser.AllErrors,
	)
	file := &sourceFile{
		relativePath: relativePath,
		packagePath:  strings.TrimSuffix(relativePath, "/"+filepath.Base(relativePath)),
		component:    descriptor,
		test:         strings.HasSuffix(relativePath, "_test.go"),
	}
	if parseError != nil {
		file.diagnostics = append(file.diagnostics, Diagnostic{
			Path:    relativePath,
			Message: parseError.Error(),
		})
	}
	if parsed == nil {
		return file, nil
	}

	importsByPackage := make(map[string]sourceImport)
	for _, specification := range parsed.Imports {
		importPath, err := strconv.Unquote(specification.Path.Value)
		if err != nil {
			file.diagnostics = append(file.diagnostics, Diagnostic{
				Path:    relativePath,
				Message: fmt.Sprintf("decode import %s: %v", specification.Path.Value, err),
			})
			continue
		}
		if !strings.HasPrefix(importPath, modulePath+"/") {
			continue
		}
		targetPackage := strings.TrimPrefix(importPath, modulePath+"/")
		ignored, err := analyzer.ignoredPaths.matches(targetPackage)
		if err != nil {
			return nil, err
		}
		if ignored {
			continue
		}
		target, classified := analyzer.classifier.classify(targetPackage)
		if classified {
			position := fileSet.Position(specification.Pos())
			alias := ""
			if specification.Name != nil {
				alias = specification.Name.Name
			}
			importsByPackage[targetPackage] = sourceImport{
				packagePath: targetPackage,
				component:   target,
				site: ImportSite{
					SourcePackage: file.packagePath,
					TargetPackage: targetPackage,
					Path:          relativePath,
					Line:          position.Line,
					Alias:         alias,
					Test:          file.test,
				},
			}
		}
	}
	imports := make([]sourceImport, 0, len(importsByPackage))
	for _, imported := range importsByPackage {
		imports = append(imports, imported)
	}
	slices.SortFunc(imports, func(first, second sourceImport) int {
		return strings.Compare(first.packagePath, second.packagePath)
	})
	file.imports = imports
	file.abstractTypes, file.concreteTypes = countNamedTypes(parsed)
	file.hasFunctionData = hasFunctionData(parsed)
	return file, nil
}

func hasFunctionData(file *ast.File) bool {
	for node := range ast.Preorder(file) {
		switch node.(type) {
		case *ast.FuncDecl, *ast.FuncType, *ast.CallExpr:
			return true
		}
	}
	return false
}

func countNamedTypes(file *ast.File) (int, int) {
	abstractTypes := 0
	concreteTypes := 0
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.TYPE {
			continue
		}
		for _, specification := range group.Specs {
			namedType, ok := specification.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, ok := namedType.Type.(*ast.InterfaceType); ok {
				abstractTypes++
				continue
			}
			concreteTypes++
		}
	}
	return abstractTypes, concreteTypes
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"9e335bf9f8efc8a4ff229777d20720d1a81efa362482489bddafd81c68a95901","functions":[{"id":"func/newAnalyzer","name":"newAnalyzer","line":36,"end_line":81,"hash":"6bbc8f8f8a7ba171d9ef1c4dfe8602d038e9dd4655c3165746b9a778d5c91411"},{"id":"func/analyzer.analyze","name":"analyzer.analyze","line":83,"end_line":139,"hash":"31ba8e6cfd598ac0c8b95b05208ae6975e01b4b2146d54732cf08ae87cf7b274"},{"id":"func/analyzer.repositoryPath","name":"analyzer.repositoryPath","line":141,"end_line":143,"hash":"a11e0cbd14146576a5f65fa33b4ac3b96e43a3e4c7169bcf6b29d33466888347"},{"id":"func/analyzer.snapshot","name":"analyzer.snapshot","line":145,"end_line":189,"hash":"f67a63353a874597c197adcd70a0ff7100406cce6315a93dcc33a62d5f96bf52"},{"id":"func/readModulePath","name":"readModulePath","line":191,"end_line":211,"hash":"d63ae450bb33816fb980c26f89bc81f0e31d1692cca2fbb220485fe0f0977242"},{"id":"func/normalizeAnalysisPaths","name":"normalizeAnalysisPaths","line":213,"end_line":229,"hash":"b1cdd3fa4483d93aaaced9f5335f83ae92bbf16f0b26b3940ba55249ccdf3e60"},{"id":"func/normalizeRepositoryPath","name":"normalizeRepositoryPath","line":231,"end_line":246,"hash":"edb6360b3d494b9569d986e6c123db2501b1e188c6730785a26111d49ea3060d"},{"id":"func/newIgnoredPathMatcher","name":"newIgnoredPathMatcher","line":248,"end_line":280,"hash":"ef2f71ea6d53cb654ead9892c2baec64e82ac350f75449bfc911e5763625790a"},{"id":"func/ignoredPathMatcher.matches","name":"ignoredPathMatcher.matches","line":282,"end_line":319,"hash":"43b8e9b598c99327306a6cf812fa9ebabf8d407ef340e1a4c6177194b99eb86f"},{"id":"func/analyzer.sourcePaths","name":"analyzer.sourcePaths","line":321,"end_line":335,"hash":"ca5e2646bf79b54208e93b34b57d82df06947375a097580d763d936727bce443"},{"id":"func/analyzer.collectConfiguredSourcePath","name":"analyzer.collectConfiguredSourcePath","line":337,"end_line":357,"hash":"87f2e357d890bc5c5973300d3b6e3130480cee207eff170136e588192bb37495"},{"id":"func/analyzer.collectSourceFile","name":"analyzer.collectSourceFile","line":359,"end_line":378,"hash":"79cc529297d8b83d19e8dcce2e62ffffe7757fd3d7ca9f8da150fb897da8d45a"},{"id":"func/analyzer.walkSourceDirectory","name":"analyzer.walkSourceDirectory","line":380,"end_line":393,"hash":"d7e440a774d33003138f07ab9690311d93d3bb11b0b59112d90779d5f66953d6"},{"id":"func/analyzer.collectWalkedSourcePath","name":"analyzer.collectWalkedSourcePath","line":395,"end_line":430,"hash":"ef47e247f5af713d44b7f0cebb1b25cee2578618fde6b411509d595966687827"},{"id":"func/analyzer.inspectSourceFile","name":"analyzer.inspectSourceFile","line":432,"end_line":523,"hash":"7b8c0f0132c89aa51098e4c2d648efb3462fdd780755e815f0f274e6ebd40993"},{"id":"func/hasFunctionData","name":"hasFunctionData","line":525,"end_line":533,"hash":"d06cecd4aa7d0ef4549ae5d251c85d54c830f5bf95dff40d27593b244be2ed37"},{"id":"func/countNamedTypes","name":"countNamedTypes","line":535,"end_line":556,"hash":"7269d3e77aa1bf662cb4e54d5c09fa72e159000c99cf165e2e385f33b6b8dd99"}]}
// mutate4go-manifest-end
