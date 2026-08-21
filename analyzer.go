package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const graphSchemaVersion = 1

var errModuleDeclarationNotFound = errors.New("module declaration not found")

type componentKind string

const (
	componentKindApplication       componentKind = "application"
	componentKindApplicationModule componentKind = "application-module"
	componentKindSharedModule      componentKind = "shared-module"
	componentKindLibrary           componentKind = "library"
	componentKindInfrastructure    componentKind = "infrastructure"
	componentKindDevelopment       componentKind = "development"
)

// Graph is a stable representation of the repository's architectural dependencies.
type Graph struct {
	SchemaVersion int            `json:"schemaVersion"`
	Revision      string         `json:"revision"`
	ModulePath    string         `json:"modulePath"`
	Summary       GraphSummary   `json:"summary"`
	Components    []Component    `json:"components"`
	Relationships []Relationship `json:"relationships"`
	Cycles        [][]string     `json:"cycles"`
	Diagnostics   []Diagnostic   `json:"diagnostics"`
}

// GraphSummary contains aggregate values shown before the detailed graph.
type GraphSummary struct {
	Components              int `json:"components"`
	Relationships           int `json:"relationships"`
	ProductionRelationships int `json:"productionRelationships"`
	TestOnlyRelationships   int `json:"testOnlyRelationships"`
	Applications            int `json:"applications"`
	ApplicationModules      int `json:"applicationModules"`
	SharedModules           int `json:"sharedModules"`
	Libraries               int `json:"libraries"`
	Infrastructure          int `json:"infrastructure"`
	DevelopmentTools        int `json:"developmentTools"`
	Cycles                  int `json:"cycles"`
	Concerns                int `json:"concerns"`
}

// Component describes one architectural unit and its coupling metrics.
type Component struct {
	Identifier                 string        `json:"id"`
	Name                       string        `json:"name"`
	Kind                       componentKind `json:"kind"`
	Application                string        `json:"application,omitempty"`
	Packages                   int           `json:"packages"`
	SourceFiles                int           `json:"sourceFiles"`
	ProductionFiles            int           `json:"productionFiles"`
	TestFiles                  int           `json:"testFiles"`
	DirectDependencies         int           `json:"directDependencies"`
	ProductionDependencies     int           `json:"productionDependencies"`
	TestOnlyDependencies       int           `json:"testOnlyDependencies"`
	DirectDependants           int           `json:"directDependants"`
	ProductionDependants       int           `json:"productionDependants"`
	TestOnlyDependants         int           `json:"testOnlyDependants"`
	TransitiveDependencies     int           `json:"transitiveDependencies"`
	TransitiveDependants       int           `json:"transitiveDependants"`
	ImporterPackages           int           `json:"importerPackages"`
	ProductionImporterPackages int           `json:"productionImporterPackages"`
	TestImporterPackages       int           `json:"testImporterPackages"`
	ApplicationReach           int           `json:"applicationReach"`
	Applications               []string      `json:"applications"`
	Instability                float64       `json:"instability"`
	InCycle                    bool          `json:"inCycle"`
}

// Relationship describes imports between two architectural units.
type Relationship struct {
	Source               string   `json:"source"`
	Target               string   `json:"target"`
	ProductionReferences int      `json:"productionReferences"`
	TestReferences       int      `json:"testReferences"`
	SourcePackages       []string `json:"sourcePackages"`
	TargetPackages       []string `json:"targetPackages"`
	TestOnly             bool     `json:"testOnly"`
	Concerns             []string `json:"concerns"`
}

// Diagnostic reports a source file that could not be fully inspected.
type Diagnostic struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type analyzer struct {
	repositoryRoot string
}

type componentDescriptor struct {
	identifier  string
	name        string
	kind        componentKind
	application string
}

type sourceFile struct {
	relativePath string
	packagePath  string
	component    componentDescriptor
	test         bool
	imports      []string
	diagnostics  []Diagnostic
}

type componentAccumulator struct {
	descriptor      componentDescriptor
	packages        stringSet
	sourceFiles     stringSet
	productionFiles stringSet
	testFiles       stringSet
}

type relationshipKey struct {
	source string
	target string
}

type relationshipAccumulator struct {
	productionFiles          stringSet
	testFiles                stringSet
	sourcePackages           stringSet
	productionSourcePackages stringSet
	testSourcePackages       stringSet
	targetPackages           stringSet
}

