package query

import (
	"errors"
	"testing"

	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/architecture"
	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/failure"
)

func TestFindingsQuery_FilterWithoutExternalTools(t *testing.T) {
	t.Run("Scenario: An agent requests only errors related to one component", func(t *testing.T) {
		var graph Graph
		var result FindingsResult

		t.Run("Given deterministic findings with different severities and subjects", func(t *testing.T) {
			graph = queryFixtureGraph()
		})

		t.Run("When the query filters findings by error severity and component", func(t *testing.T) {
			result = Findings(graph, FindingsParams{
				Severity:  FindingSeverityError,
				Component: "packages/beta",
			})
		})

		if !t.Run("Then the result contains only one applicable finding", func(t *testing.T) {
			if result.Matched != 1 || result.Returned != 1 || len(result.Findings) != 1 {
				t.Fatalf("finding query matches %d and returns %d", result.Matched, len(result.Findings))
			}
		}) {
			return
		}

		t.Run("And the result contains the graph identity and finding data", func(t *testing.T) {
			if result.Analysis.Revision != "revision-1" ||
				result.Findings[0].Rule != "dependency-cycle" {
				t.Errorf("unexpected finding query result: %+v", result)
			}
		})
	})
}

func TestComponentsQuery_RankWithoutExternalTools(t *testing.T) {
	t.Run("Scenario: An agent requests the most imported library", func(t *testing.T) {
		var graph Graph
		var result ComponentsResult

		t.Run("Given components from several kinds with tied coupling values", func(t *testing.T) {
			graph = queryFixtureGraph()
		})

		t.Run("When the query sorts libraries by afferent coupling and applies a limit", func(t *testing.T) {
			result = Components(graph, ComponentsParams{
				Role:  string(componentRoleLibrary),
				Sort:  ComponentSortAfferent,
				Limit: 1,
			})
		})

		if !t.Run("Then the result reports all matches and returns the requested count", func(t *testing.T) {
			if result.Matched != 2 || result.Returned != 1 || len(result.Components) != 1 {
				t.Fatalf("component query matches %d and returns %d", result.Matched, len(result.Components))
			}
		}) {
			return
		}

		t.Run("And deterministic identifier order resolves the metric tie", func(t *testing.T) {
			if result.Components[0].Identifier != "packages/alpha" {
				t.Errorf("top component is %q, want packages/alpha", result.Components[0].Identifier)
			}
		})
	})
}

func TestComponentsQuery_FilterConfiguredCategory(t *testing.T) {
	t.Run("Scenario: An agent requests one configured component category", func(t *testing.T) {
		var graph Graph
		var result ComponentsResult

		t.Run("Given two libraries in different presentation categories", func(t *testing.T) {
			graph = queryFixtureGraph()
			graph.Components[0].Category = "plugin"
			graph.Components[1].Category = "library"
		})

		t.Run("When the query selects the custom plugin category", func(t *testing.T) {
			result = Components(graph, ComponentsParams{
				Role:     "all",
				Category: "plugin",
				Sort:     ComponentSortIdentifier,
			})
		})

		t.Run("Then the result contains only the component in that category", func(t *testing.T) {
			if result.Matched != 1 || len(result.Components) != 1 {
				t.Fatalf("component query matches %d and returns %d", result.Matched, len(result.Components))
			}
			if result.Components[0].Identifier != "packages/alpha" {
				t.Errorf("component is %q, want packages/alpha", result.Components[0].Identifier)
			}
		})
	})
}

func TestComponentsQuery_SortByEveryDocumentedField(t *testing.T) {
	testCases := []struct {
		name      string
		Sort      ComponentSort
		wantFirst string
	}{
		{name: "identifier order", Sort: ComponentSortIdentifier, wantFirst: "packages/alpha"},
		{name: "afferent coupling", Sort: ComponentSortAfferent, wantFirst: "packages/alpha"},
		{name: "efferent coupling", Sort: ComponentSortEfferent, wantFirst: "packages/beta"},
		{name: "transitive importers", Sort: ComponentSortImporters, wantFirst: "packages/alpha"},
		{name: "transitive dependencies", Sort: ComponentSortDependencies, wantFirst: "packages/beta"},
		{name: "instability", Sort: ComponentSortInstability, wantFirst: "packages/beta"},
		{name: "abstractness", Sort: ComponentSortAbstractness, wantFirst: "packages/alpha"},
		{name: "main sequence distance", Sort: ComponentSortDistance, wantFirst: "packages/beta"},
		{name: "source files", Sort: ComponentSortFiles, wantFirst: "packages/alpha"},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: The query sorts libraries by "+testCase.name, func(t *testing.T) {
			var graph Graph
			var result ComponentsResult

			t.Run("Given components with distinct deterministic metrics", func(t *testing.T) {
				graph = queryFixtureGraph()
			})

			t.Run("When the query applies the documented sort order", func(t *testing.T) {
				result = Components(graph, ComponentsParams{
					Role: "library",
					Sort: testCase.Sort,
				})
			})

			t.Run("Then the result lists the expected component first", func(t *testing.T) {
				if len(result.Components) == 0 || result.Components[0].Identifier != testCase.wantFirst {
					t.Fatalf("sort result is %v, want %s first", result.Components, testCase.wantFirst)
				}
			})
		})
	}
}

