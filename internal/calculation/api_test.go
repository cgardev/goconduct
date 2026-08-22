package calculation

import (
	"reflect"
	"slices"
	"testing"

	"github.com/cgardev/goconduct/internal/architecture"
)

func TestAPI_CollectAndBuildComponentGraph(t *testing.T) {
	t.Run("Scenario: Analyzer facts enter the calculation package through its public API", func(t *testing.T) {
		var components map[string]*ComponentAccumulator
		var relationships map[RelationshipKey]*RelationshipAccumulator
		var graph Graph
		var application ComponentDescriptor
		var library ComponentDescriptor
		var development ComponentDescriptor

		t.Run("Given production and test files with classified imports", func(*testing.T) {
			components = make(map[string]*ComponentAccumulator)
			relationships = make(map[RelationshipKey]*RelationshipAccumulator)
			application = ComponentDescriptor{
				Identifier:  "cmd/control",
				Name:        "control",
				Role:        architecture.RoleApplication,
				Category:    "application",
				Application: "control",
			}
			library = ComponentDescriptor{
				Identifier: "internal/library/data",
				Name:       "data",
				Role:       architecture.RoleLibrary,
			}
			development = ComponentDescriptor{
				Identifier: "internal/devtool/generator",
				Name:       "generator",
				Role:       architecture.RoleDevelopment,
				Category:   "development-tool",
			}

			production := SourceFile{
				RelativePath:  "cmd/control/main.go",
				PackagePath:   "cmd/control",
				Component:     application,
				AbstractTypes: 1,
				ConcreteTypes: 3,
				Imports: []SourceImport{
					{
						PackagePath: "cmd/control/internal",
						Component:   application,
						Site:        ImportSite{Path: "cmd/control/main.go", Line: 7},
					},
					{
						PackagePath: "internal/library/data",
						Component:   library,
						Site: ImportSite{
							SourcePackage: "cmd/control",
							TargetPackage: "internal/library/data",
							Path:          "cmd/control/main.go",
							Line:          12,
							Alias:         "records",
						},
					},
					{
						PackagePath: "internal/devtool/generator",
						Component:   development,
						Site: ImportSite{
							SourcePackage: "cmd/control",
							TargetPackage: "internal/devtool/generator",
							Path:          "cmd/control/main.go",
							Line:          9,
						},
					},
				},
			}
			testFile := SourceFile{
				RelativePath: "cmd/control/main_test.go",
				PackagePath:  "cmd/control",
				Component:    application,
				Test:         true,
				Imports: []SourceImport{{
					PackagePath: "internal/library/data/testfixture",
					Component:   library,
					Site: ImportSite{
						SourcePackage: "cmd/control",
						TargetPackage: "internal/library/data/testfixture",
						Path:          "cmd/control/main_test.go",
						Line:          5,
						Test:          true,
					},
				}},
			}
			libraryFile := SourceFile{
				RelativePath:  "internal/library/data/data.go",
				PackagePath:   "internal/library/data",
				Component:     library,
				ConcreteTypes: 2,
			}

			CollectComponentFile(components, production)
			CollectComponentFile(components, testFile)
			CollectComponentFile(components, libraryFile)
			CollectRelationships(components, relationships, production)
			CollectRelationships(components, relationships, testFile)
		})

		if !t.Run("When the public builder calculates the report", func(t *testing.T) {
			existing := GetOrCreateComponent(components, library)
			if existing != components[library.Identifier] {
				t.Fatal("the existing component accumulator changed")
			}
			created := GetOrCreateComponent(components, development)
			if created != components[development.Identifier] {
				t.Fatal("the created component accumulator is absent")
			}
			graph = BuildGraph(
				"example.com/repository",
				components,
				relationships,
				[]Diagnostic{
					{Path: "z.go", Message: "second"},
					{Path: "a.go", Message: "first"},
				},
			)
			if graph.ModulePath != "example.com/repository" {
				t.Fatalf("module path is %q", graph.ModulePath)
			}
		}) {
			return
		}

		t.Run("Then component descriptors and source counters survive conversion", func(t *testing.T) {
			control := apiTestComponentByIdentifier(t, graph, application.Identifier)
			if control.Name != application.Name || control.Role != application.Role ||
				control.Category != application.Category || control.Application != application.Application {
				t.Errorf("component descriptor changed: %+v", control)
			}
			if control.Packages != 1 || control.SourceFiles != 2 || control.ProductionFiles != 1 ||
				control.TestFiles != 1 || control.AbstractTypes != 1 || control.ConcreteTypes != 3 {
				t.Errorf("component source counters are incorrect: %+v", control)
			}
			data := apiTestComponentByIdentifier(t, graph, library.Identifier)
			if data.Category != string(architecture.RoleLibrary) || data.ConcreteTypes != 2 {
				t.Errorf("library fallback category or type count is incorrect: %+v", data)
			}
		})

		t.Run("And relationships retain production, test, package, and import-site facts", func(t *testing.T) {
			dependency := apiTestRelationship(t, graph, application.Identifier, library.Identifier)
			if dependency.ProductionReferencingFiles != 1 || dependency.TestReferencingFiles != 1 ||
				dependency.TestOnly {
				t.Errorf("relationship file counters are incorrect: %+v", dependency)
			}
			if !slices.Equal(dependency.SourcePackages, []string{"cmd/control"}) ||
				!slices.Equal(
					dependency.TargetPackages,
					[]string{"internal/library/data", "internal/library/data/testfixture"},
				) {
				t.Errorf("relationship packages are incorrect: %+v", dependency)
			}
			if len(dependency.ImportSites) != 2 || dependency.ImportSites[0].Path != "cmd/control/main.go" ||
				dependency.ImportSites[0].Alias != "records" || dependency.ImportSites[1].Line != 5 {
				t.Errorf("relationship import sites are incorrect: %+v", dependency.ImportSites)
			}
		})

		t.Run("And diagnostics, policy, findings, and summary are deterministic", func(t *testing.T) {
			if len(graph.Diagnostics) != 2 || graph.Diagnostics[0].Path != "a.go" {
				t.Errorf("diagnostics are not sorted: %+v", graph.Diagnostics)
			}
			if graph.Policy.InstabilityFormula != "Ce/(Ca+Ce)" || graph.Summary.Components != 3 ||
				graph.Summary.Relationships != 2 {
				t.Errorf("policy or summary is incorrect: %+v %+v", graph.Policy, graph.Summary)
			}
			if graph.Policy.IsolatedInstability != 0 || graph.Policy.UntypedAbstractness != 0 ||
				graph.Policy.StableLowAbstraction.MinimumAfferentCoupling != 1 ||
				!graph.Policy.StableDependencyPrinciple.ProductionOnly {
				t.Errorf("numeric or scope policy is incorrect: %+v", graph.Policy)
			}
			if len(graph.Findings) == 0 {
				t.Error("default rules did not produce findings")
			}
		})
	})
}

