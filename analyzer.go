package main

import (
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

func (analyzer *analyzer) analyze() (Graph, error) {
	modulePath, err := readModulePath(analyzer.repositoryRoot)
	if err != nil {
		return Graph{}, err
	}
	paths, err := analyzer.sourcePaths()
	if err != nil {
		return Graph{}, err
	}

	components := make(map[string]*componentAccumulator)
	relationships := make(map[relationshipKey]*relationshipAccumulator)
	var diagnostics []Diagnostic
	for _, path := range paths {
		file, err := analyzer.inspectSourceFile(modulePath, path)
		if err != nil {
			return Graph{}, err
		}
		if file == nil {
			continue
		}
		diagnostics = append(diagnostics, file.diagnostics...)
		collectComponentFile(components, *file)
		collectRelationships(components, relationships, *file)
	}

	graph := buildGraph(modulePath, components, relationships, diagnostics)
	graph.Scope = analyzer.scope
	payload, err := json.Marshal(graph)
	if err != nil {
		return Graph{}, fmt.Errorf("encode graph revision input: %w", err)
	}
	digest := sha256.Sum256(payload)
	graph.Revision = hex.EncodeToString(digest[:])
	return graph, nil
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
	cleaned := strings.Trim(filepathToSlash(relativePath), "/")
	if cleaned == "" || cleaned == "." {
		return false, nil
	}
	segments := strings.Split(cleaned, "/")
	for _, pattern := range matcher.patterns {
		if !strings.Contains(pattern, "/") {
			for _, segment := range segments {
				matches, err := path.Match(pattern, segment)
				if err != nil {
					return false, fmt.Errorf("match ignored path pattern %q: %w", pattern, err)
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
				return false, fmt.Errorf("match ignored path pattern %q: %w", pattern, err)
			}
			if matches {
				return true, nil
			}
		}
	}
	return false, nil
}

func (analyzer *analyzer) sourcePaths() ([]string, error) {
	paths := make(stringSet)
	for _, analysisPath := range analyzer.analysisPaths {
		absolutePath := filepath.Join(analyzer.repositoryRoot, filepath.FromSlash(analysisPath))
		information, err := os.Stat(absolutePath)
		if err != nil {
			return nil, fmt.Errorf("inspect analysis path %s: %w", analysisPath, err)
		}
		if !information.IsDir() {
			if filepath.Ext(absolutePath) != ".go" {
				return nil, fmt.Errorf("analysis path %s must be a directory or Go source file", analysisPath)
			}
			ignored, err := analyzer.ignoredPaths.matches(analysisPath)
			if err != nil {
				return nil, err
			}
			if !ignored {
				paths.add(absolutePath)
			}
			continue
		}

		err = filepath.WalkDir(absolutePath, func(sourcePath string, entry fs.DirEntry, walkError error) error {
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
		})
		if err != nil {
			return nil, fmt.Errorf("walk analysis path %s: %w", analysisPath, err)
		}
	}
	return sortedSet(paths), nil
}

func (analyzer *analyzer) inspectSourceFile(modulePath, sourcePath string) (*sourceFile, error) {
	relativePath, err := filepath.Rel(analyzer.repositoryRoot, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("resolve source path %s: %w", sourcePath, err)
	}
	relativePath = filepath.ToSlash(relativePath)
	descriptor, modeled := analyzer.classifier.classify(relativePath)
	if !modeled {
		return nil, nil
	}
	payload, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read source file %s: %w", relativePath, err)
	}

	parsed, parseError := parser.ParseFile(
		token.NewFileSet(),
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
		target, modeled := analyzer.classifier.classify(targetPackage)
		if modeled {
			importsByPackage[targetPackage] = sourceImport{
				packagePath: targetPackage,
				component:   target,
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
	return file, nil
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
// {"version":1,"tested_at":"2026-08-21T08:10:17Z","module_hash":"be544827ece121e24cf1d6346b1d1426556c2a9a24151bb59d90d842263cfda3","functions":[{"id":"func/newAnalyzer","name":"newAnalyzer","line":35,"end_line":74,"hash":"ba0447ed5957ebe3850942c846425c639341cc2d2f711a71e2412eeb8d1b8d05"},{"id":"func/analyzer.analyze","name":"analyzer.analyze","line":76,"end_line":111,"hash":"ab998b93726f6fe162d3a61835435ed1f15a345a5814e25852ad04c9f26702db"},{"id":"func/readModulePath","name":"readModulePath","line":113,"end_line":133,"hash":"3e80b06afd0e3742f8fa44577f3c85f873ebc83d596df3fae12f1cc2b6d0efdc"},{"id":"func/normalizeAnalysisPaths","name":"normalizeAnalysisPaths","line":135,"end_line":148,"hash":"0a1550906681003eb46f370ec3e1578e74ce0329eab580e6e1684ad1d9c5853b"},{"id":"func/normalizeRepositoryPath","name":"normalizeRepositoryPath","line":150,"end_line":165,"hash":"6957293b95ea88edb99c66a74bce84790b941736461cf8d9f4f1d8970a55274a"},{"id":"func/newIgnoredPathMatcher","name":"newIgnoredPathMatcher","line":167,"end_line":194,"hash":"723906109af7a1ccc25c5e6b58920f45ccc9db85e64efbc97c006179c251fb2d"},{"id":"func/ignoredPathMatcher.matches","name":"ignoredPathMatcher.matches","line":196,"end_line":227,"hash":"3589d80c58884e6dd1b35917b01654f7460221a7a7c0db1f442fa9f6259024c8"},{"id":"func/analyzer.sourcePaths","name":"analyzer.sourcePaths","line":229,"end_line":280,"hash":"20f179474d954ca33b6059ca0a1ae1964030d55b11f3762b6247a62300d75e8c"},{"id":"func/analyzer.inspectSourceFile","name":"analyzer.inspectSourceFile","line":282,"end_line":358,"hash":"0bcfbdb164f63028f1fbd39f215d52ccf8d1e54a7f0b1c7892855b8c82379733"},{"id":"func/countNamedTypes","name":"countNamedTypes","line":360,"end_line":381,"hash":"7269d3e77aa1bf662cb4e54d5c09fa72e159000c99cf165e2e385f33b6b8dd99"}]}
// mutate4go-manifest-end
