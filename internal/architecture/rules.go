package architecture

import (
	"cmp"
	"slices"
	"strings"
)

const (
	ruleCrossApplicationModuleImport      = "cross-application-module-import"
	ruleDependencyCycle                   = "dependency-cycle"
	ruleLibraryImportsFeature             = "library-imports-feature"
	ruleProductionImportsDevelopment      = "production-imports-development"
	ruleSharedComponentImportsApplication = "shared-component-imports-application"
	ruleSourceDiagnostic                  = "source-diagnostic"
	ruleStableComponentLowAbstraction     = "stable-component-low-abstraction"
	// RuleStableDependencyPrinciple identifies the stable dependency policy.
	RuleStableDependencyPrinciple = "stable-dependency-principle"
)

// Rule evaluates one architecture policy.
type Rule interface {
	Evaluate(Graph) []Finding
}

// Registry composes independent architecture rules.
type Registry struct {
	rules []Rule
}

// NewRegistry creates a registry with a defensive copy of the rules.
func NewRegistry(rules ...Rule) Registry {
	return Registry{rules: slices.Clone(rules)}
}

// DefaultRegistry creates the standard architecture policy registry.
func DefaultRegistry() Registry {
	return NewRegistry(
		DependencyCycleRule{},
		SourceDiagnosticRule{},
		StableComponentLowAbstractionRule{},
		StableDependencyPrincipleRule{},
		ProductionImportsDevelopmentRule{},
		LibraryImportsFeatureRule{},
		SharedComponentImportsApplicationRule{},
		CrossApplicationModuleImportRule{},
	)
}

// Evaluate returns all findings in a deterministic order.
func (registry Registry) Evaluate(graph Graph) []Finding {
	findings := make([]Finding, 0)
	for _, rule := range registry.rules {
		findings = append(findings, rule.Evaluate(graph)...)
	}
	slices.SortFunc(findings, func(first, second Finding) int {
		return cmp.Or(
			strings.Compare(first.Rule, second.Rule),
			strings.Compare(first.Subject, second.Subject),
			strings.Compare(first.Source, second.Source),
			strings.Compare(first.Target, second.Target),
		)
	})
	return findings
}

// DependencyCycleRule detects production dependency cycles.
type DependencyCycleRule struct{}

// Evaluate returns one finding for each production dependency cycle.
func (DependencyCycleRule) Evaluate(graph Graph) []Finding {
	findings := make([]Finding, 0, len(graph.Cycles))
	for _, cycle := range graph.Cycles {
		findings = append(findings, Finding{
			Rule:       ruleDependencyCycle,
			Severity:   SeverityError,
			Subject:    strings.Join(cycle, " -> "),
			Message:    "Production dependencies form a cycle.",
			Components: slices.Clone(cycle),
			Metrics: map[string]float64{
				"componentCount": float64(len(cycle)),
			},
		})
	}
	return findings
}

// SourceDiagnosticRule converts source diagnostics to findings.
type SourceDiagnosticRule struct{}

// Evaluate returns one finding for each source diagnostic.
func (SourceDiagnosticRule) Evaluate(graph Graph) []Finding {
	findings := make([]Finding, 0, len(graph.Diagnostics))
	for _, diagnostic := range graph.Diagnostics {
		findings = append(findings, Finding{
			Rule:     ruleSourceDiagnostic,
			Severity: SeverityError,
			Subject:  diagnostic.Path,
			Message:  diagnostic.Message,
		})
	}
	return findings
}

// StableComponentLowAbstractionRule detects stable concrete components.
type StableComponentLowAbstractionRule struct{}

// Evaluate returns one finding for each stable component with low abstraction.
func (StableComponentLowAbstractionRule) Evaluate(graph Graph) []Finding {
	findings := make([]Finding, 0)
	for _, component := range graph.Components {
		if !component.StableWithLowAbstraction {
			continue
		}
		findings = append(findings, Finding{
			Rule:     ruleStableComponentLowAbstraction,
			Severity: SeverityWarning,
			Subject:  component.Identifier,
			Message:  "One or more production components import this stable component, which has low abstraction.",
			Metrics: map[string]float64{
				"abstractness":         component.Abstractness,
				"afferentCoupling":     float64(component.AfferentCoupling),
				"efferentCoupling":     float64(component.EfferentCoupling),
				"instability":          component.Instability,
				"mainSequenceDistance": component.MainSequenceDistance,
			},
		})
	}
	return findings
}

