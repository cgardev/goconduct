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

var errModuleDeclarationNotFound = errors.New("module declaration not found")

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
		return nil, fmt.Errorf("repository root must not be empty")
	}
	absoluteRoot, err := filepath.Abs(configuration.RepositoryRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root %s: %w", configuration.RepositoryRoot, err)
	}
	information, err := os.Stat(filepath.Join(absoluteRoot, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("inspect repository module file: %w", err)
	}
	if information.IsDir() {
		return nil, fmt.Errorf("inspect repository module file: %w", errModuleDeclarationNotFound)
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
		return Graph{}, fmt.Errorf("encode graph revision input: %w", err)
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
		return "", fmt.Errorf("inspect repository module sum: %w", err)
	}

	var buffer []byte
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		information, err := os.Stat(path)
		if err != nil {
			return "", fmt.Errorf("inspect repository source file %s: %w", path, err)
		}
		relativePath, err := filepath.Rel(analyzer.repositoryRoot, path)
		if err != nil {
			return "", fmt.Errorf("resolve repository source file %s: %w", path, err)
		}
		buffer = append(buffer, filepath.ToSlash(relativePath)...)
		buffer = append(buffer, "\x00"...)
		buffer = strconv.AppendInt(buffer, information.Size(), 10)
		buffer = append(buffer, "\x00"...)
		buffer = strconv.AppendInt(buffer, information.ModTime().UnixNano(), 10)
		buffer = append(buffer, '\n')
	}
	digest := sha256.Sum256(buffer)
	return hex.EncodeToString(digest[:]), nil
}

func readModulePath(repositoryRoot string) (string, error) {
	path := filepath.Join(repositoryRoot, "go.mod")
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read module file: %w", err)
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
	return "", fmt.Errorf("read module file: %w", errModuleDeclarationNotFound)
}

func normalizeAnalysisPaths(configuredPaths []string) ([]string, error) {
	if len(configuredPaths) == 0 {
		return nil, fmt.Errorf("analysis paths must contain at least one repository-relative path")
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
		return "", fmt.Errorf("path must be a non-empty relative path")
	}
	if configuredPath != strings.TrimSpace(configuredPath) {
		return "", fmt.Errorf("path must be a non-empty relative path")
	}
	if filepath.IsAbs(configuredPath) {
		return "", fmt.Errorf("path must be a non-empty relative path")
	}
	cleaned := filepath.Clean(configuredPath)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must remain inside the repository root")
	}
	return filepath.ToSlash(cleaned), nil
}

