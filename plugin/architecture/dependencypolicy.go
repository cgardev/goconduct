package architecture

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cgardev/goconduct/failure"
	"github.com/cgardev/goconduct/internal/application"
)

const configuredDependencyRule = "configured-dependency"

type dependencyPolicyGraphSourceFactory struct {
	base   graphSourceFactory
	policy DependencyPolicyConfiguration
}

func newDependencyPolicyGraphSourceFactory(
	base graphSourceFactory,
	policy DependencyPolicyConfiguration,
) graphSourceFactory {
	return dependencyPolicyGraphSourceFactory{base: base, policy: cloneDependencyPolicy(policy)}
}

func (factory dependencyPolicyGraphSourceFactory) NewSource(
	configuration AnalysisConfiguration,
) (application.GraphSource[Graph], error) {
	source, err := factory.base.NewSource(configuration)
	if err != nil {
		return nil, err
	}
	return dependencyPolicyGraphSource{GraphSource: source, policy: factory.policy}, nil
}

func (factory dependencyPolicyGraphSourceFactory) NewMonitorSource(
	configuration AnalysisConfiguration,
) (graphMonitorSource, error) {
	source, err := factory.base.NewMonitorSource(configuration)
	if err != nil {
		return nil, err
	}
	return dependencyPolicyMonitorSource{graphMonitorSource: source, policy: factory.policy}, nil
}

type dependencyPolicyGraphSource struct {
	application.GraphSource[Graph]
	policy DependencyPolicyConfiguration
}

func (source dependencyPolicyGraphSource) Analyze(ctx context.Context) (Graph, error) {
	graph, err := source.GraphSource.Analyze(ctx)
	if err != nil {
		return Graph{}, err
	}
	if err := applyDependencyPolicy(&graph, source.policy); err != nil {
		return Graph{}, err
	}
	return graph, nil
}

func (source dependencyPolicyGraphSource) CacheKey() (string, error) {
	baseKey, err := source.GraphSource.CacheKey()
	if err != nil {
		return "", err
	}
	return dependencyPolicyCacheKey(baseKey, source.policy)
}

type dependencyPolicyMonitorSource struct {
	graphMonitorSource
	policy DependencyPolicyConfiguration
}

func (source dependencyPolicyMonitorSource) analyze(ctx context.Context) (Graph, error) {
	graph, err := source.graphMonitorSource.analyze(ctx)
	if err != nil {
		return Graph{}, err
	}
	if err := applyDependencyPolicy(&graph, source.policy); err != nil {
		return Graph{}, err
	}
	return graph, nil
}

func (source dependencyPolicyMonitorSource) graphCacheKey() (string, error) {
	baseKey, err := source.graphMonitorSource.graphCacheKey()
	if err != nil {
		return "", err
	}
	return dependencyPolicyCacheKey(baseKey, source.policy)
}

