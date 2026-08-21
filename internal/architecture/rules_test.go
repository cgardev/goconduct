package architecture

import (
	"slices"
	"testing"
)

func TestDefaultRules_EvaluateEachPolicy(t *testing.T) {
	testCases := []struct {
		name  string
		rule  Rule
		graph Graph
		want  string
	}{
		{
			name:  "a production dependency cycle exists",
			rule:  DependencyCycleRule{},
			graph: Graph{Cycles: [][]string{{"a", "b"}}},
			want:  ruleDependencyCycle,
		},
		{
			name:  "a source diagnostic exists",
			rule:  SourceDiagnosticRule{},
			graph: Graph{Diagnostics: []Diagnostic{{Path: "a.go", Message: "invalid source"}}},
			want:  ruleSourceDiagnostic,
		},
		{
			name: "a stable component has low abstraction",
			rule: StableComponentLowAbstractionRule{},
			graph: Graph{Components: []Component{{
				Identifier:               "stable",
				StableWithLowAbstraction: true,
			}}},
			want: ruleStableComponentLowAbstraction,
		},
		{
			name:  "a stable component imports a less stable component",
			rule:  StableDependencyPrincipleRule{},
			graph: relationshipRuleGraph(RoleLibrary, RoleLibrary, "", "", 0.2, 0.8),
			want:  RuleStableDependencyPrinciple,
		},
		{
			name:  "production code imports development code",
			rule:  ProductionImportsDevelopmentRule{},
			graph: relationshipRuleGraph(RoleApplication, RoleDevelopment, "", "", 0, 0),
			want:  ruleProductionImportsDevelopment,
		},
		{
			name:  "a shared library imports module code",
			rule:  LibraryImportsFeatureRule{},
			graph: relationshipRuleGraph(RoleLibrary, RoleSharedModule, "", "", 0, 0),
			want:  ruleLibraryImportsFeature,
		},
		{
			name:  "shared infrastructure imports application code",
			rule:  SharedComponentImportsApplicationRule{},
			graph: relationshipRuleGraph(RoleInfrastructure, RoleApplication, "", "", 0, 0),
			want:  ruleSharedComponentImportsApplication,
		},
		{
			name: "one application module imports a module from another application",
			rule: CrossApplicationModuleImportRule{},
			graph: relationshipRuleGraph(
				RoleApplicationModule,
				RoleApplicationModule,
				"control",
				"portal",
				0,
				0,
			),
			want: ruleCrossApplicationModuleImport,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var findings []Finding

			t.Run("Given one graph that violates one independent rule", func(*testing.T) {})

			t.Run("When the evaluator checks the graph", func(*testing.T) {
				findings = testCase.rule.Evaluate(testCase.graph)
			})

			t.Run("Then the evaluator returns its stable rule identifier", func(t *testing.T) {
				if len(findings) != 1 || findings[0].Rule != testCase.want {
					t.Fatalf("findings are %+v, want rule %s", findings, testCase.want)
				}
			})
		})
	}
}

func TestRelationshipRules_IgnoreTestDependencies(t *testing.T) {
	t.Run("Scenario: A test imports development code", func(t *testing.T) {
		var graph Graph
		var findings []Finding

		t.Run("Given a relationship that exists only in tests", func(*testing.T) {
			graph = relationshipRuleGraph(RoleLibrary, RoleDevelopment, "", "", 0, 1)
			graph.Relationships[0].TestOnly = true
		})

		t.Run("When the default registry evaluates the graph", func(*testing.T) {
			findings = DefaultRegistry().Evaluate(graph)
		})

		t.Run("Then no production architecture rule reports the relationship", func(t *testing.T) {
			if len(findings) != 0 {
				t.Fatalf("test relationship findings are %+v", findings)
			}
		})
	})
}

func TestStableDependencyPrincipleRule_RequireStrictInstabilityIncrease(t *testing.T) {
	testCases := []struct {
		name              string
		sourceInstability float64
		targetInstability float64
		wantFindings      int
	}{
		{name: "the target is less stable", sourceInstability: 0.2, targetInstability: 0.8, wantFindings: 1},
		{name: "both components have equal stability", sourceInstability: 0.5, targetInstability: 0.5},
		{name: "the target is more stable", sourceInstability: 0.8, targetInstability: 0.2},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var graph Graph
			var findings []Finding

			t.Run("Given two components with explicit instability values", func(*testing.T) {
				graph = relationshipRuleGraph(
					RoleLibrary,
					RoleLibrary,
					"",
					"",
					testCase.sourceInstability,
					testCase.targetInstability,
				)
			})

			t.Run("When the stable dependency rule evaluates the relationship", func(*testing.T) {
				findings = (StableDependencyPrincipleRule{}).Evaluate(graph)
			})

			t.Run("Then only a strict instability increase creates a finding", func(t *testing.T) {
				if len(findings) != testCase.wantFindings {
					t.Fatalf("finding count is %d, want %d", len(findings), testCase.wantFindings)
				}
			})
		})
	}
}

