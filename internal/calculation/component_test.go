package calculation

import (
	"slices"
	"testing"

	"github.com/cgardev/goconduct/internal/architecture"
)

func TestFindings_DetectArchitectureFindings(t *testing.T) {
	t.Run("Scenario: A graph contains each deterministic architecture finding", func(t *testing.T) {
		var graph Graph
		var findings []Finding
		var summary GraphSummary

		t.Run(
			"Given a cycle, a diagnostic, a stability violation, and a stable component with low abstraction",
			func(t *testing.T) {
				graph = Graph{
					Components: []Component{
						{
							Identifier:                 "foundation",
							AfferentCoupling:           4,
							Instability:                0,
							Abstractness:               0,
							MainSequenceDistance:       1,
							IsStableWithLowAbstraction: true,
						},
						{Identifier: "origin", Instability: 0.2},
						{Identifier: "dependency", Instability: 0.8},
					},
					Relationships: []Relationship{
						{
							Source:         "origin",
							Target:         "dependency",
							RuleViolations: []string{"stable-dependency-principle"},
						},
					},
					Cycles:      [][]string{{"cycle-a", "cycle-b"}},
					Diagnostics: []Diagnostic{{Path: "broken.go", Message: "invalid import block"}},
				}
			},
		)

		t.Run("When the calculator creates findings and summary counts", func(t *testing.T) {
			findings = detectFindings(graph)
			graph.Findings = findings
			summary = summarizeGraph(graph)
		})

		t.Run("Then each finding has a sorted rule identifier", func(t *testing.T) {
			rules := make([]string, 0, len(findings))
			for _, finding := range findings {
				rules = append(rules, finding.Rule)
			}
			want := []string{
				"dependency-cycle",
				"source-diagnostic",
				"stable-component-low-abstraction",
				"stable-dependency-principle",
			}
			if !slices.Equal(rules, want) {
				t.Fatalf("finding rules are %v, want %v", rules, want)
			}
		})

		t.Run("And each finding contains its numeric evidence", func(t *testing.T) {
			stableDependencyFinding := findingWithRule(t, findings, "stable-dependency-principle")
			if stableDependencyFinding.Metrics["sourceInstability"] != 0.2 ||
				stableDependencyFinding.Metrics["targetInstability"] != 0.8 {
				t.Errorf("unexpected stable dependency principle metrics: %v", stableDependencyFinding.Metrics)
			}
			stableLowAbstractionFinding := findingWithRule(t, findings, "stable-component-low-abstraction")
			if stableLowAbstractionFinding.Metrics["afferentCoupling"] != 4 ||
				stableLowAbstractionFinding.Metrics["mainSequenceDistance"] != 1 {
				t.Errorf("unexpected stable-low-abstraction metrics: %v", stableLowAbstractionFinding.Metrics)
			}
			if summary.Findings != 4 || summary.Errors != 2 || summary.Warnings != 2 {
				t.Errorf("unexpected finding summary: %+v", summary)
			}
		})
	})
}

func findingWithRule(t *testing.T, findings []Finding, rule string) Finding {
	t.Helper()
	for _, finding := range findings {
		if finding.Rule == rule {
			return finding
		}
	}
	t.Fatalf("finding rule %q is absent", rule)
	return Finding{}
}

func TestArchitectureRules_ApplyCustomRule(t *testing.T) {
	t.Run("Scenario: The composition root adds one architecture rule", func(t *testing.T) {
		var graph Graph

		t.Run("Given a graph and a registry with one custom evaluator", func(t *testing.T) {
			graph = Graph{
				Components: []Component{
					{Identifier: "source", Role: componentRoleLibrary},
					{Identifier: "target", Role: componentRoleLibrary},
				},
				Relationships: []Relationship{{Source: "source", Target: "target"}},
			}
		})

		t.Run("When the calculator applies the composed registry", func(t *testing.T) {
			applyArchitectureRules(
				&graph,
				architecture.NewRegistry(customArchitectureRule{}),
			)
		})

		t.Run("Then the graph contains the custom finding", func(t *testing.T) {
			if len(graph.Findings) != 1 || graph.Findings[0].Rule != "custom-dependency" {
				t.Fatalf("unexpected custom findings: %+v", graph.Findings)
			}
		})

		t.Run("And the relationship contains the custom rule identifier", func(t *testing.T) {
			if !slices.Equal(graph.Relationships[0].RuleViolations, []string{"custom-dependency"}) {
				t.Errorf("unexpected relationship rules: %v", graph.Relationships[0].RuleViolations)
			}
		})
	})
}

func TestArchitectureRules_IgnoreFindingsWithoutCompleteRelationship(t *testing.T) {
	t.Run("Scenario: Custom findings omit one relationship endpoint", func(t *testing.T) {
		var graph Graph

		t.Run("Given relationships that use one empty endpoint", func(*testing.T) {
			graph = Graph{Relationships: []Relationship{
				{Source: "", Target: "target"},
				{Source: "source", Target: ""},
			}}
		})

		t.Run("When the calculator applies findings with the same incomplete endpoints", func(*testing.T) {
			applyArchitectureRules(
				&graph,
				architecture.NewRegistry(incompleteRelationshipRule{}),
			)
		})

		t.Run("Then no relationship receives an incomplete finding", func(t *testing.T) {
			for _, relationship := range graph.Relationships {
				if len(relationship.RuleViolations) != 0 {
					t.Fatalf("incomplete relationship rules are %v", relationship.RuleViolations)
				}
			}
		})
	})
}

