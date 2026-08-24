package architecture

import (
	"context"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cgardev/goconduct/pkg/failure"
)

func fixtureAnalysisConfiguration(repositoryRoot string) AnalysisConfiguration {
	configuration := DefaultApplicationConfiguration().Analysis
	configuration.RepositoryRoot = repositoryRoot
	return configuration
}

func TestAnalyzer_StopCanceledAnalysis(t *testing.T) {
	t.Run("Scenario: The analysis context ends before repository inspection", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var analysisContext context.Context
		var analysisError error

		t.Run("Given an analyzer with a canceled context", func(*testing.T) {
			sourceAnalyzer = &analyzer{}
			var cancel context.CancelFunc
			analysisContext, cancel = context.WithCancel(t.Context())
			cancel()
		})

		t.Run("When the analyzer starts the repository analysis", func(*testing.T) {
			_, analysisError = sourceAnalyzer.analyze(analysisContext)
		})

		t.Run("Then the analyzer returns the context cancellation", func(t *testing.T) {
			if !errors.Is(analysisError, context.Canceled) {
				t.Fatalf("analysis error is %v, want context.Canceled", analysisError)
			}
		})
	})
}

func TestSourcePaths_StopCanceledDiscovery(t *testing.T) {
	t.Run("Scenario: The source discovery context ends before the repository walk", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var discoveryContext context.Context
		var discoveryError error

		t.Run("Given an analyzer with a canceled source discovery context", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			var err error
			sourceAnalyzer, err = newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			var cancel context.CancelFunc
			discoveryContext, cancel = context.WithCancel(t.Context())
			cancel()
		})

		t.Run("When the analyzer starts the repository walk", func(*testing.T) {
			_, discoveryError = sourceAnalyzer.sourcePaths(discoveryContext)
		})

		t.Run("Then source discovery returns the context cancellation", func(t *testing.T) {
			if !errors.Is(discoveryError, context.Canceled) {
				t.Fatalf("source discovery error is %v, want context.Canceled", discoveryError)
			}
		})
	})
}

func TestAnalyzer_DetectFunctionData(t *testing.T) {
	t.Run("Scenario: Go files contain different callable syntax", func(t *testing.T) {
		var sources []string
		var results []bool
		var parseError error

		t.Run("Given files with no callable syntax, a declaration, a type, and a call", func(*testing.T) {
			sources = []string{
				"package sample\nconst Value = 1\n",
				"package sample\nfunc Run() {}\n",
				"package sample\nvar Callback func()\n",
				"package sample\nvar Value = build()\n",
			}
		})

		t.Run("When the analyzer inspects each syntax tree", func(*testing.T) {
			for _, source := range sources {
				file, err := parser.ParseFile(token.NewFileSet(), "sample.go", source, 0)
				if err != nil {
					parseError = err
					return
				}
				results = append(results, hasFunctionData(file))
			}
		})

		if !t.Run("Then only callable syntax requires type analysis", func(t *testing.T) {
			if parseError != nil {
				t.Fatalf("parse function data fixture: %v", parseError)
			}
			if !slices.Equal(results, []bool{false, true, true, true}) {
				t.Errorf("function data results are %v", results)
			}
		}) {
			return
		}
	})
}

func TestAnalyzer_AnalyzeSingleFunctionFile(t *testing.T) {
	t.Run("Scenario: The analysis scope contains exactly one function file", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var graph Graph
		var analysisError error

		t.Run("Given one classified package with one function", func(step *testing.T) {
			repositoryRoot := t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/single\n\ngo 1.26\n")
			writeFixtureFile(
				step,
				repositoryRoot,
				"internal/library/sample/sample.go",
				"package sample\n\nfunc Run() {}\n",
			)
			var err error
			sourceAnalyzer, err = newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("build analyzer: %v", err)
			}
		})

		t.Run("When the analyzer analyzes the repository", func(*testing.T) {
			graph, analysisError = sourceAnalyzer.analyze(t.Context())
		})

		t.Run("Then the only function is present in the graph", func(t *testing.T) {
			if analysisError != nil {
				t.Fatalf("analyze one function file: %v", analysisError)
			}
			if graph.Summary.Functions != 1 || len(graph.Functions) != 1 ||
				graph.Functions[0].Identifier != "internal/library/sample.Run" {
				t.Errorf("unexpected single-function graph: %+v", graph)
			}
		})
	})
}

