package main

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var errComponentNotFound = errors.New("component not found")

type findingSeverityFilter string

const (
	findingSeverityAllFilter     findingSeverityFilter = "all"
	findingSeverityErrorFilter   findingSeverityFilter = "error"
	findingSeverityWarningFilter findingSeverityFilter = "warning"
)

type componentSort string

const (
	componentSortIdentifier   componentSort = "identifier"
	componentSortAfferent     componentSort = "afferent"
	componentSortEfferent     componentSort = "efferent"
	componentSortImporters    componentSort = "importers"
	componentSortDependencies componentSort = "dependencies"
	componentSortInstability  componentSort = "instability"
	componentSortAbstractness componentSort = "abstractness"
	componentSortDistance     componentSort = "distance"
	componentSortFiles        componentSort = "files"
)

type analysisQueryHeader struct {
	SchemaVersion int    `json:"schemaVersion"`
	Revision      string `json:"revision"`
	ModulePath    string `json:"modulePath"`
}

type summaryQueryResult struct {
	Analysis analysisQueryHeader `json:"analysis"`
	Scope    AnalysisScope       `json:"scope"`
	Policy   AnalysisPolicy      `json:"policy"`
	Summary  GraphSummary        `json:"summary"`
}

type findingsQuery struct {
	severity  findingSeverityFilter
	rule      string
	component string
	limit     int
}

type findingsQueryResult struct {
	Analysis analysisQueryHeader `json:"analysis"`
	Matched  int                 `json:"matched"`
	Returned int                 `json:"returned"`
	Findings []Finding           `json:"findings"`
}

type componentsQuery struct {
	kind  string
	sort  componentSort
	limit int
}

type componentsQueryResult struct {
	Analysis   analysisQueryHeader `json:"analysis"`
	Matched    int                 `json:"matched"`
	Returned   int                 `json:"returned"`
	Components []componentOverview `json:"components"`
}

type componentOverview struct {
	Identifier                    string        `json:"id"`
	Name                          string        `json:"name"`
	Kind                          componentKind `json:"kind"`
	Application                   string        `json:"application,omitempty"`
	SourceFiles                   int           `json:"sourceFiles"`
	AfferentCoupling              int           `json:"afferentCoupling"`
	EfferentCoupling              int           `json:"efferentCoupling"`
	ProductionImportingComponents int           `json:"productionImportingComponents"`
	TransitiveImportingComponents int           `json:"transitiveImportingComponents"`
	UsingApplicationCount         int           `json:"usingApplicationCount"`
	Instability                   float64       `json:"instability"`
	Abstractness                  float64       `json:"abstractness"`
	MainSequenceDistance          float64       `json:"mainSequenceDistance"`
	IsStableWithLowAbstraction    bool          `json:"isStableWithLowAbstraction"`
	InCycle                       bool          `json:"inCycle"`
}

type componentQueryResult struct {
	Analysis               analysisQueryHeader `json:"analysis"`
	Component              Component           `json:"component"`
	Dependencies           []Relationship      `json:"dependencies"`
	ImportingRelationships []Relationship      `json:"importingRelationships"`
	Functions              []Function          `json:"functions"`
	FunctionCalls          []FunctionCall      `json:"functionCalls"`
	Findings               []Finding           `json:"findings"`
}

func queryHeader(graph Graph) analysisQueryHeader {
	return analysisQueryHeader{
		SchemaVersion: graph.SchemaVersion,
		Revision:      graph.Revision,
		ModulePath:    graph.ModulePath,
	}
}

func querySummary(graph Graph) summaryQueryResult {
	return summaryQueryResult{
		Analysis: queryHeader(graph),
		Scope:    graph.Scope,
		Policy:   graph.Policy,
		Summary:  graph.Summary,
	}
}

func parseFindingSeverityFilter(value string) (findingSeverityFilter, error) {
	filter := findingSeverityFilter(value)
	switch filter {
	case findingSeverityAllFilter, findingSeverityErrorFilter, findingSeverityWarningFilter:
		return filter, nil
	default:
		return "", fmt.Errorf("finding severity %q must be all, warning, or error", value)
	}
}

