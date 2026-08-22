package main

import (
	"fmt"
	"slices"
	"strings"
)

type componentClassifier struct {
	rules []componentPathRule
}

type componentPathRule struct {
	role     componentRole
	category string
	segments []componentPathSegment
}

type componentPathSegment struct {
	literal     string
	placeholder string
}

type componentRuleSet struct {
	role      componentRole
	category  string
	templates []string
}

func newComponentClassifier(configuration ComponentRules) (componentClassifier, error) {
	sets, err := componentRuleSets(configuration)
	if err != nil {
		return componentClassifier{}, err
	}
	seen := make(map[string]string)
	var rules []componentPathRule
	for _, set := range sets {
		for _, template := range set.templates {
			if previousCategory, exists := seen[template]; exists {
				return componentClassifier{}, newValidationError(fmt.Sprintf(
					"the configuration assigns component template %q to both %s and %s",
					template,
					previousCategory,
					set.category,
				), nil)
			}
			rule, err := compileComponentPathRule(template, set.role, set.category)
			if err != nil {
				return componentClassifier{}, err
			}
			seen[template] = set.category
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 {
		return componentClassifier{}, newValidationError(
			"component rules must contain at least one path template",
			nil,
		)
	}
	return componentClassifier{rules: rules}, nil
}

func componentRuleSets(configuration ComponentRules) ([]componentRuleSet, error) {
	sets := make([]componentRuleSet, 0, len(configuration.Taxonomy)+6)
	categoryIdentifiers := make(stringSet)
	for _, category := range configuration.Taxonomy {
		if category.Identifier == "" || category.Identifier != strings.TrimSpace(category.Identifier) {
			return nil, newValidationError(fmt.Sprintf(
				"component category identifier %q must be non-empty and have no surrounding spaces",
				category.Identifier,
			), nil)
		}
		if categoryIdentifiers.contains(category.Identifier) {
			return nil, newValidationError(
				fmt.Sprintf("the taxonomy repeats component category identifier %q", category.Identifier),
				nil,
			)
		}
		if !validComponentRole(category.Role) {
			return nil, newValidationError(
				fmt.Sprintf("component category %q has unknown role %q", category.Identifier, category.Role),
				nil,
			)
		}
		if len(category.Paths) == 0 {
			return nil, newValidationError(fmt.Sprintf(
				"component category %q must contain at least one path template",
				category.Identifier,
			), nil)
		}
		categoryIdentifiers.add(category.Identifier)
		sets = append(sets, componentRuleSet{
			role:      category.Role,
			category:  category.Identifier,
			templates: category.Paths,
		})
	}
	sets = append(sets,
		legacyComponentRuleSet(componentRoleApplicationModule, configuration.ApplicationModules),
		legacyComponentRuleSet(componentRoleSharedModule, configuration.SharedModules),
		legacyComponentRuleSet(componentRoleLibrary, configuration.Libraries),
		legacyComponentRuleSet(componentRoleDevelopment, configuration.DevelopmentTools),
		legacyComponentRuleSet(componentRoleInfrastructure, configuration.Infrastructure),
		legacyComponentRuleSet(componentRoleApplication, configuration.Applications),
	)
	return sets, nil
}

func legacyComponentRuleSet(role componentRole, templates []string) componentRuleSet {
	return componentRuleSet{role: role, category: string(role), templates: templates}
}

func compileComponentPathRule(
	template string,
	role componentRole,
	category string,
) (componentPathRule, error) {
	if !validComponentRole(role) {
		return componentPathRule{}, newValidationError(
			fmt.Sprintf("component template %q has unknown role %q", template, role),
			nil,
		)
	}
	if template == "" || template != strings.TrimSpace(template) || strings.Contains(template, "\\") {
		return componentPathRule{}, newValidationError(fmt.Sprintf(
			"component template %q must be a non-empty relative path that uses forward slashes "+
				"and has no surrounding spaces",
			template,
		), nil)
	}
	pathSegments := strings.Split(template, "/")
	placeholders := make(stringSet)
	segments := make([]componentPathSegment, 0, len(pathSegments))
	for _, pathSegment := range pathSegments {
		if pathSegment == "" || pathSegment == "." || pathSegment == ".." {
			return componentPathRule{}, newValidationError(
				fmt.Sprintf("component template %q contains an invalid path segment", template),
				nil,
			)
		}
		if strings.HasPrefix(pathSegment, "{") && strings.HasSuffix(pathSegment, "}") {
			placeholder := strings.TrimSuffix(strings.TrimPrefix(pathSegment, "{"), "}")
			if placeholder != "component" && placeholder != "application" {
				return componentPathRule{}, newValidationError(fmt.Sprintf(
					"component template %q has unknown placeholder %q",
					template,
					placeholder,
				), nil)
			}
			if placeholders.contains(placeholder) {
				return componentPathRule{}, newValidationError(fmt.Sprintf(
					"component template %q repeats placeholder %q",
					template,
					placeholder,
				), nil)
			}
			placeholders.add(placeholder)
			segments = append(segments, componentPathSegment{placeholder: placeholder})
			continue
		}
		if strings.ContainsAny(pathSegment, "{}*?[") {
			return componentPathRule{}, newValidationError(fmt.Sprintf(
				"component template %q contains an invalid literal segment %q",
				template,
				pathSegment,
			), nil)
		}
		segments = append(segments, componentPathSegment{literal: pathSegment})
	}
	if role == componentRoleApplication {
		if !placeholders.contains("application") {
			return componentPathRule{}, newValidationError(fmt.Sprintf(
				"application template %q must contain {application}",
				template,
			), nil)
		}
	} else if !placeholders.contains("component") {
		return componentPathRule{}, newValidationError(fmt.Sprintf(
			"%s template %q must contain {component}",
			role,
			template,
		), nil)
	}
	if role == componentRoleApplicationModule && !placeholders.contains("application") {
		return componentPathRule{}, newValidationError(fmt.Sprintf(
			"application-module template %q must contain {application}",
			template,
		), nil)
	}
	return componentPathRule{role: role, category: category, segments: segments}, nil
}

func (classifier componentClassifier) classify(relativePath string) (componentDescriptor, bool) {
	normalizedPath := strings.Trim(pathWithForwardSlashes(relativePath), "/")
	if normalizedPath == "" {
		return componentDescriptor{}, false
	}
	pathSegments := strings.Split(normalizedPath, "/")
	for _, rule := range classifier.rules {
		placeholderValues, ruleMatches := rule.matchPath(pathSegments)
		if !ruleMatches {
			if rule.role != componentRoleApplicationModule && rule.matchesStrictPrefix(pathSegments) {
				return componentDescriptor{}, false
			}
			continue
		}
		componentName := placeholderValues["component"]
		if rule.role == componentRoleApplication {
			componentName = placeholderValues["application"]
		}
		return componentDescriptor{
			identifier:  strings.Join(pathSegments[:len(rule.segments)], "/"),
			name:        componentName,
			role:        rule.role,
			category:    rule.category,
			application: placeholderValues["application"],
		}, true
	}
	return componentDescriptor{}, false
}

func (rule componentPathRule) matchesStrictPrefix(pathSegments []string) bool {
	if len(pathSegments) >= len(rule.segments) {
		return false
	}
	for index, pathSegment := range pathSegments {
		segment := rule.segments[index]
		if segment.literal != "" && segment.literal != pathSegment {
			return false
		}
	}
	return true
}

func (rule componentPathRule) matchPath(pathSegments []string) (map[string]string, bool) {
	if len(pathSegments) < len(rule.segments) {
		return nil, false
	}
	placeholderValues := make(map[string]string, 2)
	for index, segment := range rule.segments {
		if segment.placeholder != "" {
			placeholderValues[segment.placeholder] = pathSegments[index]
			continue
		}
		if segment.literal != pathSegments[index] {
			return nil, false
		}
	}
	return placeholderValues, true
}

func cloneComponentRules(rules ComponentRules) ComponentRules {
	taxonomy := []ComponentCategoryRule{}
	for _, category := range rules.Taxonomy {
		taxonomy = append(taxonomy, ComponentCategoryRule{
			Identifier: category.Identifier,
			Role:       category.Role,
			Paths:      slices.Clone(category.Paths),
		})
	}
	return ComponentRules{
		Applications:       slices.Clone(rules.Applications),
		ApplicationModules: slices.Clone(rules.ApplicationModules),
		SharedModules:      slices.Clone(rules.SharedModules),
		Libraries:          slices.Clone(rules.Libraries),
		Infrastructure:     slices.Clone(rules.Infrastructure),
		DevelopmentTools:   slices.Clone(rules.DevelopmentTools),
		Taxonomy:           taxonomy,
	}
}

func pathWithForwardSlashes(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T18:18:14Z","module_hash":"186852beb2a68f4188c28ec34e7db0d9a3c8f8261c1bc8360376d2605aa1295e","functions":[{"id":"func/newComponentClassifier","name":"newComponentClassifier","line":30,"end_line":62,"hash":"2eab3865d7597af8980b9db91830018bb10d6c6c0e40487f965e28708aac33ee"},{"id":"func/componentRuleSets","name":"componentRuleSets","line":64,"end_line":108,"hash":"b9cba9a801c7dff230e2ac8a07a68b35059ca814ae31d9aa58dbc6bee85006aa"},{"id":"func/legacyComponentRuleSet","name":"legacyComponentRuleSet","line":110,"end_line":112,"hash":"5bfe3f74596ea8f2bdd3494c56d878208eb366d9943cb4c8cee2108a54637a95"},{"id":"func/compileComponentPathRule","name":"compileComponentPathRule","line":114,"end_line":192,"hash":"9131a31e331de114a967230f49ffad0ed2dbaee8833bc95a168a3a930f8d0cab"},{"id":"func/componentClassifier.classify","name":"componentClassifier.classify","line":194,"end_line":221,"hash":"c7e20a967a01ce642ed2771e610a6bec4e5cab523fb89e673986b2cec3be4672"},{"id":"func/componentPathRule.matchesStrictPrefix","name":"componentPathRule.matchesStrictPrefix","line":223,"end_line":234,"hash":"eaac74b06884265a0619ba6a9e33672bbea72fd2ffb3abe369c1cd606d66f853"},{"id":"func/componentPathRule.matchPath","name":"componentPathRule.matchPath","line":236,"end_line":251,"hash":"5b3033d6322d6c69fa1b70f3e6a0bf24dd98b2b80349e3fb389b945d93861b8a"},{"id":"func/cloneComponentRules","name":"cloneComponentRules","line":253,"end_line":271,"hash":"98a87adfe2f93fe532a79d59a9102d9bbbf8b860505424296cde9948dcf345ea"},{"id":"func/pathWithForwardSlashes","name":"pathWithForwardSlashes","line":273,"end_line":275,"hash":"185c18d6c96a7ba28d3bc6dd03aa6b2e89feaf7e19dfd2e354cb7f5a3f3057c1"}]}
// mutate4go-manifest-end
