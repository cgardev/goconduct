// Package query selects deterministic views from one dependency graph report.
package query

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/report"
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

// ComponentTypesResult contains the declared types of one component and the
// relations that reach them from other components.
type ComponentTypesResult struct {
	Analysis  AnalysisHeader           `json:"analysis"`
	Component string                   `json:"component"`
	Types     []report.TypeDeclaration `json:"types"`
	Incoming  []IncomingTypeRelation   `json:"incoming"`
}

// IncomingTypeRelation records one relation from a type another component
// declares toward one type of the selected component. Go satisfies interfaces
// implicitly, so the declaring component cannot list its implementers itself;
// this inverse view supplies them.
type IncomingTypeRelation struct {
	Kind            string `json:"kind"`
	SourceID        string `json:"sourceId"`
	SourceComponent string `json:"sourceComponent"`
	TargetID        string `json:"targetId"`
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
		return "", failure.New(
			failure.ErrValidation,
			fmt.Sprintf("finding severity %q must be all, warning, or error", value),
			nil,
		)
	}
}

// Findings returns findings that match the supplied parameters.
func Findings(graph report.Graph, query FindingsParams) FindingsResult {
	selection := Select(graph.Findings, func(finding report.Finding) bool {
		if query.Severity != FindingSeverityAll && string(finding.Severity) != string(query.Severity) {
			return false
		}
		if query.Rule != "" && finding.Rule != query.Rule {
			return false
		}
		if query.Component != "" && !findingMatchesComponent(finding, query.Component) {
			return false
		}
		return true
	}, nil, query.Limit)
	return FindingsResult{
		Analysis: analysisHeader(graph),
		Matched:  selection.Matched,
		Returned: len(selection.Values),
		Findings: selection.Values,
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
	if value == "all" || report.ValidComponentRole(report.ComponentRole(value)) {
		return value, nil
	}
	return "", failure.New(
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
	return "", failure.New(
		failure.ErrValidation,
		fmt.Sprintf("component sort %q must be %s", value, describeComponentSorts()),
		nil,
	)
}

// Components returns component summaries that match the supplied parameters.
func Components(graph report.Graph, query ComponentsParams) ComponentsResult {
	selection := Select(graph.Components, func(component report.Component) bool {
		if query.Role != "all" && string(component.Role) != query.Role {
			return false
		}
		if query.Category != "" && component.Category != query.Category {
			return false
		}
		return true
	}, componentComparison(query.Sort), query.Limit)
	overviews := make([]ComponentOverview, 0, len(selection.Values))
	for _, component := range selection.Values {
		overviews = append(overviews, newComponentOverview(component))
	}
	return ComponentsResult{
		Analysis:   analysisHeader(graph),
		Matched:    selection.Matched,
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
		descriptor = componentSortRegistry[0]
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
		return ComponentResult{}, failure.NotFound(
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

// ComponentTypes returns the declared types of one component.
// The types keep the deterministic identifier order of the graph.
func ComponentTypes(graph report.Graph, identifier string) (ComponentTypesResult, error) {
	if identifier == "" {
		return ComponentTypesResult{}, failure.New(
			failure.ErrValidation,
			"component identifier must not be empty",
			nil,
		)
	}
	found := slices.ContainsFunc(graph.Components, func(component report.Component) bool {
		return component.Identifier == identifier
	})
	if !found {
		return ComponentTypesResult{}, failure.NotFound(
			"dependency graph component",
			identifier,
			nil,
		)
	}
	declarations := make([]report.TypeDeclaration, 0)
	for _, declaration := range graph.Types {
		if declaration.Component == identifier {
			declarations = append(declarations, declaration)
		}
	}
	return ComponentTypesResult{
		Analysis:  analysisHeader(graph),
		Component: identifier,
		Types:     declarations,
		Incoming:  incomingTypeRelations(graph, identifier),
	}, nil
}

// incomingTypeRelations collects every relation whose target the selected
// component declares and whose source another component declares. The result
// is sorted by target, kind, and source, so two runs over one graph agree.
func incomingTypeRelations(graph report.Graph, identifier string) []IncomingTypeRelation {
	incoming := make([]IncomingTypeRelation, 0)
	for _, declaration := range graph.Types {
		if declaration.Component == identifier {
			continue
		}
		for kind, references := range map[string][]report.TypeReference{
			"implements": declaration.Implements,
			"embeds":     declaration.Embeds,
			"references": declaration.References,
		} {
			for _, reference := range references {
				if reference.Component != identifier {
					continue
				}
				incoming = append(incoming, IncomingTypeRelation{
					Kind:            kind,
					SourceID:        declaration.Identifier,
					SourceComponent: declaration.Component,
					TargetID:        reference.Identifier,
				})
			}
		}
	}
	slices.SortFunc(incoming, func(first, second IncomingTypeRelation) int {
		return cmp.Or(
			strings.Compare(first.TargetID, second.TargetID),
			strings.Compare(first.Kind, second.Kind),
			strings.Compare(first.SourceID, second.SourceID),
		)
	})
	return incoming
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-23T21:34:41Z","module_hash":"4f99b1e8146b46de2a7c83b75aba340dcbe48b000507b2313fb7c35ef13cdf45","functions":[{"id":"func/analysisHeader","name":"analysisHeader","line":188,"end_line":194,"hash":"2bc962a44bad128b22ff7f0b775f8787264f1e74bb4e7b6b2d39a63df12f6970"},{"id":"func/Summary","name":"Summary","line":197,"end_line":204,"hash":"f0f783ba977624a9143c8c3b95c7c398a38074db75813a2b90c3ce66475ea526"},{"id":"func/ParseFindingSeverity","name":"ParseFindingSeverity","line":207,"end_line":219,"hash":"a6767835522fb4b742820397869cdf4e28e3b37bcf42c1953c42707897cd10d7"},{"id":"func/Findings","name":"Findings","line":222,"end_line":241,"hash":"7f08ed5a766d3b3b1d5bb51ec2867085ce8be64efc5300fe1528796b271c9ed5"},{"id":"func/findingMatchesComponent","name":"findingMatchesComponent","line":243,"end_line":248,"hash":"7f9eb7ad2e917babae1aa50acf24bb0e5b6f15db10f19be9d6b9119b3460de2a"},{"id":"func/ParseComponentRole","name":"ParseComponentRole","line":251,"end_line":264,"hash":"9efb923ae56333394e9f37ca5e94da0ce523860429e8439859907d69fb73e167"},{"id":"func/ParseComponentSort","name":"ParseComponentSort","line":267,"end_line":277,"hash":"93834511e15b78e275ebcf0e7ce433ebd420c7ed1b0f587fc70ce7e4aa629664"},{"id":"func/Components","name":"Components","line":280,"end_line":300,"hash":"d794ab332bd01b6455e88c09b503e80779cf5474d545927a2526df1efecf0d80"},{"id":"func/newComponentOverview","name":"newComponentOverview","line":302,"end_line":321,"hash":"080825db79472832cf10ce479e75849fd5e2c7b2a4eabcbc1a9e146124e50d72"},{"id":"func/componentComparison","name":"componentComparison","line":323,"end_line":332,"hash":"040e9dd18a93affaf2c3de869946027dee4375ee176d314d2e48ea7b2e3b6884"},{"id":"func/componentSortDescriptorFor","name":"componentSortDescriptorFor","line":334,"end_line":341,"hash":"7993cfd4d946f96e840e79725b945aebb975ef988d5f66ff2183bc6510606f87"},{"id":"func/describeComponentSorts","name":"describeComponentSorts","line":343,"end_line":349,"hash":"c1a50dbf9f19f0da55e43666cd41adf3c0f19ff918f33b902a84f000f848fbb8"},{"id":"func/GetComponent","name":"GetComponent","line":352,"end_line":406,"hash":"b91eebff576cffe4a74889d38924ec5e02962ea93cb4a1374f5c54ebf22621da"}]}
// mutate4go-manifest-end
