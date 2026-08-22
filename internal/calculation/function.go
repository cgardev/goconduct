package calculation

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

type functionDeclaration struct {
	identifier      string
	name            string
	packagePath     string
	component       string
	relativePath    string
	line            int
	receiver        string
	method          bool
	exported        bool
	test            bool
	synthetic       bool
	inAnalysisScope bool
	sourcePosition  int
}

type functionReference struct {
	source string
	target string
	test   bool
	site   CallSite
}

type functionAccumulator struct {
	declaration           functionDeclaration
	productionCallers     stringSet
	productionCallees     stringSet
	testCallers           stringSet
	testCallees           stringSet
	crossComponentCallers stringSet
	crossComponentCallees stringSet
	incomingCallSites     int
	outgoingCallSites     int
	testIncomingCallSites int
	testOutgoingCallSites int
}

type functionCallKey struct {
	source string
	target string
	test   bool
}

type functionCallAccumulator struct {
	sites map[string]CallSite
}

func calculateFunctionGraph(
	declarations []functionDeclaration,
	references []functionReference,
) ([]Function, []FunctionCall, [][]string) {
	functions := newFunctionAccumulators(declarations)
	calls := make(map[functionCallKey]*functionCallAccumulator)
	collectFunctionReferences(functions, calls, references)
	functionValues, cycles := buildFunctions(functions)
	return functionValues, buildFunctionCalls(functions, calls), cycles
}

func newFunctionAccumulators(declarations []functionDeclaration) map[string]*functionAccumulator {
	functions := make(map[string]*functionAccumulator, len(declarations))
	for _, declaration := range declarations {
		current, exists := functions[declaration.identifier]
		if exists {
			current.declaration = mergeFunctionDeclarations(current.declaration, declaration)
			continue
		}
		functions[declaration.identifier] = &functionAccumulator{
			declaration:           declaration,
			productionCallers:     make(stringSet),
			productionCallees:     make(stringSet),
			testCallers:           make(stringSet),
			testCallees:           make(stringSet),
			crossComponentCallers: make(stringSet),
			crossComponentCallees: make(stringSet),
		}
	}
	return functions
}

func mergeFunctionDeclarations(first, second functionDeclaration) functionDeclaration {
	if first.inAnalysisScope != second.inAnalysisScope {
		if second.inAnalysisScope {
			return second
		}
		return first
	}
	if compareFunctionDeclarationSource(second, first) == -1 {
		return second
	}
	return first
}

func compareFunctionDeclarationSource(first, second functionDeclaration) int {
	if first.relativePath == second.relativePath {
		return cmp.Compare(first.line, second.line)
	}
	if first.relativePath == "" {
		return 1
	}
	if second.relativePath == "" {
		return -1
	}
	return strings.Compare(first.relativePath, second.relativePath)
}

func collectFunctionReferences(
	functions map[string]*functionAccumulator,
	calls map[functionCallKey]*functionCallAccumulator,
	references []functionReference,
) {
	for _, reference := range references {
		source, sourceExists := functions[reference.source]
		target, targetExists := functions[reference.target]
		if !sourceExists || !targetExists {
			continue
		}
		collectFunctionCallSite(source, target, reference)
		if reference.source != reference.target {
			collectFunctionCoupling(source, target, reference)
		}
		collectFunctionCall(calls, reference)
	}
}

func collectFunctionCallSite(
	source *functionAccumulator,
	target *functionAccumulator,
	reference functionReference,
) {
	if reference.test {
		source.testOutgoingCallSites++
		target.testIncomingCallSites++
		return
	}
	source.outgoingCallSites++
	target.incomingCallSites++
}

func collectFunctionCoupling(
	source *functionAccumulator,
	target *functionAccumulator,
	reference functionReference,
) {
	if reference.test {
		source.testCallees.add(reference.target)
		target.testCallers.add(reference.source)
		return
	}
	source.productionCallees.add(reference.target)
	target.productionCallers.add(reference.source)
	if source.declaration.component == target.declaration.component {
		return
	}
	source.crossComponentCallees.add(reference.target)
	target.crossComponentCallers.add(reference.source)
}