func TestAnalyzer_ReturnDeterministicDependencyMetrics(t *testing.T) {
	t.Run("Scenario: Production and test imports produce a deterministic component graph", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var first Graph
		var second Graph
		var err error

		if !t.Run("Given a repository with production, test, and cyclic imports", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err = newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
		}) {
			return
		}

		if !t.Run("When the analyzer analyzes the repository twice without source changes", func(t *testing.T) {
			first, err = sourceAnalyzer.analyze(t.Context())
			if err != nil {
				t.Fatalf("the first analysis fails: %v", err)
			}
			second, err = sourceAnalyzer.analyze(t.Context())
			if err != nil {
				t.Fatalf("the second analysis fails: %v", err)
			}
		}) {
			return
		}

		if !t.Run("Then both analyses produce the same revision and payload", func(t *testing.T) {
			firstPayload, marshalError := json.Marshal(first)
			if marshalError != nil {
				t.Fatalf("encode the first graph: %v", marshalError)
			}
			secondPayload, marshalError := json.Marshal(second)
			if marshalError != nil {
				t.Fatalf("encode the second graph: %v", marshalError)
			}
			if !slices.Equal(firstPayload, secondPayload) {
				t.Fatal("two analyses of unchanged source produce different graphs")
			}
			if first.Revision == "" {
				t.Fatal("the graph contains no content revision")
			}
			if first.SchemaVersion != graphSchemaVersion {
				t.Fatalf("schema version %d does not match the configured version", first.SchemaVersion)
			}
			if first.SchemaVersion != 9 {
				t.Fatalf("unexpected graph schema version %d", first.SchemaVersion)
			}
		}) {
			return
		}

		t.Run("And the summary reports every classified component and relationship", func(t *testing.T) {
			if first.Summary.Components != 8 {
				t.Errorf("components are %d, want 8", first.Summary.Components)
			}
			if first.Summary.ProductionRelationships != 9 || first.Summary.TestOnlyRelationships != 1 {
				t.Errorf(
					"relationships are %d production and %d test-only, want 9 and 1",
					first.Summary.ProductionRelationships,
					first.Summary.TestOnlyRelationships,
				)
			}
			if first.Summary.RelationshipRuleViolations != 1 {
				t.Errorf("rule violations are %d, want 1", first.Summary.RelationshipRuleViolations)
			}
			if first.Summary.StableDependencyPrincipleViolations != 0 ||
				first.Summary.StableLowAbstractionComponents != 1 {
				t.Errorf(
					"findings are %d violations of the stable dependency principle and "+
						"%d stable components with low abstraction; want 0 and 1",
					first.Summary.StableDependencyPrincipleViolations,
					first.Summary.StableLowAbstractionComponents,
				)
			}
			if first.Summary.Findings != 3 || first.Summary.Errors != 1 || first.Summary.Warnings != 2 {
				t.Errorf(
					"findings are %d total, %d errors, and %d warnings; want 3, 1, and 2",
					first.Summary.Findings,
					first.Summary.Errors,
					first.Summary.Warnings,
				)
			}
			if first.Summary.Applications != 1 || first.Summary.ApplicationModules != 1 ||
				first.Summary.SharedModules != 3 || first.Summary.Libraries != 2 ||
				first.Summary.Infrastructure != 0 || first.Summary.DevelopmentTools != 1 ||
				first.Summary.Cycles != 1 || first.Summary.Relationships != 10 {
				t.Errorf("unexpected graph summary: %+v", first.Summary)
			}
		})

		t.Run("And coupling metrics show how many components import the logging library", func(t *testing.T) {
			logging := componentWithIdentifier(t, first, "internal/library/logging")
			if logging.ProductionImportingComponents != 3 {
				t.Errorf("production importing components are %d, want 3", logging.ProductionImportingComponents)
			}
			if logging.UsingApplicationCount != 1 || !slices.Equal(logging.UsingApplications, []string{"control"}) {
				t.Errorf(
					"unexpected applications that use the component: %d, %v",
					logging.UsingApplicationCount,
					logging.UsingApplications,
				)
			}
			if logging.TransitiveImportingComponents != 5 {
				t.Errorf("transitive importing components are %d, want 5", logging.TransitiveImportingComponents)
			}
			if logging.AfferentCoupling != 3 || logging.EfferentCoupling != 0 || logging.Instability != 0 {
				t.Errorf(
					"stability coupling is Ca=%d, Ce=%d, I=%v; want Ca=3, Ce=0, I=0",
					logging.AfferentCoupling,
					logging.EfferentCoupling,
					logging.Instability,
				)
			}
			if logging.AbstractTypes != 0 || logging.ConcreteTypes != 1 ||
				logging.Abstractness != 0 || logging.MainSequenceDistance != 1 || !logging.IsStableWithLowAbstraction {
				t.Errorf("unexpected stable-abstraction metrics: %+v", logging)
			}
			if logging.DirectDependencies != 1 || logging.ProductionDependencies != 0 ||
				logging.TestOnlyDependencies != 1 {
				t.Errorf(
					"dependencies are %d total, %d production, and %d test-only",
					logging.DirectDependencies,
					logging.ProductionDependencies,
					logging.TestOnlyDependencies,
				)
			}
		})

		t.Run("And only production imports create the expected cycle", func(t *testing.T) {
			logging := componentWithIdentifier(t, first, "internal/library/logging")
			audit := componentWithIdentifier(t, first, "internal/module/audit")
			if audit.ProductionImportingComponents != 3 || audit.TestOnlyImportingComponents != 1 {
				t.Errorf(
					"audit importing components are %d production and %d test-only, want 3 and 1",
					audit.ProductionImportingComponents,
					audit.TestOnlyImportingComponents,
				)
			}
			if audit.InCycle || logging.InCycle {
				t.Error("a test-only reverse import creates a production cycle")
			}
			if len(first.Cycles) != 1 {
				t.Fatalf("cycles are %v, want one cycle", first.Cycles)
			}
			if !slices.Equal(
				first.Cycles[0],
				[]string{"internal/module/cycleone", "internal/module/cycletwo"},
			) {
				t.Errorf("unexpected production cycle: %v", first.Cycles[0])
			}
		})

		t.Run("And relationships distinguish test imports from layer rule violations", func(t *testing.T) {
			testRelationship := relationshipBetween(
				t,
				first,
				"internal/library/logging",
				"internal/module/audit",
			)
			if !testRelationship.TestOnly || len(testRelationship.RuleViolations) != 0 {
				t.Errorf("unexpected test relationship: %+v", testRelationship)
			}
			layerRuleViolation := relationshipBetween(
				t,
				first,
				"internal/library/facade",
				"internal/module/audit",
			)
			if !slices.Equal(layerRuleViolation.RuleViolations, []string{"library-imports-feature"}) {
				t.Errorf("unexpected layer rule violation: %v", layerRuleViolation.RuleViolations)
			}
		})

		t.Run("And findings provide stable rule identifiers", func(t *testing.T) {
			rules := make([]string, 0, len(first.Findings))
			for _, finding := range first.Findings {
				rules = append(rules, finding.Rule)
			}
			want := []string{"dependency-cycle", "library-imports-feature", "stable-component-low-abstraction"}
			if !slices.Equal(rules, want) {
				t.Errorf("finding rules are %v, want %v", rules, want)
			}
		})

		t.Run("And the graph declares its exact mathematical policy", func(t *testing.T) {
			if first.Policy.InstabilityFormula != "Ce/(Ca+Ce)" ||
				first.Policy.IsolatedInstability != 0 ||
				first.Policy.MainSequenceDistanceFormula != "abs(A+I-1)" ||
				first.Policy.UntypedAbstractness != 0 ||
				first.Policy.StableLowAbstraction.MinimumAfferentCoupling != 1 ||
				first.Policy.StableLowAbstraction.MaximumInstability != 0.2 ||
				first.Policy.StableLowAbstraction.MaximumAbstractness != 0.2 ||
				!first.Policy.StableDependencyPrinciple.ProductionOnly ||
				first.Policy.StableDependencyPrinciple.RequiredRelation !=
					"targetInstability <= sourceInstability" {
				t.Errorf("unexpected mathematical policy: %+v", first.Policy)
			}
		})

		t.Run("And the graph declares the exact configured analysis scope", func(t *testing.T) {
			defaults := DefaultApplicationConfiguration().Analysis
			if !slices.Equal(first.Scope.Paths, defaults.Paths) ||
				!slices.Equal(first.Scope.IgnoredPaths, defaults.IgnoredPaths) ||
				!slices.Equal(first.Scope.Components.Libraries, defaults.Components.Libraries) {
				t.Errorf("unexpected analysis scope: %+v", first.Scope)
			}
		})
	})
}
func TestAnalyzer_AnalyzeInvalidImportBlock(t *testing.T) {
	t.Run("Scenario: An invalid import block remains visible as a diagnostic", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var graph Graph
		var err error

		if !t.Run("Given a classified component with an invalid import block", func(step *testing.T) {
			repositoryRoot := t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/invalid\n\ngo 1.26\n")
			writeFixtureFile(
				step,
				repositoryRoot,
				"internal/library/broken/broken.go",
				"package broken\n\nimport \"example.com/invalid/internal/library/missing\n",
			)
			sourceAnalyzer, err = newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
		}) {
			return
		}

		if !t.Run("When the analyzer analyzes the repository", func(t *testing.T) {
			graph, err = sourceAnalyzer.analyze(t.Context())
		}) {
			return
		}

		if !t.Run("Then the analysis succeeds and keeps the broken component", func(t *testing.T) {
			if err != nil {
				t.Fatalf("the analysis fails: %v", err)
			}
			if graph.Summary.Components != 1 {
				t.Fatalf("components are %d, want 1", graph.Summary.Components)
			}
		}) {
			return
		}

		t.Run("And the graph reports the invalid source path", func(t *testing.T) {
			if len(graph.Diagnostics) == 0 {
				t.Fatal("the graph contains no diagnostic")
			}
			if graph.Diagnostics[0].Path != "internal/library/broken/broken.go" {
				t.Errorf("unexpected diagnostic path %q", graph.Diagnostics[0].Path)
			}
			if graph.Functions == nil || graph.FunctionCalls == nil || graph.FunctionCycles == nil ||
				len(graph.Functions) != 0 || len(graph.FunctionCalls) != 0 || len(graph.FunctionCycles) != 0 {
				t.Errorf(
					"empty function data is functions=%v calls=%v cycles=%v",
					graph.Functions,
					graph.FunctionCalls,
					graph.FunctionCycles,
				)
			}
		})
	})
}