type customArchitectureRule struct{}

func (customArchitectureRule) Evaluate(architecture.Graph) []architecture.Finding {
	return []architecture.Finding{{
		Rule:     "custom-dependency",
		Severity: architecture.SeverityWarning,
		Subject:  "source -> target",
		Message:  "A custom policy rejects this dependency.",
		Source:   "source",
		Target:   "target",
	}}
}

type incompleteRelationshipRule struct{}

func (incompleteRelationshipRule) Evaluate(architecture.Graph) []architecture.Finding {
	return []architecture.Finding{
		{Rule: "missing-source", Source: "", Target: "target"},
		{Rule: "missing-target", Source: "source", Target: ""},
	}
}

func TestGraphSummary_CountConfiguredCategories(t *testing.T) {
	t.Run("Scenario: Multiple presentation categories use one strategic role", func(t *testing.T) {
		var graph Graph
		var summary GraphSummary

		t.Run("Given plugin and extension components with the library role", func(*testing.T) {
			graph = Graph{Components: []Component{
				{Identifier: "plugins/auth", Role: componentRoleLibrary, Category: "plugin"},
				{Identifier: "plugins/log", Role: componentRoleLibrary, Category: "plugin"},
				{Identifier: "extensions/report", Role: componentRoleLibrary, Category: "extension"},
			}}
		})

		t.Run("When the calculator creates the graph summary", func(*testing.T) {
			summary = summarizeGraph(graph)
		})

		t.Run("Then the summary counts each configured category", func(t *testing.T) {
			if summary.Categories["plugin"] != 2 || summary.Categories["extension"] != 1 {
				t.Fatalf("unexpected category counts: %v", summary.Categories)
			}
		})

		t.Run("And the compatible role count includes all three components", func(t *testing.T) {
			if summary.Libraries != 3 {
				t.Errorf("library role count is %d", summary.Libraries)
			}
		})
	})
}

