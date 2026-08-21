package query

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/report"
)

// ErrFunctionNotFound identifies a function that is absent from the graph.
var ErrFunctionNotFound = errors.New("function not found")

// FunctionSort selects the metric that orders functions.
type FunctionSort string

const (
	// FunctionSortIdentifier orders functions by identifier.
	FunctionSortIdentifier FunctionSort = "identifier"
	// FunctionSortIncomingCallSites orders functions by incoming call sites.
	FunctionSortIncomingCallSites FunctionSort = "incoming-call-sites"
	// FunctionSortOutgoingCallSites orders functions by outgoing call sites.
	FunctionSortOutgoingCallSites FunctionSort = "outgoing-call-sites"
	// FunctionSortAfferent orders functions by afferent coupling.
	FunctionSortAfferent FunctionSort = "afferent"
	// FunctionSortEfferent orders functions by efferent coupling.
	FunctionSortEfferent FunctionSort = "efferent"
	// FunctionSortTransitiveCallers orders functions by transitive callers.
	FunctionSortTransitiveCallers FunctionSort = "transitive-callers"
	// FunctionSortTransitiveCallees orders functions by transitive callees.
	FunctionSortTransitiveCallees FunctionSort = "transitive-callees"
	// FunctionSortInstability orders functions by instability.
	FunctionSortInstability FunctionSort = "instability"
)

type functionSortDescriptor struct {
	name    FunctionSort
	compare func(report.Function, report.Function, bool) int
}

var functionSortRegistry = []functionSortDescriptor{
	{name: FunctionSortIdentifier, compare: func(report.Function, report.Function, bool) int { return 0 }},
	{
		name: FunctionSortIncomingCallSites,
		compare: func(first, second report.Function, includeTests bool) int {
			return cmp.Compare(
				functionIncomingCallSites(second, includeTests),
				functionIncomingCallSites(first, includeTests),
			)
		},
	},
	{
		name: FunctionSortOutgoingCallSites,
		compare: func(first, second report.Function, includeTests bool) int {
			return cmp.Compare(
				functionOutgoingCallSites(second, includeTests),
				functionOutgoingCallSites(first, includeTests),
			)
		},
	},
	{
		name: FunctionSortAfferent,
		compare: func(first, second report.Function, includeTests bool) int {
			return cmp.Compare(
				functionAfferentCoupling(second, includeTests),
				functionAfferentCoupling(first, includeTests),
			)
		},
	},
	{
		name: FunctionSortEfferent,
		compare: func(first, second report.Function, includeTests bool) int {
			return cmp.Compare(
				functionEfferentCoupling(second, includeTests),
				functionEfferentCoupling(first, includeTests),
			)
		},
	},
	{
		name: FunctionSortTransitiveCallers,
		compare: func(first, second report.Function, _ bool) int {
			return cmp.Compare(second.TransitiveCallerFunctions, first.TransitiveCallerFunctions)
		},
	},
	{
		name: FunctionSortTransitiveCallees,
		compare: func(first, second report.Function, _ bool) int {
			return cmp.Compare(second.TransitiveCalleeFunctions, first.TransitiveCalleeFunctions)
		},
	},
	{
		name: FunctionSortInstability,
		compare: func(first, second report.Function, _ bool) int {
			return cmp.Compare(second.Instability, first.Instability)
		},
	},
}

// FunctionsParams defines function filters, ordering, and the result limit.
type FunctionsParams struct {
	Component    string
	PackagePath  string
	Sort         FunctionSort
	IncludeTests bool
	Limit        int
}

// FunctionsResult contains functions that match one query.
type FunctionsResult struct {
	Analysis  AnalysisHeader    `json:"analysis"`
	Matched   int               `json:"matched"`
	Returned  int               `json:"returned"`
	Functions []report.Function `json:"functions"`
}

// FunctionResult contains one function and its direct calls.
type FunctionResult struct {
	Analysis      AnalysisHeader        `json:"analysis"`
	Function      report.Function       `json:"function"`
	IncomingCalls []report.FunctionCall `json:"incomingCalls"`
	OutgoingCalls []report.FunctionCall `json:"outgoingCalls"`
}

// FunctionCallsParams defines resolved call filters and the result limit.
type FunctionCallsParams struct {
	SourceComponent string
	TargetComponent string
	SourceFunction  string
	TargetFunction  string
	IncludeTests    bool
	Limit           int
}

