package main

import (
	"errors"
	"testing"
)

func TestFindingsQuery_FilterWithoutExternalTools(t *testing.T) {
	t.Run("Scenario: An agent requests only errors related to one component", func(t *testing.T) {
		var graph Graph
		var result findingsQueryResult

		t.Run("Given deterministic findings with different severities and subjects", func(t *testing.T) {
			graph = queryFixtureGraph()
		})

		t.Run("When the query filters findings by error severity and component", func(t *testing.T) {
			result = queryFindings(graph, findingsQuery{
				severity:  findingSeverityErrorFilter,
				component: "packages/beta",
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
		var result componentsQueryResult

		t.Run("Given components from several kinds with tied coupling values", func(t *testing.T) {
			graph = queryFixtureGraph()
		})

		t.Run("When the query sorts libraries by afferent coupling and applies a limit", func(t *testing.T) {
			result = queryComponents(graph, componentsQuery{
				kind:  string(componentKindLibrary),
				sort:  componentSortAfferent,
				limit: 1,
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

func TestComponentsQuery_SortByEveryDocumentedField(t *testing.T) {
	testCases := []struct {
		name      string
		sort      componentSort
		wantFirst string
	}{
		{name: "identifier order", sort: componentSortIdentifier, wantFirst: "packages/alpha"},
		{name: "afferent coupling", sort: componentSortAfferent, wantFirst: "packages/alpha"},
		{name: "efferent coupling", sort: componentSortEfferent, wantFirst: "packages/beta"},
		{name: "transitive importers", sort: componentSortImporters, wantFirst: "packages/alpha"},
		{name: "transitive dependencies", sort: componentSortDependencies, wantFirst: "packages/beta"},
		{name: "instability", sort: componentSortInstability, wantFirst: "packages/beta"},
		{name: "abstractness", sort: componentSortAbstractness, wantFirst: "packages/alpha"},
		{name: "main sequence distance", sort: componentSortDistance, wantFirst: "packages/beta"},
		{name: "source files", sort: componentSortFiles, wantFirst: "packages/alpha"},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: The query sorts libraries by "+testCase.name, func(t *testing.T) {
			var graph Graph
			var result componentsQueryResult

			t.Run("Given components with distinct deterministic metrics", func(t *testing.T) {
				graph = queryFixtureGraph()
			})

			t.Run("When the query applies the documented sort order", func(t *testing.T) {
				result = queryComponents(graph, componentsQuery{
					kind: "library",
					sort: testCase.sort,
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
		var result findingsQueryResult

		t.Run("Given several warnings from different architecture rules", func(t *testing.T) {
			graph = queryFixtureGraph()
		})

		t.Run("When the query filters warning findings by rule and applies a limit", func(t *testing.T) {
			result = queryFindings(graph, findingsQuery{
				severity: findingSeverityWarningFilter,
				rule:     "stable-component-low-abstraction",
				limit:    1,
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
		var result componentQueryResult
		var queryError error

		t.Run("Given a component with imports, functions, calls, and findings", func(t *testing.T) {
			graph = queryFixtureGraph()
		})

		t.Run("When the agent queries the exact identifier", func(t *testing.T) {
			result, queryError = queryComponent(graph, "packages/beta")
		})

		if !t.Run("Then the result contains the component", func(t *testing.T) {
			if queryError != nil {
				t.Fatalf("queryComponent fails: %v", queryError)
			}
			if result.Component.Identifier != "packages/beta" {
				t.Fatalf("unexpected component: %+v", result.Component)
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
			_, queryError = queryComponent(graph, "packages/missing")
		})

		t.Run("Then the function returns a typed not-found error", func(t *testing.T) {
			if !errors.Is(queryError, errComponentNotFound) {
				t.Fatalf("component query error is %v, want errComponentNotFound", queryError)
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
				_, err := parseFindingSeverityFilter("critical")
				return err
			},
		},
		{
			name: "component kind is unknown",
			parse: func() error {
				_, err := parseComponentKindFilter("service")
				return err
			},
		},
		{
			name: "component sort is unknown",
			parse: func() error {
				_, err := parseComponentSort("weight")
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

			t.Run("Then the query rejects the unknown value", func(t *testing.T) {
				if parseError == nil {
					t.Fatal("the parser accepts an unknown query option")
				}
			})
		})
	}
}

func queryFixtureGraph() Graph {
	return Graph{
		SchemaVersion: graphSchemaVersion,
		Revision:      "revision-1",
		ModulePath:    "example.com/query",
		Components: []Component{
			{
				Identifier:                    "packages/alpha",
				Kind:                          componentKindLibrary,
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
				Kind:                          componentKindLibrary,
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
				Kind:             componentKindApplication,
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
