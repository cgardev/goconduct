package main

import (
	"slices"
	"testing"
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

func TestFindingMessage_ReturnRuleMessage(t *testing.T) {
	testCases := []struct {
		rule string
		want string
	}{
		{
			rule: "cross-application-module-import",
			want: "An application module imports a module from another application.",
		},
		{
			rule: "library-imports-feature",
			want: "A shared library imports a feature module.",
		},
		{
			rule: "production-imports-development",
			want: "Production code imports development code.",
		},
		{
			rule: "shared-component-imports-application",
			want: "A shared component imports application-specific code.",
		},
		{
			rule: "stable-dependency-principle",
			want: "The source component imports a less stable target component.",
		},
		{
			rule: "unknown-rule",
			want: "The dependency violates an architecture rule.",
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: The finding rule is "+testCase.rule, func(t *testing.T) {
			var rule string
			var message string

			t.Run("Given a stable rule identifier", func(t *testing.T) {
				rule = testCase.rule
			})

			t.Run("When the function creates the user message", func(t *testing.T) {
				message = findingMessage(rule)
			})

			t.Run("Then the message matches the rule", func(t *testing.T) {
				if message != testCase.want {
					t.Fatalf("message is %q, want %q", message, testCase.want)
				}
			})
		})
	}
}