func queryFindings(graph Graph, query findingsQuery) findingsQueryResult {
	findings := make([]Finding, 0)
	for _, finding := range graph.Findings {
		if query.severity != findingSeverityAllFilter && string(finding.Severity) != string(query.severity) {
			continue
		}
		if query.rule != "" && finding.Rule != query.rule {
			continue
		}
		if query.component != "" && !findingMatchesComponent(finding, query.component) {
			continue
		}
		findings = append(findings, finding)
	}
	matched := len(findings)
	findings = applyLimit(findings, query.limit)
	return findingsQueryResult{
		Analysis: queryHeader(graph),
		Matched:  matched,
		Returned: len(findings),
		Findings: findings,
	}
}

func findingMatchesComponent(finding Finding, identifier string) bool {
	return finding.Subject == identifier ||
		finding.Source == identifier ||
		finding.Target == identifier ||
		slices.Contains(finding.Components, identifier)
}

func parseComponentKindFilter(value string) (string, error) {
	if value == "all" || validComponentKind(componentKind(value)) {
		return value, nil
	}
	return "", fmt.Errorf(
		"component kind %q must be all, application, application-module, shared-module, "+
			"library, infrastructure, or development",
		value,
	)
}

func parseComponentSort(value string) (componentSort, error) {
	sortOrder := componentSort(value)
	switch sortOrder {
	case componentSortIdentifier,
		componentSortAfferent,
		componentSortEfferent,
		componentSortImporters,
		componentSortDependencies,
		componentSortInstability,
		componentSortAbstractness,
		componentSortDistance,
		componentSortFiles:
		return sortOrder, nil
	default:
		return "", fmt.Errorf(
			"component sort %q must be identifier, afferent, efferent, importers, dependencies, "+
				"instability, abstractness, distance, or files",
			value,
		)
	}
}

func queryComponents(graph Graph, query componentsQuery) componentsQueryResult {
	components := make([]Component, 0)
	for _, component := range graph.Components {
		if query.kind == "all" || string(component.Kind) == query.kind {
			components = append(components, component)
		}
	}
	slices.SortFunc(components, componentComparison(query.sort))
	matched := len(components)
	components = applyLimit(components, query.limit)
	overviews := make([]componentOverview, 0, len(components))
	for _, component := range components {
		overviews = append(overviews, newComponentOverview(component))
	}
	return componentsQueryResult{
		Analysis:   queryHeader(graph),
		Matched:    matched,
		Returned:   len(overviews),
		Components: overviews,
	}
}

func newComponentOverview(component Component) componentOverview {
	return componentOverview{
		Identifier:                    component.Identifier,
		Name:                          component.Name,
		Kind:                          component.Kind,
		Application:                   component.Application,
		SourceFiles:                   component.SourceFiles,
		AfferentCoupling:              component.AfferentCoupling,
		EfferentCoupling:              component.EfferentCoupling,
		ProductionImportingComponents: component.ProductionImportingComponents,
		TransitiveImportingComponents: component.TransitiveImportingComponents,
		UsingApplicationCount:         component.UsingApplicationCount,
		Instability:                   component.Instability,
		Abstractness:                  component.Abstractness,
		MainSequenceDistance:          component.MainSequenceDistance,
		IsStableWithLowAbstraction:    component.IsStableWithLowAbstraction,
		InCycle:                       component.InCycle,
	}
}

func componentComparison(sortOrder componentSort) func(Component, Component) int {
	return func(first, second Component) int {
		var result int
		switch sortOrder {
		case componentSortAfferent:
			result = cmp.Compare(second.AfferentCoupling, first.AfferentCoupling)
		case componentSortEfferent:
			result = cmp.Compare(second.EfferentCoupling, first.EfferentCoupling)
		case componentSortImporters:
			result = cmp.Compare(second.TransitiveImportingComponents, first.TransitiveImportingComponents)
		case componentSortDependencies:
			result = cmp.Compare(second.TransitiveDependencies, first.TransitiveDependencies)
		case componentSortInstability:
			result = cmp.Compare(second.Instability, first.Instability)
		case componentSortAbstractness:
			result = cmp.Compare(second.Abstractness, first.Abstractness)
		case componentSortDistance:
			result = cmp.Compare(second.MainSequenceDistance, first.MainSequenceDistance)
		case componentSortFiles:
			result = cmp.Compare(second.SourceFiles, first.SourceFiles)
		}
		return cmp.Or(result, strings.Compare(first.Identifier, second.Identifier))
	}
}

