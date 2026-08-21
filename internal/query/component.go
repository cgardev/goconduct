// Package query selects deterministic views from one dependency graph report.
package query

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/architecture"
	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/report"
)

// ErrComponentNotFound identifies a component that is absent from the graph.
var ErrComponentNotFound = errors.New("component not found")

// FindingSeverity selects findings by severity.
type FindingSeverity string

const (
	// FindingSeverityAll selects findings of each severity.
	FindingSeverityAll FindingSeverity = "all"
	// FindingSeverityError selects error findings.
	FindingSeverityError FindingSeverity = "error"
	// FindingSeverityWarning selects warning findings.
	FindingSeverityWarning FindingSeverity = "warning"
)

// ComponentSort selects the metric that orders components.
type ComponentSort string

const (
	// ComponentSortIdentifier orders components by identifier.
	ComponentSortIdentifier ComponentSort = "identifier"
	// ComponentSortAfferent orders components by afferent coupling.
	ComponentSortAfferent ComponentSort = "afferent"
	// ComponentSortEfferent orders components by efferent coupling.
	ComponentSortEfferent ComponentSort = "efferent"
	// ComponentSortImporters orders components by transitive importers.
	ComponentSortImporters ComponentSort = "importers"
	// ComponentSortDependencies orders components by transitive dependencies.
	ComponentSortDependencies ComponentSort = "dependencies"
	// ComponentSortInstability orders components by instability.
	ComponentSortInstability ComponentSort = "instability"
	// ComponentSortAbstractness orders components by abstractness.
	ComponentSortAbstractness ComponentSort = "abstractness"
	// ComponentSortDistance orders components by main sequence distance.
	ComponentSortDistance ComponentSort = "distance"
	// ComponentSortFiles orders components by source file count.
	ComponentSortFiles ComponentSort = "files"
)

type componentSortDescriptor struct {
	name    ComponentSort
	compare func(report.Component, report.Component) int
}

var componentSortRegistry = []componentSortDescriptor{
	{name: ComponentSortIdentifier, compare: func(report.Component, report.Component) int { return 0 }},
	{
		name: ComponentSortAfferent,
		compare: func(first, second report.Component) int {
			return cmp.Compare(second.AfferentCoupling, first.AfferentCoupling)
		},
	},
	{
		name: ComponentSortEfferent,
		compare: func(first, second report.Component) int {
			return cmp.Compare(second.EfferentCoupling, first.EfferentCoupling)
		},
	},
	{
		name: ComponentSortImporters,
		compare: func(first, second report.Component) int {
			return cmp.Compare(
				second.TransitiveImportingComponents,
				first.TransitiveImportingComponents,
			)
		},
	},
	{
		name: ComponentSortDependencies,
		compare: func(first, second report.Component) int {
			return cmp.Compare(second.TransitiveDependencies, first.TransitiveDependencies)
		},
	},
	{
		name: ComponentSortInstability,
		compare: func(first, second report.Component) int {
			return cmp.Compare(second.Instability, first.Instability)
		},
	},
	{
		name: ComponentSortAbstractness,
		compare: func(first, second report.Component) int {
			return cmp.Compare(second.Abstractness, first.Abstractness)
		},
	},
	{
		name: ComponentSortDistance,
		compare: func(first, second report.Component) int {
			return cmp.Compare(second.MainSequenceDistance, first.MainSequenceDistance)
		},
	},
	{
		name: ComponentSortFiles,
		compare: func(first, second report.Component) int {
			return cmp.Compare(second.SourceFiles, first.SourceFiles)
		},
	},
}

// AnalysisHeader identifies the analyzed graph.
type AnalysisHeader struct {
	SchemaVersion int    `json:"schemaVersion"`
	Revision      string `json:"revision"`
	ModulePath    string `json:"modulePath"`
}

// SummaryResult contains the analysis scope, policy, and counts.
type SummaryResult struct {
	Analysis AnalysisHeader        `json:"analysis"`
	Scope    report.AnalysisScope  `json:"scope"`
	Policy   report.AnalysisPolicy `json:"policy"`
	Summary  report.GraphSummary   `json:"summary"`
}

