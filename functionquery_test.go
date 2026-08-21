package main

import (
	"errors"
	"slices"
	"testing"
)

func TestFunctionQueries_ReturnDirectFunctionResources(t *testing.T) {
	t.Run("Scenario: Native queries filter functions and exact calls without output pipes", func(t *testing.T) {
		var graph Graph
		var functions functionsQueryResult
		var selected functionQueryResult
		var calls functionCallsQueryResult
		var queryError error

		t.Run("Given production and test calls between two components", func(*testing.T) {
			graph = functionQueryFixture()
		})

		t.Run("When each focused function query executes", func(*testing.T) {
			functions = queryFunctions(graph, functionsQuery{
				component:    "component/target",
				sort:         functionSortIncomingCalls,
				includeTests: true,
				limit:        1,
			})
			selected, queryError = queryFunction(graph, "component/target.Read", true)
			calls = queryFunctionCalls(graph, functionCallsQuery{
				sourceComponent: "component/source",
				targetComponent: "component/target",
				includeTests:    false,
				limit:           10,
			})
		})

		if !t.Run("Then each query returns its direct resource", func(t *testing.T) {
			if queryError != nil {
				t.Fatalf("queryFunction fails: %v", queryError)
			}
			if functions.Matched != 1 || functions.Returned != 1 ||
				functions.Functions[0].Identifier != "component/target.Read" {
				t.Fatalf("unexpected function list: %+v", functions)
			}
			if selected.Function.Identifier != "component/target.Read" {
				t.Fatalf("unexpected selected function: %+v", selected)
			}
		}) {
			return
		}

		t.Run("And call filters retain exact source locations and exclude tests", func(t *testing.T) {
			if calls.Matched != 1 || calls.Returned != 1 || calls.CallSites != 2 {
				t.Errorf("unexpected call query: %+v", calls)
			}
			if len(selected.Callers) != 2 || len(selected.Callees) != 0 {
				t.Errorf("unexpected caller and callee lists: %+v", selected)
			}
			if !slices.Equal(
				calls.Calls[0].CallSites,
				[]CallSite{
					{Path: "source.go", Line: 12, Column: 2},
					{Path: "source.go", Line: 13, Column: 2},
				},
			) {
				t.Errorf("unexpected exact call sites: %+v", calls.Calls[0].CallSites)
			}
		})
	})
}

func TestFunctionQuery_RejectUnknownFunction(t *testing.T) {
	t.Run("Scenario: A caller requests a function identifier that is absent", func(t *testing.T) {
		var graph Graph
		var queryError error

		t.Run("Given an empty function graph", func(*testing.T) {
			graph = Graph{}
		})

		t.Run("When the exact function query executes", func(*testing.T) {
			_, queryError = queryFunction(graph, "absent.Function", false)
		})

		t.Run("Then the query returns the typed not-found error", func(t *testing.T) {
			if !errors.Is(queryError, errFunctionNotFound) {
				t.Fatalf("error is %v, want errFunctionNotFound", queryError)
			}
		})
	})
}

func TestFunctionSort_ParseClosedVocabulary(t *testing.T) {
	testCases := []struct {
		value string
		valid bool
	}{
		{value: "identifier", valid: true},
		{value: "incoming-calls", valid: true},
		{value: "outgoing-calls", valid: true},
		{value: "afferent", valid: true},
		{value: "efferent", valid: true},
		{value: "transitive-callers", valid: true},
		{value: "transitive-callees", valid: true},
		{value: "instability", valid: true},
		{value: "weight", valid: false},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: The function sort value is "+testCase.value, func(t *testing.T) {
			var parseError error

			t.Run("Given one function sort value", func(*testing.T) {})

			t.Run("When the parser checks the closed vocabulary", func(*testing.T) {
				_, parseError = parseFunctionSort(testCase.value)
			})

			t.Run("Then the parser returns the expected validity", func(t *testing.T) {
				if (parseError == nil) != testCase.valid {
					t.Fatalf("parse error is %v, valid is %t", parseError, testCase.valid)
				}
			})
		})
	}
}