func TestProductionImportsDevelopmentRule_RequireProductionSource(t *testing.T) {
	testCases := []struct {
		name         string
		sourceRole   Role
		targetRole   Role
		wantFindings int
	}{
		{
			name: "application code imports development code", sourceRole: RoleApplication,
			targetRole: RoleDevelopment, wantFindings: 1,
		},
		{
			name: "development code imports other development code", sourceRole: RoleDevelopment,
			targetRole: RoleDevelopment,
		},
		{
			name: "development code imports a library", sourceRole: RoleDevelopment,
			targetRole: RoleLibrary,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var graph Graph
			var findings []Finding

			t.Run("Given one relationship with explicit strategic roles", func(*testing.T) {
				graph = relationshipRuleGraph(testCase.sourceRole, testCase.targetRole, "", "", 0, 0)
			})

			t.Run("When the production boundary rule evaluates the relationship", func(*testing.T) {
				findings = (ProductionImportsDevelopmentRule{}).Evaluate(graph)
			})

			t.Run("Then only a production source creates a finding", func(t *testing.T) {
				if len(findings) != testCase.wantFindings {
					t.Fatalf("finding count is %d, want %d", len(findings), testCase.wantFindings)
				}
			})
		})
	}
}

func TestLibraryImportsFeatureRule_RequireLibraryAndFeature(t *testing.T) {
	testCases := []struct {
		name         string
		sourceRole   Role
		targetRole   Role
		wantFindings int
	}{
		{
			name: "a library imports an application", sourceRole: RoleLibrary,
			targetRole: RoleApplication, wantFindings: 1,
		},
		{
			name: "a library imports an application module", sourceRole: RoleLibrary,
			targetRole: RoleApplicationModule, wantFindings: 1,
		},
		{
			name: "a library imports a shared module", sourceRole: RoleLibrary,
			targetRole: RoleSharedModule, wantFindings: 1,
		},
		{name: "an application imports a shared module", sourceRole: RoleApplication, targetRole: RoleSharedModule},
		{name: "a library imports infrastructure", sourceRole: RoleLibrary, targetRole: RoleInfrastructure},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var graph Graph
			var findings []Finding

			t.Run("Given one production relationship", func(*testing.T) {
				graph = relationshipRuleGraph(testCase.sourceRole, testCase.targetRole, "", "", 0, 0)
			})

			t.Run("When the library boundary rule evaluates the relationship", func(*testing.T) {
				findings = (LibraryImportsFeatureRule{}).Evaluate(graph)
			})

			t.Run("Then the source and target roles determine the finding", func(t *testing.T) {
				if len(findings) != testCase.wantFindings {
					t.Fatalf("finding count is %d, want %d", len(findings), testCase.wantFindings)
				}
			})
		})
	}
}

func TestSharedComponentImportsApplicationRule_RequireSharedAndApplicationRoles(t *testing.T) {
	testCases := []struct {
		name         string
		sourceRole   Role
		targetRole   Role
		wantFindings int
	}{
		{
			name: "a shared module imports an application", sourceRole: RoleSharedModule,
			targetRole: RoleApplication, wantFindings: 1,
		},
		{
			name: "infrastructure imports an application module", sourceRole: RoleInfrastructure,
			targetRole: RoleApplicationModule, wantFindings: 1,
		},
		{name: "a library imports an application", sourceRole: RoleLibrary, targetRole: RoleApplication},
		{name: "a shared module imports a library", sourceRole: RoleSharedModule, targetRole: RoleLibrary},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var graph Graph
			var findings []Finding

			t.Run("Given one production relationship", func(*testing.T) {
				graph = relationshipRuleGraph(testCase.sourceRole, testCase.targetRole, "", "", 0, 0)
			})

			t.Run("When the shared component rule evaluates the relationship", func(*testing.T) {
				findings = (SharedComponentImportsApplicationRule{}).Evaluate(graph)
			})

			t.Run("Then both boundary roles determine the finding", func(t *testing.T) {
				if len(findings) != testCase.wantFindings {
					t.Fatalf("finding count is %d, want %d", len(findings), testCase.wantFindings)
				}
			})
		})
	}
}

