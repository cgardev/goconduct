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
	literal string
	capture string
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
		return componentPathRule{}, fmt.Errorf("component template %q must be a clean relative slash path", template)
	}
	parts := strings.Split(template, "/")
	captures := make(stringSet)
	segments := make([]componentPathSegment, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return componentPathRule{}, fmt.Errorf("component template %q contains an invalid path segment", template)
		}
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			capture := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			if capture != "component" && capture != "application" {
				return componentPathRule{}, fmt.Errorf(
					"component template %q has unknown capture %q",
					template,
					capture,
				)
			}
			if captures.contains(capture) {
				return componentPathRule{}, fmt.Errorf(
					"component template %q repeats capture %q",
					template,
					capture,
				)
			}
			captures.add(capture)
			segments = append(segments, componentPathSegment{capture: capture})
			continue
		}
		if strings.ContainsAny(part, "{}*?[") {
			return componentPathRule{}, fmt.Errorf(
				"component template %q contains an invalid literal segment %q",
				template,
				part,
			)
		}
		segments = append(segments, componentPathSegment{literal: part})
	}
	if kind == componentKindApplication {
		if !captures.contains("application") {
			return componentPathRule{}, fmt.Errorf(
				"application template %q must contain {application}",
				template,
			)
		}
	} else if !captures.contains("component") {
		return componentPathRule{}, fmt.Errorf(
			"%s template %q must contain {component}",
			kind,
			template,
		)
	}
	if kind == componentKindApplicationModule && !captures.contains("application") {
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
	cleaned := strings.Trim(filepathToSlash(relativePath), "/")
	if cleaned == "" {
		return componentDescriptor{}, false
	}
	parts := strings.Split(cleaned, "/")
	for _, rule := range classifier.rules {
		captures, matches := rule.match(parts)
		if !matches {
			if rule.kind != componentKindApplicationModule && rule.matchesStrictPrefix(parts) {
				return componentDescriptor{}, false
			}
			continue
		}
		name := captures["component"]
		if rule.kind == componentKindApplication {
			name = captures["application"]
		}
		return componentDescriptor{
			identifier:  strings.Join(parts[:len(rule.segments)], "/"),
			name:        name,
			kind:        rule.kind,
			application: captures["application"],
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

func (rule componentPathRule) match(parts []string) (map[string]string, bool) {
	if len(parts) < len(rule.segments) {
		return nil, false
	}
	captures := make(map[string]string, 2)
	for index, segment := range rule.segments {
		if segment.capture != "" {
			captures[segment.capture] = parts[index]
			continue
		}
		if segment.literal != parts[index] {
			return nil, false
		}
	}
	return captures, true
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

func filepathToSlash(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T07:56:57Z","module_hash":"91d4978782351c7181b9b3ff09f0d0b73bd44c078d6354095828ca3b9487c5db","functions":[{"id":"func/newComponentClassifier","name":"newComponentClassifier","line":28,"end_line":61,"hash":"2f3890a29459eb73eecf10f8f08b392fbafca6366b8e8fde586380a46276eeb7"},{"id":"func/compileComponentPathRule","name":"compileComponentPathRule","line":63,"end_line":127,"hash":"b691dac138dc95a41afda07d4998d99acea170f502f5c37d2ae3a89c02e5ba6c"},{"id":"func/validComponentKind","name":"validComponentKind","line":129,"end_line":141,"hash":"43490af2e3b20e1bb6268e5d13feac6ce30f8aa47899004ad8912f213ddb7b81"},{"id":"func/componentClassifier.classify","name":"componentClassifier.classify","line":143,"end_line":169,"hash":"bb8cf40efed195878240aa6faa935c17cd007398d3a2917ec14e5c2d5f49f881"},{"id":"func/componentPathRule.matchesStrictPrefix","name":"componentPathRule.matchesStrictPrefix","line":171,"end_line":182,"hash":"d3bd58f10c7de5419efb0626764d4129c01b382fbec5fb977b8e61a8b64c2fac"},{"id":"func/componentPathRule.match","name":"componentPathRule.match","line":184,"end_line":199,"hash":"9fc76f2c5a0e4093d607058740a142c547f65ea6463d0b565218d37cd40daff0"},{"id":"func/cloneComponentRules","name":"cloneComponentRules","line":201,"end_line":210,"hash":"286483ba1b5c1b9926ecaf8a47b5ee67ae1f108127dd95c2cd3dd219ef842bdc"},{"id":"func/filepathToSlash","name":"filepathToSlash","line":212,"end_line":214,"hash":"43d2179e245f7cc730af531005b9ec6b0a14acb41e46edc8b692bc4945b1f35d"}]}
// mutate4go-manifest-end