func TestAnalyzer_ApplyConfiguredScope(t *testing.T) {
	t.Run("Scenario: A generic repository layout selects and excludes explicit paths", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var graph Graph
		var analysisError error

		if !t.Run("Given custom service and package paths with one excluded package", func(step *testing.T) {
			repositoryRoot := t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/generic\n\ngo 1.26\n")
			writeFixtureFile(
				step,
				repositoryRoot,
				"services/control/features/orders/orders.go",
				`package orders

import (
	_ "example.com/generic/packages/legacy"
	_ "example.com/generic/packages/shared"
)
`,
			)
			writeFixtureFile(
				step,
				repositoryRoot,
				"packages/shared/shared.go",
				"package shared\n",
			)
			writeFixtureFile(
				step,
				repositoryRoot,
				"packages/legacy/legacy.go",
				"package legacy\n",
			)
			writeFixtureFile(
				step,
				repositoryRoot,
				"tools/generator/main.go",
				"package main\n",
			)
			configuration := AnalysisConfiguration{
				RepositoryRoot: repositoryRoot,
				Paths:          []string{"services", "packages"},
				IgnoredPaths:   []string{"packages/legacy"},
				Components: ComponentRulesConfiguration{
					Applications:       []string{"services/{application}"},
					ApplicationModules: []string{"services/{application}/features/{component}"},
					Libraries:          []string{"packages/{component}"},
					DevelopmentTools:   []string{"tools/{component}"},
				},
			}
			var err error
			sourceAnalyzer, err = newAnalyzer(configuration)
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
		}) {
			return
		}

		t.Run("When the analyzer analyzes the configured repository scope", func(t *testing.T) {
			graph, analysisError = sourceAnalyzer.analyze(t.Context())
		})

		if !t.Run(
			"Then the graph contains only components that the ignore rules do not exclude",
			func(t *testing.T) {
				if analysisError != nil {
					t.Fatalf("the analysis fails: %v", analysisError)
				}
				if graph.Summary.Components != 2 || graph.Summary.Relationships != 1 {
					t.Fatalf("unexpected configured graph summary: %+v", graph.Summary)
				}
				componentWithIdentifier(t, graph, "services/control/features/orders")
				componentWithIdentifier(t, graph, "packages/shared")
			},
		) {
			return
		}

		t.Run("And excluded and unselected components do not occur in the graph", func(t *testing.T) {
			for _, identifier := range []string{"packages/legacy", "tools/generator"} {
				for _, component := range graph.Components {
					if component.Identifier == identifier {
						t.Errorf("excluded component %q is present", identifier)
					}
				}
			}
		})

		t.Run("And the JSON output declares the normalized configured scope", func(t *testing.T) {
			if !slices.Equal(graph.Scope.Paths, []string{"packages", "services"}) ||
				!slices.Equal(graph.Scope.IgnoredPaths, []string{"packages/legacy"}) ||
				!slices.Equal(
					graph.Scope.Components.ApplicationModules,
					[]string{"services/{application}/features/{component}"},
				) {
				t.Errorf("unexpected graph scope: %+v", graph.Scope)
			}
		})
	})
}