func collectFunctionCall(
	calls map[functionCallKey]*functionCallAccumulator,
	reference functionReference,
) {
	key := functionCallKey{source: reference.source, target: reference.target, test: reference.test}
	call, exists := calls[key]
	if !exists {
		call = &functionCallAccumulator{sites: make(map[string]CallSite)}
		calls[key] = call
	}
	siteKey := reference.site.Path + ":" + strconv.Itoa(reference.site.Line) + ":" +
		strconv.Itoa(reference.site.Column)
	call.sites[siteKey] = reference.site
}

func buildFunctions(functionData map[string]*functionAccumulator) ([]Function, [][]string) {
	identifiers := sortedMapKeys(functionData)
	productionDependencies := functionAdjacency(functionData, false)
	productionCallers := reverseFunctionAdjacency(identifiers, productionDependencies)
	cycles := stronglyConnectedComponents(identifiers, productionDependencies)
	cycleMembers := make(stringSet)
	for _, cycle := range cycles {
		cycleMembers.addAll(newStringSet(cycle...))
	}
	functions := make([]Function, 0, len(identifiers))
	for _, identifier := range identifiers {
		data := functionData[identifier]
		declaration := data.declaration
		functions = append(functions, Function{
			Identifier:                    identifier,
			Name:                          declaration.name,
			Package:                       declaration.packagePath,
			Component:                     declaration.component,
			Path:                          declaration.relativePath,
			Line:                          declaration.line,
			Receiver:                      declaration.receiver,
			Method:                        declaration.method,
			Exported:                      declaration.exported,
			Test:                          declaration.test,
			Synthetic:                     declaration.synthetic,
			InAnalysisScope:               declaration.inAnalysisScope,
			AfferentCoupling:              len(data.productionCallers),
			EfferentCoupling:              len(data.productionCallees),
			TestAfferentCoupling:          len(data.testCallers),
			TestEfferentCoupling:          len(data.testCallees),
			IncomingCallSites:             data.incomingCallSites,
			OutgoingCallSites:             data.outgoingCallSites,
			TestIncomingCallSites:         data.testIncomingCallSites,
			TestOutgoingCallSites:         data.testOutgoingCallSites,
			CrossComponentCallerFunctions: len(data.crossComponentCallers),
			CrossComponentCalleeFunctions: len(data.crossComponentCallees),
			TransitiveCallerFunctions:     len(reachable(identifier, productionCallers)),
			TransitiveCalleeFunctions:     len(reachable(identifier, productionDependencies)),
			InCycle:                       cycleMembers.contains(identifier),
			Instability: instability(
				len(data.productionCallers),
				len(data.productionCallees),
			),
		})
	}
	return functions, cycles
}

func functionAdjacency(
	functionData map[string]*functionAccumulator,
	includeTests bool,
) map[string]stringSet {
	identifiers := sortedMapKeys(functionData)
	adjacency := newAdjacency(identifiers)
	for identifier, data := range functionData {
		adjacency[identifier].addAll(data.productionCallees)
		if includeTests {
			adjacency[identifier].addAll(data.testCallees)
		}
	}
	return adjacency
}

func reverseFunctionAdjacency(
	identifiers []string,
	dependencies map[string]stringSet,
) map[string]stringSet {
	callers := newAdjacency(identifiers)
	for source, targets := range dependencies {
		for target := range targets {
			callers[target].add(source)
		}
	}
	return callers
}

func buildFunctionCalls(
	functions map[string]*functionAccumulator,
	callData map[functionCallKey]*functionCallAccumulator,
) []FunctionCall {
	keys := make([]functionCallKey, 0, len(callData))
	for key := range callData {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, compareFunctionCallKeys)
	calls := make([]FunctionCall, 0, len(keys))
	for _, key := range keys {
		source := functions[key.source].declaration
		target := functions[key.target].declaration
		sites := sortedCallSites(callData[key].sites)
		calls = append(calls, FunctionCall{
			Source:          key.source,
			Target:          key.target,
			SourceComponent: source.component,
			TargetComponent: target.component,
			CallSites:       sites,
			Calls:           len(sites),
			TestOnly:        key.test,
			CrossComponent:  source.component != target.component,
		})
	}
	return calls
}

func compareFunctionCallKeys(first, second functionCallKey) int {
	result := cmp.Or(
		strings.Compare(first.source, second.source),
		strings.Compare(first.target, second.target),
	)
	if result != 0 || first.test == second.test {
		return result
	}
	if first.test {
		return 1
	}
	return -1
}