func TestAPI_ExposeRulesAndGraphAlgorithms(t *testing.T) {
	t.Run("Scenario: A caller uses graph utilities without analyzer types", func(t *testing.T) {
		var graph Graph
		var violations []string
		var reachableNodes StringSet
		var cycles [][]string

		t.Run("Given a component relationship and one directed cycle", func(*testing.T) {
			graph = Graph{
				Components: []Component{
					{Identifier: "source", Instability: 0.1},
					{Identifier: "target", Instability: 0.9},
				},
				Relationships: []Relationship{{Source: "source", Target: "target"}},
			}
			adjacency := NewAdjacency([]string{"a", "b", "c"})
			adjacency["a"].add("b")
			adjacency["b"].add("a")
			adjacency["b"].add("c")
			reachableNodes = Reachable("a", adjacency)
			cycles = StronglyConnectedComponents([]string{"a", "b", "c"}, adjacency)
		})

		t.Run("When the public rule helpers evaluate the graph", func(*testing.T) {
			violations = RelationshipRuleViolations(
				ComponentDescriptor{Role: architecture.RoleApplication},
				ComponentDescriptor{Role: architecture.RoleDevelopment},
				false,
			)
			AnnotateStableDependencyPrincipleViolations(graph.Relationships, graph.Components)
			ApplyArchitectureRules(&graph, architecture.NewRegistry(apiTestArchitectureRule{}))
			graph.Summary = SummarizeGraph(graph)
		})

		t.Run("Then the public rule helpers preserve rule results", func(t *testing.T) {
			if !slices.Equal(violations, []string{"production-imports-development"}) {
				t.Errorf("relationship violations are %v", violations)
			}
			if len(graph.Findings) != 1 || graph.Findings[0].Rule != "api-test-rule" ||
				!slices.Equal(
					graph.Relationships[0].RuleViolations,
					[]string{"api-test-rule", "stable-dependency-principle"},
				) {
				t.Errorf("custom API rule result is incorrect: %+v", graph)
			}
			if graph.Summary.Findings != 1 || graph.Summary.Warnings != 1 ||
				graph.Summary.RelationshipRuleViolations != 2 {
				t.Errorf("public summary is incorrect: %+v", graph.Summary)
			}
		})

		t.Run("And the public graph algorithms return sorted deterministic values", func(t *testing.T) {
			if !slices.Equal(SortedSet(reachableNodes), []string{"b", "c"}) {
				t.Errorf("reachable nodes are %v", SortedSet(reachableNodes))
			}
			if !reflect.DeepEqual(cycles, [][]string{{"a", "b"}}) {
				t.Errorf("cycles are %v", cycles)
			}
			set := NewStringSet("z", "a", "z")
			if !slices.Equal(SortedSet(set), []string{"a", "z"}) {
				t.Errorf("sorted set is %v", SortedSet(set))
			}
		})

		t.Run("And default finding detection remains available", func(t *testing.T) {
			findings := DetectFindings(Graph{Diagnostics: []Diagnostic{{Path: "bad.go", Message: "bad"}}})
			if len(findings) != 1 || findings[0].Rule != "source-diagnostic" {
				t.Errorf("default findings are %+v", findings)
			}
			if testViolations := RelationshipRuleViolations(
				ComponentDescriptor{Role: architecture.RoleApplication},
				ComponentDescriptor{Role: architecture.RoleDevelopment},
				true,
			); len(testViolations) != 0 {
				t.Errorf("test-only violations are %v", testViolations)
			}
		})
	})
}