// FunctionCallsResult contains resolved calls that match one query.
type FunctionCallsResult struct {
	Analysis  AnalysisHeader        `json:"analysis"`
	Matched   int                   `json:"matched"`
	Returned  int                   `json:"returned"`
	CallSites int                   `json:"callSites"`
	Calls     []report.FunctionCall `json:"calls"`
}

// ParseFunctionSort validates one function sort value.
func ParseFunctionSort(value string) (FunctionSort, error) {
	sortOrder := FunctionSort(value)
	if _, found := functionSortDescriptorFor(sortOrder); found {
		return sortOrder, nil
	}
	return "", fmt.Errorf("function sort %q must be %s", value, describeFunctionSorts())
}

// Functions returns functions that match the supplied parameters.
func Functions(graph report.Graph, query FunctionsParams) FunctionsResult {
	functions := make([]report.Function, 0)
	for _, function := range graph.Functions {
		if !functionMatchesQuery(function, query) {
			continue
		}
		functions = append(functions, function)
	}
	slices.SortFunc(functions, functionComparison(query.Sort, query.IncludeTests))
	matched := len(functions)
	functions = applyLimit(functions, query.Limit)
	return FunctionsResult{
		Analysis:  analysisHeader(graph),
		Matched:   matched,
		Returned:  len(functions),
		Functions: functions,
	}
}

func functionMatchesQuery(function report.Function, query FunctionsParams) bool {
	if !query.IncludeTests && function.Test {
		return false
	}
	if query.Component != "" && function.Component != query.Component {
		return false
	}
	return query.PackagePath == "" || function.Package == query.PackagePath
}

func functionComparison(
	sortOrder FunctionSort,
	includeTests bool,
) func(report.Function, report.Function) int {
	descriptor, found := functionSortDescriptorFor(sortOrder)
	if !found {
		descriptor, _ = functionSortDescriptorFor(FunctionSortIdentifier)
	}
	return func(first, second report.Function) int {
		result := descriptor.compare(first, second, includeTests)
		return cmp.Or(result, strings.Compare(first.Identifier, second.Identifier))
	}
}

func functionSortDescriptorFor(sortOrder FunctionSort) (functionSortDescriptor, bool) {
	for _, descriptor := range functionSortRegistry {
		if descriptor.name == sortOrder {
			return descriptor, true
		}
	}
	return functionSortDescriptor{}, false
}

func describeFunctionSorts() string {
	names := make([]string, 0, len(functionSortRegistry))
	for _, descriptor := range functionSortRegistry {
		names = append(names, string(descriptor.name))
	}
	return strings.Join(names[:len(names)-1], ", ") + ", or " + names[len(names)-1]
}

func functionIncomingCallSites(function report.Function, includeTests bool) int {
	if includeTests {
		return function.IncomingCallSites + function.TestIncomingCallSites
	}
	return function.IncomingCallSites
}

func functionOutgoingCallSites(function report.Function, includeTests bool) int {
	if includeTests {
		return function.OutgoingCallSites + function.TestOutgoingCallSites
	}
	return function.OutgoingCallSites
}

func functionAfferentCoupling(function report.Function, includeTests bool) int {
	if includeTests {
		return function.AfferentCoupling + function.TestAfferentCoupling
	}
	return function.AfferentCoupling
}

func functionEfferentCoupling(function report.Function, includeTests bool) int {
	if includeTests {
		return function.EfferentCoupling + function.TestEfferentCoupling
	}
	return function.EfferentCoupling
}

// GetFunction returns one function and its direct calls.
func GetFunction(graph report.Graph, identifier string, includeTests bool) (FunctionResult, error) {
	function, found := findFunction(graph.Functions, identifier)
	if !found {
		return FunctionResult{}, fmt.Errorf("%w: %s", ErrFunctionNotFound, identifier)
	}
	incomingCalls := make([]report.FunctionCall, 0, len(graph.FunctionCalls))
	outgoingCalls := make([]report.FunctionCall, 0, len(graph.FunctionCalls))
	for _, call := range graph.FunctionCalls {
		if call.TestOnly && !includeTests {
			continue
		}
		if call.Target == identifier {
			incomingCalls = append(incomingCalls, call)
		}
		if call.Source == identifier {
			outgoingCalls = append(outgoingCalls, call)
		}
	}
	return FunctionResult{
		Analysis:      analysisHeader(graph),
		Function:      function,
		IncomingCalls: incomingCalls,
		OutgoingCalls: outgoingCalls,
	}, nil
}

func findFunction(functions []report.Function, identifier string) (report.Function, bool) {
	for _, function := range functions {
		if function.Identifier == identifier {
			return function, true
		}
	}
	return report.Function{}, false
}