func TestAnalyzer_RejectInvalidConfiguredScope(t *testing.T) {
	testCases := []struct {
		name      string
		configure func(*testing.T, AnalysisConfiguration, string) AnalysisConfiguration
	}{
		{
			name: "the analysis path list is empty",
			configure: func(_ *testing.T, configuration AnalysisConfiguration, _ string) AnalysisConfiguration {
				configuration.Paths = nil
				return configuration
			},
		},
		{
			name: "an analysis path is absolute",
			configure: func(
				_ *testing.T,
				configuration AnalysisConfiguration,
				repositoryRoot string,
			) AnalysisConfiguration {
				configuration.Paths = []string{repositoryRoot}
				return configuration
			},
		},
		{
			name: "an analysis path is empty",
			configure: func(_ *testing.T, configuration AnalysisConfiguration, _ string) AnalysisConfiguration {
				configuration.Paths = []string{""}
				return configuration
			},
		},
		{
			name: "an analysis path has surrounding space",
			configure: func(_ *testing.T, configuration AnalysisConfiguration, _ string) AnalysisConfiguration {
				configuration.Paths = []string{" internal"}
				return configuration
			},
		},
		{
			name: "an analysis path escapes the repository",
			configure: func(_ *testing.T, configuration AnalysisConfiguration, _ string) AnalysisConfiguration {
				configuration.Paths = []string{"../outside"}
				return configuration
			},
		},
		{
			name: "an analysis path is the parent directory",
			configure: func(_ *testing.T, configuration AnalysisConfiguration, _ string) AnalysisConfiguration {
				configuration.Paths = []string{".."}
				return configuration
			},
		},
		{
			name: "an ignored path pattern has surrounding space",
			configure: func(_ *testing.T, configuration AnalysisConfiguration, _ string) AnalysisConfiguration {
				configuration.IgnoredPaths = []string{" vendor"}
				return configuration
			},
		},
		{
			name: "an ignored path pattern contains a backslash",
			configure: func(_ *testing.T, configuration AnalysisConfiguration, _ string) AnalysisConfiguration {
				configuration.IgnoredPaths = []string{`internal\generated`}
				return configuration
			},
		},
		{
			name: "an ignored path pattern has invalid syntax",
			configure: func(_ *testing.T, configuration AnalysisConfiguration, _ string) AnalysisConfiguration {
				configuration.IgnoredPaths = []string{"[invalid"}
				return configuration
			},
		},
		{
			name: "the component layout has no templates",
			configure: func(_ *testing.T, configuration AnalysisConfiguration, _ string) AnalysisConfiguration {
				configuration.Components = ComponentRulesConfiguration{}
				return configuration
			},
		},
		{
			name: "a selected source path does not exist",
			configure: func(_ *testing.T, configuration AnalysisConfiguration, _ string) AnalysisConfiguration {
				configuration.Paths = []string{"absent"}
				return configuration
			},
		},
		{
			name: "a selected source path is not Go source",
			configure: func(
				step *testing.T,
				configuration AnalysisConfiguration,
				repositoryRoot string,
			) AnalysisConfiguration {
				writeFixtureFile(step, repositoryRoot, "README.md", "documentation\n")
				configuration.Paths = []string{"README.md"}
				return configuration
			},
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var configuration AnalysisConfiguration
			var startupError error

			t.Run("Given a repository and the invalid scope configuration", func(step *testing.T) {
				repositoryRoot := t.TempDir()
				writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/invalidscope\n\ngo 1.26\n")
				configuration = fixtureAnalysisConfiguration(repositoryRoot)
				configuration = testCase.configure(step, configuration, repositoryRoot)
			})

			t.Run("When the analyzer validates and discovers its source scope", func(t *testing.T) {
				sourceAnalyzer, err := newAnalyzer(configuration)
				startupError = err
				if err == nil {
					_, startupError = sourceAnalyzer.sourcePaths(t.Context())
				}
			})

			t.Run("Then startup rejects the invalid configuration", func(t *testing.T) {
				if startupError == nil {
					t.Fatal("the analyzer accepts an invalid analysis scope")
				}
			})
		})
	}
}
func TestModulePath_ReadAbsentDeclaration(t *testing.T) {
	t.Run("Scenario: A module file has no module declaration", func(t *testing.T) {
		var repositoryRoot string
		var err error

		t.Run("Given a go.mod file with only a Go version", func(step *testing.T) {
			repositoryRoot = t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "go 1.26\n")
		})

		t.Run("When the reader reads the module path", func(t *testing.T) {
			_, err = readModulePath(repositoryRoot)
		})

		t.Run("Then the reader returns the data integrity error category", func(t *testing.T) {
			if !errors.Is(err, failure.ErrDataIntegrity) {
				t.Fatalf("error is %v, want ErrDataIntegrity", err)
			}
		})
	})
}
func TestModulePath_ReadQuotedDeclaration(t *testing.T) {
	t.Run("Scenario: A quoted module declaration has an inline comment", func(t *testing.T) {
		var repositoryRoot string
		var modulePath string
		var err error

		t.Run("Given a quoted active declaration after a commented declaration", func(step *testing.T) {
			repositoryRoot = t.TempDir()
			writeFixtureFile(
				step,
				repositoryRoot,
				"go.mod",
				"// module ignored.example/old\nmodule \"example.com/current\" // active module\ngo 1.26\n",
			)
		})

		t.Run("When the reader reads the module path", func(t *testing.T) {
			modulePath, err = readModulePath(repositoryRoot)
		})

		if !t.Run("Then the reader reads the declaration without an error", func(t *testing.T) {
			if err != nil {
				t.Fatalf("readModulePath fails: %v", err)
			}
		}) {
			return
		}

		t.Run("And the reader removes quotes and comments from the path", func(t *testing.T) {
			if modulePath != "example.com/current" {
				t.Errorf("module path is %q, want %q", modulePath, "example.com/current")
			}
		})
	})
}
func TestSourcePaths_FindIncludedGoFiles(t *testing.T) {
	t.Run("Scenario: A repository contains ignored directories and non-Go entries", func(t *testing.T) {
		var repositoryRoot string
		var sourceAnalyzer *analyzer
		var paths []string
		var err error

		if !t.Run("Given one visible Go file and several ignored entries", func(step *testing.T) {
			repositoryRoot = t.TempDir()
			writeFixtureFile(step, repositoryRoot, "visible/first.go", "package visible\n")
			writeFixtureFile(step, repositoryRoot, "visible/readme.md", "not Go\n")
			if mkdirError := os.MkdirAll(
				filepath.Join(repositoryRoot, "visible", "directory.go"),
				0o755,
			); mkdirError != nil {
				step.Fatalf("create directory ending in .go: %v", mkdirError)
			}
			for _, directory := range []string{
				".cache",
				"node_modules",
				"vendor",
				"testdata",
				"target",
				"_resources",
			} {
				writeFixtureFile(step, repositoryRoot, directory+"/ignored.go", "package ignored\n")
			}
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/sourcepaths\n\ngo 1.26\n")
			sourceAnalyzer, err = newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
		}) {
			return
		}

		t.Run("When the analyzer collects Go source paths", func(t *testing.T) {
			paths, err = sourceAnalyzer.sourcePaths(t.Context())
		})

		if !t.Run("Then source discovery succeeds", func(t *testing.T) {
			if err != nil {
				t.Fatalf("goSourcePaths fails: %v", err)
			}
		}) {
			return
		}

		t.Run("And the analyzer returns only the visible Go file", func(t *testing.T) {
			want := []string{filepath.Join(repositoryRoot, "visible", "first.go")}
			if !slices.Equal(paths, want) {
				t.Errorf("source paths are %v, want %v", paths, want)
			}
		})
	})
}
func TestIgnoredPaths_MatchConfiguredPatterns(t *testing.T) {
	testCases := []struct {
		name    string
		ignored bool
	}{
		{name: ".git", ignored: true},
		{name: "node_modules", ignored: true},
		{name: "vendor", ignored: true},
		{name: "testdata", ignored: true},
		{name: "target", ignored: true},
		{name: "_resources", ignored: true},
		{name: "internal", ignored: false},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: The repository path contains "+testCase.name, func(t *testing.T) {
			var name string
			var result bool
			var matchError error
			var matcher ignoredPathMatcher

			t.Run("Given the default configured ignore patterns", func(t *testing.T) {
				name = testCase.name
				var err error
				matcher, err = newIgnoredPathMatcher(
					DefaultApplicationConfiguration().Analysis.IgnoredPaths,
				)
				if err != nil {
					t.Fatalf("newIgnoredPathMatcher fails: %v", err)
				}
			})

			t.Run("When the matcher evaluates the configured path rule", func(t *testing.T) {
				result, matchError = matcher.matches(name)
			})

			t.Run("Then the result matches the expected decision", func(t *testing.T) {
				if matchError != nil {
					t.Fatalf("match ignored path: %v", matchError)
				}
				if result != testCase.ignored {
					t.Fatalf("ignored path %q is %t, want %t", name, result, testCase.ignored)
				}
			})
		})
	}
}
func TestComponent_ClassifyPaths(t *testing.T) {
	testCases := []struct {
		name        string
		path        string
		classified  bool
		identifier  string
		component   string
		role        componentRole
		application string
	}{
		{
			name:        "an application module",
			path:        "cmd/control/internal/module/orders/domain.go",
			classified:  true,
			identifier:  "cmd/control/internal/module/orders",
			component:   "orders",
			role:        componentRoleApplicationModule,
			application: "control",
		},
		{
			name:        "an application composition root",
			path:        "cmd/control/main.go",
			classified:  true,
			identifier:  "cmd/control",
			component:   "control",
			role:        componentRoleApplication,
			application: "control",
		},
		{
			name:        "an application package import",
			path:        "cmd/control",
			classified:  true,
			identifier:  "cmd/control",
			component:   "control",
			role:        componentRoleApplication,
			application: "control",
		},
		{
			name:        "a command package that is not an application module",
			path:        "cmd/control/internal/adapter/http.go",
			classified:  true,
			identifier:  "cmd/control",
			component:   "control",
			role:        componentRoleApplication,
			application: "control",
		},
		{
			name:       "a nested library package",
			path:       "internal/library/authorization/openfga/adapter.go",
			classified: true,
			identifier: "internal/library/authorization",
			component:  "authorization",
			role:       componentRoleLibrary,
		},
		{
			name:       "a shared module",
			path:       "internal/module/audit/module.go",
			classified: true,
			identifier: "internal/module/audit",
			component:  "audit",
			role:       componentRoleSharedModule,
		},
		{
			name:       "a development tool",
			path:       "internal/devtool/generator/main.go",
			classified: true,
			identifier: "internal/devtool/generator",
			component:  "generator",
			role:       componentRoleDevelopment,
		},
		{
			name:       "a development tool package import",
			path:       "internal/devtool/generator",
			classified: true,
			identifier: "internal/devtool/generator",
			component:  "generator",
			role:       componentRoleDevelopment,
		},
		{
			name:       "shared infrastructure",
			path:       "internal/kernel/kernel.go",
			classified: true,
			identifier: "internal/kernel",
			component:  "kernel",
			role:       componentRoleInfrastructure,
		},
		{
			name:       "a shared infrastructure package import",
			path:       "internal/kernel",
			classified: true,
			identifier: "internal/kernel",
			component:  "kernel",
			role:       componentRoleInfrastructure,
		},
		{name: "a path without an architecture root", path: "main.go", classified: false},
		{name: "an incomplete module root", path: "internal/module", classified: false},
		{name: "an incomplete library root", path: "internal/library", classified: false},
		{name: "an incomplete development root", path: "internal/devtool", classified: false},
		{name: "an incomplete command root", path: "cmd", classified: false},
		{name: "an unrelated top-level package", path: "generate/protobuf/main.go", classified: false},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: The source path is "+testCase.name, func(t *testing.T) {
			var path string
			var classifier componentClassifier
			var descriptor componentDescriptor
			var classified bool

			t.Run("Given a repository-relative path and configured component templates", func(t *testing.T) {
				path = testCase.path
				var err error
				classifier, err = newComponentClassifier(
					DefaultApplicationConfiguration().Analysis.Components.domainRules(),
				)
				if err != nil {
					t.Fatalf("newComponentClassifier fails: %v", err)
				}
			})

			t.Run("When the classifier classifies the component", func(t *testing.T) {
				descriptor, classified = classifier.classify(path)
			})

			if !t.Run("Then the path has the expected classified state", func(t *testing.T) {
				if classified != testCase.classified {
					t.Fatalf("classified is %t, want %t", classified, testCase.classified)
				}
			}) {
				return
			}

			t.Run("And a classified path has the expected component descriptor", func(t *testing.T) {
				if !classified {
					return
				}
				if descriptor.identifier != testCase.identifier || descriptor.name != testCase.component ||
					descriptor.role != testCase.role || descriptor.application != testCase.application {
					t.Errorf("unexpected descriptor: %+v", descriptor)
				}
			})
		})
	}
}
func TestSourceFile_InspectImports(t *testing.T) {
	t.Run("Scenario: A classified source imports internal and external packages", func(t *testing.T) {
		var repositoryRoot string
		var sourceAnalyzer *analyzer
		var sourcePath string
		var file *sourceFile
		var err error

		t.Run("Given a source file with root, internal, and external imports", func(step *testing.T) {
			repositoryRoot = t.TempDir()
			relativePath := "internal/library/example/example.go"
			writeFixtureFile(
				step,
				repositoryRoot,
				relativePath,
				`package example

import (
	_ "example.com/current"
	_ "example.com/current/internal/module/audit"
	_ "example.net/external"
)
`,
			)
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/current\n\ngo 1.26\n")
			var analyzerError error
			sourceAnalyzer, analyzerError = newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if analyzerError != nil {
				step.Fatalf("newAnalyzer fails: %v", analyzerError)
			}
			sourcePath = filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		})

		t.Run("When the analyzer inspects the source file", func(t *testing.T) {
			file, err = sourceAnalyzer.inspectSourceFile("example.com/current", sourcePath)
		})

		if !t.Run("Then source inspection succeeds", func(t *testing.T) {
			if err != nil {
				t.Fatalf("inspectSourceFile fails: %v", err)
			}
			if file == nil {
				t.Fatal("inspectSourceFile returns no source file")
			}
		}) {
			return
		}

		t.Run("And the analyzer retains only current-module imports", func(t *testing.T) {
			var importedPackages []string
			for _, imported := range file.imports {
				importedPackages = append(importedPackages, imported.packagePath)
			}
			want := []string{"internal/module/audit"}
			if !slices.Equal(importedPackages, want) {
				t.Errorf("internal imports are %v, want %v", importedPackages, want)
			}
		})
	})
}