// FindingsParams defines finding filters and the result limit.
type FindingsParams struct {
	Severity  FindingSeverity
	Rule      string
	Component string
	Limit     int
}

// FindingsResult contains findings that match one query.
type FindingsResult struct {
	Analysis AnalysisHeader   `json:"analysis"`
	Matched  int              `json:"matched"`
	Returned int              `json:"returned"`
	Findings []report.Finding `json:"findings"`
}

// ComponentsParams defines component filters, ordering, and the result limit.
type ComponentsParams struct {
	Role     string
	Category string
	Sort     ComponentSort
	Limit    int
}

// ComponentsResult contains component summaries that match one query.
type ComponentsResult struct {
	Analysis   AnalysisHeader      `json:"analysis"`
	Matched    int                 `json:"matched"`
	Returned   int                 `json:"returned"`
	Components []ComponentOverview `json:"components"`
}

// ComponentOverview contains the strategic metrics for one component.
type ComponentOverview struct {
	Identifier                    string               `json:"id"`
	Name                          string               `json:"name"`
	Role                          report.ComponentRole `json:"role"`
	Category                      string               `json:"category,omitempty"`
	Application                   string               `json:"application,omitempty"`
	SourceFiles                   int                  `json:"sourceFiles"`
	AfferentCoupling              int                  `json:"afferentCoupling"`
	EfferentCoupling              int                  `json:"efferentCoupling"`
	ProductionImportingComponents int                  `json:"productionImportingComponents"`
	TransitiveImportingComponents int                  `json:"transitiveImportingComponents"`
	UsingApplicationCount         int                  `json:"usingApplicationCount"`
	Instability                   float64              `json:"instability"`
	Abstractness                  float64              `json:"abstractness"`
	MainSequenceDistance          float64              `json:"mainSequenceDistance"`
	IsStableWithLowAbstraction    bool                 `json:"isStableWithLowAbstraction"`
	InCycle                       bool                 `json:"inCycle"`
}

// ComponentResult contains one component and its related graph resources.
type ComponentResult struct {
	Analysis               AnalysisHeader        `json:"analysis"`
	Component              report.Component      `json:"component"`
	Dependencies           []report.Relationship `json:"dependencies"`
	ImportingRelationships []report.Relationship `json:"importingRelationships"`
	Functions              []report.Function     `json:"functions"`
	FunctionCalls          []report.FunctionCall `json:"functionCalls"`
	Findings               []report.Finding      `json:"findings"`
}

func analysisHeader(graph report.Graph) AnalysisHeader {
	return AnalysisHeader{
		SchemaVersion: graph.SchemaVersion,
		Revision:      graph.Revision,
		ModulePath:    graph.ModulePath,
	}
}

// Summary returns the scope, policy, and counts for one graph.
func Summary(graph report.Graph) SummaryResult {
	return SummaryResult{
		Analysis: analysisHeader(graph),
		Scope:    graph.Scope,
		Policy:   graph.Policy,
		Summary:  graph.Summary,
	}
}

// ParseFindingSeverity validates one finding severity filter.
func ParseFindingSeverity(value string) (FindingSeverity, error) {
	filter := FindingSeverity(value)
	switch filter {
	case FindingSeverityAll, FindingSeverityError, FindingSeverityWarning:
		return filter, nil
	default:
		return "", fmt.Errorf("finding severity %q must be all, warning, or error", value)
	}
}

// Findings returns findings that match the supplied parameters.
func Findings(graph report.Graph, query FindingsParams) FindingsResult {
	findings := make([]report.Finding, 0)
	for _, finding := range graph.Findings {
		if query.Severity != FindingSeverityAll && string(finding.Severity) != string(query.Severity) {
			continue
		}
		if query.Rule != "" && finding.Rule != query.Rule {
			continue
		}
		if query.Component != "" && !findingMatchesComponent(finding, query.Component) {
			continue
		}
		findings = append(findings, finding)
	}
	matched := len(findings)
	findings = applyLimit(findings, query.Limit)
	return FindingsResult{
		Analysis: analysisHeader(graph),
		Matched:  matched,
		Returned: len(findings),
		Findings: findings,
	}
}

