package main

import (
	"slices"
	"testing"
)

func TestFindings_DetectMathematicalRisks(t *testing.T) {
	t.Run("Scenario: A graph contains every deterministic architectural risk", func(t *testing.T) {
		var graph Graph
		var findings []Finding
		var summary GraphSummary

		t.Run("Given a cycle, a diagnostic, an SDP violation, and a pain-zone component", func(t *testing.T) {
			graph = Graph{
				Components: []Component{
					{
						Identifier:           "foundation",
						AfferentCoupling:     4,
						Instability:          0,
						Abstractness:         0,
						MainSequenceDistance: 1,
						InZoneOfPain:         true,
					},
					{Identifier: "origin", Instability: 0.2},
					{Identifier: "dependency", Instability: 0.8},
				},
				Relationships: []Relationship{
					{
						Source:   "origin",
						Target:   "dependency",
						Concerns: []string{"stable-dependency-principle"},
					},
				},
				Cycles:      [][]string{{"cycle-a", "cycle-b"}},
				Diagnostics: []Diagnostic{{Path: "broken.go", Message: "invalid import block"}},
			}
		})

		t.Run("When findings and their summary are calculated", func(t *testing.T) {
			findings = detectFindings(graph)
			graph.Findings = findings
			summary = summarizeGraph(graph)
		})

		t.Run("Then every risk has a stable sorted rule identifier", func(t *testing.T) {
			rules := make([]string, 0, len(findings))
			for _, finding := range findings {
				rules = append(rules, finding.Rule)
			}
			want := []string{
				"dependency-cycle",
				"source-diagnostic",
				"stable-dependency-principle",
				"zone-of-pain",
			}
			if !slices.Equal(rules, want) {
				t.Fatalf("finding rules are %v, want %v", rules, want)
			}
		})

		t.Run("And mathematical evidence is retained without prose parsing", func(t *testing.T) {
			sdp := findingWithRule(t, findings, "stable-dependency-principle")
			if sdp.Metrics["sourceInstability"] != 0.2 || sdp.Metrics["targetInstability"] != 0.8 {
				t.Errorf("unexpected SDP metrics: %v", sdp.Metrics)
			}
			pain := findingWithRule(t, findings, "zone-of-pain")
			if pain.Metrics["afferentCoupling"] != 4 || pain.Metrics["mainSequenceDistance"] != 1 {
				t.Errorf("unexpected pain-zone metrics: %v", pain.Metrics)
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

func TestFindingMessage_NormalizeRule(t *testing.T) {
	testCases := []struct {
		rule string
		want string
	}{
		{
			rule: "cross-application-module-dependency",
			want: "An application module depends on a module owned by another application.",
		},
		{
			rule: "library-depends-on-feature",
			want: "A shared library depends on a feature module.",
		},
		{
			rule: "production-depends-on-development",
			want: "Production code depends on development-only tooling.",
		},
		{
			rule: "shared-foundation-depends-on-application",
			want: "Shared foundation code depends on application-specific code.",
		},
		{
			rule: "stable-dependency-principle",
			want: "A dependency points to a less stable component.",
		},
		{
			rule: "unknown-rule",
			want: "A strategic dependency rule is violated.",
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: The finding rule is "+testCase.rule, func(t *testing.T) {
			var rule string
			var message string

			t.Run("Given a stable machine-readable rule identifier", func(t *testing.T) {
				rule = testCase.rule
			})

			t.Run("When its human message is normalized", func(t *testing.T) {
				message = findingMessage(rule)
			})

			t.Run("Then the exact technical message is returned", func(t *testing.T) {
				if message != testCase.want {
					t.Fatalf("message is %q, want %q", message, testCase.want)
				}
			})
		})
	}
}
