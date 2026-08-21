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
				return componentClassifier{}, fmt.Errorf(
					"the configuration assigns component template %q to both %s and %s",
					template,
					previousCategory,
					set.category,
				)
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
		return componentClassifier{}, fmt.Errorf("component rules must contain at least one path template")
	}
	return componentClassifier{rules: rules}, nil
}

func componentRuleSets(configuration ComponentRules) ([]componentRuleSet, error) {
	sets := make([]componentRuleSet, 0, len(configuration.Taxonomy)+6)
	categoryIdentifiers := make(stringSet)
	for _, category := range configuration.Taxonomy {
		if category.Identifier == "" || category.Identifier != strings.TrimSpace(category.Identifier) {
			return nil, fmt.Errorf(
				"component category identifier %q must be non-empty and have no surrounding spaces",
				category.Identifier,
			)
		}
		if categoryIdentifiers.contains(category.Identifier) {
			return nil, fmt.Errorf("the taxonomy repeats component category identifier %q", category.Identifier)
		}
		if !validComponentRole(category.Role) {
			return nil, fmt.Errorf("component category %q has unknown role %q", category.Identifier, category.Role)
		}
		if len(category.Paths) == 0 {
			return nil, fmt.Errorf(
				"component category %q must contain at least one path template",
				category.Identifier,
			)
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
		return componentPathRule{}, fmt.Errorf("component template %q has unknown role %q", template, role)
	}
	if template == "" || template != strings.TrimSpace(template) || strings.Contains(template, "\\") {
		return componentPathRule{}, fmt.Errorf(
			"component template %q must be a non-empty relative path that uses forward slashes "+
				"and has no surrounding spaces",
			template,
		)
	}
	pathSegments := strings.Split(template, "/")
	placeholders := make(stringSet)
	segments := make([]componentPathSegment, 0, len(pathSegments))
	for _, pathSegment := range pathSegments {
		if pathSegment == "" || pathSegment == "." || pathSegment == ".." {
			return componentPathRule{}, fmt.Errorf("component template %q contains an invalid path segment", template)
		}
		if strings.HasPrefix(pathSegment, "{") && strings.HasSuffix(pathSegment, "}") {
			placeholder := strings.TrimSuffix(strings.TrimPrefix(pathSegment, "{"), "}")
			if placeholder != "component" && placeholder != "application" {
				return componentPathRule{}, fmt.Errorf(
					"component template %q has unknown placeholder %q",
					template,
					placeholder,
				)
			}
			if placeholders.contains(placeholder) {
				return componentPathRule{}, fmt.Errorf(
					"component template %q repeats placeholder %q",
					template,
					placeholder,
				)
			}
			placeholders.add(placeholder)
			segments = append(segments, componentPathSegment{placeholder: placeholder})
			continue
		}
		if strings.ContainsAny(pathSegment, "{}*?[") {
			return componentPathRule{}, fmt.Errorf(
				"component template %q contains an invalid literal segment %q",
				template,
				pathSegment,
			)
		}
		segments = append(segments, componentPathSegment{literal: pathSegment})
	}
	if role == componentRoleApplication {
		if !placeholders.contains("application") {
			return componentPathRule{}, fmt.Errorf(
				"application template %q must contain {application}",
				template,
			)
		}
	} else if !placeholders.contains("component") {
		return componentPathRule{}, fmt.Errorf(
			"%s template %q must contain {component}",
			role,
			template,
		)
	}
	if role == componentRoleApplicationModule && !placeholders.contains("application") {
		return componentPathRule{}, fmt.Errorf(
			"application-module template %q must contain {application}",
			template,
		)
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
// {"version":1,"tested_at":"2026-08-21T17:14:46Z","module_hash":"f3d8cc9185d69ce4daa2d9cf7d55858118992c8efab27a0ee94dad43c3f21e9f","functions":[{"id":"func/newComponentClassifier","name":"newComponentClassifier","line":30,"end_line":59,"hash":"190ac0525d0997e26374f11c2822901a58e9c05be20e77590712af44dfea0bab"},{"id":"func/componentRuleSets","name":"componentRuleSets","line":61,"end_line":99,"hash":"28f2071ac184fad74e0dfc7a46118dc74e5069f1bb84e71a82d1a470cff8e4db"},{"id":"func/legacyComponentRuleSet","name":"legacyComponentRuleSet","line":101,"end_line":103,"hash":"5bfe3f74596ea8f2bdd3494c56d878208eb366d9943cb4c8cee2108a54637a95"},{"id":"func/compileComponentPathRule","name":"compileComponentPathRule","line":105,"end_line":177,"hash":"1ce50894eda2400c9067774531156636c62f38416be0496121a595f7eb01368b"},{"id":"func/componentClassifier.classify","name":"componentClassifier.classify","line":179,"end_line":206,"hash":"c7e20a967a01ce642ed2771e610a6bec4e5cab523fb89e673986b2cec3be4672"},{"id":"func/componentPathRule.matchesStrictPrefix","name":"componentPathRule.matchesStrictPrefix","line":208,"end_line":219,"hash":"eaac74b06884265a0619ba6a9e33672bbea72fd2ffb3abe369c1cd606d66f853"},{"id":"func/componentPathRule.matchPath","name":"componentPathRule.matchPath","line":221,"end_line":236,"hash":"5b3033d6322d6c69fa1b70f3e6a0bf24dd98b2b80349e3fb389b945d93861b8a"},{"id":"func/cloneComponentRules","name":"cloneComponentRules","line":238,"end_line":256,"hash":"98a87adfe2f93fe532a79d59a9102d9bbbf8b860505424296cde9948dcf345ea"},{"id":"func/pathWithForwardSlashes","name":"pathWithForwardSlashes","line":258,"end_line":260,"hash":"185c18d6c96a7ba28d3bc6dd03aa6b2e89feaf7e19dfd2e354cb7f5a3f3057c1"}]}
// mutate4go-manifest-end