// FunctionCalls returns resolved calls that match the supplied parameters.
func FunctionCalls(graph report.Graph, query FunctionCallsParams) FunctionCallsResult {
	calls := make([]report.FunctionCall, 0)
	for _, call := range graph.FunctionCalls {
		if functionCallMatchesQuery(call, query) {
			calls = append(calls, call)
		}
	}
	matched := len(calls)
	calls = applyLimit(calls, query.Limit)
	callSites := 0
	for _, call := range calls {
		callSites += call.Calls
	}
	return FunctionCallsResult{
		Analysis:  analysisHeader(graph),
		Matched:   matched,
		Returned:  len(calls),
		CallSites: callSites,
		Calls:     calls,
	}
}

func functionCallMatchesQuery(call report.FunctionCall, query FunctionCallsParams) bool {
	if call.TestOnly && !query.IncludeTests {
		return false
	}
	return (query.SourceComponent == "" || call.SourceComponent == query.SourceComponent) &&
		(query.TargetComponent == "" || call.TargetComponent == query.TargetComponent) &&
		(query.SourceFunction == "" || call.Source == query.SourceFunction) &&
		(query.TargetFunction == "" || call.Target == query.TargetFunction)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T16:37:10Z","module_hash":"d7f7f236ea096d5b020be090f914c5a23adc720fb8564ba86ed5002a52da0c02","functions":[{"id":"func/ParseFunctionSort","name":"ParseFunctionSort","line":146,"end_line":152,"hash":"9eca8177c5847d4c17e17e81feda1ac5d186916cc41f93ec46d35aa9eaa40beb"},{"id":"func/Functions","name":"Functions","line":155,"end_line":172,"hash":"fd462794e25ba004fbfecd32fc2381ae09c7db7250236825dfeebd755b41c290"},{"id":"func/functionMatchesQuery","name":"functionMatchesQuery","line":174,"end_line":182,"hash":"22e62a56d83addb2eb9ccccd24d4a26b753bbca9a1d8af0cdde52553dc565811"},{"id":"func/functionComparison","name":"functionComparison","line":184,"end_line":196,"hash":"13d66804c722f5d225728f48748d91b9eee74891b2eb9d8e5eec13b38cd51c8c"},{"id":"func/functionSortDescriptorFor","name":"functionSortDescriptorFor","line":198,"end_line":205,"hash":"f63da7aa23895b1baa24f7aaa3e0349ba58e3ffecd178b6171f3cbbb78913e62"},{"id":"func/describeFunctionSorts","name":"describeFunctionSorts","line":207,"end_line":213,"hash":"32a9059db4cfa579c0ce184cb2d5d232a4d37eed5c726056266203ffbf76d20e"},{"id":"func/functionIncomingCallSites","name":"functionIncomingCallSites","line":215,"end_line":220,"hash":"e086225040ec47c33ea3d6a86e6784cf5445191fdff5ca067326a326ea71cec8"},{"id":"func/functionOutgoingCallSites","name":"functionOutgoingCallSites","line":222,"end_line":227,"hash":"6746f1eaee26b6b01c23c0d43cb9435facd2267445f6abfe0f0db54ac516e461"},{"id":"func/functionAfferentCoupling","name":"functionAfferentCoupling","line":229,"end_line":234,"hash":"62a25eae6b1363efde96619d99c57edf48c2b4578dd1215b6223db48d7f357f6"},{"id":"func/functionEfferentCoupling","name":"functionEfferentCoupling","line":236,"end_line":241,"hash":"adb37c4341117d020038b33f404881c7bc634a1986851c32050eba74fecc012b"},{"id":"func/GetFunction","name":"GetFunction","line":244,"end_line":268,"hash":"f3e03fe1462fa3b0acc5ab2336bf8ebdc3838cdb025cb78b12a718a51e9696cc"},{"id":"func/findFunction","name":"findFunction","line":270,"end_line":277,"hash":"52ddee9d8e90501c1c0206d07c50bed6236c9c17d845311b4a6bb01194015b4f"},{"id":"func/FunctionCalls","name":"FunctionCalls","line":280,"end_line":300,"hash":"84f21d40a031c92c034b8b38947eea4a5d9fdb312c6b421766710a3a2ea36b20"},{"id":"func/functionCallMatchesQuery","name":"functionCallMatchesQuery","line":302,"end_line":310,"hash":"f4918674e7845a62a3c442b9125daaf0fe86abd0162981b33b9ec469e1aea24e"}]}
// mutate4go-manifest-end