func findingMatchesComponent(finding report.Finding, identifier string) bool {
	return finding.Subject == identifier ||
		finding.Source == identifier ||
		finding.Target == identifier ||
		slices.Contains(finding.Components, identifier)
}

// ParseComponentRole validates one component role filter.
func ParseComponentRole(value string) (string, error) {
	if value == "all" || architecture.ValidRole(report.ComponentRole(value)) {
		return value, nil
	}
	return "", fmt.Errorf(
		"component role %q must be all, application, application-module, shared-module, "+
			"library, infrastructure, or development",
		value,
	)
}

// ParseComponentSort validates one component sort value.
func ParseComponentSort(value string) (ComponentSort, error) {
	sortOrder := ComponentSort(value)
	if _, found := componentSortDescriptorFor(sortOrder); found {
		return sortOrder, nil
	}
	return "", fmt.Errorf(
		"component sort %q must be %s",
		value,
		describeComponentSorts(),
	)
}

// Components returns component summaries that match the supplied parameters.
func Components(graph report.Graph, query ComponentsParams) ComponentsResult {
	components := make([]report.Component, 0)
	for _, component := range graph.Components {
		if query.Role != "all" && string(component.Role) != query.Role {
			continue
		}
		if query.Category != "" && component.Category != query.Category {
			continue
		}
		components = append(components, component)
	}
	slices.SortFunc(components, componentComparison(query.Sort))
	matched := len(components)
	components = applyLimit(components, query.Limit)
	overviews := make([]ComponentOverview, 0, len(components))
	for _, component := range components {
		overviews = append(overviews, newComponentOverview(component))
	}
	return ComponentsResult{
		Analysis:   analysisHeader(graph),
		Matched:    matched,
		Returned:   len(overviews),
		Components: overviews,
	}
}