func dependencyPolicyCacheKey(baseKey string, policy DependencyPolicyConfiguration) (string, error) {
	policy = cloneDependencyPolicy(policy)
	if dependencyPolicyIsNoop(policy) {
		return baseKey, nil
	}
	payload, err := json.Marshal(struct {
		BaseKey string                        `json:"baseKey"`
		Policy  DependencyPolicyConfiguration `json:"policy"`
	}{BaseKey: baseKey, Policy: policy})
	if err != nil {
		return "", failure.Internal("encode dependency policy cache identity", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func applyDependencyPolicy(graph *Graph, policy DependencyPolicyConfiguration) error {
	policy = cloneDependencyPolicy(policy)
	if err := validateDependencyPolicy(policy); err != nil {
		return err
	}
	if dependencyPolicyIsNoop(policy) {
		return nil
	}
	if err := validateDependencyRuleReferences(graph.Components, policy); err != nil {
		return err
	}
	components := make(map[string]Component, len(graph.Components))
	for _, component := range graph.Components {
		components[component.Identifier] = component
	}
	for index := range graph.Relationships {
		relationship := &graph.Relationships[index]
		source, sourceExists := components[relationship.Source]
		target, targetExists := components[relationship.Target]
		if !sourceExists || !targetExists {
			return failure.Internal(
				fmt.Sprintf("resolve configured dependency %s -> %s", relationship.Source, relationship.Target),
				nil,
			)
		}
		denial, denied := firstMatchingDependencyRule(policy.Deny, source, target)
		allowedByDefault := policy.ProductionDefault == DependencyDefaultAllow
		if relationship.TestOnly {
			allowedByDefault = policy.TestDefault == DependencyDefaultAllow
		}
		_, granted := firstMatchingDependencyRule(policy.Allow, source, target)
		if !denied && (allowedByDefault || granted) {
			continue
		}
		relationship.RuleViolations = append(
			slices.Clone(relationship.RuleViolations),
			configuredDependencyRule,
		)
		slices.Sort(relationship.RuleViolations)
		relationship.RuleViolations = slices.Compact(relationship.RuleViolations)
		message := "The configured dependency policy does not grant this relationship."
		if denied {
			message = fmt.Sprintf(
				"Dependency rule %q prohibits this relationship: %s",
				denial.Identifier,
				denial.Reason,
			)
		}
		graph.Findings = append(graph.Findings, Finding{
			Rule: configuredDependencyRule, Severity: findingSeverityError,
			Subject: relationship.Source + " -> " + relationship.Target,
			Message: message, Source: relationship.Source, Target: relationship.Target,
		})
	}
	slices.SortFunc(graph.Findings, func(left, right Finding) int {
		return cmp.Or(
			strings.Compare(left.Rule, right.Rule),
			strings.Compare(left.Subject, right.Subject),
			strings.Compare(left.Source, right.Source),
			strings.Compare(left.Target, right.Target),
		)
	})
	graph.Summary = summarizeGraph(*graph)
	return refreshGraphRevision(graph)
}

func dependencyPolicyIsNoop(policy DependencyPolicyConfiguration) bool {
	return policy.ProductionDefault == DependencyDefaultAllow &&
		policy.TestDefault == DependencyDefaultAllow &&
		len(policy.Allow) == 0 &&
		len(policy.Deny) == 0
}

func refreshGraphRevision(graph *Graph) error {
	graph.Revision = ""
	payload, err := json.Marshal(graph)
	if err != nil {
		return failure.Internal("encode configured graph revision input", err)
	}
	digest := sha256.Sum256(payload)
	graph.Revision = hex.EncodeToString(digest[:])
	return nil
}

func validateDependencyPolicy(policy DependencyPolicyConfiguration) error {
	if err := validateDependencyDefault("production", policy.ProductionDefault); err != nil {
		return err
	}
	if err := validateDependencyDefault("test", policy.TestDefault); err != nil {
		return err
	}
	identifiers := make(map[string]struct{}, len(policy.Allow)+len(policy.Deny))
	for _, group := range [][]DependencyRuleConfiguration{policy.Allow, policy.Deny} {
		for _, rule := range group {
			if strings.TrimSpace(rule.Identifier) == "" || strings.TrimSpace(rule.Identifier) != rule.Identifier {
				return failure.Validation(fmt.Sprintf("dependency rule identifier %q is invalid", rule.Identifier), nil)
			}
			if _, duplicate := identifiers[rule.Identifier]; duplicate {
				return failure.Validation(fmt.Sprintf("dependency rule %q is duplicated", rule.Identifier), nil)
			}
			identifiers[rule.Identifier] = struct{}{}
			if strings.TrimSpace(rule.Reason) == "" {
				return failure.Validation(fmt.Sprintf("dependency rule %q reason is empty", rule.Identifier), nil)
			}
			if err := validateComponentSelector(rule.From); err != nil {
				return failure.Validation(fmt.Sprintf("dependency rule %q source selector", rule.Identifier), err)
			}
			if err := validateComponentSelector(rule.To); err != nil {
				return failure.Validation(fmt.Sprintf("dependency rule %q target selector", rule.Identifier), err)
			}
		}
	}
	return nil
}

func validateDependencyDefault(kind string, value DependencyDefault) error {
	if value != "" && value != DependencyDefaultAllow && value != DependencyDefaultDeny {
		return failure.Validation(fmt.Sprintf("%s dependency default %q is invalid", kind, value), nil)
	}
	return nil
}

func validateComponentSelector(selector ComponentSelectorConfiguration) error {
	if len(selector.Identifiers)+len(selector.Roles)+len(selector.Categories)+len(selector.Applications) == 0 {
		return failure.Validation("selector is empty", nil)
	}
	if err := validateSelectorStrings("identifier", selector.Identifiers); err != nil {
		return err
	}
	if err := validateSelectorStrings("category", selector.Categories); err != nil {
		return err
	}
	if err := validateSelectorStrings("application", selector.Applications); err != nil {
		return err
	}
	roles := make(map[ComponentRole]struct{}, len(selector.Roles))
	for _, role := range selector.Roles {
		if !validComponentRole(role) {
			return failure.Validation(fmt.Sprintf("role %q is invalid", role), nil)
		}
		if _, duplicate := roles[role]; duplicate {
			return failure.Validation(fmt.Sprintf("role %q is duplicated", role), nil)
		}
		roles[role] = struct{}{}
	}
	return nil
}

func validateSelectorStrings(kind string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return failure.Validation(fmt.Sprintf("%s %q is invalid", kind, value), nil)
		}
		if _, duplicate := seen[value]; duplicate {
			return failure.Validation(fmt.Sprintf("%s %q is duplicated", kind, value), nil)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateDependencyRuleReferences(
	components []Component,
	policy DependencyPolicyConfiguration,
) error {
	for _, group := range [][]DependencyRuleConfiguration{policy.Allow, policy.Deny} {
		for _, rule := range group {
			if !selectorMatchesAny(rule.From, components) {
				return failure.Validation(fmt.Sprintf(
					"dependency rule %q source selector matches no components",
					rule.Identifier,
				), nil)
			}
			if !selectorMatchesAny(rule.To, components) {
				return failure.Validation(fmt.Sprintf(
					"dependency rule %q target selector matches no components",
					rule.Identifier,
				), nil)
			}
		}
	}
	return nil
}

func selectorMatchesAny(selector ComponentSelectorConfiguration, components []Component) bool {
	for _, component := range components {
		if componentSelectorMatches(selector, component) {
			return true
		}
	}
	return false
}

func firstMatchingDependencyRule(
	rules []DependencyRuleConfiguration,
	source Component,
	target Component,
) (DependencyRuleConfiguration, bool) {
	for _, rule := range rules {
		if !componentSelectorMatches(rule.From, source) || !componentSelectorMatches(rule.To, target) {
			continue
		}
		if rule.SameApplication && (source.Application == "" || source.Application != target.Application) {
			continue
		}
		return rule, true
	}
	return DependencyRuleConfiguration{}, false
}

func componentSelectorMatches(selector ComponentSelectorConfiguration, component Component) bool {
	return matchesStringSelector(selector.Identifiers, component.Identifier) &&
		matchesRoleSelector(selector.Roles, component.Role) &&
		matchesStringSelector(selector.Categories, component.Category) &&
		matchesStringSelector(selector.Applications, component.Application)
}

func matchesStringSelector(values []string, candidate string) bool {
	return len(values) == 0 || slices.Contains(values, candidate)
}

func matchesRoleSelector(values []ComponentRole, candidate ComponentRole) bool {
	return len(values) == 0 || slices.Contains(values, candidate)
}

func cloneDependencyPolicy(policy DependencyPolicyConfiguration) DependencyPolicyConfiguration {
	result := DependencyPolicyConfiguration{
		ProductionDefault: policy.ProductionDefault,
		TestDefault:       policy.TestDefault,
		Allow:             cloneDependencyRules(policy.Allow),
		Deny:              cloneDependencyRules(policy.Deny),
	}
	if result.ProductionDefault == "" {
		result.ProductionDefault = DependencyDefaultAllow
	}
	if result.TestDefault == "" {
		result.TestDefault = DependencyDefaultAllow
	}
	return result
}

func cloneDependencyRules(rules []DependencyRuleConfiguration) []DependencyRuleConfiguration {
	result := make([]DependencyRuleConfiguration, 0, len(rules))
	for _, rule := range rules {
		result = append(result, DependencyRuleConfiguration{
			Identifier: rule.Identifier, SameApplication: rule.SameApplication, Reason: rule.Reason,
			From: cloneComponentSelector(rule.From), To: cloneComponentSelector(rule.To),
		})
	}
	return result
}

func cloneComponentSelector(selector ComponentSelectorConfiguration) ComponentSelectorConfiguration {
	return ComponentSelectorConfiguration{
		Identifiers: slices.Clone(selector.Identifiers), Roles: slices.Clone(selector.Roles),
		Categories: slices.Clone(selector.Categories), Applications: slices.Clone(selector.Applications),
	}
}
