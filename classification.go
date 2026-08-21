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
	kind     componentKind
	segments []componentPathSegment
}

type componentPathSegment struct {
	literal     string
	placeholder string
}

type componentRuleSet struct {
	kind      componentKind
	templates []string
}

func newComponentClassifier(configuration ComponentRules) (componentClassifier, error) {
	sets := []componentRuleSet{
		{kind: componentKindApplicationModule, templates: configuration.ApplicationModules},
		{kind: componentKindSharedModule, templates: configuration.SharedModules},
		{kind: componentKindLibrary, templates: configuration.Libraries},
		{kind: componentKindDevelopment, templates: configuration.DevelopmentTools},
		{kind: componentKindInfrastructure, templates: configuration.Infrastructure},
		{kind: componentKindApplication, templates: configuration.Applications},
	}
	seen := make(map[string]componentKind)
	var rules []componentPathRule
	for _, set := range sets {
		for _, template := range set.templates {
			if previousKind, exists := seen[template]; exists {
				return componentClassifier{}, fmt.Errorf(
					"component template %q is assigned to both %s and %s",
					template,
					previousKind,
					set.kind,
				)
			}
			rule, err := compileComponentPathRule(template, set.kind)
			if err != nil {
				return componentClassifier{}, err
			}
			seen[template] = set.kind
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 {
		return componentClassifier{}, fmt.Errorf("component rules must contain at least one path template")
	}
	return componentClassifier{rules: rules}, nil
}

func compileComponentPathRule(template string, kind componentKind) (componentPathRule, error) {
	if !validComponentKind(kind) {
		return componentPathRule{}, fmt.Errorf("component template %q has unknown kind %q", template, kind)
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
	if kind == componentKindApplication {
		if !placeholders.contains("application") {
			return componentPathRule{}, fmt.Errorf(
				"application template %q must contain {application}",
				template,
			)
		}
	} else if !placeholders.contains("component") {
		return componentPathRule{}, fmt.Errorf(
			"%s template %q must contain {component}",
			kind,
			template,
		)
	}
	if kind == componentKindApplicationModule && !placeholders.contains("application") {
		return componentPathRule{}, fmt.Errorf(
			"application-module template %q must contain {application}",
			template,
		)
	}
	return componentPathRule{kind: kind, segments: segments}, nil
}

func validComponentKind(kind componentKind) bool {
	switch kind {
	case componentKindApplication,
		componentKindApplicationModule,
		componentKindSharedModule,
		componentKindLibrary,
		componentKindInfrastructure,
		componentKindDevelopment:
		return true
	default:
		return false
	}
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
			if rule.kind != componentKindApplicationModule && rule.matchesStrictPrefix(pathSegments) {
				return componentDescriptor{}, false
			}
			continue
		}
		componentName := placeholderValues["component"]
		if rule.kind == componentKindApplication {
			componentName = placeholderValues["application"]
		}
		return componentDescriptor{
			identifier:  strings.Join(pathSegments[:len(rule.segments)], "/"),
			name:        componentName,
			kind:        rule.kind,
			application: placeholderValues["application"],
		}, true
	}
	return componentDescriptor{}, false
}

func (rule componentPathRule) matchesStrictPrefix(parts []string) bool {
	if len(parts) >= len(rule.segments) {
		return false
	}
	for index, part := range parts {
		segment := rule.segments[index]
		if segment.literal != "" && segment.literal != part {
			return false
		}
	}
	return true
}

func (rule componentPathRule) matchPath(parts []string) (map[string]string, bool) {
	if len(parts) < len(rule.segments) {
		return nil, false
	}
	placeholderValues := make(map[string]string, 2)
	for index, segment := range rule.segments {
		if segment.placeholder != "" {
			placeholderValues[segment.placeholder] = parts[index]
			continue
		}
		if segment.literal != parts[index] {
			return nil, false
		}
	}
	return placeholderValues, true
}

func cloneComponentRules(rules ComponentRules) ComponentRules {
	return ComponentRules{
		Applications:       slices.Clone(rules.Applications),
		ApplicationModules: slices.Clone(rules.ApplicationModules),
		SharedModules:      slices.Clone(rules.SharedModules),
		Libraries:          slices.Clone(rules.Libraries),
		Infrastructure:     slices.Clone(rules.Infrastructure),
		DevelopmentTools:   slices.Clone(rules.DevelopmentTools),
	}
}

func pathWithForwardSlashes(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T09:14:48Z","module_hash":"56740936a66fc7c6535d1d981444389c2e35b2e60a9743aa0295a0932cebb207","functions":[{"id":"func/newComponentClassifier","name":"newComponentClassifier","line":28,"end_line":61,"hash":"2f3890a29459eb73eecf10f8f08b392fbafca6366b8e8fde586380a46276eeb7"},{"id":"func/compileComponentPathRule","name":"compileComponentPathRule","line":63,"end_line":131,"hash":"2da5cb6fb77afa463471c5bcba7a5e40cf82377bc86ef1dba9c7856286a890a4"},{"id":"func/validComponentKind","name":"validComponentKind","line":133,"end_line":145,"hash":"43490af2e3b20e1bb6268e5d13feac6ce30f8aa47899004ad8912f213ddb7b81"},{"id":"func/componentClassifier.classify","name":"componentClassifier.classify","line":147,"end_line":173,"hash":"0b35c10d328cf919bfb0ae70c7efa1a2c2320bda8f06d4717c0b8c047b7b6fd6"},{"id":"func/componentPathRule.matchesStrictPrefix","name":"componentPathRule.matchesStrictPrefix","line":175,"end_line":186,"hash":"d3bd58f10c7de5419efb0626764d4129c01b382fbec5fb977b8e61a8b64c2fac"},{"id":"func/componentPathRule.matchPath","name":"componentPathRule.matchPath","line":188,"end_line":203,"hash":"df87f4925f00c7dcb9f17f18ed07a7886df9ee71a0e11655d29238cd2a034146"},{"id":"func/cloneComponentRules","name":"cloneComponentRules","line":205,"end_line":214,"hash":"286483ba1b5c1b9926ecaf8a47b5ee67ae1f108127dd95c2cd3dd219ef842bdc"},{"id":"func/pathWithForwardSlashes","name":"pathWithForwardSlashes","line":216,"end_line":218,"hash":"185c18d6c96a7ba28d3bc6dd03aa6b2e89feaf7e19dfd2e354cb7f5a3f3057c1"}]}
// mutate4go-manifest-end