func newComponentOverview(component report.Component) ComponentOverview {
	return ComponentOverview{
		Identifier:                    component.Identifier,
		Name:                          component.Name,
		Role:                          component.Role,
		Category:                      component.Category,
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

func componentComparison(sortOrder ComponentSort) func(report.Component, report.Component) int {
	descriptor, found := componentSortDescriptorFor(sortOrder)
	if !found {
		descriptor, _ = componentSortDescriptorFor(ComponentSortIdentifier)
	}
	return func(first, second report.Component) int {
		result := descriptor.compare(first, second)
		return cmp.Or(result, strings.Compare(first.Identifier, second.Identifier))
	}
}

func componentSortDescriptorFor(sortOrder ComponentSort) (componentSortDescriptor, bool) {
	for _, descriptor := range componentSortRegistry {
		if descriptor.name == sortOrder {
			return descriptor, true
		}
	}
	return componentSortDescriptor{}, false
}

func describeComponentSorts() string {
	names := make([]string, 0, len(componentSortRegistry))
	for _, descriptor := range componentSortRegistry {
		names = append(names, string(descriptor.name))
	}
	return strings.Join(names[:len(names)-1], ", ") + ", or " + names[len(names)-1]
}

// GetComponent returns one component and its related graph resources.
func GetComponent(graph report.Graph, identifier string) (ComponentResult, error) {
	var selected report.Component
	found := false
	for _, component := range graph.Components {
		if component.Identifier == identifier {
			selected = component
			found = true
			break
		}
	}
	if !found {
		return ComponentResult{}, fmt.Errorf("%w: %s", ErrComponentNotFound, identifier)
	}
	dependencies := make([]report.Relationship, 0)
	importingRelationships := make([]report.Relationship, 0)
	for _, relationship := range graph.Relationships {
		if relationship.Source == identifier {
			dependencies = append(dependencies, relationship)
		}
		if relationship.Target == identifier {
			importingRelationships = append(importingRelationships, relationship)
		}
	}
	findings := make([]report.Finding, 0)
	for _, finding := range graph.Findings {
		if findingMatchesComponent(finding, identifier) {
			findings = append(findings, finding)
		}
	}
	functions := make([]report.Function, 0)
	for _, function := range graph.Functions {
		if function.Component == identifier {
			functions = append(functions, function)
		}
	}
	functionCalls := make([]report.FunctionCall, 0)
	for _, call := range graph.FunctionCalls {
		if call.SourceComponent == identifier || call.TargetComponent == identifier {
			functionCalls = append(functionCalls, call)
		}
	}
	return ComponentResult{
		Analysis:               analysisHeader(graph),
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
// {"version":1,"tested_at":"2026-08-21T16:36:55Z","module_hash":"0a07c46f700e0b38a7c84710d2b20464b024153c6ccd352078478d6c17899330","functions":[{"id":"func/analysisHeader","name":"analysisHeader","line":192,"end_line":198,"hash":"2bc962a44bad128b22ff7f0b775f8787264f1e74bb4e7b6b2d39a63df12f6970"},{"id":"func/Summary","name":"Summary","line":201,"end_line":208,"hash":"f0f783ba977624a9143c8c3b95c7c398a38074db75813a2b90c3ce66475ea526"},{"id":"func/ParseFindingSeverity","name":"ParseFindingSeverity","line":211,"end_line":219,"hash":"73529fee9d5487e3745fe1a76730583b06645c326c1e242e43c2be387d6f3b83"},{"id":"func/Findings","name":"Findings","line":222,"end_line":244,"hash":"91e5c36d66c095e0b221a231630db37df6315211ec7af989fa4e142ba7061e3c"},{"id":"func/findingMatchesComponent","name":"findingMatchesComponent","line":246,"end_line":251,"hash":"7f9eb7ad2e917babae1aa50acf24bb0e5b6f15db10f19be9d6b9119b3460de2a"},{"id":"func/ParseComponentRole","name":"ParseComponentRole","line":254,"end_line":263,"hash":"c636390a1bdc2b18d37f1fb358aab279b70b5910b3360fe7ddd9e2611b517023"},{"id":"func/ParseComponentSort","name":"ParseComponentSort","line":266,"end_line":276,"hash":"95d1818e13eb25d847dc009ff1da55dc3bb246ef8458c980000441ccb59b7905"},{"id":"func/Components","name":"Components","line":279,"end_line":303,"hash":"a6115500ed62db24af66da909e10fe3606388c23c68d019c9147cdd29567712c"},{"id":"func/newComponentOverview","name":"newComponentOverview","line":305,"end_line":324,"hash":"080825db79472832cf10ce479e75849fd5e2c7b2a4eabcbc1a9e146124e50d72"},{"id":"func/componentComparison","name":"componentComparison","line":326,"end_line":335,"hash":"c21eefb39a9f17b38be5bb93bf3f91beb247084634779bb78dc27d42af7561c5"},{"id":"func/componentSortDescriptorFor","name":"componentSortDescriptorFor","line":337,"end_line":344,"hash":"7993cfd4d946f96e840e79725b945aebb975ef988d5f66ff2183bc6510606f87"},{"id":"func/describeComponentSorts","name":"describeComponentSorts","line":346,"end_line":352,"hash":"c1a50dbf9f19f0da55e43666cd41adf3c0f19ff918f33b902a84f000f848fbb8"},{"id":"func/GetComponent","name":"GetComponent","line":355,"end_line":405,"hash":"fa96091a6cb110ea9a68ce94d0cdd7546fa842b072b99656754aee738d5b4338"},{"id":"func/applyLimit","name":"applyLimit","line":407,"end_line":412,"hash":"f69f1b495bd0b7dda6d20a1b9b3ff20c1da38af6ede1a34cfee2369223b89f43"}]}
// mutate4go-manifest-end
