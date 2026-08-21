package main

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var errComponentNotFound = errors.New("architectural component not found")

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
	componentSortDependants   componentSort = "dependants"
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
	Identifier           string        `json:"id"`
	Name                 string        `json:"name"`
	Kind                 componentKind `json:"kind"`
	Application          string        `json:"application,omitempty"`
	SourceFiles          int           `json:"sourceFiles"`
	AfferentCoupling     int           `json:"afferentCoupling"`
	EfferentCoupling     int           `json:"efferentCoupling"`
	ProductionDependants int           `json:"productionDependants"`
	TransitiveDependants int           `json:"transitiveDependants"`
	ApplicationReach     int           `json:"applicationReach"`
	Instability          float64       `json:"instability"`
	Abstractness         float64       `json:"abstractness"`
	MainSequenceDistance float64       `json:"mainSequenceDistance"`
	InZoneOfPain         bool          `json:"inZoneOfPain"`
	InCycle              bool          `json:"inCycle"`
}

type componentQueryResult struct {
	Analysis     analysisQueryHeader `json:"analysis"`
	Component    Component           `json:"component"`
	Dependencies []Relationship      `json:"dependencies"`
	Dependants   []Relationship      `json:"dependants"`
	Findings     []Finding           `json:"findings"`
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
	findings = limited(findings, query.limit)
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
		"component kind %q must be all, application, application-module, shared-module, library, infrastructure, or development",
		value,
	)
}

func parseComponentSort(value string) (componentSort, error) {
	sortOrder := componentSort(value)
	switch sortOrder {
	case componentSortIdentifier,
		componentSortAfferent,
		componentSortEfferent,
		componentSortDependants,
		componentSortDependencies,
		componentSortInstability,
		componentSortAbstractness,
		componentSortDistance,
		componentSortFiles:
		return sortOrder, nil
	default:
		return "", fmt.Errorf(
			"component sort %q must be identifier, afferent, efferent, dependants, dependencies, instability, abstractness, distance, or files",
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
	components = limited(components, query.limit)
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
		Identifier:           component.Identifier,
		Name:                 component.Name,
		Kind:                 component.Kind,
		Application:          component.Application,
		SourceFiles:          component.SourceFiles,
		AfferentCoupling:     component.AfferentCoupling,
		EfferentCoupling:     component.EfferentCoupling,
		ProductionDependants: component.ProductionDependants,
		TransitiveDependants: component.TransitiveDependants,
		ApplicationReach:     component.ApplicationReach,
		Instability:          component.Instability,
		Abstractness:         component.Abstractness,
		MainSequenceDistance: component.MainSequenceDistance,
		InZoneOfPain:         component.InZoneOfPain,
		InCycle:              component.InCycle,
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
		case componentSortDependants:
			result = cmp.Compare(second.TransitiveDependants, first.TransitiveDependants)
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
	dependants := make([]Relationship, 0)
	for _, relationship := range graph.Relationships {
		if relationship.Source == identifier {
			dependencies = append(dependencies, relationship)
		}
		if relationship.Target == identifier {
			dependants = append(dependants, relationship)
		}
	}
	findings := make([]Finding, 0)
	for _, finding := range graph.Findings {
		if findingMatchesComponent(finding, identifier) {
			findings = append(findings, finding)
		}
	}
	return componentQueryResult{
		Analysis:     queryHeader(graph),
		Component:    selected,
		Dependencies: dependencies,
		Dependants:   dependants,
		Findings:     findings,
	}, nil
}

func limited[Value any](values []Value, limit int) []Value {
	if limit <= 0 {
		return values
	}
	return values[:min(limit, len(values))]
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T07:59:06Z","module_hash":"eb52f4c13b8351a68660b8c4f2383eff7cdf3b19e03d4e959258be4e4ffdd24c","functions":[{"id":"func/queryHeader","name":"queryHeader","line":101,"end_line":107,"hash":"7ea3e1c8eb4057694e566bb423af890b436a65768b861b1ef99464d6c05144bb"},{"id":"func/querySummary","name":"querySummary","line":109,"end_line":116,"hash":"a3d7382f8aab34a96422a54da0b11ad460044dbfa4c87ea81fb11bdf97bdc81b"},{"id":"func/parseFindingSeverityFilter","name":"parseFindingSeverityFilter","line":118,"end_line":126,"hash":"d869004d42983846bab2f0a27b0901fc58641e2bd999466ea01cecb9f9e3b2e5"},{"id":"func/queryFindings","name":"queryFindings","line":128,"end_line":150,"hash":"1b11d7319fafb42e968758c07fa9c303f11b27c2a0008129858dce1fa53d18a8"},{"id":"func/findingMatchesComponent","name":"findingMatchesComponent","line":152,"end_line":157,"hash":"3510643f7250d0893bb491957263682e0ee583c6e80ff89ea3482e65b870cd41"},{"id":"func/parseComponentKindFilter","name":"parseComponentKindFilter","line":159,"end_line":167,"hash":"4be1c42af1e0d050ad6451c4f92875e3f61bbc521b0c659cffd0ae39376c4e5b"},{"id":"func/parseComponentSort","name":"parseComponentSort","line":169,"end_line":188,"hash":"378c9c11e17d001141878f55aaecc7f911864aed95ab53ebaa770d8963c489bd"},{"id":"func/queryComponents","name":"queryComponents","line":190,"end_line":210,"hash":"78f76ed1ebb9c34a3dfad03bcefbb275953fadab06c18d9eb9e8bb4ffac49008"},{"id":"func/newComponentOverview","name":"newComponentOverview","line":212,"end_line":230,"hash":"1cbf57650b596e8bdb3b4a9d7bacb513f23b1fd85ae394ed760eeef5ff177d76"},{"id":"func/componentComparison","name":"componentComparison","line":232,"end_line":255,"hash":"c80a7c0219c56bc5699c705da55d54cb3287720bc0ed5fbbc770488887abd918"},{"id":"func/queryComponent","name":"queryComponent","line":257,"end_line":293,"hash":"afe418ab51c81dfb870c3fed1f34743eb9c22991acd4a34c46d150ae3ccbab25"},{"id":"func/limited","name":"limited","line":295,"end_line":300,"hash":"82b5edf5623cf481def248757e3a7516935667fa822617f2021bb544ab2abe8e"}]}
// mutate4go-manifest-end