func queryComponent(graph Graph, identifier string) (componentQueryResult, error) {
	var selected Component
	found := false
	for _, component := range graph.Components {
		if component.Identifier == identifier {
			selected = component
			found = true
			break
		}
	}
	if !found {
		return componentQueryResult{}, fmt.Errorf("%w: %s", errComponentNotFound, identifier)
	}
	dependencies := make([]Relationship, 0)
	importingRelationships := make([]Relationship, 0)
	for _, relationship := range graph.Relationships {
		if relationship.Source == identifier {
			dependencies = append(dependencies, relationship)
		}
		if relationship.Target == identifier {
			importingRelationships = append(importingRelationships, relationship)
		}
	}
	findings := make([]Finding, 0)
	for _, finding := range graph.Findings {
		if findingMatchesComponent(finding, identifier) {
			findings = append(findings, finding)
		}
	}
	functions := make([]Function, 0)
	for _, function := range graph.Functions {
		if function.Component == identifier {
			functions = append(functions, function)
		}
	}
	functionCalls := make([]FunctionCall, 0)
	for _, call := range graph.FunctionCalls {
		if call.SourceComponent == identifier || call.TargetComponent == identifier {
			functionCalls = append(functionCalls, call)
		}
	}
	return componentQueryResult{
		Analysis:               queryHeader(graph),
		Component:              selected,
		Dependencies:           dependencies,
		ImportingRelationships: importingRelationships,
		Functions:              functions,
		FunctionCalls:          functionCalls,
		Findings:               findings,
	}, nil
}

func applyLimit[Value any](values []Value, limit int) []Value {
	if limit <= 0 {
		return values
	}
	return values[:min(limit, len(values))]
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T10:49:11Z","module_hash":"842e4e24fbf853693b67cc2ef3c8ecfc66311ff1d743ec8c36e5f0777fbcc8c9","functions":[{"id":"func/queryHeader","name":"queryHeader","line":103,"end_line":109,"hash":"7ea3e1c8eb4057694e566bb423af890b436a65768b861b1ef99464d6c05144bb"},{"id":"func/querySummary","name":"querySummary","line":111,"end_line":118,"hash":"a3d7382f8aab34a96422a54da0b11ad460044dbfa4c87ea81fb11bdf97bdc81b"},{"id":"func/parseFindingSeverityFilter","name":"parseFindingSeverityFilter","line":120,"end_line":128,"hash":"d869004d42983846bab2f0a27b0901fc58641e2bd999466ea01cecb9f9e3b2e5"},{"id":"func/queryFindings","name":"queryFindings","line":130,"end_line":152,"hash":"ff6450d6b51faa15997c651b051de66cbfe681beb12a592366b2cc12c362a78d"},{"id":"func/findingMatchesComponent","name":"findingMatchesComponent","line":154,"end_line":159,"hash":"3510643f7250d0893bb491957263682e0ee583c6e80ff89ea3482e65b870cd41"},{"id":"func/parseComponentKindFilter","name":"parseComponentKindFilter","line":161,"end_line":170,"hash":"b8dd4ab9d356ec07605a73d319d14c496d9d6d73b98e7510fa91ac61fbd4c034"},{"id":"func/parseComponentSort","name":"parseComponentSort","line":172,"end_line":192,"hash":"e62614b755e6e2f634906bde36597a24b3bc80a1c97f7d1b137c45e3c679dedb"},{"id":"func/queryComponents","name":"queryComponents","line":194,"end_line":214,"hash":"72f6ddb169e4eb55bca8be18083b5fa722983d8088c7e6d94c6455c34b11353a"},{"id":"func/newComponentOverview","name":"newComponentOverview","line":216,"end_line":234,"hash":"a126a90c4c46bc4e847e8d85ed9bdcd6561d0a83a4ba8dad1e6bfae93431dc50"},{"id":"func/componentComparison","name":"componentComparison","line":236,"end_line":259,"hash":"5eb22fec8d8d34322da09a39f79c20aa691540344c046959a1b5e0690dc8ef1f"},{"id":"func/queryComponent","name":"queryComponent","line":261,"end_line":311,"hash":"2b0a0f2e940b05470bb9f03bf14001fd8e10744f612544f67eff0d0a77199592"},{"id":"func/applyLimit","name":"applyLimit","line":313,"end_line":318,"hash":"f69f1b495bd0b7dda6d20a1b9b3ff20c1da38af6ede1a34cfee2369223b89f43"}]}
// mutate4go-manifest-end
