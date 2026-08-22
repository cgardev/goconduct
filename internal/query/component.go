// Package query selects deterministic views from one dependency graph report.
package query

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/cgardev/goconduct/internal/architecture"
	"github.com/cgardev/goconduct/internal/failure"
	"github.com/cgardev/goconduct/internal/report"
)

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
		return "", failure.NewError(
			failure.ErrValidation,
			fmt.Sprintf("finding severity %q must be all, warning, or error", value),
			nil,
		)
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
	return "", failure.NewError(
		failure.ErrValidation,
		fmt.Sprintf(
			"component role %q must be all, application, application-module, shared-module, "+
				"library, infrastructure, or development",
			value,
		),
		nil,
	)
}

// ParseComponentSort validates one component sort value.
func ParseComponentSort(value string) (ComponentSort, error) {
	sortOrder := ComponentSort(value)
	if _, found := componentSortDescriptorFor(sortOrder); found {
		return sortOrder, nil
	}
	return "", failure.NewError(
		failure.ErrValidation,
		fmt.Sprintf("component sort %q must be %s", value, describeComponentSorts()),
		nil,
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
		return ComponentResult{}, failure.NewEntityNotFoundError(
			"dependency graph component",
			identifier,
			nil,
		)
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
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"1de9534d30b1e882b9669a0e18d25a538d4db6f160eccbcb4dc6b77f65113981","functions":[{"id":"func/analysisHeader","name":"analysisHeader","line":189,"end_line":195,"hash":"2bc962a44bad128b22ff7f0b775f8787264f1e74bb4e7b6b2d39a63df12f6970"},{"id":"func/Summary","name":"Summary","line":198,"end_line":205,"hash":"f0f783ba977624a9143c8c3b95c7c398a38074db75813a2b90c3ce66475ea526"},{"id":"func/ParseFindingSeverity","name":"ParseFindingSeverity","line":208,"end_line":220,"hash":"fd56dc87de374d44ebf2d673395b2593882860195b7e7a200bb2b52f8345bd4d"},{"id":"func/Findings","name":"Findings","line":223,"end_line":245,"hash":"91e5c36d66c095e0b221a231630db37df6315211ec7af989fa4e142ba7061e3c"},{"id":"func/findingMatchesComponent","name":"findingMatchesComponent","line":247,"end_line":252,"hash":"7f9eb7ad2e917babae1aa50acf24bb0e5b6f15db10f19be9d6b9119b3460de2a"},{"id":"func/ParseComponentRole","name":"ParseComponentRole","line":255,"end_line":268,"hash":"075e38d0fecc8905191b7198c6c3161ce27bb984a5b588c40bd7e3b21996f068"},{"id":"func/ParseComponentSort","name":"ParseComponentSort","line":271,"end_line":281,"hash":"329bbe45e3aea55ea79aee786d1163b837bbbc084955e2d48653400476cdadaf"},{"id":"func/Components","name":"Components","line":284,"end_line":308,"hash":"a6115500ed62db24af66da909e10fe3606388c23c68d019c9147cdd29567712c"},{"id":"func/newComponentOverview","name":"newComponentOverview","line":310,"end_line":329,"hash":"080825db79472832cf10ce479e75849fd5e2c7b2a4eabcbc1a9e146124e50d72"},{"id":"func/componentComparison","name":"componentComparison","line":331,"end_line":340,"hash":"c21eefb39a9f17b38be5bb93bf3f91beb247084634779bb78dc27d42af7561c5"},{"id":"func/componentSortDescriptorFor","name":"componentSortDescriptorFor","line":342,"end_line":349,"hash":"7993cfd4d946f96e840e79725b945aebb975ef988d5f66ff2183bc6510606f87"},{"id":"func/describeComponentSorts","name":"describeComponentSorts","line":351,"end_line":357,"hash":"c1a50dbf9f19f0da55e43666cd41adf3c0f19ff918f33b902a84f000f848fbb8"},{"id":"func/GetComponent","name":"GetComponent","line":360,"end_line":414,"hash":"05112c328a6cb1c8fa1c10fcf1e0519a99563a50ccc42dfb7de9aa997842f7b4"},{"id":"func/applyLimit","name":"applyLimit","line":416,"end_line":421,"hash":"f69f1b495bd0b7dda6d20a1b9b3ff20c1da38af6ede1a34cfee2369223b89f43"}]}
// mutate4go-manifest-end
