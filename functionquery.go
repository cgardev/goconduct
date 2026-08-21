package main

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var errFunctionNotFound = errors.New("function not found")

type functionSort string

const (
	functionSortIdentifier        functionSort = "identifier"
	functionSortIncomingCalls     functionSort = "incoming-calls"
	functionSortOutgoingCalls     functionSort = "outgoing-calls"
	functionSortAfferent          functionSort = "afferent"
	functionSortEfferent          functionSort = "efferent"
	functionSortTransitiveCallers functionSort = "transitive-callers"
	functionSortTransitiveCallees functionSort = "transitive-callees"
	functionSortInstability       functionSort = "instability"
)

type functionsQuery struct {
	component    string
	packagePath  string
	sort         functionSort
	includeTests bool
	limit        int
}

type functionsQueryResult struct {
	Analysis  analysisQueryHeader `json:"analysis"`
	Matched   int                 `json:"matched"`
	Returned  int                 `json:"returned"`
	Functions []Function          `json:"functions"`
}

type functionQueryResult struct {
	Analysis analysisQueryHeader `json:"analysis"`
	Function Function            `json:"function"`
	Callers  []FunctionCall      `json:"callers"`
	Callees  []FunctionCall      `json:"callees"`
}

type functionCallsQuery struct {
	sourceComponent string
	targetComponent string
	sourceFunction  string
	targetFunction  string
	includeTests    bool
	limit           int
}

type functionCallsQueryResult struct {
	Analysis  analysisQueryHeader `json:"analysis"`
	Matched   int                 `json:"matched"`
	Returned  int                 `json:"returned"`
	CallSites int                 `json:"callSites"`
	Calls     []FunctionCall      `json:"calls"`
}

func parseFunctionSort(value string) (functionSort, error) {
	sortOrder := functionSort(value)
	switch sortOrder {
	case functionSortIdentifier,
		functionSortIncomingCalls,
		functionSortOutgoingCalls,
		functionSortAfferent,
		functionSortEfferent,
		functionSortTransitiveCallers,
		functionSortTransitiveCallees,
		functionSortInstability:
		return sortOrder, nil
	default:
		return "", fmt.Errorf(
			"function sort %q must be identifier, incoming-calls, outgoing-calls, afferent, "+
				"efferent, transitive-callers, transitive-callees, or instability",
			value,
		)
	}
}

func queryFunctions(graph Graph, query functionsQuery) functionsQueryResult {
	functions := make([]Function, 0)
	for _, function := range graph.Functions {
		if !functionMatchesQuery(function, query) {
			continue
		}
		functions = append(functions, function)
	}
	slices.SortFunc(functions, functionComparison(query.sort, query.includeTests))
	matched := len(functions)
	functions = applyLimit(functions, query.limit)
	return functionsQueryResult{
		Analysis:  queryHeader(graph),
		Matched:   matched,
		Returned:  len(functions),
		Functions: functions,
	}
}

func functionMatchesQuery(function Function, query functionsQuery) bool {
	if !query.includeTests && function.Test {
		return false
	}
	if query.component != "" && function.Component != query.component {
		return false
	}
	return query.packagePath == "" || function.Package == query.packagePath
}

func functionComparison(sortOrder functionSort, includeTests bool) func(Function, Function) int {
	return func(first, second Function) int {
		var result int
		switch sortOrder {
		case functionSortIncomingCalls:
			result = cmp.Compare(
				functionIncomingCalls(second, includeTests),
				functionIncomingCalls(first, includeTests),
			)
		case functionSortOutgoingCalls:
			result = cmp.Compare(
				functionOutgoingCalls(second, includeTests),
				functionOutgoingCalls(first, includeTests),
			)
		case functionSortAfferent:
			result = cmp.Compare(
				functionAfferentCoupling(second, includeTests),
				functionAfferentCoupling(first, includeTests),
			)
		case functionSortEfferent:
			result = cmp.Compare(
				functionEfferentCoupling(second, includeTests),
				functionEfferentCoupling(first, includeTests),
			)
		case functionSortTransitiveCallers:
			result = cmp.Compare(second.TransitiveCallerFunctions, first.TransitiveCallerFunctions)
		case functionSortTransitiveCallees:
			result = cmp.Compare(second.TransitiveCalledFunctions, first.TransitiveCalledFunctions)
		case functionSortInstability:
			result = cmp.Compare(second.Instability, first.Instability)
		}
		return cmp.Or(result, strings.Compare(first.Identifier, second.Identifier))
	}
}

func functionIncomingCalls(function Function, includeTests bool) int {
	if includeTests {
		return function.IncomingCallSites + function.TestIncomingCallSites
	}
	return function.IncomingCallSites
}

func functionOutgoingCalls(function Function, includeTests bool) int {
	if includeTests {
		return function.OutgoingCallSites + function.TestOutgoingCallSites
	}
	return function.OutgoingCallSites
}

func functionAfferentCoupling(function Function, includeTests bool) int {
	if includeTests {
		return function.AfferentCoupling + function.TestAfferentCoupling
	}
	return function.AfferentCoupling
}

