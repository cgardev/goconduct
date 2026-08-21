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
			var classified bool

			t.Run("Given component templates unrelated to the repository default layout", func(t *testing.T) {
				var err error
				classifier, err = newComponentClassifier(ComponentRules{
					Applications:       []string{"services/{application}"},
					ApplicationModules: []string{"services/{application}/features/{component}"},
					Libraries:          []string{"packages/{component}"},
				})
				if err != nil {
					t.Fatalf("newComponentClassifier fails: %v", err)
				}
			})

			t.Run("When the classifier classifies the configured path", func(t *testing.T) {
				descriptor, classified = classifier.classify(testCase.path)
			})

			if !t.Run("Then the classifier identifies the path", func(t *testing.T) {
				if !classified {
					t.Fatal("the classifier does not identify the custom path")
				}
			}) {
				return
			}

			t.Run("And placeholders define its generic component identity", func(t *testing.T) {
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
		{name: "the rule set has no component template", rules: ComponentRules{}},
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
			name:  "an application template omits its application placeholder",
			rules: ComponentRules{Applications: []string{"services/control"}},
		},
		{
			name: "an application-module template omits its application placeholder",
			rules: ComponentRules{
				ApplicationModules: []string{"features/{component}"},
			},
		},
		{
			name:  "a template contains an unknown placeholder",
			rules: ComponentRules{Libraries: []string{"packages/{library}"}},
		},
		{
			name:  "a component placeholder has no closing brace",
			rules: ComponentRules{Libraries: []string{"packages/{component"}},
		},
		{
			name:  "a component placeholder has no opening brace",
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

			t.Run("When the constructor builds the component classifier", func(t *testing.T) {
				_, classifierError = newComponentClassifier(rules)
			})

			t.Run("Then startup rejects the ambiguous classification policy", func(t *testing.T) {
				if classifierError == nil {
					t.Fatal("the classifier accepts invalid component templates")
				}
			})
		})
	}
}

func TestComponentPathRule_MatchStrictPrefix(t *testing.T) {
	t.Run("Scenario: A path has the same length as a component template", func(t *testing.T) {
		var rule componentPathRule
		var matches bool

		t.Run("Given a component rule and an exact path that matches", func(t *testing.T) {
			rule = componentPathRule{
				kind: componentKindLibrary,
				segments: []componentPathSegment{
					{literal: "packages"},
					{placeholder: "component"},
				},
			}
		})

		t.Run("When the rule checks the path as a strict prefix", func(t *testing.T) {
			matches = rule.matchesStrictPrefix([]string{"packages", "telemetry"})
		})

		t.Run("Then the rule rejects the exact path as a strict prefix", func(t *testing.T) {
			if matches {
				t.Error("the rule accepts an exact path as a strict prefix")
			}
		})
	})
}