// StableDependencyPrincipleRule detects dependencies on less stable components.
type StableDependencyPrincipleRule struct{}

// Evaluate returns one finding for each production dependency that violates SDP.
func (StableDependencyPrincipleRule) Evaluate(graph Graph) []Finding {
	components := componentIndex(graph.Components)
	return relationshipFindings(
		graph.Relationships,
		components,
		RuleStableDependencyPrinciple,
		"The source component imports a less stable target component.",
		func(_ Relationship, source, target Component) bool {
			return target.Instability > source.Instability
		},
		func(source, target Component) map[string]float64 {
			return map[string]float64{
				"sourceInstability": source.Instability,
				"targetInstability": target.Instability,
			}
		},
	)
}

// ProductionImportsDevelopmentRule detects production dependencies on development code.
type ProductionImportsDevelopmentRule struct{}

// Evaluate returns one finding for each production dependency on development code.
func (ProductionImportsDevelopmentRule) Evaluate(graph Graph) []Finding {
	return roleRelationshipFindings(
		graph,
		ruleProductionImportsDevelopment,
		"Production code imports development code.",
		func(source, target Component) bool {
			return source.Role != RoleDevelopment && target.Role == RoleDevelopment
		},
	)
}

// LibraryImportsFeatureRule detects shared libraries that import application or module code.
type LibraryImportsFeatureRule struct{}

// Evaluate returns one finding for each library dependency on application or module code.
func (LibraryImportsFeatureRule) Evaluate(graph Graph) []Finding {
	return roleRelationshipFindings(
		graph,
		ruleLibraryImportsFeature,
		"A shared library imports application or module code.",
		func(source, target Component) bool {
			return source.Role == RoleLibrary && isFeatureRole(target.Role)
		},
	)
}

// SharedComponentImportsApplicationRule detects shared code that imports application code.
type SharedComponentImportsApplicationRule struct{}

// Evaluate returns one finding for each shared dependency on application code.
func (SharedComponentImportsApplicationRule) Evaluate(graph Graph) []Finding {
	return roleRelationshipFindings(
		graph,
		ruleSharedComponentImportsApplication,
		"A shared component imports application-specific code.",
		func(source, target Component) bool {
			sharedSource := source.Role == RoleSharedModule || source.Role == RoleInfrastructure
			applicationTarget := target.Role == RoleApplication || target.Role == RoleApplicationModule
			return sharedSource && applicationTarget
		},
	)
}

// CrossApplicationModuleImportRule detects dependencies across application boundaries.
type CrossApplicationModuleImportRule struct{}

// Evaluate returns one finding for each dependency between modules of different applications.
func (CrossApplicationModuleImportRule) Evaluate(graph Graph) []Finding {
	return roleRelationshipFindings(
		graph,
		ruleCrossApplicationModuleImport,
		"An application module imports a module from another application.",
		func(source, target Component) bool {
			return source.Role == RoleApplicationModule &&
				target.Role == RoleApplicationModule &&
				source.Application != target.Application
		},
	)
}

func roleRelationshipFindings(
	graph Graph,
	rule string,
	message string,
	violates func(Component, Component) bool,
) []Finding {
	return relationshipFindings(
		graph.Relationships,
		componentIndex(graph.Components),
		rule,
		message,
		func(_ Relationship, source, target Component) bool { return violates(source, target) },
		nil,
	)
}

func relationshipFindings(
	relationships []Relationship,
	components map[string]Component,
	rule string,
	message string,
	violates func(Relationship, Component, Component) bool,
	metrics func(Component, Component) map[string]float64,
) []Finding {
	findings := make([]Finding, 0)
	for _, relationship := range relationships {
		if relationship.TestOnly {
			continue
		}
		source, sourceExists := components[relationship.Source]
		target, targetExists := components[relationship.Target]
		if !sourceExists || !targetExists || !violates(relationship, source, target) {
			continue
		}
		finding := Finding{
			Rule:     rule,
			Severity: SeverityWarning,
			Subject:  relationship.Source + " -> " + relationship.Target,
			Message:  message,
			Source:   relationship.Source,
			Target:   relationship.Target,
		}
		if metrics != nil {
			finding.Metrics = metrics(source, target)
		}
		findings = append(findings, finding)
	}
	return findings
}