func TestSourceFile_InspectNamedTypes(t *testing.T) {
	t.Run("Scenario: Production source declares abstract and concrete named types", func(t *testing.T) {
		var repositoryRoot string
		var sourceAnalyzer *analyzer
		var sourcePath string
		var file *sourceFile
		var err error

		t.Run("Given a Go file with one interface and two concrete types", func(step *testing.T) {
			repositoryRoot = t.TempDir()
			relativePath := "internal/library/contracts/contracts.go"
			writeFixtureFile(
				step,
				repositoryRoot,
				relativePath,
				`package contracts

type Reader interface {
	Read() error
}

type Record struct{}
type Identifier string

var current Record

func NewRecord() Record { return current }
`,
			)
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/current\n\ngo 1.26\n")
			var analyzerError error
			sourceAnalyzer, analyzerError = newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if analyzerError != nil {
				step.Fatalf("newAnalyzer fails: %v", analyzerError)
			}
			sourcePath = filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		})

		t.Run("When the analyzer inspects the source file", func(t *testing.T) {
			file, err = sourceAnalyzer.inspectSourceFile("example.com/current", sourcePath)
		})

		if !t.Run("Then source inspection succeeds", func(t *testing.T) {
			if err != nil {
				t.Fatalf("inspectSourceFile fails: %v", err)
			}
			if file == nil {
				t.Fatal("inspectSourceFile returns no source file")
			}
		}) {
			return
		}

		t.Run("And the analyzer counts named interfaces and concrete types separately", func(t *testing.T) {
			if file.abstractTypes != 1 || file.concreteTypes != 2 {
				t.Errorf(
					"named types are %d abstract and %d concrete, want 1 and 2",
					file.abstractTypes,
					file.concreteTypes,
				)
			}
		})
	})
}