func TestFindingsQuery_ApplyRuleAndLimit(t *testing.T) {
	t.Run("Scenario: An agent requests one finding from a named rule", func(t *testing.T) {
		var graph Graph
		var result FindingsResult

		t.Run("Given several warnings from different architecture rules", func(t *testing.T) {
			graph = queryFixtureGraph()
		})

		t.Run("When the query filters warning findings by rule and applies a limit", func(t *testing.T) {
			result = Findings(graph, FindingsParams{
				Severity: FindingSeverityWarning,
				Rule:     "stable-component-low-abstraction",
				Limit:    1,
			})
		})

		t.Run("Then the direct result contains only that rule", func(t *testing.T) {
			if result.Matched != 1 || result.Returned != 1 ||
				len(result.Findings) != 1 || result.Findings[0].Rule != "stable-component-low-abstraction" {
				t.Fatalf("unexpected rule-filtered findings: %+v", result)
			}
		})
	})
}

func TestFinding_MatchComponent(t *testing.T) {
	testCases := []struct {
		name       string
		finding    Finding
		identifier string
		want       bool
	}{
		{
			name:       "the component is the finding subject",
			finding:    Finding{Subject: "packages/alpha"},
			identifier: "packages/alpha",
			want:       true,
		},
		{
			name:       "the component is the finding source",
			finding:    Finding{Source: "packages/alpha"},
			identifier: "packages/alpha",
			want:       true,
		},
		{
			name:       "the component is the finding target",
			finding:    Finding{Target: "packages/alpha"},
			identifier: "packages/alpha",
			want:       true,
		},
		{
			name:       "the component is in the finding component set",
			finding:    Finding{Components: []string{"packages/alpha"}},
			identifier: "packages/alpha",
			want:       true,
		},
		{
			name:       "the component has no relation to the finding",
			finding:    Finding{Source: "packages/beta"},
			identifier: "packages/alpha",
			want:       false,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var finding Finding
			var identifier string
			var matches bool

			t.Run("Given one finding and one component identifier", func(t *testing.T) {
				finding = testCase.finding
				identifier = testCase.identifier
			})

			t.Run("When the function checks the component relation", func(t *testing.T) {
				matches = findingMatchesComponent(finding, identifier)
			})

			t.Run("Then the result matches the expected relation", func(t *testing.T) {
				if matches != testCase.want {
					t.Errorf("component relation is %t, want %t", matches, testCase.want)
				}
			})
		})
	}
}

func TestComponentQuery_ReturnDependenciesAndImporters(t *testing.T) {
	t.Run("Scenario: An agent requests one exact component", func(t *testing.T) {
		var graph Graph
		var result ComponentResult
		var queryError error

		t.Run("Given a component with imports, functions, calls, and findings", func(t *testing.T) {
			graph = queryFixtureGraph()
		})

		t.Run("When the agent queries the exact identifier", func(t *testing.T) {
			result, queryError = GetComponent(graph, "packages/beta")
		})

		if !t.Run("Then the result contains the component", func(t *testing.T) {
			if queryError != nil {
				t.Fatalf("Component fails: %v", queryError)
			}
			if result.Component.Identifier != "packages/beta" {
				t.Fatalf("unexpected Component: %+v", result.Component)
			}
		}) {
			return
		}

		t.Run("And the result contains its imports and importing relationships", func(t *testing.T) {
			if len(result.Dependencies) != 1 || len(result.ImportingRelationships) != 2 || len(result.Findings) != 3 {
				t.Errorf(
					"component data has %d imported relationships, %d importing relationships, and %d findings",
					len(result.Dependencies),
					len(result.ImportingRelationships),
					len(result.Findings),
				)
			}
			if len(result.Functions) != 1 || result.Functions[0].Identifier != "packages/beta.Run" ||
				len(result.FunctionCalls) != 2 {
				t.Errorf(
					"component data has functions=%v and calls=%v",
					result.Functions,
					result.FunctionCalls,
				)
			}
		})
	})

	t.Run("Scenario: An agent requests an unknown component", func(t *testing.T) {
		var graph Graph
		var queryError error

		t.Run("Given a deterministic graph without the requested identifier", func(t *testing.T) {
			graph = queryFixtureGraph()
		})

		t.Run("When the agent queries the missing identifier", func(t *testing.T) {
			_, queryError = GetComponent(graph, "packages/missing")
		})

		t.Run("Then the function returns a typed not-found error", func(t *testing.T) {
			if !errors.Is(queryError, failure.ErrNotFound) {
				t.Fatalf("component query error is %v, want ErrNotFound", queryError)
			}
			var domainError *failure.Error
			if !errors.As(queryError, &domainError) {
				t.Fatalf("component query error type is %T, want *failure.Error", queryError)
			}
			if domainError.Entity != "dependency graph component" || domainError.ID != "packages/missing" {
				t.Errorf("component query error context is entity=%q id=%v", domainError.Entity, domainError.ID)
			}
		})
	})
}