func TestAPI_ConvertFunctionFactsAndAttachMetrics(t *testing.T) {
	t.Run("Scenario: Function analyzer facts enter through the public calculation API", func(t *testing.T) {
		var functions []Function
		var calls []FunctionCall
		var cycles [][]string
		var merged FunctionDeclaration
		var graph Graph

		t.Run("Given complete declarations and production and test calls", func(*testing.T) {
			declarations := []FunctionDeclaration{
				{
					Identifier:      "cmd/control.Service.Start",
					Name:            "Start",
					PackagePath:     "cmd/control",
					Component:       "cmd/control",
					RelativePath:    "cmd/control/service.go",
					Line:            10,
					Receiver:        "Service",
					Method:          true,
					Exported:        true,
					InAnalysisScope: true,
					SourcePosition:  100,
				},
				{
					Identifier:      "internal/library/data.Read",
					Name:            "Read",
					PackagePath:     "internal/library/data",
					Component:       "internal/library/data",
					RelativePath:    "internal/library/data/read.go",
					Line:            20,
					Synthetic:       true,
					InAnalysisScope: true,
					SourcePosition:  200,
				},
			}
			references := []FunctionReference{
				{
					Source: "cmd/control.Service.Start",
					Target: "internal/library/data.Read",
					Site:   CallSite{Path: "cmd/control/service.go", Line: 15, Column: 2},
				},
				{
					Source: "cmd/control.Service.Start",
					Target: "internal/library/data.Read",
					Test:   true,
					Site:   CallSite{Path: "cmd/control/service_test.go", Line: 7, Column: 3},
				},
			}
			functions, calls, cycles = CalculateFunctionGraph(declarations, references)
			merged = MergeFunctionDeclarations(
				FunctionDeclaration{Identifier: "duplicate", InAnalysisScope: false},
				declarations[0],
			)
		})

		t.Run("When the public API attaches function metrics", func(*testing.T) {
			graph = Graph{
				Components: []Component{
					{Identifier: "cmd/control", Application: "control"},
					{Identifier: "internal/library/data"},
				},
				Relationships: []Relationship{{
					Source: "cmd/control",
					Target: "internal/library/data",
				}},
				Functions:      functions,
				FunctionCalls:  calls,
				FunctionCycles: cycles,
			}
			AttachFunctionMetrics(&graph)
		})

		t.Run("Then declaration fields and call sites survive both conversions", func(t *testing.T) {
			start := apiTestFunctionByIdentifier(t, graph, "cmd/control.Service.Start")
			if start.Name != "Start" || start.Package != "cmd/control" ||
				start.Path != "cmd/control/service.go" || start.Line != 10 ||
				start.Receiver != "Service" || !start.Method || !start.Exported ||
				!start.InAnalysisScope {
				t.Errorf("function declaration changed: %+v", start)
			}
			if len(graph.FunctionCalls) != 2 || graph.FunctionCalls[0].TestOnly ||
				!graph.FunctionCalls[1].TestOnly || graph.FunctionCalls[0].CallSites[0].Column != 2 {
				t.Errorf("function references changed: %+v", graph.FunctionCalls)
			}
			if merged.Identifier != "cmd/control.Service.Start" || merged.SourcePosition != 100 ||
				merged.Receiver != "Service" || !merged.Method || !merged.Exported {
				t.Errorf("merged declaration changed: %+v", merged)
			}
		})

		t.Run("And function metrics reach components and relationships", func(t *testing.T) {
			control := apiTestComponentByIdentifier(t, graph, "cmd/control")
			data := apiTestComponentByIdentifier(t, graph, "internal/library/data")
			relationship := graph.Relationships[0]
			if control.ProductionFunctions != 1 || control.ProductionOutgoingCallSites != 1 ||
				control.TestOutgoingCallSites != 1 || data.ProductionIncomingCallSites != 1 ||
				data.TestIncomingCallSites != 1 {
				t.Errorf("component function metrics are incorrect: %+v %+v", control, data)
			}
			if relationship.ProductionFunctionCallSites != 1 || relationship.TestFunctionCallSites != 1 ||
				relationship.CallerFunctions != 1 || relationship.CalleeFunctions != 1 {
				t.Errorf("relationship function metrics are incorrect: %+v", relationship)
			}
		})

		t.Run("And the public comparator keeps production calls before test calls", func(t *testing.T) {
			production := FunctionCallKey{Source: "a", Target: "b"}
			testCall := FunctionCallKey{Source: "a", Target: "b", Test: true}
			if CompareFunctionCallKeys(production, testCall) >= 0 ||
				CompareFunctionCallKeys(testCall, production) <= 0 {
				t.Error("function call comparator did not order production before test")
			}
		})
	})
}