func sortedCallSites(siteData map[string]CallSite) []CallSite {
	sites := make([]CallSite, 0, len(siteData))
	for _, site := range siteData {
		sites = append(sites, site)
	}
	slices.SortFunc(sites, func(first, second CallSite) int {
		return cmp.Or(
			strings.Compare(first.Path, second.Path),
			cmp.Compare(first.Line, second.Line),
			cmp.Compare(first.Column, second.Column),
		)
	})
	return sites
}

func attachFunctionMetrics(graph *Graph) {
	components := make(map[string]*Component, len(graph.Components))
	for index := range graph.Components {
		component := &graph.Components[index]
		components[component.Identifier] = component
	}
	for _, function := range graph.Functions {
		attachFunctionToComponent(components[function.Component], function)
	}
	attachFunctionCalls(graph, components)
	attachFunctionApplicationUsage(graph, components)
	graph.Summary = summarizeGraph(*graph)
}

func attachFunctionApplicationUsage(graph *Graph, components map[string]*Component) {
	callers := make(map[string]stringSet, len(graph.Functions))
	functionComponents := make(map[string]string, len(graph.Functions))
	for _, function := range graph.Functions {
		callers[function.Identifier] = make(stringSet)
		functionComponents[function.Identifier] = function.Component
	}
	for _, call := range graph.FunctionCalls {
		targetCallers, targetExists := callers[call.Target]
		if !call.TestOnly && call.Source != call.Target && targetExists {
			targetCallers.add(call.Source)
		}
	}
	for index := range graph.Functions {
		function := &graph.Functions[index]
		usingFunctions := reachable(function.Identifier, callers)
		usingFunctions.add(function.Identifier)
		applications := make(stringSet)
		for identifier := range usingFunctions {
			component := components[functionComponents[identifier]]
			if component != nil && component.Application != "" {
				applications.add(component.Application)
			}
		}
		function.UsingApplications = sortedSet(applications)
		function.UsingApplicationCount = len(function.UsingApplications)
	}
}

func attachFunctionToComponent(component *Component, function Function) {
	if component == nil || !function.InAnalysisScope {
		return
	}
	if function.Test {
		component.TestFunctions++
		return
	}
	component.ProductionFunctions++
}

func attachFunctionCalls(graph *Graph, components map[string]*Component) {
	relationships := make(map[relationshipKey]*Relationship, len(graph.Relationships))
	callerFunctions := make(map[relationshipKey]stringSet)
	calleeFunctions := make(map[relationshipKey]stringSet)
	for index := range graph.Relationships {
		relationship := &graph.Relationships[index]
		key := relationshipKey{source: relationship.Source, target: relationship.Target}
		relationships[key] = relationship
		callerFunctions[key] = make(stringSet)
		calleeFunctions[key] = make(stringSet)
	}
	for _, call := range graph.FunctionCalls {
		attachCallToComponents(components, call)
		if !call.CrossComponent {
			continue
		}
		key := relationshipKey{source: call.SourceComponent, target: call.TargetComponent}
		relationship := relationships[key]
		if relationship == nil {
			continue
		}
		if call.TestOnly {
			relationship.TestFunctionCallSites += call.Calls
		} else {
			relationship.ProductionFunctionCallSites += call.Calls
		}
		callerFunctions[key].add(call.Source)
		calleeFunctions[key].add(call.Target)
	}
	for key, relationship := range relationships {
		relationship.CallerFunctions = len(callerFunctions[key])
		relationship.CalleeFunctions = len(calleeFunctions[key])
	}
}

