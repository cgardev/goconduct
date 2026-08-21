package calculation

import (
	"reflect"
	"slices"
	"testing"
)

func TestFunctionCalculation_DetectCyclesAndUsingApplications(t *testing.T) {
	t.Run("Scenario: Application and library functions form a transitive call graph", func(t *testing.T) {
		var graph Graph
		var functions []Function
		var calls []FunctionCall
		var cycles [][]string
		var declarations []functionDeclaration
		var references []functionReference

		t.Run("Given one application caller and two mutually dependent library functions", func(*testing.T) {
			declarations = []functionDeclaration{
				{identifier: "cmd/control.Start", component: "cmd/control", inAnalysisScope: true},
				{
					identifier:      "internal/library/data.Read",
					component:       "internal/library/data",
					inAnalysisScope: true,
				},
				{
					identifier:      "internal/library/data.Write",
					component:       "internal/library/data",
					inAnalysisScope: true,
				},
			}
			references = []functionReference{
				{source: "cmd/control.Start", target: "internal/library/data.Read", site: CallSite{Line: 1}},
				{
					source: "internal/library/data.Read",
					target: "internal/library/data.Write",
					site:   CallSite{Line: 2},
				},
				{
					source: "internal/library/data.Write",
					target: "internal/library/data.Read",
					site:   CallSite{Line: 3},
				},
			}
		})

		t.Run("When the calculator attaches function metrics to component data", func(*testing.T) {
			functions, calls, cycles = calculateFunctionGraph(declarations, references)
			graph = Graph{
				Functions:      functions,
				FunctionCalls:  calls,
				FunctionCycles: cycles,
				Components: []Component{
					{Identifier: "cmd/control", Application: "control"},
					{Identifier: "internal/library/data"},
				},
			}
			attachFunctionMetrics(&graph)
		})

		if !t.Run("Then both library functions belong to one deterministic cycle", func(t *testing.T) {
			want := [][]string{{"internal/library/data.Read", "internal/library/data.Write"}}
			if !slices.EqualFunc(graph.FunctionCycles, want, func(first, second []string) bool {
				return slices.Equal(first, second)
			}) {
				t.Fatalf("function cycles are %v, want %v", graph.FunctionCycles, want)
			}
			if graph.Summary.FunctionCycles != 1 {
				t.Errorf("function cycle count is %d, want 1", graph.Summary.FunctionCycles)
			}
		}) {
			return
		}

		t.Run("And transitive caller metrics identify the using application", func(t *testing.T) {
			read := functionWithIdentifier(t, graph, "internal/library/data.Read")
			if !read.InCycle || read.UsingApplicationCount != 1 ||
				!slices.Equal(read.UsingApplications, []string{"control"}) {
				t.Errorf("unexpected function application metrics: %+v", read)
			}
			if read.TransitiveCallerFunctions != 2 || read.TransitiveCalleeFunctions != 1 {
				t.Errorf("unexpected transitive function coupling: %+v", read)
			}
		})
	})
}

func TestFunctionCalculation_MergeDeclarations(t *testing.T) {
	t.Run("Scenario: Type information provides more than one declaration for a function", func(t *testing.T) {
		var selected functionDeclaration
		var completed functionDeclaration
		var retained functionDeclaration

		t.Run("Given declarations with different scope and source data", func(*testing.T) {
			selected = functionDeclaration{
				identifier:      "internal/library/data.Read",
				relativePath:    "external/data.go",
				line:            4,
				inAnalysisScope: false,
			}
			completed = functionDeclaration{
				identifier:      "internal/library/data.Read",
				relativePath:    "internal/library/data/read.go",
				line:            12,
				inAnalysisScope: true,
			}
			retained = functionDeclaration{
				identifier:      "internal/library/data.Write",
				relativePath:    "internal/library/data/write.go",
				line:            20,
				inAnalysisScope: true,
			}
		})

		t.Run("When the calculator merges duplicate declarations", func(*testing.T) {
			selected = mergeFunctionDeclarations(selected, completed)
			completed = mergeFunctionDeclarations(
				functionDeclaration{identifier: completed.identifier, inAnalysisScope: true},
				completed,
			)
			retained = mergeFunctionDeclarations(
				retained,
				functionDeclaration{
					identifier:      retained.identifier,
					relativePath:    "replacement.go",
					line:            99,
					inAnalysisScope: true,
				},
			)
		})

		t.Run("Then the calculator selects the declaration in the analysis scope", func(t *testing.T) {
			if !selected.inAnalysisScope || selected.relativePath != "internal/library/data/read.go" ||
				selected.line != 12 {
				t.Errorf("selected declaration is incorrect: %+v", selected)
			}
		})

		t.Run("And the calculator adds missing source data and retains existing source data", func(t *testing.T) {
			if completed.relativePath != "internal/library/data/read.go" || completed.line != 12 {
				t.Errorf("completed declaration is incorrect: %+v", completed)
			}
			if retained.relativePath != "internal/library/data/write.go" || retained.line != 20 {
				t.Errorf("retained declaration is incorrect: %+v", retained)
			}
		})
	})
}