type stringSet map[string]struct{}

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
	case "node_modules", "vendor", "testdata", "_resources":
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
		parser.ImportsOnly|parser.AllErrors,
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
	return file, nil
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

func collectComponentFile(components map[string]*componentAccumulator, file sourceFile) {
	component := ensureComponent(components, file.component)
	component.packages.add(file.packagePath)
	component.sourceFiles.add(file.relativePath)
	if file.test {
		component.testFiles.add(file.relativePath)
		return
	}
	component.productionFiles.add(file.relativePath)
}

func collectRelationships(
	components map[string]*componentAccumulator,
	relationships map[relationshipKey]*relationshipAccumulator,
	modulePath string,
	file sourceFile,
) {
	for _, importPath := range file.imports {
		targetPackage := strings.TrimPrefix(importPath, modulePath+"/")
		target, modeled := classifyComponent(targetPackage)
		if !modeled || target.identifier == file.component.identifier {
			continue
		}
		ensureComponent(components, target)
		key := relationshipKey{source: file.component.identifier, target: target.identifier}
		relationship, exists := relationships[key]
		if !exists {
			relationship = &relationshipAccumulator{
				productionFiles:          make(stringSet),
				testFiles:                make(stringSet),
				sourcePackages:           make(stringSet),
				productionSourcePackages: make(stringSet),
				testSourcePackages:       make(stringSet),
				targetPackages:           make(stringSet),
			}
			relationships[key] = relationship
		}
		relationship.sourcePackages.add(file.packagePath)
		relationship.targetPackages.add(targetPackage)
		if file.test {
			relationship.testFiles.add(file.relativePath)
			relationship.testSourcePackages.add(file.packagePath)
			continue
		}
		relationship.productionFiles.add(file.relativePath)
		relationship.productionSourcePackages.add(file.packagePath)
	}
}

func ensureComponent(
	components map[string]*componentAccumulator,
	descriptor componentDescriptor,
) *componentAccumulator {
	component, exists := components[descriptor.identifier]
	if exists {
		return component
	}
	component = &componentAccumulator{
		descriptor:      descriptor,
		packages:        make(stringSet),
		sourceFiles:     make(stringSet),
		productionFiles: make(stringSet),
		testFiles:       make(stringSet),
	}
	components[descriptor.identifier] = component
	return component
}