func attachCallToComponents(components map[string]*Component, call FunctionCall) {
	source := components[call.SourceComponent]
	target := components[call.TargetComponent]
	if source == nil || target == nil {
		return
	}
	if call.TestOnly {
		source.TestOutgoingCallSites += call.Calls
		target.TestIncomingCallSites += call.Calls
		return
	}
	source.ProductionOutgoingCallSites += call.Calls
	target.ProductionIncomingCallSites += call.Calls
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:16:00Z","module_hash":"693c8b63bece32efe1ab6eee45833cac0465fa26aa910a0756c0085db6696367","functions":[{"id":"func/calculateFunctionGraph","name":"calculateFunctionGraph","line":57,"end_line":66,"hash":"a72f3dae7165acda271bdf26da1eaeb7841876447ad7a7f675b9de8d5c43678c"},{"id":"func/newFunctionAccumulators","name":"newFunctionAccumulators","line":68,"end_line":87,"hash":"03542aceab7ce5cc9fcb65e3d006f8c950bf55d98dd4e7d955c07f83eb9999c6"},{"id":"func/mergeFunctionDeclarations","name":"mergeFunctionDeclarations","line":89,"end_line":100,"hash":"42a3df86f7f28027ad9db03cf1369f3b91f2bd5055ed8d32a78763f03ee69805"},{"id":"func/compareFunctionDeclarationSource","name":"compareFunctionDeclarationSource","line":102,"end_line":113,"hash":"177ea326df01e5c2c3ec1b10ed1c1fe2fb027e4f013390e7209894c5f10eba97"},{"id":"func/collectFunctionReferences","name":"collectFunctionReferences","line":115,"end_line":132,"hash":"ef61db8381c387a08009f1778b9da8cbb3d1f5433a3566fa3c7874b58fe4c0fc"},{"id":"func/collectFunctionCallSite","name":"collectFunctionCallSite","line":134,"end_line":146,"hash":"3f405d3e9373f9cef7d23acf2efb5f5e395b66f2baf104fef58ea2cf3e70539b"},{"id":"func/collectFunctionCoupling","name":"collectFunctionCoupling","line":148,"end_line":165,"hash":"1ebab62852773dd37dd9cb77b72c2cefa7a35b14824e064bd0bc6c80891ed71b"},{"id":"func/collectFunctionCall","name":"collectFunctionCall","line":167,"end_line":180,"hash":"d48ee773d1b8794700c4677cb0f707d950f82465b774559020ff01ec93604a14"},{"id":"func/buildFunctions","name":"buildFunctions","line":182,"end_line":228,"hash":"21387d851f375217ef9579eeb265dbbe230b2abae036c682cdead84e7692e631"},{"id":"func/functionAdjacency","name":"functionAdjacency","line":230,"end_line":243,"hash":"ae84aaba78b449c9ab47fbc2c51f661dc1626c17d4742110cd455007c8cde374"},{"id":"func/reverseFunctionAdjacency","name":"reverseFunctionAdjacency","line":245,"end_line":256,"hash":"f1d79214ba8f5fe9daef69de70e87895441dc9f763ce694aca52d4a56f4a8f50"},{"id":"func/buildFunctionCalls","name":"buildFunctionCalls","line":258,"end_line":284,"hash":"fbe5c29e12bb4c3a82552b4536f2f15dfb060fc017aea8ae1d1864166f0b9c29"},{"id":"func/compareFunctionCallKeys","name":"compareFunctionCallKeys","line":286,"end_line":298,"hash":"52616209dd2fa52b8ac7af050b6ab2097c240e03cb50c6b8a7840dcd4aff5e12"},{"id":"func/sortedCallSites","name":"sortedCallSites","line":300,"end_line":313,"hash":"a82ad55f41f29f8ffe5e10ce7441b7229864a73c7cc85b41dd0ec3b3c2eb262e"},{"id":"func/attachFunctionMetrics","name":"attachFunctionMetrics","line":315,"end_line":327,"hash":"a674e3a5225dadc70af61eb1930fa861327262a5e5d66800aa11b8ecf87d3282"},{"id":"func/attachFunctionApplicationUsage","name":"attachFunctionApplicationUsage","line":329,"end_line":356,"hash":"3e3fdef9fa3b358fca10f4d888088c6768c87c79881c2b6a52bf423d3b5567ad"},{"id":"func/attachFunctionToComponent","name":"attachFunctionToComponent","line":358,"end_line":367,"hash":"fe8bc0b651c20ac25a0235625a82f646430bd3bf602cbbce2ba13039a7dc8347"},{"id":"func/attachFunctionCalls","name":"attachFunctionCalls","line":369,"end_line":402,"hash":"b214a08338e89b83ad2ec7b3680d3164d77a7e7ad0e6a33c91fb0134960972a6"},{"id":"func/attachCallToComponents","name":"attachCallToComponents","line":404,"end_line":417,"hash":"3f96a3995699a1bda7a0d5962aa6c1676834dec4b648a369069e7a92dbc7417c"}]}
// mutate4go-manifest-end