func TestFunctionCalculation_MergeDeclarationsDeterministically(t *testing.T) {
	t.Run("Scenario: Two anonymous interface methods have the same function identifier", func(t *testing.T) {
		var earlier functionDeclaration
		var later functionDeclaration
		var forward functionDeclaration
		var reverse functionDeclaration

		t.Run("Given two declarations at different lines in the same source file", func(*testing.T) {
			earlier = functionDeclaration{
				identifier:      "internal/devtool/dependencygraph.interface{ExecuteContext() error}.ExecuteContext",
				relativePath:    "internal/devtool/dependencygraph/main_test.go",
				line:            183,
				test:            true,
				inAnalysisScope: true,
			}
			later = earlier
			later.line = 216
		})

		t.Run("When the calculator merges the declarations in both orders", func(*testing.T) {
			forward = mergeFunctionDeclarations(earlier, later)
			reverse = mergeFunctionDeclarations(later, earlier)
		})

		if !t.Run("Then both orders select the same declaration", func(t *testing.T) {
			if !reflect.DeepEqual(forward, reverse) {
				t.Fatalf("merged declarations differ: %+v and %+v", forward, reverse)
			}
		}) {
			return
		}

		t.Run("And the selected declaration has the first source position", func(t *testing.T) {
			if forward.relativePath != earlier.relativePath || forward.line != earlier.line {
				t.Errorf("selected declaration is %+v, want %+v", forward, earlier)
			}
		})
	})
}

func TestFunctionCalculation_RejectUnknownReferences(t *testing.T) {
	t.Run("Scenario: A type result refers to a function outside the loaded graph", func(t *testing.T) {
		var functions []Function
		var calls []FunctionCall

		t.Run("Given references with a missing caller and a missing target", func(*testing.T) {
			functions = nil
			calls = nil
		})

		t.Run("When the calculator processes the incomplete references", func(*testing.T) {
			declarations := []functionDeclaration{
				{identifier: "internal/library/data.Read", inAnalysisScope: true},
				{identifier: "internal/library/data.Write", inAnalysisScope: true},
			}
			references := []functionReference{
				{source: "missing.Caller", target: "internal/library/data.Read"},
				{source: "internal/library/data.Write", target: "missing.Target"},
			}
			functions, calls, _ = calculateFunctionGraph(declarations, references)
		})

		t.Run("Then the calculator omits both incomplete references", func(t *testing.T) {
			if len(functions) != 2 {
				t.Fatalf("function count is %d, want 2", len(functions))
			}
			if len(calls) != 0 {
				t.Errorf("calls are %v, want no calls", calls)
			}
			for _, function := range functions {
				if function.AfferentCoupling != 0 || function.EfferentCoupling != 0 ||
					function.IncomingCallSites != 0 || function.OutgoingCallSites != 0 {
					t.Errorf("function has metrics from an incomplete reference: %+v", function)
				}
			}
		})
	})
}