func buildGraph(
	modulePath string,
	componentData map[string]*componentAccumulator,
	relationshipData map[relationshipKey]*relationshipAccumulator,
	diagnostics []Diagnostic,
) Graph {
	identifiers := sortedMapKeys(componentData)
	relationshipKeys := make([]relationshipKey, 0, len(relationshipData))
	for key := range relationshipData {
		relationshipKeys = append(relationshipKeys, key)
	}
	sort.Slice(relationshipKeys, func(first, second int) bool {
		if relationshipKeys[first].source == relationshipKeys[second].source {
			return relationshipKeys[first].target < relationshipKeys[second].target
		}
		return relationshipKeys[first].source < relationshipKeys[second].source
	})

	allDependencies := newAdjacency(identifiers)
	allDependants := newAdjacency(identifiers)
	productionDependencies := newAdjacency(identifiers)
	productionDependants := newAdjacency(identifiers)
	incomingPackages := make(map[string]stringSet, len(identifiers))
	incomingProductionPackages := make(map[string]stringSet, len(identifiers))
	incomingTestPackages := make(map[string]stringSet, len(identifiers))
	productionIncoming := make(map[string]stringSet, len(identifiers))
	testOnlyIncoming := make(map[string]stringSet, len(identifiers))
	testOnlyOutgoing := make(map[string]stringSet, len(identifiers))
	for _, identifier := range identifiers {
		incomingPackages[identifier] = make(stringSet)
		incomingProductionPackages[identifier] = make(stringSet)
		incomingTestPackages[identifier] = make(stringSet)
		productionIncoming[identifier] = make(stringSet)
		testOnlyIncoming[identifier] = make(stringSet)
		testOnlyOutgoing[identifier] = make(stringSet)
	}

	relationships := make([]Relationship, 0, len(relationshipKeys))
	for _, key := range relationshipKeys {
		data := relationshipData[key]
		testOnly := len(data.productionFiles) == 0
		allDependencies[key.source].add(key.target)
		allDependants[key.target].add(key.source)
		if testOnly {
			testOnlyIncoming[key.target].add(key.source)
			testOnlyOutgoing[key.source].add(key.target)
		} else {
			productionDependencies[key.source].add(key.target)
			productionDependants[key.target].add(key.source)
			productionIncoming[key.target].add(key.source)
		}
		incomingPackages[key.target].addAll(data.sourcePackages)
		incomingProductionPackages[key.target].addAll(data.productionSourcePackages)
		incomingTestPackages[key.target].addAll(data.testSourcePackages)
		relationships = append(relationships, Relationship{
			Source:               key.source,
			Target:               key.target,
			ProductionReferences: len(data.productionFiles),
			TestReferences:       len(data.testFiles),
			SourcePackages:       sortedSet(data.sourcePackages),
			TargetPackages:       sortedSet(data.targetPackages),
			TestOnly:             testOnly,
			Concerns: relationshipConcerns(
				componentData[key.source].descriptor,
				componentData[key.target].descriptor,
				testOnly,
			),
		})
	}

	cycles := stronglyConnectedComponents(identifiers, productionDependencies)
	cycleMembers := make(stringSet)
	for _, cycle := range cycles {
		cycleMembers.addAll(newStringSet(cycle...))
	}

	components := make([]Component, 0, len(identifiers))
	for _, identifier := range identifiers {
		data := componentData[identifier]
		transitiveDependencies := reachable(identifier, productionDependencies)
		transitiveDependants := reachable(identifier, productionDependants)
		applications := reachedApplications(identifier, productionDependants, componentData)
		fanIn := len(productionIncoming[identifier])
		fanOut := len(productionDependencies[identifier])
		components = append(components, Component{
			Identifier:                 identifier,
			Name:                       data.descriptor.name,
			Kind:                       data.descriptor.kind,
			Application:                data.descriptor.application,
			Packages:                   len(data.packages),
			SourceFiles:                len(data.sourceFiles),
			ProductionFiles:            len(data.productionFiles),
			TestFiles:                  len(data.testFiles),
			DirectDependencies:         len(allDependencies[identifier]),
			ProductionDependencies:     len(productionDependencies[identifier]),
			TestOnlyDependencies:       len(testOnlyOutgoing[identifier]),
			DirectDependants:           len(allDependants[identifier]),
			ProductionDependants:       fanIn,
			TestOnlyDependants:         len(testOnlyIncoming[identifier]),
			TransitiveDependencies:     len(transitiveDependencies),
			TransitiveDependants:       len(transitiveDependants),
			ImporterPackages:           len(incomingPackages[identifier]),
			ProductionImporterPackages: len(incomingProductionPackages[identifier]),
			TestImporterPackages:       len(incomingTestPackages[identifier]),
			ApplicationReach:           len(applications),
			Applications:               applications,
			Instability:                instability(fanIn, fanOut),
			InCycle:                    cycleMembers.contains(identifier),
		})
	}

	sort.Slice(diagnostics, func(first, second int) bool {
		if diagnostics[first].Path == diagnostics[second].Path {
			return diagnostics[first].Message < diagnostics[second].Message
		}
		return diagnostics[first].Path < diagnostics[second].Path
	})
	graph := Graph{
		SchemaVersion: graphSchemaVersion,
		ModulePath:    modulePath,
		Components:    components,
		Relationships: relationships,
		Cycles:        cycles,
		Diagnostics:   diagnostics,
	}
	graph.Summary = summarizeGraph(graph)
	return graph
}

func summarizeGraph(graph Graph) GraphSummary {
	summary := GraphSummary{
		Components:    len(graph.Components),
		Relationships: len(graph.Relationships),
		Cycles:        len(graph.Cycles),
	}
	for _, component := range graph.Components {
		switch component.Kind {
		case componentKindApplication:
			summary.Applications++
		case componentKindApplicationModule:
			summary.ApplicationModules++
		case componentKindSharedModule:
			summary.SharedModules++
		case componentKindLibrary:
			summary.Libraries++
		case componentKindInfrastructure:
			summary.Infrastructure++
		case componentKindDevelopment:
			summary.DevelopmentTools++
		}
	}
	for _, relationship := range graph.Relationships {
		if relationship.TestOnly {
			summary.TestOnlyRelationships++
		} else {
			summary.ProductionRelationships++
		}
		summary.Concerns += len(relationship.Concerns)
	}
	return summary
}