func TestFunctionComparison_OrderEveryMetric(t *testing.T) {
	testCases := []struct {
		name         string
		sort         functionSort
		includeTests bool
		want         string
	}{
		{name: "identifier", sort: functionSortIdentifier, want: "a"},
		{name: "production incoming calls", sort: functionSortIncomingCalls, want: "b"},
		{name: "all outgoing calls", sort: functionSortOutgoingCalls, includeTests: true, want: "b"},
		{name: "production outgoing calls", sort: functionSortOutgoingCalls, want: "b"},
		{name: "all afferent coupling", sort: functionSortAfferent, includeTests: true, want: "b"},
		{name: "production afferent coupling", sort: functionSortAfferent, want: "b"},
		{name: "all efferent coupling", sort: functionSortEfferent, includeTests: true, want: "b"},
		{name: "production efferent coupling", sort: functionSortEfferent, want: "b"},
		{name: "transitive callers", sort: functionSortTransitiveCallers, want: "b"},
		{name: "transitive callees", sort: functionSortTransitiveCallees, want: "b"},
		{name: "instability", sort: functionSortInstability, want: "b"},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: Functions sort by "+testCase.name, func(t *testing.T) {
			var functions []Function

			t.Run("Given two functions with different metric values", func(*testing.T) {
				functions = []Function{
					{Identifier: "a"},
					{
						Identifier:                "b",
						IncomingCallSites:         1,
						OutgoingCallSites:         1,
						TestOutgoingCallSites:     1,
						AfferentCoupling:          1,
						TestAfferentCoupling:      1,
						EfferentCoupling:          1,
						TestEfferentCoupling:      1,
						TransitiveCallerFunctions: 1,
						TransitiveCalledFunctions: 1,
						Instability:               1,
					},
				}
			})

			t.Run("When the selected metric comparison sorts the functions", func(*testing.T) {
				slices.SortFunc(
					functions,
					functionComparison(testCase.sort, testCase.includeTests),
				)
			})

			t.Run("Then the expected function occurs first", func(t *testing.T) {
				if functions[0].Identifier != testCase.want {
					t.Fatalf("first function is %q, want %q", functions[0].Identifier, testCase.want)
				}
			})
		})
	}
}

func TestFunctionQuery_MatchEveryFilter(t *testing.T) {
	testCases := []struct {
		name     string
		function Function
		query    functionsQuery
		want     bool
	}{
		{
			name:     "empty filters accept a production function",
			function: Function{Component: "component", Package: "package"},
			want:     true,
		},
		{
			name:     "test exclusion rejects a test function",
			function: Function{Component: "component", Package: "package", Test: true},
			want:     false,
		},
		{
			name:     "test inclusion accepts a test function",
			function: Function{Component: "component", Package: "package", Test: true},
			query:    functionsQuery{includeTests: true},
			want:     true,
		},
		{
			name:     "an exact component accepts a function",
			function: Function{Component: "component", Package: "package"},
			query:    functionsQuery{component: "component"},
			want:     true,
		},
		{
			name:     "a different component rejects a function",
			function: Function{Component: "component", Package: "package"},
			query:    functionsQuery{component: "different"},
			want:     false,
		},
		{
			name:     "an exact package accepts a function",
			function: Function{Component: "component", Package: "package"},
			query:    functionsQuery{packagePath: "package"},
			want:     true,
		},
		{
			name:     "a different package rejects a function",
			function: Function{Component: "component", Package: "package"},
			query:    functionsQuery{packagePath: "different"},
			want:     false,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var matched bool

			t.Run("Given one function and one native query", func(*testing.T) {})

			t.Run("When the query evaluates every active filter", func(*testing.T) {
				matched = functionMatchesQuery(testCase.function, testCase.query)
			})

			t.Run("Then the match result is exact", func(t *testing.T) {
				if matched != testCase.want {
					t.Fatalf("matched is %t, want %t", matched, testCase.want)
				}
			})
		})
	}
}

