package architecture

import (
	"strings"
	"testing"
)

func TestDependencyPolicyUsesDefaultDenyAndGrantUnion(t *testing.T) {
	graph := dependencyPolicyFixtureGraph()
	policy := DependencyPolicyConfiguration{
		ProductionDefault: DependencyDefaultDeny,
		TestDefault:       DependencyDefaultAllow,
		Allow: []DependencyRuleConfiguration{{
			Identifier: "applications-use-libraries",
			From:       ComponentSelectorConfiguration{Roles: []ComponentRole{componentRoleApplication}},
			To:         ComponentSelectorConfiguration{Roles: []ComponentRole{componentRoleLibrary}},
			Reason:     "Applications compose shared libraries.",
		}},
	}

	if err := applyDependencyPolicy(&graph, policy); err != nil {
		t.Fatalf("apply dependency policy: %v", err)
	}
	if len(graph.Findings) != 1 {
		t.Fatalf("finding count is %d", len(graph.Findings))
	}
	if graph.Findings[0].Source != "cmd/tool" || graph.Findings[0].Target != "internal/devtool/generator" {
		t.Fatalf("finding is %+v", graph.Findings[0])
	}
	if graph.Summary.Errors != 1 || graph.Revision == "" {
		t.Fatalf("summary or revision is invalid: %+v %q", graph.Summary, graph.Revision)
	}
}

func TestDependencyPolicyDenialOverridesGrant(t *testing.T) {
	graph := dependencyPolicyFixtureGraph()
	policy := DependencyPolicyConfiguration{
		ProductionDefault: DependencyDefaultAllow,
		TestDefault:       DependencyDefaultAllow,
		Allow: []DependencyRuleConfiguration{{
			Identifier: "all-application-targets",
			From:       ComponentSelectorConfiguration{Roles: []ComponentRole{componentRoleApplication}},
			To: ComponentSelectorConfiguration{Roles: []ComponentRole{
				componentRoleLibrary,
				componentRoleDevelopment,
			}},
			Reason: "The application composes selected tools.",
		}},
		Deny: []DependencyRuleConfiguration{{
			Identifier: "no-development-imports",
			From:       ComponentSelectorConfiguration{Roles: []ComponentRole{componentRoleApplication}},
			To:         ComponentSelectorConfiguration{Roles: []ComponentRole{componentRoleDevelopment}},
			Reason:     "Production applications must not import development tools.",
		}},
	}

	if err := applyDependencyPolicy(&graph, policy); err != nil {
		t.Fatalf("apply dependency policy: %v", err)
	}
	if len(graph.Findings) != 1 || !strings.Contains(graph.Findings[0].Message, "no-development-imports") {
		t.Fatalf("findings are %+v", graph.Findings)
	}
}

func TestDependencyPolicyRejectsStaleSelectors(t *testing.T) {
	graph := dependencyPolicyFixtureGraph()
	policy := DependencyPolicyConfiguration{
		ProductionDefault: DependencyDefaultDeny,
		TestDefault:       DependencyDefaultAllow,
		Allow: []DependencyRuleConfiguration{{
			Identifier: "missing-component",
			From:       ComponentSelectorConfiguration{Identifiers: []string{"missing"}},
			To:         ComponentSelectorConfiguration{Roles: []ComponentRole{componentRoleLibrary}},
			Reason:     "The retired component used this library.",
		}},
	}

	err := applyDependencyPolicy(&graph, policy)
	if err == nil || !strings.Contains(err.Error(), "matches no components") {
		t.Fatalf("error is %v", err)
	}
}

func TestDependencyPolicyCacheKeyIncludesRules(t *testing.T) {
	first, err := dependencyPolicyCacheKey("base", DependencyPolicyConfiguration{
		ProductionDefault: DependencyDefaultAllow,
		TestDefault:       DependencyDefaultAllow,
	})
	if err != nil {
		t.Fatalf("create first key: %v", err)
	}
	second, err := dependencyPolicyCacheKey("base", DependencyPolicyConfiguration{
		ProductionDefault: DependencyDefaultDeny,
		TestDefault:       DependencyDefaultAllow,
	})
	if err != nil {
		t.Fatalf("create second key: %v", err)
	}
	if first == second {
		t.Fatalf("cache keys are equal: %q", first)
	}
}

func dependencyPolicyFixtureGraph() Graph {
	return Graph{
		Components: []Component{
			{Identifier: "cmd/tool", Role: componentRoleApplication},
			{Identifier: "internal/library/report", Role: componentRoleLibrary},
			{Identifier: "internal/devtool/generator", Role: componentRoleDevelopment},
		},
		Relationships: []Relationship{
			{Source: "cmd/tool", Target: "internal/library/report"},
			{Source: "cmd/tool", Target: "internal/devtool/generator"},
		},
	}
}
