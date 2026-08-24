package calculation

import (
	"github.com/cgardev/goconduct/internal/architecture"
	"github.com/cgardev/goconduct/pkg/report"
)

// ComponentDescriptor identifies one classified source component.
type ComponentDescriptor struct {
	Identifier  string
	Name        string
	Role        report.ComponentRole
	Category    string
	Application string
}

// SourceFile contains the analyzer facts required for component calculation.
type SourceFile struct {
	RelativePath  string
	PackagePath   string
	Component     ComponentDescriptor
	Test          bool
	Imports       []SourceImport
	AbstractTypes int
	ConcreteTypes int
}

// SourceImport contains one classified import and its source location.
type SourceImport struct {
	PackagePath string
	Component   ComponentDescriptor
	Site        ImportSite
}

// ComponentAccumulator contains the collected facts for one component.
type ComponentAccumulator = componentAccumulator

// RelationshipAccumulator contains the collected facts for one relationship.
type RelationshipAccumulator = relationshipAccumulator

// RelationshipKey identifies one directed component relationship.
type RelationshipKey struct {
	Source string
	Target string
}

// StringSet contains unique string values used by graph algorithms.
type StringSet = stringSet

// FunctionDeclaration contains the analyzer facts for one function.
type FunctionDeclaration struct {
	Identifier      string
	Name            string
	PackagePath     string
	Component       string
	RelativePath    string
	Line            int
	Receiver        string
	Method          bool
	Exported        bool
	Test            bool
	Synthetic       bool
	InAnalysisScope bool
	SourcePosition  int
}

// FunctionReference contains one resolved function call.
type FunctionReference struct {
	Source string
	Target string
	Test   bool
	Site   CallSite
}

// FunctionCallKey identifies one calculated call edge.
type FunctionCallKey struct {
	Source string
	Target string
	Test   bool
}

// CollectComponentFile adds source file facts to their component.
func CollectComponentFile(components map[string]*ComponentAccumulator, file SourceFile) {
	collectComponentFile(components, internalSourceFile(file))
}

// CollectRelationships adds each cross-component import to its relationship.
func CollectRelationships(
	components map[string]*ComponentAccumulator,
	relationships map[RelationshipKey]*RelationshipAccumulator,
	file SourceFile,
) {
	internalRelationships := make(map[relationshipKey]*relationshipAccumulator, len(relationships))
	for key, relationship := range relationships {
		internalRelationships[relationshipKey{source: key.Source, target: key.Target}] = relationship
	}
	collectRelationships(components, internalRelationships, internalSourceFile(file))
	for key, relationship := range internalRelationships {
		relationships[RelationshipKey{Source: key.source, Target: key.target}] = relationship
	}
}

// GetOrCreateComponent returns the accumulator for one component.
func GetOrCreateComponent(
	components map[string]*ComponentAccumulator,
	descriptor ComponentDescriptor,
) *ComponentAccumulator {
	return getOrCreateComponent(components, internalComponentDescriptor(descriptor))
}

// BuildGraph calculates one report graph with the default architecture rules.
func BuildGraph(
	modulePath string,
	components map[string]*ComponentAccumulator,
	relationships map[RelationshipKey]*RelationshipAccumulator,
	diagnostics []Diagnostic,
) Graph {
	return buildGraph(modulePath, components, internalRelationshipMap(relationships), diagnostics)
}

// BuildGraphWithRules calculates one report graph with explicit architecture rules.
func BuildGraphWithRules(
	modulePath string,
	components map[string]*ComponentAccumulator,
	relationships map[RelationshipKey]*RelationshipAccumulator,
	diagnostics []Diagnostic,
	rules architecture.Registry,
) Graph {
	return buildGraphWithRules(
		modulePath,
		components,
		internalRelationshipMap(relationships),
		diagnostics,
		rules,
	)
}

// SummarizeGraph calculates all report counters.
func SummarizeGraph(graph Graph) GraphSummary {
	return summarizeGraph(graph)
}

// DetectFindings evaluates the default architecture rules.
func DetectFindings(graph Graph) []Finding {
	return detectFindings(graph)
}

// RelationshipRuleViolations evaluates one classified relationship.
func RelationshipRuleViolations(
	source ComponentDescriptor,
	target ComponentDescriptor,
	testOnly bool,
) []string {
	return relationshipRuleViolations(
		internalComponentDescriptor(source),
		internalComponentDescriptor(target),
		testOnly,
	)
}

// AnnotateStableDependencyPrincipleViolations annotates relationship metrics.
func AnnotateStableDependencyPrincipleViolations(relationships []Relationship, components []Component) {
	annotateStableDependencyPrincipleViolations(relationships, components)
}

// ApplyArchitectureRules evaluates the supplied architecture registry.
func ApplyArchitectureRules(graph *Graph, registry architecture.Registry) {
	applyArchitectureRules(graph, registry)
}