func TestFunctionCallQuery_MatchEveryFilter(t *testing.T) {
	call := FunctionCall{
		Source:          "source.Function",
		Target:          "target.Function",
		SourceComponent: "source/component",
		TargetComponent: "target/component",
	}
	testCases := []struct {
		name  string
		call  FunctionCall
		query functionCallsQuery
		want  bool
	}{
		{name: "empty filters accept a production call", call: call, want: true},
		{
			name:  "test exclusion rejects a test call",
			call:  FunctionCall{TestOnly: true},
			query: functionCallsQuery{},
			want:  false,
		},
		{
			name:  "test inclusion accepts a test call",
			call:  FunctionCall{TestOnly: true},
			query: functionCallsQuery{includeTests: true},
			want:  true,
		},
		{
			name: "exact filters accept the call",
			call: call,
			query: functionCallsQuery{
				sourceComponent: "source/component",
				targetComponent: "target/component",
				sourceFunction:  "source.Function",
				targetFunction:  "target.Function",
			},
			want: true,
		},
		{
			name:  "a different source component rejects the call",
			call:  call,
			query: functionCallsQuery{sourceComponent: "different"},
			want:  false,
		},
		{
			name:  "a different target component rejects the call",
			call:  call,
			query: functionCallsQuery{targetComponent: "different"},
			want:  false,
		},
		{
			name:  "a different source function rejects the call",
			call:  call,
			query: functionCallsQuery{sourceFunction: "different"},
			want:  false,
		},
		{
			name:  "a different target function rejects the call",
			call:  call,
			query: functionCallsQuery{targetFunction: "different"},
			want:  false,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var matched bool

			t.Run("Given one resolved call and one native query", func(*testing.T) {})

			t.Run("When the query evaluates every active filter", func(*testing.T) {
				matched = functionCallMatchesQuery(testCase.call, testCase.query)
			})

			t.Run("Then the match result is exact", func(t *testing.T) {
				if matched != testCase.want {
					t.Fatalf("matched is %t, want %t", matched, testCase.want)
				}
			})
		})
	}
}

func TestFunctionMetrics_IncludeTestValuesOnRequest(t *testing.T) {
	t.Run("Scenario: A caller selects production metrics with and without test values", func(t *testing.T) {
		var function Function
		var production []int
		var all []int

		t.Run("Given a function with one production value and two test values", func(*testing.T) {
			function = Function{
				IncomingCallSites:     1,
				OutgoingCallSites:     1,
				AfferentCoupling:      1,
				EfferentCoupling:      1,
				TestIncomingCallSites: 2,
				TestOutgoingCallSites: 2,
				TestAfferentCoupling:  2,
				TestEfferentCoupling:  2,
			}
		})

		t.Run("When the metric functions calculate both scopes", func(*testing.T) {
			production = []int{
				functionIncomingCalls(function, false),
				functionOutgoingCalls(function, false),
				functionAfferentCoupling(function, false),
				functionEfferentCoupling(function, false),
			}
			all = []int{
				functionIncomingCalls(function, true),
				functionOutgoingCalls(function, true),
				functionAfferentCoupling(function, true),
				functionEfferentCoupling(function, true),
			}
		})

		t.Run("Then test values change each requested metric", func(t *testing.T) {
			if !slices.Equal(production, []int{1, 1, 1, 1}) || !slices.Equal(all, []int{3, 3, 3, 3}) {
				t.Fatalf("production metrics are %v and all metrics are %v", production, all)
			}
		})
	})
}

func functionQueryFixture() Graph {
	return Graph{
		SchemaVersion: graphSchemaVersion,
		Functions: []Function{
			{
				Identifier:            "component/source.Execute",
				Component:             "component/source",
				OutgoingCallSites:     2,
				TestOutgoingCallSites: 1,
				EfferentCoupling:      1,
				TestEfferentCoupling:  1,
			},
			{
				Identifier:            "component/target.Read",
				Component:             "component/target",
				IncomingCallSites:     2,
				TestIncomingCallSites: 1,
				AfferentCoupling:      1,
				TestAfferentCoupling:  1,
			},
			{
				Identifier: "component/test.Verify",
				Component:  "component/source",
				Test:       true,
			},
		},
		FunctionCalls: []FunctionCall{
			{
				Source:          "component/source.Execute",
				Target:          "component/target.Read",
				SourceComponent: "component/source",
				TargetComponent: "component/target",
				Calls:           2,
				CallSites: []CallSite{
					{Path: "source.go", Line: 12, Column: 2},
					{Path: "source.go", Line: 13, Column: 2},
				},
				CrossComponent: true,
			},
			{
				Source:          "component/test.Verify",
				Target:          "component/target.Read",
				SourceComponent: "component/source",
				TargetComponent: "component/target",
				Calls:           1,
				CallSites:       []CallSite{{Path: "source_test.go", Line: 8, Column: 2}},
				TestOnly:        true,
				CrossComponent:  true,
			},
		},
	}
}
