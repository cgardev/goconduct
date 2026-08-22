package architecture

import calculationdomain "github.com/cgardev/goconduct/internal/calculation"

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

type functionCallKey struct {
	source string
	target string
	test   bool
}

func calculateFunctionGraph(
	declarations []functionDeclaration,
	references []functionReference,
) ([]Function, []FunctionCall, [][]string) {
	calculatedDeclarations := make([]calculationdomain.FunctionDeclaration, len(declarations))
	for index, declaration := range declarations {
		calculatedDeclarations[index] = calculationFunctionDeclaration(declaration)
	}
	calculatedReferences := make([]calculationdomain.FunctionReference, len(references))
	for index, reference := range references {
		calculatedReferences[index] = calculationdomain.FunctionReference{
			Source: reference.source,
			Target: reference.target,
			Test:   reference.test,
			Site:   reference.site,
		}
	}
	return calculationdomain.CalculateFunctionGraph(calculatedDeclarations, calculatedReferences)
}

func mergeFunctionDeclarations(first, second functionDeclaration) functionDeclaration {
	return localFunctionDeclaration(calculationdomain.MergeFunctionDeclarations(
		calculationFunctionDeclaration(first),
		calculationFunctionDeclaration(second),
	))
}

func compareFunctionCallKeys(first, second functionCallKey) int {
	return calculationdomain.CompareFunctionCallKeys(
		calculationdomain.FunctionCallKey{
			Source: first.source,
			Target: first.target,
			Test:   first.test,
		},
		calculationdomain.FunctionCallKey{
			Source: second.source,
			Target: second.target,
			Test:   second.test,
		},
	)
}

func attachFunctionMetrics(graph *Graph) {
	calculationdomain.AttachFunctionMetrics(graph)
}

func calculationFunctionDeclaration(
	declaration functionDeclaration,
) calculationdomain.FunctionDeclaration {
	return calculationdomain.FunctionDeclaration{
		Identifier:      declaration.identifier,
		Name:            declaration.name,
		PackagePath:     declaration.packagePath,
		Component:       declaration.component,
		RelativePath:    declaration.relativePath,
		Line:            declaration.line,
		Receiver:        declaration.receiver,
		Method:          declaration.method,
		Exported:        declaration.exported,
		Test:            declaration.test,
		Synthetic:       declaration.synthetic,
		InAnalysisScope: declaration.inAnalysisScope,
		SourcePosition:  declaration.sourcePosition,
	}
}

func localFunctionDeclaration(
	declaration calculationdomain.FunctionDeclaration,
) functionDeclaration {
	return functionDeclaration{
		identifier:      declaration.Identifier,
		name:            declaration.Name,
		packagePath:     declaration.PackagePath,
		component:       declaration.Component,
		relativePath:    declaration.RelativePath,
		line:            declaration.Line,
		receiver:        declaration.Receiver,
		method:          declaration.Method,
		exported:        declaration.Exported,
		test:            declaration.Test,
		synthetic:       declaration.Synthetic,
		inAnalysisScope: declaration.InAnalysisScope,
		sourcePosition:  declaration.SourcePosition,
	}
}