func TestComponentCalculation_BuildStrategicGraph(t *testing.T) {
	t.Run("Scenario: Production and test imports connect each strategic role", func(t *testing.T) {
		var components map[string]*componentAccumulator
		var relationships map[relationshipKey]*relationshipAccumulator
		var graph Graph

		t.Run("Given source files with cycles, applications, test imports, and each role", func(*testing.T) {
			applicationA := componentDescriptor{
				identifier: "cmd/alpha", name: "alpha", role: componentRoleApplication, application: "alpha",
			}
			applicationB := componentDescriptor{
				identifier: "cmd/beta", name: "beta", role: componentRoleApplication, application: "beta",
			}
			libraryA := componentDescriptor{
				identifier: "internal/library/alpha", name: "alpha", role: componentRoleLibrary,
			}
			libraryB := componentDescriptor{
				identifier: "internal/library/beta", name: "beta", role: componentRoleLibrary,
				category: "utility",
			}
			development := componentDescriptor{
				identifier: "internal/devtool/generator", name: "generator", role: componentRoleDevelopment,
			}
			applicationModule := componentDescriptor{
				identifier: "cmd/alpha/internal/module/orders", name: "orders",
				role: componentRoleApplicationModule, application: "alpha",
			}
			sharedModule := componentDescriptor{
				identifier: "internal/module/audit", name: "audit", role: componentRoleSharedModule,
			}
			infrastructure := componentDescriptor{
				identifier: "internal/kernel", name: "kernel", role: componentRoleInfrastructure,
			}

			files := []sourceFile{
				{
					relativePath: "cmd/alpha/main.go", packagePath: "cmd/alpha", component: applicationA,
					concreteTypes: 1,
					imports: []sourceImport{
						{
							packagePath: "internal/library/alpha", component: libraryA,
							site: ImportSite{Path: "cmd/alpha/main.go", Line: 8, TargetPackage: "alpha"},
						},
						{
							packagePath: "internal/devtool/generator", component: development,
							site: ImportSite{Path: "cmd/alpha/main.go", Line: 7, TargetPackage: "generator"},
						},
						{packagePath: "cmd/alpha", component: applicationA},
					},
				},
				{
					relativePath: "cmd/alpha/run.go", packagePath: "cmd/alpha", component: applicationA,
					imports: []sourceImport{{
						packagePath: "internal/library/alpha", component: libraryA,
						site: ImportSite{Path: "cmd/alpha/run.go", Line: 3, TargetPackage: "alpha", Alias: "a"},
					}},
				},
				{
					relativePath: "cmd/alpha/main_test.go", packagePath: "cmd/alpha", component: applicationA,
					test: true,
					imports: []sourceImport{{
						packagePath: "internal/library/alpha", component: libraryA,
						site: ImportSite{Path: "cmd/alpha/main_test.go", Line: 4, TargetPackage: "alpha"},
					}},
				},
				{
					relativePath: "cmd/beta/main.go", packagePath: "cmd/beta", component: applicationB,
					imports: []sourceImport{{
						packagePath: "internal/library/alpha", component: libraryA,
						site: ImportSite{Path: "cmd/beta/main.go", Line: 5, TargetPackage: "alpha"},
					}},
				},
				{
					relativePath: "internal/library/alpha/alpha.go", packagePath: "internal/library/alpha",
					component: libraryA, abstractTypes: 1, concreteTypes: 1,
					imports: []sourceImport{{
						packagePath: "internal/library/beta", component: libraryB,
						site: ImportSite{Path: "internal/library/alpha/alpha.go", Line: 6, TargetPackage: "beta"},
					}},
				},
				{
					relativePath: "internal/library/alpha/alpha_test.go", packagePath: "internal/library/alpha",
					component: libraryA, test: true,
					imports: []sourceImport{{
						packagePath: "cmd/alpha", component: applicationA,
						site: ImportSite{Path: "internal/library/alpha/alpha_test.go", Line: 5, TargetPackage: "alpha"},
					}},
				},
				{
					relativePath: "internal/library/beta/beta.go", packagePath: "internal/library/beta",
					component: libraryB, concreteTypes: 2,
					imports: []sourceImport{{
						packagePath: "internal/library/alpha", component: libraryA,
						site: ImportSite{Path: "internal/library/beta/beta.go", Line: 4, TargetPackage: "alpha"},
					}},
				},
				{
					relativePath: "internal/devtool/generator/main.go", packagePath: "internal/devtool/generator",
					component: development,
				},
				{
					relativePath: "cmd/alpha/internal/module/orders/domain.go",
					packagePath:  "cmd/alpha/internal/module/orders", component: applicationModule,
				},
				{
					relativePath: "internal/module/audit/domain.go", packagePath: "internal/module/audit",
					component: sharedModule,
				},
				{
					relativePath: "internal/kernel/kernel.go", packagePath: "internal/kernel",
					component: infrastructure,
				},
			}

			components = make(map[string]*componentAccumulator)
			relationships = make(map[relationshipKey]*relationshipAccumulator)
			for _, file := range files {
				collectComponentFile(components, file)
				collectRelationships(components, relationships, file)
			}
		})

		t.Run("When the calculator builds the strategic graph", func(*testing.T) {
			graph = buildGraph(
				"example.com/repository",
				components,
				relationships,
				[]Diagnostic{
					{Path: "z.go", Message: "last"},
					{Path: "a.go", Message: "first"},
				},
			)
		})

		if !t.Run("Then the graph contains deterministic component and relationship metrics", func(t *testing.T) {
			if len(graph.Components) != 8 || len(graph.Relationships) != 6 {
				t.Fatalf(
					"graph has %d components and %d relationships, want 8 and 6",
					len(graph.Components),
					len(graph.Relationships),
				)
			}
			library := componentWithIdentifier(t, graph, "internal/library/alpha")
			if library.AfferentCoupling != 3 || library.EfferentCoupling != 1 || !library.InCycle {
				t.Fatalf("unexpected library coupling: %+v", library)
			}
			if !slices.Equal(library.UsingApplications, []string{"alpha", "beta"}) {
				t.Errorf("using applications are %v, want [alpha beta]", library.UsingApplications)
			}
		}) {
			return
		}

		t.Run("And production and test relationships remain distinct", func(t *testing.T) {
			production := relationshipWithComponents(t, graph, "cmd/alpha", "internal/library/alpha")
			if production.TestOnly || production.ProductionReferencingFiles != 2 ||
				production.TestReferencingFiles != 1 || len(production.ImportSites) != 3 {
				t.Errorf("unexpected production relationship: %+v", production)
			}
			testOnly := relationshipWithComponents(t, graph, "internal/library/alpha", "cmd/alpha")
			if !testOnly.TestOnly || testOnly.TestReferencingFiles != 1 {
				t.Errorf("unexpected test relationship: %+v", testOnly)
			}
		})

		t.Run("And the summary contains every role, cycle, and sorted diagnostic", func(t *testing.T) {
			if graph.Summary.Applications != 2 || graph.Summary.ApplicationModules != 1 ||
				graph.Summary.SharedModules != 1 || graph.Summary.Libraries != 2 ||
				graph.Summary.Infrastructure != 1 || graph.Summary.DevelopmentTools != 1 ||
				graph.Summary.Cycles != 1 || graph.Summary.TestOnlyRelationships != 1 {
				t.Errorf("unexpected graph summary: %+v", graph.Summary)
			}
			if len(graph.Diagnostics) != 2 || graph.Diagnostics[0].Path != "a.go" {
				t.Errorf("diagnostics are not sorted: %+v", graph.Diagnostics)
			}
		})
	})
}

func relationshipWithComponents(t *testing.T, graph Graph, source, target string) Relationship {
	t.Helper()
	for _, relationship := range graph.Relationships {
		if relationship.Source == source && relationship.Target == target {
			return relationship
		}
	}
	t.Fatalf("relationship %q -> %q is absent", source, target)
	return Relationship{}
}