func componentIndex(components []Component) map[string]Component {
	index := make(map[string]Component, len(components))
	for _, component := range components {
		index[component.Identifier] = component
	}
	return index
}

func isFeatureRole(role Role) bool {
	return role == RoleApplication ||
		role == RoleApplicationModule ||
		role == RoleSharedModule
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T16:35:12Z","module_hash":"44246099eaba6538a8de1fe676b93c95db7ee070b46bd7a93cce36f01b6e0973","functions":[{"id":"func/NewRegistry","name":"NewRegistry","line":32,"end_line":34,"hash":"217fe1dd33af75543d755d4afe1a4eb1427bc4fe7b7f67fd35d1b6562491b208"},{"id":"func/DefaultRegistry","name":"DefaultRegistry","line":37,"end_line":48,"hash":"e0f4df743c5e7ccff3bf0663fe3f074d5d1f744ac8cc442d9c79d6cd7f481023"},{"id":"func/Registry.Evaluate","name":"Registry.Evaluate","line":51,"end_line":65,"hash":"c3436b008f9c51c6a797d92e0511479e45bea2775bd1e8f8b5644b4e3fc25a58"},{"id":"func/DependencyCycleRule.Evaluate","name":"DependencyCycleRule.Evaluate","line":71,"end_line":86,"hash":"a271c973331e22eabbbd6d1949b1f805c3ce346f0b65836da3e0df2bd3e77f3b"},{"id":"func/SourceDiagnosticRule.Evaluate","name":"SourceDiagnosticRule.Evaluate","line":92,"end_line":103,"hash":"1db9e331b1811a9bb253e1e14fe9bde14bb124fb765623f843328106aa14da5b"},{"id":"func/StableComponentLowAbstractionRule.Evaluate","name":"StableComponentLowAbstractionRule.Evaluate","line":109,"end_line":130,"hash":"7323c2d07ce22ced165104abdbadefeaaea2144c3a179c75de94db7edd45a91c"},{"id":"func/StableDependencyPrincipleRule.Evaluate","name":"StableDependencyPrincipleRule.Evaluate","line":136,"end_line":153,"hash":"7ed36632075fff9d302628523bd4fca2cedac06caaaa20ded7989a7d14ed9bfb"},{"id":"func/ProductionImportsDevelopmentRule.Evaluate","name":"ProductionImportsDevelopmentRule.Evaluate","line":159,"end_line":168,"hash":"2f28fea7b1451cd1b30df5be77ae707668153fe0394b0eeb5a16c04a42a0d415"},{"id":"func/LibraryImportsFeatureRule.Evaluate","name":"LibraryImportsFeatureRule.Evaluate","line":174,"end_line":183,"hash":"3456ed481803406aba6fbb4f5c7223054d337b3927c4cf1d27cab5d40fa09af6"},{"id":"func/SharedComponentImportsApplicationRule.Evaluate","name":"SharedComponentImportsApplicationRule.Evaluate","line":189,"end_line":200,"hash":"c799a04cc762e33bc55ae895ffb3b172b714b6871efc622e368fcbdc3669798d"},{"id":"func/CrossApplicationModuleImportRule.Evaluate","name":"CrossApplicationModuleImportRule.Evaluate","line":206,"end_line":217,"hash":"dc52fc90d16712b43695386a363d0fbd61e9c63e9e33bde1154879e6f19b0c05"},{"id":"func/roleRelationshipFindings","name":"roleRelationshipFindings","line":219,"end_line":233,"hash":"c8d7275b3c0e0d98569c37726f6736555a9ec98d2bf795c2a9b12f4b1b840cf2"},{"id":"func/relationshipFindings","name":"relationshipFindings","line":235,"end_line":267,"hash":"95c3e4a869b6868f3496134da9e0fa0c7e17e8b5214b05d94ebbbdd86aa02d63"},{"id":"func/componentIndex","name":"componentIndex","line":269,"end_line":275,"hash":"3ada87f9890e2fb667ae0915b39a118a3d1a3749c88f3675c57435971bec81f7"},{"id":"func/isFeatureRole","name":"isFeatureRole","line":277,"end_line":281,"hash":"994af74af71c90e13b62cb7e40c4b89100358f3d38e9d16b4c86c3ae40c80f6f"}]}
// mutate4go-manifest-end
