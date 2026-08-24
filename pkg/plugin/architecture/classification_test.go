package architecture

import "testing"

func TestComponentClassifier_ClassifyCustomLayout(t *testing.T) {
	testCases := []struct {
		name        string
		path        string
		identifier  string
		component   string
		role        componentRole
		application string
	}{
		{
			name:        "an application feature in a services layout",
			path:        "services/control/features/orders/usecase.go",
			identifier:  "services/control/features/orders",
			component:   "orders",
			role:        componentRoleApplicationModule,
			application: "control",
		},
		{
			name:        "an application root in a services layout",
			path:        "services/control/main.go",
			identifier:  "services/control",
			component:   "control",
			role:        componentRoleApplication,
			application: "control",
		},
		{
			name:        "an application root directory in a services layout",
			path:        "services/control",
			identifier:  "services/control",
			component:   "control",
			role:        componentRoleApplication,
			application: "control",
		},
		{
			name:       "a shared package in a packages layout",
			path:       "packages/telemetry/codec/codec.go",
			identifier: "packages/telemetry",
			component:  "telemetry",
			role:       componentRoleLibrary,
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
					descriptor.role != testCase.role ||
					descriptor.application != testCase.application {
					t.Errorf("unexpected descriptor: %+v", descriptor)
				}
			})
		})
	}
}