type apiTestArchitectureRule struct{}

func (apiTestArchitectureRule) Evaluate(architecture.Graph) []architecture.Finding {
	return []architecture.Finding{{
		Rule:     "api-test-rule",
		Severity: architecture.SeverityWarning,
		Subject:  "source -> target",
		Message:  "The API test rejects this dependency.",
		Source:   "source",
		Target:   "target",
	}}
}

func apiTestComponentByIdentifier(t *testing.T, graph Graph, identifier string) Component {
	t.Helper()
	for _, component := range graph.Components {
		if component.Identifier == identifier {
			return component
		}
	}
	t.Fatalf("component %q does not exist", identifier)
	return Component{}
}

func apiTestRelationship(t *testing.T, graph Graph, source, target string) Relationship {
	t.Helper()
	for _, relationship := range graph.Relationships {
		if relationship.Source == source && relationship.Target == target {
			return relationship
		}
	}
	t.Fatalf("relationship %q -> %q does not exist", source, target)
	return Relationship{}
}

func apiTestFunctionByIdentifier(t *testing.T, graph Graph, identifier string) Function {
	t.Helper()
	for _, function := range graph.Functions {
		if function.Identifier == identifier {
			return function
		}
	}
	t.Fatalf("function %q does not exist", identifier)
	return Function{}
}
