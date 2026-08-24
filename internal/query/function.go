package query

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/report"
)

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
	return "", failure.New(
		failure.ErrValidation,
		fmt.Sprintf("function sort %q must be %s", value, describeFunctionSorts()),
		nil,
	)
}

// Functions returns functions that match the supplied parameters.
func Functions(graph report.Graph, query FunctionsParams) FunctionsResult {
	selection := Select(graph.Functions, func(function report.Function) bool {
		return functionMatchesQuery(function, query)
	}, functionComparison(query.Sort, query.IncludeTests), query.Limit)
	return FunctionsResult{
		Analysis:  analysisHeader(graph),
		Matched:   selection.Matched,
		Returned:  len(selection.Values),
		Functions: selection.Values,
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
		descriptor = functionSortRegistry[0]
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
		return FunctionResult{}, failure.NotFound(
			"dependency graph function",
			identifier,
			nil,
		)
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
	selection := Select(graph.FunctionCalls, func(call report.FunctionCall) bool {
		return functionCallMatchesQuery(call, query)
	}, nil, query.Limit)
	callSites := 0
	for _, call := range selection.Values {
		callSites += call.Calls
	}
	return FunctionCallsResult{
		Analysis:  analysisHeader(graph),
		Matched:   selection.Matched,
		Returned:  len(selection.Values),
		CallSites: callSites,
		Calls:     selection.Values,
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
// {"version":1,"tested_at":"2026-08-23T21:34:41Z","module_hash":"88916104c1ddbe88dbe95e78aeb3d77086cd70f8b166496f4f873497aa9e0868","functions":[{"id":"func/ParseFunctionSort","name":"ParseFunctionSort","line":142,"end_line":152,"hash":"84c4d08395e487a658842f2fb64c5d7976ab53d61f89f9d7de9f4997830275cf"},{"id":"func/Functions","name":"Functions","line":155,"end_line":165,"hash":"db2ff5fb2e21c2b74a17ace6ab0e433bf7a869f053bb8e3a288d7ec5924da4d3"},{"id":"func/functionMatchesQuery","name":"functionMatchesQuery","line":167,"end_line":175,"hash":"22e62a56d83addb2eb9ccccd24d4a26b753bbca9a1d8af0cdde52553dc565811"},{"id":"func/functionComparison","name":"functionComparison","line":177,"end_line":189,"hash":"99cb354e13cf64bb4eedfcc16037c21782bcd895fd8eb5f6745778c4e7493ef7"},{"id":"func/functionSortDescriptorFor","name":"functionSortDescriptorFor","line":191,"end_line":198,"hash":"f63da7aa23895b1baa24f7aaa3e0349ba58e3ffecd178b6171f3cbbb78913e62"},{"id":"func/describeFunctionSorts","name":"describeFunctionSorts","line":200,"end_line":206,"hash":"32a9059db4cfa579c0ce184cb2d5d232a4d37eed5c726056266203ffbf76d20e"},{"id":"func/functionIncomingCallSites","name":"functionIncomingCallSites","line":208,"end_line":213,"hash":"e086225040ec47c33ea3d6a86e6784cf5445191fdff5ca067326a326ea71cec8"},{"id":"func/functionOutgoingCallSites","name":"functionOutgoingCallSites","line":215,"end_line":220,"hash":"6746f1eaee26b6b01c23c0d43cb9435facd2267445f6abfe0f0db54ac516e461"},{"id":"func/functionAfferentCoupling","name":"functionAfferentCoupling","line":222,"end_line":227,"hash":"62a25eae6b1363efde96619d99c57edf48c2b4578dd1215b6223db48d7f357f6"},{"id":"func/functionEfferentCoupling","name":"functionEfferentCoupling","line":229,"end_line":234,"hash":"adb37c4341117d020038b33f404881c7bc634a1986851c32050eba74fecc012b"},{"id":"func/GetFunction","name":"GetFunction","line":237,"end_line":265,"hash":"bd39b460474a9deeba15fd5fc97f5713dfa5c9b3bd8b5013c82dad0fa01bb31d"},{"id":"func/findFunction","name":"findFunction","line":267,"end_line":274,"hash":"52ddee9d8e90501c1c0206d07c50bed6236c9c17d845311b4a6bb01194015b4f"},{"id":"func/FunctionCalls","name":"FunctionCalls","line":277,"end_line":292,"hash":"9afcd562a4eadeb012ba581d12e1a77839ac9f43b105d74a59c9dc6040cd8762"},{"id":"func/functionCallMatchesQuery","name":"functionCallMatchesQuery","line":294,"end_line":302,"hash":"f4918674e7845a62a3c442b9125daaf0fe86abd0162981b33b9ec469e1aea24e"}]}
// mutate4go-manifest-end