func TestRelationships_CollectClassifiedImports(t *testing.T) {
	t.Run("Scenario: A component has normalized imports to itself and another component", func(t *testing.T) {
		var source componentDescriptor
		var file sourceFile
		var components map[string]*componentAccumulator
		var relationships map[relationshipKey]*relationshipAccumulator

		t.Run("Given a library component and two normalized imports", func(t *testing.T) {
			source = componentDescriptor{
				identifier: "internal/library/logging",
				name:       "logging",
				role:       componentRoleLibrary,
			}
			components = make(map[string]*componentAccumulator)
			getOrCreateComponent(components, source)
			relationships = make(map[relationshipKey]*relationshipAccumulator)
			file = sourceFile{
				relativePath: "internal/library/logging/logging.go",
				packagePath:  "internal/library/logging",
				component:    source,
				imports: []sourceImport{
					{
						packagePath: "internal/library/logging",
						component:   source,
					},
					{
						packagePath: "internal/module/audit",
						component: componentDescriptor{
							identifier: "internal/module/audit",
							name:       "audit",
							role:       componentRoleSharedModule,
						},
					},
				},
			}
		})

		t.Run("When the collector collects relationships from the source imports", func(t *testing.T) {
			collectRelationships(components, relationships, file)
		})

		if !t.Run("Then only one relationship and two components exist", func(t *testing.T) {
			if len(relationships) != 1 || len(components) != 2 {
				t.Fatalf(
					"the collector creates %d relationships and %d components",
					len(relationships),
					len(components),
				)
			}
		}) {
			return
		}

		t.Run("And the collector retains the classified audit relationship", func(t *testing.T) {
			key := relationshipKey{source: source.identifier, target: "internal/module/audit"}
			if _, exists := relationships[key]; !exists {
				t.Errorf("the expected relationship is absent: %v", relationships)
			}
		})
	})
}
func TestRelationshipRuleViolations_ClassifyImport(t *testing.T) {
	descriptor := func(role componentRole, application string) componentDescriptor {
		return componentDescriptor{role: role, application: application}
	}
	testCases := []struct {
		name     string
		source   componentDescriptor
		target   componentDescriptor
		testOnly bool
		want     []string
	}{
		{
			name:   "production code depends on a development tool",
			source: descriptor(componentRoleApplication, "control"),
			target: descriptor(componentRoleDevelopment, ""),
			want:   []string{"production-imports-development"},
		},
		{
			name:   "a library depends on an application",
			source: descriptor(componentRoleLibrary, ""),
			target: descriptor(componentRoleApplication, "control"),
			want:   []string{"library-imports-feature"},
		},
		{
			name:   "a library depends on an application module",
			source: descriptor(componentRoleLibrary, ""),
			target: descriptor(componentRoleApplicationModule, "control"),
			want:   []string{"library-imports-feature"},
		},
		{
			name:   "shared infrastructure depends on an application module",
			source: descriptor(componentRoleInfrastructure, ""),
			target: descriptor(componentRoleApplicationModule, "control"),
			want:   []string{"shared-component-imports-application"},
		},
		{
			name:   "a shared module depends on an application",
			source: descriptor(componentRoleSharedModule, ""),
			target: descriptor(componentRoleApplication, "control"),
			want:   []string{"shared-component-imports-application"},
		},
		{
			name:   "one application module depends on another application",
			source: descriptor(componentRoleApplicationModule, "control"),
			target: descriptor(componentRoleApplicationModule, "portal"),
			want:   []string{"cross-application-module-import"},
		},
		{
			name:   "modules from the same application can depend on each other",
			source: descriptor(componentRoleApplicationModule, "control"),
			target: descriptor(componentRoleApplicationModule, "control"),
			want:   []string{},
		},
		{
			name:     "test imports do not create component rule violations",
			source:   descriptor(componentRoleLibrary, ""),
			target:   descriptor(componentRoleApplication, "control"),
			testOnly: true,
			want:     []string{},
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var source componentDescriptor
			var target componentDescriptor
			var testOnly bool
			var ruleViolations []string

			t.Run("Given source and target component descriptors", func(t *testing.T) {
				source = testCase.source
				target = testCase.target
				testOnly = testCase.testOnly
			})

			t.Run("When the classifier classifies relationship rule violations", func(t *testing.T) {
				ruleViolations = relationshipRuleViolations(source, target, testOnly)
			})

			t.Run("Then the result contains the expected component rule violations", func(t *testing.T) {
				if !slices.Equal(ruleViolations, testCase.want) {
					t.Fatalf("rule violations are %v, want %v", ruleViolations, testCase.want)
				}
			})
		})
	}
}
func TestReachability_TraverseDependencyGraph(t *testing.T) {
	t.Run("Scenario: A cyclic graph has one path to a terminal node", func(t *testing.T) {
		var adjacency map[string]stringSet
		var result stringSet

		t.Run("Given a three-node graph that returns to its start", func(t *testing.T) {
			adjacency = map[string]stringSet{
				"a": newStringSet("b"),
				"b": newStringSet("a", "c"),
				"c": newStringSet(),
			}
		})

		t.Run("When the traversal collects reachable nodes from the start", func(t *testing.T) {
			result = reachable("a", adjacency)
		})

		t.Run("Then the traversal returns each downstream node once without the start", func(t *testing.T) {
			sortedResult := sortedSet(result)
			if !slices.Equal(sortedResult, []string{"b", "c"}) {
				t.Fatalf("reachable nodes are %v, want [b c]", sortedResult)
			}
		})
	})
}
func TestInstability_CalculateCouplingRatio(t *testing.T) {
	testCases := []struct {
		name             string
		afferentCoupling int
		efferentCoupling int
		expectedResult   float64
	}{
		{name: "an isolated component is stable", expectedResult: 0},
		{
			name:             "one outgoing edge exists among three edges",
			afferentCoupling: 2,
			efferentCoupling: 1,
			expectedResult:   1.0 / 3.0,
		},
		{name: "only outgoing edges exist", efferentCoupling: 2, expectedResult: 1},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var afferentCoupling int
			var efferentCoupling int
			var result float64

			t.Run("Given afferent and efferent coupling counts", func(t *testing.T) {
				afferentCoupling = testCase.afferentCoupling
				efferentCoupling = testCase.efferentCoupling
			})

			t.Run("When the calculator calculates instability", func(t *testing.T) {
				result = instability(afferentCoupling, efferentCoupling)
			})

			t.Run("Then the calculator returns the expected coupling ratio", func(t *testing.T) {
				if math.Abs(result-testCase.expectedResult) > 1e-12 {
					t.Fatalf("instability is %v, want %v", result, testCase.expectedResult)
				}
			})
		})
	}
}

