package main

import (
	"slices"
	"testing"

	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/architecture"
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