func newIgnoredPathMatcher(configuredPatterns []string) (ignoredPathMatcher, error) {
	patterns := make(stringSet)
	for _, configuredPattern := range configuredPatterns {
		if configuredPattern != strings.TrimSpace(configuredPattern) || strings.Contains(configuredPattern, "\\") {
			return ignoredPathMatcher{}, fmt.Errorf(
				"ignored path pattern %q must be a non-empty relative slash path",
				configuredPattern,
			)
		}
		for segment := range strings.SplitSeq(configuredPattern, "/") {
			if segment == "" || segment == "." || segment == ".." {
				return ignoredPathMatcher{}, fmt.Errorf(
					"ignored path pattern %q contains an invalid segment",
					configuredPattern,
				)
			}
		}
		if _, err := path.Match(configuredPattern, "validation"); err != nil {
			return ignoredPathMatcher{}, fmt.Errorf(
				"compile ignored path pattern %q: %w",
				configuredPattern,
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
					return matches, fmt.Errorf("match ignored path pattern %q: %w", pattern, err)
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
				return matches, fmt.Errorf("match ignored path pattern %q: %w", pattern, err)
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
	information, err := os.Stat(absolutePath)
	if err != nil {
		return fmt.Errorf("inspect analysis path %s: %w", analysisPath, err)
	}
	if information.IsDir() {
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
		return fmt.Errorf("analysis path %s must be a directory or Go source file", analysisPath)
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
		return fmt.Errorf("walk analysis path %s: %w", analysisPath, err)
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
		return fmt.Errorf("visit %s: %w", sourcePath, walkError)
	}
	relativePath, err := filepath.Rel(analyzer.repositoryRoot, sourcePath)
	if err != nil {
		return fmt.Errorf("resolve repository source path %s: %w", sourcePath, err)
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
		return nil, fmt.Errorf("resolve source path %s: %w", sourcePath, err)
	}
	relativePath = filepath.ToSlash(relativePath)
	descriptor, classified := analyzer.classifier.classify(relativePath)
	if !classified {
		return nil, nil
	}
	payload, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read source file %s: %w", relativePath, err)
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
// {"version":1,"tested_at":"2026-08-21T17:27:33Z","module_hash":"aa06a84b9dd727a9464891da9a198637d579779198325e7efcf495b17067a615","functions":[{"id":"func/newAnalyzer","name":"newAnalyzer","line":38,"end_line":77,"hash":"ba0447ed5957ebe3850942c846425c639341cc2d2f711a71e2412eeb8d1b8d05"},{"id":"func/analyzer.analyze","name":"analyzer.analyze","line":79,"end_line":135,"hash":"9654db2ded09efa2a3d9966c4f457a8bb3a43da64daeafcd81c9c2953cbae744"},{"id":"func/analyzer.repositoryPath","name":"analyzer.repositoryPath","line":137,"end_line":139,"hash":"a11e0cbd14146576a5f65fa33b4ac3b96e43a3e4c7169bcf6b29d33466888347"},{"id":"func/analyzer.snapshot","name":"analyzer.snapshot","line":141,"end_line":179,"hash":"6c75b6bf049d34b0c0a564411c068c4018368b42967a0c0fbde66c1ae862248a"},{"id":"func/readModulePath","name":"readModulePath","line":181,"end_line":201,"hash":"3e80b06afd0e3742f8fa44577f3c85f873ebc83d596df3fae12f1cc2b6d0efdc"},{"id":"func/normalizeAnalysisPaths","name":"normalizeAnalysisPaths","line":203,"end_line":216,"hash":"0a1550906681003eb46f370ec3e1578e74ce0329eab580e6e1684ad1d9c5853b"},{"id":"func/normalizeRepositoryPath","name":"normalizeRepositoryPath","line":218,"end_line":233,"hash":"6957293b95ea88edb99c66a74bce84790b941736461cf8d9f4f1d8970a55274a"},{"id":"func/newIgnoredPathMatcher","name":"newIgnoredPathMatcher","line":235,"end_line":262,"hash":"723906109af7a1ccc25c5e6b58920f45ccc9db85e64efbc97c006179c251fb2d"},{"id":"func/ignoredPathMatcher.matches","name":"ignoredPathMatcher.matches","line":264,"end_line":295,"hash":"af0ba5f312f9bed5540e84166e618471d005e32d07ec240c7208cf496f0077e3"},{"id":"func/analyzer.sourcePaths","name":"analyzer.sourcePaths","line":297,"end_line":311,"hash":"ca5e2646bf79b54208e93b34b57d82df06947375a097580d763d936727bce443"},{"id":"func/analyzer.collectConfiguredSourcePath","name":"analyzer.collectConfiguredSourcePath","line":313,"end_line":330,"hash":"20f739d5360b81fe2c6e7f8cdd4bf7d525521a8d1b5aca61abb71ac9fd17ef37"},{"id":"func/analyzer.collectSourceFile","name":"analyzer.collectSourceFile","line":332,"end_line":348,"hash":"64fb2d40ca0f6aac41d6ec29bad1ec85ac62bad05119540172f7ce735d23be96"},{"id":"func/analyzer.walkSourceDirectory","name":"analyzer.walkSourceDirectory","line":350,"end_line":363,"hash":"ff30ce32dd5b9fbc2371b70cc4452691ac83d02ffdf962c92859ddd9daac4ea7"},{"id":"func/analyzer.collectWalkedSourcePath","name":"analyzer.collectWalkedSourcePath","line":365,"end_line":397,"hash":"35b4bd443123a921218ccf134f79477030c5d99df6579309653bd237e3ea0ca1"},{"id":"func/analyzer.inspectSourceFile","name":"analyzer.inspectSourceFile","line":399,"end_line":490,"hash":"98457d0552665a6b2f9dfba51d23bb8515ec651df44c557087bd52b153fec5e7"},{"id":"func/hasFunctionData","name":"hasFunctionData","line":492,"end_line":500,"hash":"d06cecd4aa7d0ef4549ae5d251c85d54c830f5bf95dff40d27593b244be2ed37"},{"id":"func/countNamedTypes","name":"countNamedTypes","line":502,"end_line":523,"hash":"7269d3e77aa1bf662cb4e54d5c09fa72e159000c99cf165e2e385f33b6b8dd99"}]}
// mutate4go-manifest-end