// NewStringSet creates a set with the supplied values.
func NewStringSet(values ...string) StringSet {
	return newStringSet(values...)
}

// NewAdjacency creates an empty adjacency entry for each identifier.
func NewAdjacency(identifiers []string) map[string]StringSet {
	return newAdjacency(identifiers)
}

// Reachable returns all nodes reachable from the supplied node.
func Reachable(start string, adjacency map[string]StringSet) StringSet {
	return reachable(start, adjacency)
}

// StronglyConnectedComponents returns deterministic production cycles.
func StronglyConnectedComponents(
	identifiers []string,
	adjacency map[string]StringSet,
) [][]string {
	return stronglyConnectedComponents(identifiers, adjacency)
}

// SortedSet returns the set values in lexical order.
func SortedSet(set StringSet) []string {
	return sortedSet(set)
}

// CalculateFunctionGraph calculates functions, calls, and cycles.
func CalculateFunctionGraph(
	declarations []FunctionDeclaration,
	references []FunctionReference,
) ([]Function, []FunctionCall, [][]string) {
	internalDeclarations := make([]functionDeclaration, len(declarations))
	for index, declaration := range declarations {
		internalDeclarations[index] = internalFunctionDeclaration(declaration)
	}
	internalReferences := make([]functionReference, len(references))
	for index, reference := range references {
		internalReferences[index] = functionReference{
			source: reference.Source,
			target: reference.Target,
			test:   reference.Test,
			site:   reference.Site,
		}
	}
	return calculateFunctionGraph(internalDeclarations, internalReferences)
}

// MergeFunctionDeclarations selects the most useful declaration.
func MergeFunctionDeclarations(first, second FunctionDeclaration) FunctionDeclaration {
	return externalFunctionDeclaration(mergeFunctionDeclarations(
		internalFunctionDeclaration(first),
		internalFunctionDeclaration(second),
	))
}

// CompareFunctionCallKeys orders call keys deterministically.
func CompareFunctionCallKeys(first, second FunctionCallKey) int {
	return compareFunctionCallKeys(
		functionCallKey{source: first.Source, target: first.Target, test: first.Test},
		functionCallKey{source: second.Source, target: second.Target, test: second.Test},
	)
}

// AttachFunctionMetrics adds calculated function data to components and relationships.
func AttachFunctionMetrics(graph *Graph) {
	attachFunctionMetrics(graph)
}

func internalComponentDescriptor(descriptor ComponentDescriptor) componentDescriptor {
	return componentDescriptor{
		identifier:  descriptor.Identifier,
		name:        descriptor.Name,
		role:        descriptor.Role,
		category:    descriptor.Category,
		application: descriptor.Application,
	}
}

func internalSourceFile(file SourceFile) sourceFile {
	imports := make([]sourceImport, len(file.Imports))
	for index, imported := range file.Imports {
		imports[index] = sourceImport{
			packagePath: imported.PackagePath,
			component:   internalComponentDescriptor(imported.Component),
			site:        imported.Site,
		}
	}
	return sourceFile{
		relativePath:  file.RelativePath,
		packagePath:   file.PackagePath,
		component:     internalComponentDescriptor(file.Component),
		test:          file.Test,
		imports:       imports,
		abstractTypes: file.AbstractTypes,
		concreteTypes: file.ConcreteTypes,
	}
}

func internalRelationshipMap(
	relationships map[RelationshipKey]*RelationshipAccumulator,
) map[relationshipKey]*relationshipAccumulator {
	result := make(map[relationshipKey]*relationshipAccumulator, len(relationships))
	for key, relationship := range relationships {
		result[relationshipKey{source: key.Source, target: key.Target}] = relationship
	}
	return result
}

func internalFunctionDeclaration(declaration FunctionDeclaration) functionDeclaration {
	return functionDeclaration{
		identifier:      declaration.Identifier,
		name:            declaration.Name,
		packagePath:     declaration.PackagePath,
		component:       declaration.Component,
		relativePath:    declaration.RelativePath,
		line:            declaration.Line,
		receiver:        declaration.Receiver,
		method:          declaration.Method,
		exported:        declaration.Exported,
		test:            declaration.Test,
		synthetic:       declaration.Synthetic,
		inAnalysisScope: declaration.InAnalysisScope,
		sourcePosition:  declaration.SourcePosition,
	}
}

func externalFunctionDeclaration(declaration functionDeclaration) FunctionDeclaration {
	return FunctionDeclaration{
		Identifier:      declaration.identifier,
		Name:            declaration.name,
		PackagePath:     declaration.packagePath,
		Component:       declaration.component,
		RelativePath:    declaration.relativePath,
		Line:            declaration.line,
		Receiver:        declaration.receiver,
		Method:          declaration.method,
		Exported:        declaration.exported,
		Test:            declaration.test,
		Synthetic:       declaration.synthetic,
		InAnalysisScope: declaration.inAnalysisScope,
		SourcePosition:  declaration.sourcePosition,
	}
}