func TestQueryOptions_ParseClosedVocabulary(t *testing.T) {
	testCases := []struct {
		name  string
		parse func() error
	}{
		{
			name: "finding severity is unknown",
			parse: func() error {
				_, err := ParseFindingSeverity("critical")
				return err
			},
		},
		{
			name: "component role is unknown",
			parse: func() error {
				_, err := ParseComponentRole("service")
				return err
			},
		},
		{
			name: "component sort is unknown",
			parse: func() error {
				_, err := ParseComponentSort("weight")
				return err
			},
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var parse func() error
			var parseError error

			t.Run("Given a value outside the documented query vocabulary", func(t *testing.T) {
				parse = testCase.parse
			})

			t.Run("When the parser reads the query option", func(t *testing.T) {
				parseError = parse()
			})

			t.Run("Then the query returns the validation error category", func(t *testing.T) {
				if !errors.Is(parseError, failure.ErrValidation) {
					t.Fatalf("parse error is %v, want ErrValidation", parseError)
				}
			})
		})
	}
}

func TestComponentRole_AcceptAllAndStrategicRole(t *testing.T) {
	testCases := []string{"all", string(architecture.RoleLibrary)}
	for _, value := range testCases {
		t.Run("Scenario: The component role filter is "+value, func(t *testing.T) {
			var result string
			var parseError error

			t.Run("Given a documented component role filter", func(*testing.T) {})

			t.Run("When the query parses the component role", func(*testing.T) {
				result, parseError = ParseComponentRole(value)
			})

			t.Run("Then the query accepts and retains the filter", func(t *testing.T) {
				if parseError != nil {
					t.Fatalf("parse component role: %v", parseError)
				}
				if result != value {
					t.Fatalf("component role is %q, want %q", result, value)
				}
			})
		})
	}
}

func TestComponentSort_DescribeClosedVocabulary(t *testing.T) {
	t.Run("Scenario: The CLI documents every component sort", func(t *testing.T) {
		var description string

		t.Run("Given the ordered component sort registry", func(*testing.T) {})

		t.Run("When the query layer creates the vocabulary description", func(*testing.T) {
			description = describeComponentSorts()
		})

		t.Run("Then each sort occurs once in registry order", func(t *testing.T) {
			want := "identifier, afferent, efferent, importers, dependencies, instability, " +
				"abstractness, distance, or files"
			if description != want {
				t.Fatalf("component sort description is %q, want %q", description, want)
			}
		})
	})
}

func queryFixtureGraph() Graph {
	return Graph{
		SchemaVersion: graphSchemaVersion,
		Revision:      "revision-1",
		ModulePath:    "example.com/query",
		Components: []Component{
			{
				Identifier:                    "packages/alpha",
				Role:                          componentRoleLibrary,
				SourceFiles:                   10,
				AfferentCoupling:              5,
				EfferentCoupling:              1,
				TransitiveImportingComponents: 8,
				Instability:                   0.2,
				Abstractness:                  0.9,
				MainSequenceDistance:          0.2,
			},
			{
				Identifier:                    "packages/beta",
				Role:                          componentRoleLibrary,
				SourceFiles:                   2,
				AfferentCoupling:              5,
				EfferentCoupling:              2,
				TransitiveImportingComponents: 2,
				TransitiveDependencies:        1,
				Instability:                   0.8,
				Abstractness:                  0.1,
				MainSequenceDistance:          0.9,
			},
			{
				Identifier:       "services/control",
				Role:             componentRoleApplication,
				AfferentCoupling: 10,
			},
		},
		Relationships: []Relationship{
			{Source: "packages/alpha", Target: "packages/beta"},
			{Source: "packages/beta", Target: "packages/alpha"},
			{Source: "services/control", Target: "packages/beta"},
		},
		Functions: []Function{
			{Identifier: "packages/alpha.Read", Component: "packages/alpha"},
			{Identifier: "packages/beta.Run", Component: "packages/beta"},
		},
		FunctionCalls: []FunctionCall{
			{
				Source:          "packages/beta.Run",
				Target:          "packages/alpha.Read",
				SourceComponent: "packages/beta",
				TargetComponent: "packages/alpha",
			},
			{
				Source:          "services/control.Start",
				Target:          "packages/beta.Run",
				SourceComponent: "services/control",
				TargetComponent: "packages/beta",
			},
		},
		Findings: []Finding{
			{
				Rule:     "independent-error",
				Severity: findingSeverityError,
				Subject:  "packages/gamma",
			},
			{
				Rule:       "dependency-cycle",
				Severity:   findingSeverityError,
				Subject:    "packages/alpha -> packages/beta",
				Components: []string{"packages/alpha", "packages/beta"},
			},
			{
				Rule:     "stable-dependency-principle",
				Severity: findingSeverityWarning,
				Subject:  "services/control -> packages/beta",
				Source:   "services/control",
				Target:   "packages/beta",
			},
			{
				Rule:     "stable-component-low-abstraction",
				Severity: findingSeverityWarning,
				Subject:  "packages/beta",
			},
		},
	}
}
