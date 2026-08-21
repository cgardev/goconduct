package main

import "testing"

func TestComponentClassifier_ClassifyCustomLayout(t *testing.T) {
	testCases := []struct {
		name        string
		path        string
		identifier  string
		component   string
		kind        componentKind
		application string
	}{
		{
			name:        "an application feature in a services layout",
			path:        "services/control/features/orders/usecase.go",
			identifier:  "services/control/features/orders",
			component:   "orders",
			kind:        componentKindApplicationModule,
			application: "control",
		},
		{
			name:        "an application root in a services layout",
			path:        "services/control/main.go",
			identifier:  "services/control",
			component:   "control",
			kind:        componentKindApplication,
			application: "control",
		},
		{
			name:       "a shared package in a packages layout",
			path:       "packages/telemetry/codec/codec.go",
			identifier: "packages/telemetry",
			component:  "telemetry",
			kind:       componentKindLibrary,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var classifier componentClassifier
			var descriptor componentDescriptor
			var modeled bool

			t.Run("Given component templates unrelated to the repository default layout", func(t *testing.T) {
				var err error
				classifier, err = newComponentClassifier(ComponentRules{
					Applications:       []string{"services/{application}"},
					ApplicationModules: []string{"services/{application}/features/{component}"},
					Libraries:          []string{"packages/{component}"},
				})
				if err != nil {
					t.Fatalf("newComponentClassifier failed: %v", err)
				}
			})

			t.Run("When the configured path is classified", func(t *testing.T) {
				descriptor, modeled = classifier.classify(testCase.path)
			})

			if !t.Run("Then the path is modeled", func(t *testing.T) {
				if !modeled {
					t.Fatal("custom path was not modeled")
				}
			}) {
				return
			}

			t.Run("And captures define its generic strategic identity", func(t *testing.T) {
				if descriptor.identifier != testCase.identifier ||
					descriptor.name != testCase.component ||
					descriptor.kind != testCase.kind ||
					descriptor.application != testCase.application {
					t.Errorf("unexpected descriptor: %+v", descriptor)
				}
			})
		})
	}
}

func TestComponentClassifier_RejectInvalidTemplates(t *testing.T) {
	testCases := []struct {
		name  string
		rules ComponentRules
	}{
		{name: "no component template is configured", rules: ComponentRules{}},
		{
			name:  "a component template is empty",
			rules: ComponentRules{Libraries: []string{""}},
		},
		{
			name:  "a component template has surrounding space",
			rules: ComponentRules{Libraries: []string{" packages/{component}"}},
		},
		{
			name:  "a component template is an absolute path",
			rules: ComponentRules{Libraries: []string{"/packages/{component}"}},
		},
		{
			name:  "a component template contains a backslash",
			rules: ComponentRules{Libraries: []string{`packages\{component}`}},
		},
		{
			name:  "an application template omits its application capture",
			rules: ComponentRules{Applications: []string{"services/control"}},
		},
		{
			name: "an application-module template omits its application capture",
			rules: ComponentRules{
				ApplicationModules: []string{"features/{component}"},
			},
		},
		{
			name:  "a template contains an unknown capture",
			rules: ComponentRules{Libraries: []string{"packages/{library}"}},
		},
		{
			name:  "a component capture has no closing brace",
			rules: ComponentRules{Libraries: []string{"packages/{component"}},
		},
		{
			name:  "a component capture has no opening brace",
			rules: ComponentRules{Libraries: []string{"packages/component}"}},
		},
		{
			name: "one template is assigned to two roles",
			rules: ComponentRules{
				Libraries:        []string{"packages/{component}"},
				DevelopmentTools: []string{"packages/{component}"},
			},
		},
		{
			name:  "a literal segment contains a wildcard",
			rules: ComponentRules{Libraries: []string{"pack*/{component}"}},
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var rules ComponentRules
			var classifierError error

			t.Run("Given the invalid component rule set", func(t *testing.T) {
				rules = testCase.rules
			})

			t.Run("When the component classifier is built", func(t *testing.T) {
				_, classifierError = newComponentClassifier(rules)
			})

			t.Run("Then startup rejects the ambiguous classification policy", func(t *testing.T) {
				if classifierError == nil {
					t.Fatal("invalid component templates were accepted")
				}
			})
		})
	}
}

func TestComponentPathRule_MatchStrictPrefix(t *testing.T) {
	t.Run("Scenario: A path has the same length as a component template", func(t *testing.T) {
		var rule componentPathRule
		var matches bool

		t.Run("Given a component rule and an exact matching path", func(t *testing.T) {
			rule = componentPathRule{
				kind: componentKindLibrary,
				segments: []componentPathSegment{
					{literal: "packages"},
					{capture: "component"},
				},
			}
		})

		t.Run("When the path is checked as a strict prefix", func(t *testing.T) {
			matches = rule.matchesStrictPrefix([]string{"packages", "telemetry"})
		})

		t.Run("Then the exact path is not a strict prefix", func(t *testing.T) {
			if matches {
				t.Error("an exact path was accepted as a strict prefix")
			}
		})
	})
}