func TestComponentRuleSets_PreserveConfiguredTaxonomy(t *testing.T) {
	t.Run("Scenario: One custom category extends the six compatible roles", func(t *testing.T) {
		var configuration ComponentRules
		var sets []componentRuleSet
		var setError error

		t.Run("Given one custom category rule", func(*testing.T) {
			configuration = ComponentRules{Taxonomy: []ComponentCategoryRule{{
				Identifier: "plugin",
				Role:       componentRoleLibrary,
				Paths:      []string{"plugins/{component}"},
			}}}
		})

		t.Run("When the classifier creates the ordered rule sets", func(*testing.T) {
			sets, setError = componentRuleSets(configuration)
		})

		t.Run("Then the custom category precedes exactly six compatible role sets", func(t *testing.T) {
			if setError != nil {
				t.Fatalf("componentRuleSets fails: %v", setError)
			}
			if len(sets) != 7 || sets[0].category != "plugin" {
				t.Fatalf("component rule sets are %+v", sets)
			}
		})
	})
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
			name: "the configuration assigns one template to two roles",
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

func TestComponentClassifier_ClassifyExactPackage(t *testing.T) {
	t.Run("Scenario: A public SDK package has no variable component segment", func(t *testing.T) {
		var classifier componentClassifier
		var descriptor componentDescriptor
		var classified bool

		t.Run("Given one exact package template and one nested component template", func(t *testing.T) {
			var err error
			classifier, err = newComponentClassifier(ComponentRules{Libraries: []string{
				"plugin",
				"plugin/{component}",
			}})
			if err != nil {
				t.Fatalf("newComponentClassifier fails: %v", err)
			}
		})

		t.Run("When the classifier inspects a source file in the exact package", func(*testing.T) {
			descriptor, classified = classifier.classify("plugin/catalog.go")
		})

		t.Run("Then it uses the final literal as the component name", func(t *testing.T) {
			if !classified || descriptor.identifier != "plugin" || descriptor.name != "plugin" {
				t.Fatalf("unexpected exact package descriptor: %+v", descriptor)
			}
		})

		t.Run("And the exact rule does not consume nested plugin packages", func(t *testing.T) {
			descriptor, classified = classifier.classify("plugin/coverage/evaluator.go")
			if !classified || descriptor.identifier != "plugin/coverage" || descriptor.name != "coverage" {
				t.Fatalf("unexpected nested package descriptor: %+v", descriptor)
			}
		})
	})
}

func TestComponentPathRule_MatchStrictPrefix(t *testing.T) {
	t.Run("Scenario: A path has the same length as a component template", func(t *testing.T) {
		var rule componentPathRule
		var matches bool

		t.Run("Given a component rule and an exact path that matches", func(t *testing.T) {
			rule = componentPathRule{
				role: componentRoleLibrary,
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

	t.Run("Scenario: A shorter path matches the literal part of a component template", func(t *testing.T) {
		var rule componentPathRule
		var matches bool

		t.Run("Given a component rule with one literal segment and one placeholder", func(*testing.T) {
			rule = componentPathRule{
				role: componentRoleLibrary,
				segments: []componentPathSegment{
					{literal: "packages"},
					{placeholder: "component"},
				},
			}
		})

		t.Run("When the rule checks the matching shorter path", func(*testing.T) {
			matches = rule.matchesStrictPrefix([]string{"packages"})
		})

		t.Run("Then the rule accepts the path as a strict prefix", func(t *testing.T) {
			if !matches {
				t.Error("the rule rejects a matching strict prefix")
			}
		})
	})

	t.Run("Scenario: A shorter path differs from the literal part of a component template", func(t *testing.T) {
		var rule componentPathRule
		var matches bool

		t.Run("Given a component rule with one literal segment and one placeholder", func(*testing.T) {
			rule = componentPathRule{
				role: componentRoleLibrary,
				segments: []componentPathSegment{
					{literal: "packages"},
					{placeholder: "component"},
				},
			}
		})

		t.Run("When the rule checks a different shorter path", func(*testing.T) {
			matches = rule.matchesStrictPrefix([]string{"modules"})
		})

		t.Run("Then the rule rejects the path as a strict prefix", func(t *testing.T) {
			if matches {
				t.Error("the rule accepts a different strict prefix")
			}
		})
	})
}

func TestComponentClassifier_RejectUnclassifiedPaths(t *testing.T) {
	testCases := []struct {
		name string
		path string
	}{
		{name: "a path stops before the component placeholder", path: "packages"},
		{name: "a path does not match a configured rule", path: "unknown/telemetry"},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var classifier componentClassifier
			var classified bool

			t.Run("Given one library component rule", func(*testing.T) {
				classifier = componentClassifier{rules: []componentPathRule{{
					role:     componentRoleLibrary,
					category: "library",
					segments: []componentPathSegment{
						{literal: "packages"},
						{placeholder: "component"},
					},
				}}}
			})

			t.Run("When the classifier inspects the path", func(*testing.T) {
				_, classified = classifier.classify(testCase.path)
			})

			t.Run("Then the classifier rejects the path", func(t *testing.T) {
				if classified {
					t.Error("the classifier accepts an unclassified path")
				}
			})
		})
	}
}

func TestComponentClassifier_RejectEmptyPath(t *testing.T) {
	t.Run("Scenario: A path contains separators only", func(t *testing.T) {
		var classifier componentClassifier
		var classified bool

		t.Run("Given the default component path rules", func(t *testing.T) {
			var err error
			classifier, err = newComponentClassifier(
				defaultComponentRulesConfiguration().domainRules(),
			)
			if err != nil {
				t.Fatalf("newComponentClassifier fails: %v", err)
			}
		})

		t.Run("When the classifier receives a path without segments", func(*testing.T) {
			_, classified = classifier.classify("///")
		})

		t.Run("Then the classifier rejects the path", func(t *testing.T) {
			if classified {
				t.Fatal("the classifier accepts a path without segments")
			}
		})
	})
}

func TestComponentClassifier_ClassifyCustomCategory(t *testing.T) {
	t.Run("Scenario: A repository defines a plugin category", func(t *testing.T) {
		var classifier componentClassifier
		var descriptor componentDescriptor
		var classified bool

		t.Run("Given a plugin category with the closed library role", func(t *testing.T) {
			var err error
			classifier, err = newComponentClassifier(ComponentRules{
				Infrastructure: []string{"internal/{component}"},
				Taxonomy: []ComponentCategoryRule{{
					Identifier: "plugin",
					Role:       componentRoleLibrary,
					Paths:      []string{"internal/plugin/{component}"},
				}},
			})
			if err != nil {
				t.Fatalf("build component classifier: %v", err)
			}
		})

		t.Run("When the classifier inspects a plugin source path", func(*testing.T) {
			descriptor, classified = classifier.classify("internal/plugin/codec/codec.go")
		})

		t.Run("Then the configurable category has priority over the broad fallback", func(t *testing.T) {
			if !classified || descriptor.category != "plugin" {
				t.Fatalf("unexpected custom category descriptor: %+v", descriptor)
			}
		})

		t.Run("And architecture policies receive the closed library role", func(t *testing.T) {
			if descriptor.role != componentRoleLibrary {
				t.Errorf("custom category role is %q", descriptor.role)
			}
		})
	})
}

func TestComponentClassifier_RejectInvalidTaxonomy(t *testing.T) {
	testCases := []struct {
		name     string
		taxonomy []ComponentCategoryRule
	}{
		{
			name: "a category identifier is empty",
			taxonomy: []ComponentCategoryRule{{
				Role:  componentRoleLibrary,
				Paths: []string{"plugins/{component}"},
			}},
		},
		{
			name: "the taxonomy repeats a category identifier",
			taxonomy: []ComponentCategoryRule{
				{Identifier: "plugin", Role: componentRoleLibrary, Paths: []string{"plugins/{component}"}},
				{Identifier: "plugin", Role: componentRoleLibrary, Paths: []string{"extensions/{component}"}},
			},
		},
		{
			name: "a category role is not in the closed set",
			taxonomy: []ComponentCategoryRule{{
				Identifier: "plugin",
				Role:       componentRole("plugin"),
				Paths:      []string{"plugins/{component}"},
			}},
		},
		{
			name:     "a category has no path template",
			taxonomy: []ComponentCategoryRule{{Identifier: "plugin", Role: componentRoleLibrary}},
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var classifierError error

			t.Run("Given an invalid configurable component taxonomy", func(*testing.T) {})

			t.Run("When the constructor builds the classifier", func(*testing.T) {
				_, classifierError = newComponentClassifier(ComponentRules{Taxonomy: testCase.taxonomy})
			})

			t.Run("Then startup rejects the taxonomy", func(t *testing.T) {
				if classifierError == nil {
					t.Error("the classifier accepts the invalid taxonomy")
				}
			})
		})
	}
}