func functionEfferentCoupling(function Function, includeTests bool) int {
	if includeTests {
		return function.EfferentCoupling + function.TestEfferentCoupling
	}
	return function.EfferentCoupling
}

func queryFunction(graph Graph, identifier string, includeTests bool) (functionQueryResult, error) {
	function, found := findFunction(graph.Functions, identifier)
	if !found {
		return functionQueryResult{}, fmt.Errorf("%w: %s", errFunctionNotFound, identifier)
	}
	callers := make([]FunctionCall, 0)
	callees := make([]FunctionCall, 0)
	for _, call := range graph.FunctionCalls {
		if call.TestOnly && !includeTests {
			continue
		}
		if call.Target == identifier {
			callers = append(callers, call)
		}
		if call.Source == identifier {
			callees = append(callees, call)
		}
	}
	return functionQueryResult{
		Analysis: queryHeader(graph),
		Function: function,
		Callers:  callers,
		Callees:  callees,
	}, nil
}

func findFunction(functions []Function, identifier string) (Function, bool) {
	for _, function := range functions {
		if function.Identifier == identifier {
			return function, true
		}
	}
	return Function{}, false
}

func queryFunctionCalls(graph Graph, query functionCallsQuery) functionCallsQueryResult {
	calls := make([]FunctionCall, 0)
	for _, call := range graph.FunctionCalls {
		if functionCallMatchesQuery(call, query) {
			calls = append(calls, call)
		}
	}
	matched := len(calls)
	calls = applyLimit(calls, query.limit)
	callSites := 0
	for _, call := range calls {
		callSites += call.Calls
	}
	return functionCallsQueryResult{
		Analysis:  queryHeader(graph),
		Matched:   matched,
		Returned:  len(calls),
		CallSites: callSites,
		Calls:     calls,
	}
}

func functionCallMatchesQuery(call FunctionCall, query functionCallsQuery) bool {
	if call.TestOnly && !query.includeTests {
		return false
	}
	return (query.sourceComponent == "" || call.SourceComponent == query.sourceComponent) &&
		(query.targetComponent == "" || call.TargetComponent == query.targetComponent) &&
		(query.sourceFunction == "" || call.Source == query.sourceFunction) &&
		(query.targetFunction == "" || call.Target == query.targetFunction)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T10:21:42Z","module_hash":"08a550a11e1a66d19b2715b6eb3d68333adf3d77e579f7774d30944ee3f9b49c","functions":[{"id":"func/parseFunctionSort","name":"parseFunctionSort","line":65,"end_line":84,"hash":"1feea21b0553833d47a5e679fbc469f4be662d160d3b875d0954a44cbcdffe08"},{"id":"func/queryFunctions","name":"queryFunctions","line":86,"end_line":103,"hash":"85f2bb5d81b134a5c4005af00d7c7de7861b366bd698a4170fc299aa209cb438"},{"id":"func/functionMatchesQuery","name":"functionMatchesQuery","line":105,"end_line":113,"hash":"b7e59a0c78d7205838a94d1b5c74845cf8aa40d9e3cf81802c9e37b22ebd7499"},{"id":"func/functionComparison","name":"functionComparison","line":115,"end_line":148,"hash":"1b2596e8dde7bc34a980445c41a231dd2968102f60ddbd166b41f7890e37296a"},{"id":"func/functionIncomingCalls","name":"functionIncomingCalls","line":150,"end_line":155,"hash":"b14167fdd30bdecc7834fe0f5227dfeed2dc7b620a1a616e1463f8d9796ab061"},{"id":"func/functionOutgoingCalls","name":"functionOutgoingCalls","line":157,"end_line":162,"hash":"c9a5f3efdb5a07d44a43bc1674eb01764e84a45b46a369332630203b72ff30e3"},{"id":"func/functionAfferentCoupling","name":"functionAfferentCoupling","line":164,"end_line":169,"hash":"1f1ec799095be1da5e1a332409ea6631065d8f4f2b227c6ec6b73358bc81c2a4"},{"id":"func/functionEfferentCoupling","name":"functionEfferentCoupling","line":171,"end_line":176,"hash":"329bf5aa4a72114f83fc1dcf65ba056518da2a267a77a7f24818f64589005fa3"},{"id":"func/queryFunction","name":"queryFunction","line":178,"end_line":202,"hash":"ed0c4cfc1bd76a51284ab5643fb85bdcc73d99affd0aaad5c72199d92e8a4313"},{"id":"func/findFunction","name":"findFunction","line":204,"end_line":211,"hash":"c85d3325debcc18cdb4c72deb83f2640130f0cb840afe0e7f6d59b5311db7247"},{"id":"func/queryFunctionCalls","name":"queryFunctionCalls","line":213,"end_line":233,"hash":"0f1d56367b2e81a321b0a69080889c5494b8956cdec17df43970997a868f5380"},{"id":"func/functionCallMatchesQuery","name":"functionCallMatchesQuery","line":235,"end_line":243,"hash":"75abd805a9674542dbf04aadc76f6b0a4bd61b894bdd3ae312b4f4d336330cf0"}]}
// mutate4go-manifest-end