func TestAbstractness_CalculateNamedTypeRatio(t *testing.T) {
	testCases := []struct {
		name          string
		abstractTypes int
		concreteTypes int
		want          float64
	}{
		{name: "a component declares no named type", want: 0},
		{name: "one of four named types is abstract", abstractTypes: 1, concreteTypes: 3, want: 0.25},
		{name: "all named types are abstract", abstractTypes: 2, want: 1},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var abstractTypes int
			var concreteTypes int
			var result float64

			t.Run("Given abstract and concrete named type counts", func(t *testing.T) {
				abstractTypes = testCase.abstractTypes
				concreteTypes = testCase.concreteTypes
			})

			t.Run("When the calculator calculates abstractness", func(t *testing.T) {
				result = abstractness(abstractTypes, concreteTypes)
			})

			t.Run("Then the calculator returns the expected abstract type ratio", func(t *testing.T) {
				if math.Abs(result-testCase.want) > 1e-12 {
					t.Fatalf("abstractness is %v, want %v", result, testCase.want)
				}
			})
		})
	}
}

func TestMainSequenceDistance_CalculateStabilityAbstractionBalance(t *testing.T) {
	testCases := []struct {
		name         string
		abstractness float64
		instability  float64
		want         float64
	}{
		{name: "a stable abstract component is on the main sequence", abstractness: 1, want: 0},
		{name: "an unstable concrete component is on the main sequence", instability: 1, want: 0},
		{name: "a stable component with low abstraction is maximally distant", want: 1},
		{name: "a mixed component is partially distant", abstractness: 0.2, instability: 0.3, want: 0.5},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var componentAbstractness float64
			var componentInstability float64
			var result float64

			t.Run("Given abstraction and instability values", func(t *testing.T) {
				componentAbstractness = testCase.abstractness
				componentInstability = testCase.instability
			})

			t.Run("When the calculator calculates distance from the main sequence", func(t *testing.T) {
				result = mainSequenceDistance(componentAbstractness, componentInstability)
			})

			t.Run("Then the calculator returns the expected absolute distance", func(t *testing.T) {
				if math.Abs(result-testCase.want) > 1e-12 {
					t.Fatalf("main-sequence distance is %v, want %v", result, testCase.want)
				}
			})
		})
	}
}

func TestStableLowAbstraction_ClassifyComponent(t *testing.T) {
	testCases := []struct {
		name                         string
		afferentCoupling             int
		instability                  float64
		abstractness                 float64
		wantStableWithLowAbstraction bool
	}{
		{
			name:                         "a responsible component is in the stable component with low abstraction",
			afferentCoupling:             1,
			instability:                  stableLowAbstractionMaximumInstability,
			abstractness:                 stableLowAbstractionMaximumAbstractness,
			wantStableWithLowAbstraction: true,
		},
		{name: "an isolated component has no external responsibility"},
		{
			name:             "an unstable component exceeds the maximum instability",
			afferentCoupling: 2,
			instability:      stableLowAbstractionMaximumInstability + 0.01,
		},
		{
			name:             "an abstract component exceeds the maximum abstractness",
			afferentCoupling: 2,
			abstractness:     stableLowAbstractionMaximumAbstractness + 0.01,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var afferentCoupling int
			var componentInstability float64
			var componentAbstractness float64
			var result bool

			t.Run("Given coupling, instability, and abstraction metrics", func(t *testing.T) {
				afferentCoupling = testCase.afferentCoupling
				componentInstability = testCase.instability
				componentAbstractness = testCase.abstractness
			})

			t.Run("When the classifier evaluates the stable-low-abstraction rule", func(t *testing.T) {
				result = isStableWithLowAbstraction(
					afferentCoupling,
					componentInstability,
					componentAbstractness,
				)
			})

			t.Run("Then the classifier returns the expected classification", func(t *testing.T) {
				if result != testCase.wantStableWithLowAbstraction {
					t.Fatalf(
						"stable-low-abstraction result is %t, want %t",
						result,
						testCase.wantStableWithLowAbstraction,
					)
				}
			})
		})
	}
}