func TestCrossApplicationModuleImportRule_RequireDifferentApplicationModules(t *testing.T) {
	testCases := []struct {
		name              string
		sourceRole        Role
		targetRole        Role
		sourceApplication string
		targetApplication string
		wantFindings      int
	}{
		{
			name: "application modules belong to different applications", sourceRole: RoleApplicationModule,
			targetRole: RoleApplicationModule, sourceApplication: "control",
			targetApplication: "portal", wantFindings: 1,
		},
		{
			name: "the source is an application", sourceRole: RoleApplication,
			targetRole: RoleApplicationModule, sourceApplication: "control", targetApplication: "portal",
		},
		{
			name: "the target is a shared module", sourceRole: RoleApplicationModule,
			targetRole: RoleSharedModule, sourceApplication: "control", targetApplication: "portal",
		},
		{
			name: "both modules belong to the same application", sourceRole: RoleApplicationModule,
			targetRole: RoleApplicationModule, sourceApplication: "control", targetApplication: "control",
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var graph Graph
			var findings []Finding

			t.Run("Given one relationship with explicit ownership", func(*testing.T) {
				graph = relationshipRuleGraph(
					testCase.sourceRole,
					testCase.targetRole,
					testCase.sourceApplication,
					testCase.targetApplication,
					0,
					0,
				)
			})

			t.Run("When the application boundary rule evaluates the relationship", func(*testing.T) {
				findings = (CrossApplicationModuleImportRule{}).Evaluate(graph)
			})

			t.Run("Then the rule requires two application module roles with different owners", func(t *testing.T) {
				if len(findings) != testCase.wantFindings {
					t.Fatalf("finding count is %d, want %d", len(findings), testCase.wantFindings)
				}
			})
		})
	}
}

func TestRelationshipFindings_IgnoreUnknownComponents(t *testing.T) {
	testCases := []struct {
		name       string
		components map[string]Component
	}{
		{
			name:       "the source component is unknown",
			components: map[string]Component{"target": {Identifier: "target"}},
		},
		{
			name:       "the target component is unknown",
			components: map[string]Component{"source": {Identifier: "source"}},
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var findings []Finding

			t.Run("Given a relationship with one missing component", func(*testing.T) {})

			t.Run("When the shared relationship evaluator receives an always true rule", func(*testing.T) {
				findings = relationshipFindings(
					[]Relationship{{Source: "source", Target: "target"}},
					testCase.components,
					"rule",
					"message",
					func(Relationship, Component, Component) bool { return true },
					nil,
				)
			})

			t.Run("Then the evaluator does not create a finding", func(t *testing.T) {
				if len(findings) != 0 {
					t.Fatalf("findings are %+v", findings)
				}
			})
		})
	}
}

func TestRegistry_ComposeRulesDeterministically(t *testing.T) {
	t.Run("Scenario: Independent rules return findings in reverse order", func(t *testing.T) {
		var rules []Rule
		var registry Registry
		var findings []Finding

		t.Run("Given two independent rule evaluators", func(*testing.T) {
			rules = []Rule{
				fixedRule{finding: Finding{Rule: "z-rule", Subject: "b"}},
				fixedRule{finding: Finding{Rule: "a-rule", Subject: "a"}},
			}
			registry = NewRegistry(rules...)
			rules[0] = fixedRule{finding: Finding{Rule: "changed-rule"}}
		})

		t.Run("When the registry evaluates the graph", func(*testing.T) {
			findings = registry.Evaluate(Graph{})
		})

		t.Run("Then the registry sorts findings by rule and subject", func(t *testing.T) {
			got := []string{findings[0].Rule, findings[1].Rule}
			if !slices.Equal(got, []string{"a-rule", "z-rule"}) {
				t.Fatalf("finding order is %v", got)
			}
		})

		t.Run("And later changes to the input slice do not change the registry", func(t *testing.T) {
			for _, finding := range findings {
				if finding.Rule == "changed-rule" {
					t.Error("the input slice changes the registry")
				}
			}
		})
	})
}

func TestRole_ValidateClosedSet(t *testing.T) {
	t.Run("Scenario: The policy receives one closed role and one presentation category", func(t *testing.T) {
		var valid bool
		var invalid bool

		t.Run("Given a library role and a custom plugin category", func(*testing.T) {})

		t.Run("When the core validates the policy roles", func(*testing.T) {
			valid = ValidRole(RoleLibrary)
			invalid = ValidRole(Role("plugin"))
		})

		t.Run("Then only the closed policy role is valid", func(t *testing.T) {
			if !valid || invalid {
				t.Fatalf("role validation is valid=%t invalid=%t", valid, invalid)
			}
		})
	})
}

type fixedRule struct {
	finding Finding
}

func (rule fixedRule) Evaluate(Graph) []Finding {
	return []Finding{rule.finding}
}

func relationshipRuleGraph(
	sourceRole Role,
	targetRole Role,
	sourceApplication string,
	targetApplication string,
	sourceInstability float64,
	targetInstability float64,
) Graph {
	return Graph{
		Components: []Component{
			{
				Identifier:  "source",
				Role:        sourceRole,
				Application: sourceApplication,
				Instability: sourceInstability,
			},
			{
				Identifier:  "target",
				Role:        targetRole,
				Application: targetApplication,
				Instability: targetInstability,
			},
		},
		Relationships: []Relationship{{Source: "source", Target: "target"}},
	}
}