func relationshipConcerns(
	source componentDescriptor,
	target componentDescriptor,
	testOnly bool,
) []string {
	if testOnly {
		return []string{}
	}
	concerns := make(stringSet)
	if target.kind == componentKindDevelopment {
		concerns.add("production-depends-on-development")
	}
	if source.kind == componentKindLibrary && isFeatureKind(target.kind) {
		concerns.add("library-depends-on-feature")
	}
	if (source.kind == componentKindSharedModule || source.kind == componentKindInfrastructure) &&
		(target.kind == componentKindApplication || target.kind == componentKindApplicationModule) {
		concerns.add("shared-foundation-depends-on-application")
	}
	if source.kind == componentKindApplicationModule &&
		target.kind == componentKindApplicationModule &&
		source.application != target.application {
		concerns.add("cross-application-module-dependency")
	}
	return sortedSet(concerns)
}

func isFeatureKind(kind componentKind) bool {
	return kind == componentKindApplication ||
		kind == componentKindApplicationModule ||
		kind == componentKindSharedModule
}

func newAdjacency(identifiers []string) map[string]stringSet {
	adjacency := make(map[string]stringSet, len(identifiers))
	for _, identifier := range identifiers {
		adjacency[identifier] = make(stringSet)
	}
	return adjacency
}

func reachable(start string, adjacency map[string]stringSet) stringSet {
	visited := make(stringSet)
	stack := sortedSet(adjacency[start])
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current == start || visited.contains(current) {
			continue
		}
		visited.add(current)
		stack = append(stack, sortedSet(adjacency[current])...)
	}
	return visited
}

func reachedApplications(
	start string,
	dependants map[string]stringSet,
	components map[string]*componentAccumulator,
) []string {
	reached := reachable(start, dependants)
	reached.add(start)
	applications := make(stringSet)
	for identifier := range reached {
		descriptor := components[identifier].descriptor
		if descriptor.application != "" {
			applications.add(descriptor.application)
		}
	}
	return sortedSet(applications)
}

func instability(fanIn, fanOut int) float64 {
	total := fanIn + fanOut
	if total == 0 {
		return 0
	}
	return float64(fanOut) / float64(total)
}

func stronglyConnectedComponents(
	identifiers []string,
	adjacency map[string]stringSet,
) [][]string {
	index := 0
	indices := make(map[string]int, len(identifiers))
	lowLinks := make(map[string]int, len(identifiers))
	onStack := make(stringSet)
	var stack []string
	var cycles [][]string

	var connect func(string)
	connect = func(identifier string) {
		indices[identifier] = index
		lowLinks[identifier] = index
		index++
		stack = append(stack, identifier)
		onStack.add(identifier)

		for _, dependency := range sortedSet(adjacency[identifier]) {
			dependencyIndex, visited := indices[dependency]
			if !visited {
				connect(dependency)
				lowLinks[identifier] = min(lowLinks[identifier], lowLinks[dependency])
				continue
			}
			if onStack.contains(dependency) {
				lowLinks[identifier] = min(lowLinks[identifier], dependencyIndex)
			}
		}

		if lowLinks[identifier] != indices[identifier] {
			return
		}
		var component []string
		for len(stack) > 0 {
			last := len(stack) - 1
			member := stack[last]
			stack = stack[:last]
			delete(onStack, member)
			component = append(component, member)
			if member == identifier {
				break
			}
		}
		if len(component) > 1 {
			sort.Strings(component)
			cycles = append(cycles, component)
		}
	}

	for _, identifier := range identifiers {
		if _, visited := indices[identifier]; !visited {
			connect(identifier)
		}
	}
	sort.Slice(cycles, func(first, second int) bool {
		return strings.Join(cycles[first], "\x00") < strings.Join(cycles[second], "\x00")
	})
	return cycles
}

func sortedMapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func newStringSet(values ...string) stringSet {
	set := make(stringSet, len(values))
	for _, value := range values {
		set.add(value)
	}
	return set
}

func (set stringSet) add(value string) {
	set[value] = struct{}{}
}

func (set stringSet) addAll(other stringSet) {
	for value := range other {
		set.add(value)
	}
}

func (set stringSet) contains(value string) bool {
	_, exists := set[value]
	return exists
}

func sortedSet(set stringSet) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