func TestStableDependency_AnnotateInstabilityDirection(t *testing.T) {
	t.Run("Scenario: Production and test relationships cross different stability levels", func(t *testing.T) {
		var components []Component
		var relationships []Relationship

		t.Run("Given stable, peer, and unstable components with four relationships", func(t *testing.T) {
			components = []Component{
				{Identifier: "stable", Instability: 0.2},
				{Identifier: "peer", Instability: 0.2},
				{Identifier: "unstable", Instability: 0.8},
			}
			relationships = []Relationship{
				{Source: "stable", Target: "unstable", RuleViolations: []string{"existing-rule-violation"}},
				{Source: "unstable", Target: "stable", RuleViolations: []string{}},
				{Source: "stable", Target: "peer", RuleViolations: []string{}},
				{Source: "stable", Target: "unstable", TestOnly: true, RuleViolations: []string{}},
			}
		})

		t.Run("When the analyzer marks violations of the stable dependency principle", func(t *testing.T) {
			annotateStableDependencyPrincipleViolations(relationships, components)
		})

		if !t.Run(
			"Then the analyzer marks only the production import of the less stable component",
			func(t *testing.T) {
				if !relationships[0].ViolatesStableDependencyPrinciple {
					t.Fatal("the analyzer does not mark the stable-to-unstable production import")
				}
				for index := 1; index < len(relationships); index++ {
					if relationships[index].ViolatesStableDependencyPrinciple {
						t.Fatalf("relationship %d has an unexpected stable dependency principle violation", index)
					}
				}
			},
		) {
			return
		}

		t.Run("And the analyzer sorts the violation of the stable dependency principle", func(t *testing.T) {
			want := []string{"existing-rule-violation", "stable-dependency-principle"}
			if !slices.Equal(relationships[0].RuleViolations, want) {
				t.Errorf("rule violations are %v, want %v", relationships[0].RuleViolations, want)
			}
		})
	})
}

func TestStronglyConnectedComponents_FindCycles(t *testing.T) {
	t.Run("Scenario: A graph contains two multi-node cycles and one self-loop", func(t *testing.T) {
		var identifiers []string
		var adjacency map[string]stringSet
		var cycles [][]string

		t.Run("Given two connected cycles and a separate self-loop", func(t *testing.T) {
			identifiers = []string{"a", "b", "c", "d", "e"}
			adjacency = newAdjacency(identifiers)
			adjacency["a"].add("b")
			adjacency["b"].add("a")
			adjacency["b"].add("c")
			adjacency["c"].add("d")
			adjacency["d"].add("c")
			adjacency["e"].add("e")
		})

		t.Run("When the calculator calculates strongly connected components", func(t *testing.T) {
			cycles = stronglyConnectedComponents(identifiers, adjacency)
		})

		t.Run("Then the calculator returns only the sorted multi-node cycles", func(t *testing.T) {
			want := [][]string{{"a", "b"}, {"c", "d"}}
			if !slices.EqualFunc(
				cycles,
				want,
				func(first, second []string) bool { return slices.Equal(first, second) },
			) {
				t.Fatalf("cycles are %v, want %v", cycles, want)
			}
		})
	})
}
func TestGraph_BuildSortedDiagnostics(t *testing.T) {
	t.Run("Scenario: Diagnostics arrive in an unstable path and message order", func(t *testing.T) {
		var diagnostics []Diagnostic
		var graph Graph

		t.Run("Given diagnostics with repeated paths and unsorted messages", func(t *testing.T) {
			diagnostics = []Diagnostic{
				{Path: "z.go", Message: "first"},
				{Path: "a.go", Message: "second"},
				{Path: "a.go", Message: "first"},
			}
		})

		t.Run("When the builder builds the dependency graph", func(t *testing.T) {
			graph = buildGraph(
				"example.com/current",
				map[string]*componentAccumulator{},
				nil,
				diagnostics,
			)
		})

		t.Run("Then the builder sorts diagnostics by path and message", func(t *testing.T) {
			want := []Diagnostic{
				{Path: "a.go", Message: "first"},
				{Path: "a.go", Message: "second"},
				{Path: "z.go", Message: "first"},
			}
			if !slices.Equal(graph.Diagnostics, want) {
				t.Fatalf("diagnostics are %+v, want %+v", graph.Diagnostics, want)
			}
		})

		t.Run("And the module path remains unchanged", func(t *testing.T) {
			if strings.TrimSpace(graph.ModulePath) != "example.com/current" {
				t.Errorf("unexpected module path %q", graph.ModulePath)
			}
		})
	})
}
func newAnalyzerFixture(t *testing.T) string {
	t.Helper()
	repositoryRoot := t.TempDir()
	writeFixtureFile(t, repositoryRoot, "go.mod", "module example.com/repository\n\ngo 1.26\n")
	writeFixtureFile(
		t,
		repositoryRoot,
		"cmd/control/main.go",
		`package main

import (
	_ "example.com/repository/cmd/control/internal/module/orders"
	_ "example.com/repository/internal/module/audit"
)
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"cmd/control/internal/module/orders/module.go",
		`package orders

import (
	_ "example.com/repository/internal/library/logging"
	_ "example.com/repository/internal/module/audit"
)
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/module/audit/module.go",
		`package audit

import _ "example.com/repository/internal/library/logging"
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/module/cycleone/module.go",
		`package cycleone

import _ "example.com/repository/internal/module/cycletwo"
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/module/cycletwo/module.go",
		`package cycletwo

import _ "example.com/repository/internal/module/cycleone"
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/library/logging/logging.go",
		"package logging\n\ntype Logger struct{}\n",
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/library/logging/logging_test.go",
		`package logging_test

import _ "example.com/repository/internal/module/audit"

type testContract interface {
	testOnly()
}
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/library/facade/facade.go",
		`package facade

import _ "example.com/repository/internal/module/audit"
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/devtool/generator/main.go",
		`package main

import _ "example.com/repository/internal/library/logging"
`,
	)
	return repositoryRoot
}

func writeFixtureFile(t *testing.T, repositoryRoot, relativePath, content string) {
	t.Helper()
	path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory for %s: %v", relativePath, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture file %s: %v", relativePath, err)
	}
}

func componentWithIdentifier(t *testing.T, graph Graph, identifier string) Component {
	t.Helper()
	for _, component := range graph.Components {
		if component.Identifier == identifier {
			return component
		}
	}
	t.Fatalf("the graph contains no component %q", identifier)
	return Component{}
}

func relationshipBetween(t *testing.T, graph Graph, source, target string) Relationship {
	t.Helper()
	for _, relationship := range graph.Relationships {
		if relationship.Source == source && relationship.Target == target {
			return relationship
		}
	}
	t.Fatalf("the graph contains no relationship from %q to %q", source, target)
	return Relationship{}
}
