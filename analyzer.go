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
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var errModuleDeclarationNotFound = errors.New("module declaration not found")

type analyzer struct {
	repositoryRoot string
}

func newAnalyzer(repositoryRoot string) (*analyzer, error) {
	absoluteRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root %s: %w", repositoryRoot, err)
	}
	information, err := os.Stat(filepath.Join(absoluteRoot, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("inspect repository module file: %w", err)
	}
	if information.IsDir() {
		return nil, fmt.Errorf("inspect repository module file: %w", errModuleDeclarationNotFound)
	}
	return &analyzer{repositoryRoot: absoluteRoot}, nil
}

func (analyzer *analyzer) analyze() (Graph, error) {
	modulePath, err := readModulePath(analyzer.repositoryRoot)
	if err != nil {
		return Graph{}, err
	}
	paths, err := goSourcePaths(analyzer.repositoryRoot)
	if err != nil {
		return Graph{}, err
	}

	components := make(map[string]*componentAccumulator)
	relationships := make(map[relationshipKey]*relationshipAccumulator)
	var diagnostics []Diagnostic
	for _, path := range paths {
		file, err := inspectSourceFile(analyzer.repositoryRoot, modulePath, path)
		if err != nil {
			return Graph{}, err
		}
		if file == nil {
			continue
		}
		diagnostics = append(diagnostics, file.diagnostics...)
		collectComponentFile(components, *file)
		collectRelationships(components, relationships, modulePath, *file)
	}

	graph := buildGraph(modulePath, components, relationships, diagnostics)
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

func goSourcePaths(repositoryRoot string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(repositoryRoot, func(path string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return fmt.Errorf("visit %s: %w", path, walkError)
		}
		if entry.IsDir() && path != repositoryRoot && ignoredDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk repository source files: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func ignoredDirectory(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "testdata", "target", "_resources":
		return true
	default:
		return false
	}
}

func inspectSourceFile(repositoryRoot, modulePath, path string) (*sourceFile, error) {
	relativePath, err := filepath.Rel(repositoryRoot, path)
	if err != nil {
		return nil, fmt.Errorf("resolve source path %s: %w", path, err)
	}
	relativePath = filepath.ToSlash(relativePath)
	descriptor, modeled := classifyComponent(relativePath)
	if !modeled {
		return nil, nil
	}
	payload, err := os.ReadFile(path)
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

	imports := make(stringSet)
	for _, specification := range parsed.Imports {
		importPath, err := strconv.Unquote(specification.Path.Value)
		if err != nil {
			file.diagnostics = append(file.diagnostics, Diagnostic{
				Path:    relativePath,
				Message: fmt.Sprintf("decode import %s: %v", specification.Path.Value, err),
			})
			continue
		}
		if importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/") {
			imports[importPath] = struct{}{}
		}
	}
	file.imports = sortedSet(imports)
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

func classifyComponent(relativePath string) (componentDescriptor, bool) {
	parts := strings.Split(filepath.ToSlash(relativePath), "/")
	if len(parts) < 2 {
		return componentDescriptor{}, false
	}
	if parts[0] == "cmd" {
		return classifyCommandComponent(parts)
	}
	if parts[0] != "internal" {
		return componentDescriptor{}, false
	}

	switch parts[1] {
	case "module":
		if len(parts) < 3 {
			return componentDescriptor{}, false
		}
		return componentDescriptor{
			identifier: strings.Join(parts[:3], "/"),
			name:       parts[2],
			kind:       componentKindSharedModule,
		}, true
	case "library":
		if len(parts) < 3 {
			return componentDescriptor{}, false
		}
		return componentDescriptor{
			identifier: strings.Join(parts[:3], "/"),
			name:       parts[2],
			kind:       componentKindLibrary,
		}, true
	case "devtool":
		if len(parts) < 3 {
			return componentDescriptor{}, false
		}
		return componentDescriptor{
			identifier: strings.Join(parts[:3], "/"),
			name:       parts[2],
			kind:       componentKindDevelopment,
		}, true
	default:
		return componentDescriptor{
			identifier: strings.Join(parts[:2], "/"),
			name:       parts[1],
			kind:       componentKindInfrastructure,
		}, true
	}
}

func classifyCommandComponent(parts []string) (componentDescriptor, bool) {
	if len(parts) < 2 {
		return componentDescriptor{}, false
	}
	application := parts[1]
	if len(parts) >= 5 && parts[2] == "internal" && parts[3] == "module" {
		return componentDescriptor{
			identifier:  strings.Join(parts[:5], "/"),
			name:        parts[4],
			kind:        componentKindApplicationModule,
			application: application,
		}, true
	}
	return componentDescriptor{
		identifier:  strings.Join(parts[:2], "/"),
		name:        application,
		kind:        componentKindApplication,
		application: application,
	}, true
}