func TestFunctionCalculation_SeparateProductionAndTestCalls(t *testing.T) {
	t.Run("Scenario: Test calls and incomplete component calls exist", func(t *testing.T) {
		var graph Graph

		t.Run("Given one test call, one self-call, and calls with missing components", func(*testing.T) {
			declarations := []functionDeclaration{
				{
					identifier:      "cmd/control.TestStart",
					component:       "cmd/control",
					test:            true,
					inAnalysisScope: true,
				},
				{
					identifier:      "internal/library/data.Read",
					component:       "internal/library/data",
					inAnalysisScope: true,
				},
				{identifier: "ignored.External", component: "ignored"},
				{identifier: "unmapped.Function", component: "unmapped", inAnalysisScope: true},
			}
			references := []functionReference{
				{
					source: "cmd/control.TestStart",
					target: "internal/library/data.Read",
					test:   true,
					site:   CallSite{Path: "cmd/control/start_test.go", Line: 8},
				},
				{
					source: "internal/library/data.Read",
					target: "internal/library/data.Read",
					site:   CallSite{Path: "internal/library/data/read.go", Line: 14},
				},
			}
			functions, calls, cycles := calculateFunctionGraph(declarations, references)
			graph = Graph{
				Functions:      functions,
				FunctionCalls:  calls,
				FunctionCycles: cycles,
				Components: []Component{
					{Identifier: "cmd/control", Application: "control"},
					{Identifier: "internal/library/data"},
					{Identifier: "ignored"},
				},
			}
			graph.FunctionCalls = append(
				graph.FunctionCalls,
				FunctionCall{
					Source:          "missing.Caller",
					Target:          "internal/library/data.Read",
					SourceComponent: "missing",
					TargetComponent: "internal/library/data",
					Calls:           3,
					TestOnly:        true,
					CrossComponent:  true,
				},
				FunctionCall{
					Source:          "cmd/control.TestStart",
					Target:          "missing.Target",
					SourceComponent: "cmd/control",
					TargetComponent: "missing",
					Calls:           4,
					CrossComponent:  true,
				},
			)
		})

		t.Run("When the calculator attaches aggregate metrics", func(*testing.T) {
			attachFunctionMetrics(&graph)
		})

		t.Run("Then test and self-calls do not change production coupling", func(t *testing.T) {
			if len(graph.Functions) != 4 || len(graph.FunctionCalls) != 4 {
				t.Fatalf(
					"graph has %d functions and %d calls, want 4 functions and 4 calls",
					len(graph.Functions),
					len(graph.FunctionCalls),
				)
			}
			read := functionWithIdentifier(t, graph, "internal/library/data.Read")
			if read.AfferentCoupling != 0 || read.EfferentCoupling != 0 ||
				read.TestAfferentCoupling != 1 || read.TransitiveCallerFunctions != 0 ||
				read.TransitiveCalleeFunctions != 0 || read.InCycle {
				t.Errorf("test or self-call changed production coupling: %+v", read)
			}
			if read.UsingApplicationCount != 0 || len(read.UsingApplications) != 0 {
				t.Errorf("test call changed application use: %+v", read)
			}
		})

		t.Run("And component totals exclude missing and out-of-scope functions", func(t *testing.T) {
			control := componentWithIdentifier(t, graph, "cmd/control")
			data := componentWithIdentifier(t, graph, "internal/library/data")
			ignored := componentWithIdentifier(t, graph, "ignored")
			if control.TestFunctions != 1 || control.ProductionFunctions != 0 ||
				control.TestOutgoingCallSites != 1 || control.ProductionOutgoingCallSites != 0 {
				t.Errorf("unexpected control totals: %+v", control)
			}
			if data.ProductionFunctions != 1 || data.TestIncomingCallSites != 1 ||
				data.ProductionIncomingCallSites != 1 {
				t.Errorf("unexpected data totals: %+v", data)
			}
			if !reflect.DeepEqual(ignored, Component{Identifier: "ignored"}) {
				t.Errorf("out-of-scope function changed component totals: %+v", ignored)
			}
		})
	})
}

func TestFunctionCalculation_CompareCallKeys(t *testing.T) {
	t.Run("Scenario: Function calls require a stable production-first order", func(t *testing.T) {
		var comparisons []int

		t.Run("Given keys that differ by source, target, and test scope", func(*testing.T) {
			comparisons = nil
		})

		t.Run("When the comparator compares each key pair", func(*testing.T) {
			comparisons = []int{
				compareFunctionCallKeys(
					functionCallKey{source: "a", target: "z"},
					functionCallKey{source: "b", target: "a"},
				),
				compareFunctionCallKeys(
					functionCallKey{source: "a", target: "a"},
					functionCallKey{source: "a", target: "b"},
				),
				compareFunctionCallKeys(
					functionCallKey{source: "a", target: "a"},
					functionCallKey{source: "a", target: "a"},
				),
				compareFunctionCallKeys(
					functionCallKey{source: "a", target: "a"},
					functionCallKey{source: "a", target: "a", test: true},
				),
				compareFunctionCallKeys(
					functionCallKey{source: "a", target: "a", test: true},
					functionCallKey{source: "a", target: "a"},
				),
			}
		})

		t.Run("Then source and target sort before production and test scope", func(t *testing.T) {
			if !slices.Equal(comparisons, []int{-1, -1, 0, -1, 1}) {
				t.Errorf("comparisons are %v, want [-1 -1 0 -1 1]", comparisons)
			}
		})
	})
}

func functionWithIdentifier(t *testing.T, graph Graph, identifier string) Function {
	t.Helper()
	for _, function := range graph.Functions {
		if function.Identifier == identifier {
			return function
		}
	}
	t.Fatalf("function %q is absent", identifier)
	return Function{}
}

func componentWithIdentifier(t *testing.T, graph Graph, identifier string) Component {
	t.Helper()
	for _, component := range graph.Components {
		if component.Identifier == identifier {
			return component
		}
	}
	t.Fatalf("component %q is absent", identifier)
	return Component{}
}
