package main

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAnalyzer_AnalyzeStableStrategicMetrics(t *testing.T) {
	t.Run("Scenario: Production and test imports produce a deterministic strategic graph", func(t *testing.T) {
		var sut *analyzer
		var first Graph
		var second Graph
		var err error

		if !t.Run("Given a repository with production, test, and cyclic imports", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sut, err = newAnalyzer(repositoryRoot)
			if err != nil {
				step.Fatalf("newAnalyzer failed: %v", err)
			}
		}) {
			return
		}

		if !t.Run("When the repository is analyzed twice without source changes", func(t *testing.T) {
			first, err = sut.analyze()
			if err != nil {
				t.Fatalf("first analyze failed: %v", err)
			}
			second, err = sut.analyze()
			if err != nil {
				t.Fatalf("second analyze failed: %v", err)
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
				t.Fatal("two analyses of unchanged source produced different graphs")
			}
			if first.Revision == "" {
				t.Fatal("the graph carries no content revision")
			}
			if first.SchemaVersion != graphSchemaVersion {
				t.Fatalf("schema version %d does not match the configured version", first.SchemaVersion)
			}
			if first.SchemaVersion != 1 {
				t.Fatalf("unexpected graph schema version %d", first.SchemaVersion)
			}
		}) {
			return
		}

		t.Run("And the summary reports every strategic component and relationship", func(t *testing.T) {
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
			if first.Summary.Concerns != 1 {
				t.Errorf("concerns are %d, want 1", first.Summary.Concerns)
			}
			if first.Summary.Applications != 1 || first.Summary.ApplicationModules != 1 ||
				first.Summary.SharedModules != 3 || first.Summary.Libraries != 2 ||
				first.Summary.Infrastructure != 0 || first.Summary.DevelopmentTools != 1 ||
				first.Summary.Cycles != 1 || first.Summary.Relationships != 10 {
				t.Errorf("unexpected graph summary: %+v", first.Summary)
			}
		})

		t.Run("And coupling metrics show the impact of the logging library", func(t *testing.T) {
			logging := componentWithIdentifier(t, first, "internal/library/logging")
			if logging.ProductionDependants != 3 {
				t.Errorf("production consumers are %d, want 3", logging.ProductionDependants)
			}
			if logging.ApplicationReach != 1 || !slices.Equal(logging.Applications, []string{"control"}) {
				t.Errorf("unexpected application reach: %d, %v", logging.ApplicationReach, logging.Applications)
			}
			if logging.TransitiveDependants != 5 {
				t.Errorf("transitive consumers are %d, want 5", logging.TransitiveDependants)
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
			if audit.ProductionDependants != 3 || audit.TestOnlyDependants != 1 {
				t.Errorf(
					"audit consumers are %d production and %d test-only, want 3 and 1",
					audit.ProductionDependants,
					audit.TestOnlyDependants,
				)
			}
			if audit.InCycle || logging.InCycle {
				t.Error("a test-only reverse import created a production cycle")
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

		t.Run("And relationships distinguish test imports from layer concerns", func(t *testing.T) {
			testRelationship := relationshipBetween(
				t,
				first,
				"internal/library/logging",
				"internal/module/audit",
			)
			if !testRelationship.TestOnly || len(testRelationship.Concerns) != 0 {
				t.Errorf("unexpected test relationship: %+v", testRelationship)
			}
			layerConcern := relationshipBetween(
				t,
				first,
				"internal/library/facade",
				"internal/module/audit",
			)
			if !slices.Equal(layerConcern.Concerns, []string{"library-depends-on-feature"}) {
				t.Errorf("unexpected layer concern: %v", layerConcern.Concerns)
			}
		})
	})
}
func TestAnalyzer_AnalyzeInvalidImportBlock(t *testing.T) {
	t.Run("Scenario: An invalid import block remains visible as a diagnostic", func(t *testing.T) {
		var sut *analyzer
		var graph Graph
		var err error

		if !t.Run("Given a modeled component with an invalid import block", func(step *testing.T) {
			repositoryRoot := t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/invalid\n\ngo 1.26\n")
			writeFixtureFile(
				step,
				repositoryRoot,
				"internal/library/broken/broken.go",
				"package broken\n\nimport \"example.com/invalid/internal/library/missing\n",
			)
			sut, err = newAnalyzer(repositoryRoot)
			if err != nil {
				step.Fatalf("newAnalyzer failed: %v", err)
			}
		}) {
			return
		}

		if !t.Run("When the repository is analyzed", func(t *testing.T) {
			graph, err = sut.analyze()
		}) {
			return
		}

		if !t.Run("Then the analysis succeeds and keeps the broken component", func(t *testing.T) {
			if err != nil {
				t.Fatalf("analyze failed: %v", err)
			}
			if graph.Summary.Components != 1 {
				t.Fatalf("components are %d, want 1", graph.Summary.Components)
			}
		}) {
			return
		}

		t.Run("And the graph reports the invalid source path", func(t *testing.T) {
			if len(graph.Diagnostics) == 0 {
				t.Fatal("the graph carries no diagnostic")
			}
			if graph.Diagnostics[0].Path != "internal/library/broken/broken.go" {
				t.Errorf("unexpected diagnostic path %q", graph.Diagnostics[0].Path)
			}
		})
	})
}
func TestModulePath_ReadAbsentDeclaration(t *testing.T) {
	t.Run("Scenario: A module file has no module declaration", func(t *testing.T) {
		var repositoryRoot string
		var err error

		t.Run("Given a go.mod file with only a Go version", func(step *testing.T) {
			repositoryRoot = t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "go 1.26\n")
		})

		t.Run("When the module path is read", func(t *testing.T) {
			_, err = readModulePath(repositoryRoot)
		})

		t.Run("Then the typed declaration error is returned", func(t *testing.T) {
			if !errors.Is(err, errModuleDeclarationNotFound) {
				t.Fatalf("expected errModuleDeclarationNotFound, got %v", err)
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

		t.Run("When the module path is read", func(t *testing.T) {
			modulePath, err = readModulePath(repositoryRoot)
		})

		if !t.Run("Then the declaration is read without an error", func(t *testing.T) {
			if err != nil {
				t.Fatalf("readModulePath failed: %v", err)
			}
		}) {
			return
		}

		t.Run("And quotes and comments are removed from the path", func(t *testing.T) {
			if modulePath != "example.com/current" {
				t.Errorf("module path is %q, want %q", modulePath, "example.com/current")
			}
		})
	})
}
func TestSourcePaths_FindModeledGoFiles(t *testing.T) {
	t.Run("Scenario: A repository contains ignored directories and non-Go entries", func(t *testing.T) {
		var repositoryRoot string
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
			for _, directory := range []string{".cache", "node_modules", "vendor", "testdata", "_resources"} {
				writeFixtureFile(step, repositoryRoot, directory+"/ignored.go", "package ignored\n")
			}
		}) {
			return
		}

		t.Run("When Go source paths are collected", func(t *testing.T) {
			paths, err = goSourcePaths(repositoryRoot)
		})

		if !t.Run("Then source discovery succeeds", func(t *testing.T) {
			if err != nil {
				t.Fatalf("goSourcePaths failed: %v", err)
			}
		}) {
			return
		}

		t.Run("And only the visible Go file is returned", func(t *testing.T) {
			want := []string{filepath.Join(repositoryRoot, "visible", "first.go")}
			if !slices.Equal(paths, want) {
				t.Errorf("source paths are %v, want %v", paths, want)
			}
		})
	})
}
func TestIgnoredDirectory_ClassifyName(t *testing.T) {
	testCases := []struct {
		name    string
		ignored bool
	}{
		{name: ".git", ignored: true},
		{name: "node_modules", ignored: true},
		{name: "vendor", ignored: true},
		{name: "testdata", ignored: true},
		{name: "_resources", ignored: true},
		{name: "internal", ignored: false},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: The directory name is "+testCase.name, func(t *testing.T) {
			var name string
			var result bool

			t.Run("Given a repository directory name", func(t *testing.T) {
				name = testCase.name
			})

			t.Run("When the directory ignore rule is evaluated", func(t *testing.T) {
				result = ignoredDirectory(name)
			})

			t.Run("Then the closed ignore rule returns the expected decision", func(t *testing.T) {
				if result != testCase.ignored {
					t.Fatalf("ignoredDirectory(%q) is %t, want %t", name, result, testCase.ignored)
				}
			})
		})
	}
}
func TestComponent_Classification(t *testing.T) {
	testCases := []struct {
		name        string
		path        string
		modeled     bool
		identifier  string
		component   string
		kind        componentKind
		application string
	}{
		{
			name:        "an application module",
			path:        "cmd/control/internal/module/orders/domain.go",
			modeled:     true,
			identifier:  "cmd/control/internal/module/orders",
			component:   "orders",
			kind:        componentKindApplicationModule,
			application: "control",
		},
		{
			name:        "an application composition root",
			path:        "cmd/control/main.go",
			modeled:     true,
			identifier:  "cmd/control",
			component:   "control",
			kind:        componentKindApplication,
			application: "control",
		},
		{
			name:        "a command package that is not an application module",
			path:        "cmd/control/internal/adapter/http.go",
			modeled:     true,
			identifier:  "cmd/control",
			component:   "control",
			kind:        componentKindApplication,
			application: "control",
		},
		{
			name:       "a nested library package",
			path:       "internal/library/authorization/openfga/adapter.go",
			modeled:    true,
			identifier: "internal/library/authorization",
			component:  "authorization",
			kind:       componentKindLibrary,
		},
		{
			name:       "a shared module",
			path:       "internal/module/audit/module.go",
			modeled:    true,
			identifier: "internal/module/audit",
			component:  "audit",
			kind:       componentKindSharedModule,
		},
		{
			name:       "a development tool",
			path:       "internal/devtool/generator/main.go",
			modeled:    true,
			identifier: "internal/devtool/generator",
			component:  "generator",
			kind:       componentKindDevelopment,
		},
		{
			name:       "shared infrastructure",
			path:       "internal/kernel/kernel.go",
			modeled:    true,
			identifier: "internal/kernel",
			component:  "kernel",
			kind:       componentKindInfrastructure,
		},
		{name: "a path without an architectural root", path: "main.go", modeled: false},
		{name: "an incomplete module root", path: "internal/module", modeled: false},
		{name: "an incomplete library root", path: "internal/library", modeled: false},
		{name: "an incomplete development root", path: "internal/devtool", modeled: false},
		{name: "an unrelated top-level package", path: "generate/protobuf/main.go", modeled: false},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: The source path is "+testCase.name, func(t *testing.T) {
			var path string
			var descriptor componentDescriptor
			var modeled bool

			t.Run("Given a repository source path", func(t *testing.T) {
				path = testCase.path
			})

			t.Run("When the component is classified", func(t *testing.T) {
				descriptor, modeled = classifyComponent(path)
			})

			if !t.Run("Then the path has the expected modeled state", func(t *testing.T) {
				if modeled != testCase.modeled {
					t.Fatalf("modeled is %t, want %t", modeled, testCase.modeled)
				}
			}) {
				return
			}

			t.Run("And a modeled path has the expected strategic descriptor", func(t *testing.T) {
				if !modeled {
					return
				}
				if descriptor.identifier != testCase.identifier || descriptor.name != testCase.component ||
					descriptor.kind != testCase.kind || descriptor.application != testCase.application {
					t.Errorf("unexpected descriptor: %+v", descriptor)
				}
			})
		})
	}

	t.Run("Scenario: A command path has no application segment", func(t *testing.T) {
		var parts []string
		var modeled bool

		t.Run("Given only the command root segment", func(t *testing.T) {
			parts = []string{"cmd"}
		})

		t.Run("When the command component is classified", func(t *testing.T) {
			_, modeled = classifyCommandComponent(parts)
		})

		t.Run("Then the incomplete command path is not modeled", func(t *testing.T) {
			if modeled {
				t.Fatal("an incomplete command path was modeled")
			}
		})
	})
}
func TestSourceFile_InspectImports(t *testing.T) {
	t.Run("Scenario: A modeled source imports internal and external packages", func(t *testing.T) {
		var repositoryRoot string
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
			sourcePath = filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		})

		t.Run("When the source file is inspected", func(t *testing.T) {
			file, err = inspectSourceFile(repositoryRoot, "example.com/current", sourcePath)
		})

		if !t.Run("Then source inspection succeeds", func(t *testing.T) {
			if err != nil {
				t.Fatalf("inspectSourceFile failed: %v", err)
			}
			if file == nil {
				t.Fatal("inspectSourceFile returned no source file")
			}
		}) {
			return
		}

		t.Run("And only current-module imports are retained", func(t *testing.T) {
			want := []string{"example.com/current", "example.com/current/internal/module/audit"}
			if !slices.Equal(file.imports, want) {
				t.Errorf("internal imports are %v, want %v", file.imports, want)
			}
		})
	})
}
func TestRelationships_CollectModeledImports(t *testing.T) {
	t.Run("Scenario: A component imports itself, an unmodeled path, and another component", func(t *testing.T) {
		var modulePath string
		var source componentDescriptor
		var file sourceFile
		var components map[string]*componentAccumulator
		var relationships map[relationshipKey]*relationshipAccumulator

		t.Run("Given a library component and three candidate imports", func(t *testing.T) {
			modulePath = "example.com/current"
			source = componentDescriptor{
				identifier: "internal/library/logging",
				name:       "logging",
				kind:       componentKindLibrary,
			}
			components = make(map[string]*componentAccumulator)
			ensureComponent(components, source)
			relationships = make(map[relationshipKey]*relationshipAccumulator)
			file = sourceFile{
				relativePath: "internal/library/logging/logging.go",
				packagePath:  "internal/library/logging",
				component:    source,
				imports: []string{
					modulePath + "/internal/library/logging",
					modulePath + "/generate/tool",
					modulePath + "/internal/module/audit",
				},
			}
		})

		t.Run("When relationships are collected from the source imports", func(t *testing.T) {
			collectRelationships(components, relationships, modulePath, file)
		})

		if !t.Run("Then only one relationship and two components exist", func(t *testing.T) {
			if len(relationships) != 1 || len(components) != 2 {
				t.Fatalf(
					"collected %d relationships and %d components",
					len(relationships),
					len(components),
				)
			}
		}) {
			return
		}

		t.Run("And the modeled audit relationship is retained", func(t *testing.T) {
			key := relationshipKey{source: source.identifier, target: "internal/module/audit"}
			if _, exists := relationships[key]; !exists {
				t.Errorf("the expected relationship is absent: %v", relationships)
			}
		})
	})
}
func TestRelationshipConcerns_ClassifyBoundary(t *testing.T) {
	descriptor := func(kind componentKind, application string) componentDescriptor {
		return componentDescriptor{kind: kind, application: application}
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
			source: descriptor(componentKindApplication, "control"),
			target: descriptor(componentKindDevelopment, ""),
			want:   []string{"production-depends-on-development"},
		},
		{
			name:   "a library depends on an application",
			source: descriptor(componentKindLibrary, ""),
			target: descriptor(componentKindApplication, "control"),
			want:   []string{"library-depends-on-feature"},
		},
		{
			name:   "a library depends on an application module",
			source: descriptor(componentKindLibrary, ""),
			target: descriptor(componentKindApplicationModule, "control"),
			want:   []string{"library-depends-on-feature"},
		},
		{
			name:   "shared infrastructure depends on an application module",
			source: descriptor(componentKindInfrastructure, ""),
			target: descriptor(componentKindApplicationModule, "control"),
			want:   []string{"shared-foundation-depends-on-application"},
		},
		{
			name:   "a shared module depends on an application",
			source: descriptor(componentKindSharedModule, ""),
			target: descriptor(componentKindApplication, "control"),
			want:   []string{"shared-foundation-depends-on-application"},
		},
		{
			name:   "one application module depends on another application",
			source: descriptor(componentKindApplicationModule, "control"),
			target: descriptor(componentKindApplicationModule, "portal"),
			want:   []string{"cross-application-module-dependency"},
		},
		{
			name:   "modules from the same application can depend on each other",
			source: descriptor(componentKindApplicationModule, "control"),
			target: descriptor(componentKindApplicationModule, "control"),
			want:   []string{},
		},
		{
			name:     "test imports do not create strategic concerns",
			source:   descriptor(componentKindLibrary, ""),
			target:   descriptor(componentKindApplication, "control"),
			testOnly: true,
			want:     []string{},
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var source componentDescriptor
			var target componentDescriptor
			var testOnly bool
			var concerns []string

			t.Run("Given source and target strategic descriptors", func(t *testing.T) {
				source = testCase.source
				target = testCase.target
				testOnly = testCase.testOnly
			})

			t.Run("When relationship concerns are classified", func(t *testing.T) {
				concerns = relationshipConcerns(source, target, testOnly)
			})

			t.Run("Then the expected strategic concerns are returned", func(t *testing.T) {
				if !slices.Equal(concerns, testCase.want) {
					t.Fatalf("concerns are %v, want %v", concerns, testCase.want)
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

		t.Run("When reachable nodes are collected from the start", func(t *testing.T) {
			result = reachable("a", adjacency)
		})

		t.Run("Then each downstream node is returned once without the start", func(t *testing.T) {
			got := sortedSet(result)
			if !slices.Equal(got, []string{"b", "c"}) {
				t.Fatalf("reachable nodes are %v, want [b c]", got)
			}
		})
	})
}
func TestInstability_CalculateCouplingRatio(t *testing.T) {
	testCases := []struct {
		name   string
		fanIn  int
		fanOut int
		want   float64
	}{
		{name: "an isolated component is stable", want: 0},
		{name: "one outgoing edge exists among three edges", fanIn: 2, fanOut: 1, want: 1.0 / 3.0},
		{name: "only outgoing edges exist", fanOut: 2, want: 1},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var fanIn int
			var fanOut int
			var result float64

			t.Run("Given incoming and outgoing coupling counts", func(t *testing.T) {
				fanIn = testCase.fanIn
				fanOut = testCase.fanOut
			})

			t.Run("When instability is calculated", func(t *testing.T) {
				result = instability(fanIn, fanOut)
			})

			t.Run("Then the expected coupling ratio is returned", func(t *testing.T) {
				if math.Abs(result-testCase.want) > 1e-12 {
					t.Fatalf("instability is %v, want %v", result, testCase.want)
				}
			})
		})
	}
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

		t.Run("When strongly connected components are calculated", func(t *testing.T) {
			cycles = stronglyConnectedComponents(identifiers, adjacency)
		})

		t.Run("Then only the sorted multi-node cycles are returned", func(t *testing.T) {
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

		t.Run("When the dependency graph is built", func(t *testing.T) {
			graph = buildGraph(
				"example.com/current",
				map[string]*componentAccumulator{},
				nil,
				diagnostics,
			)
		})

		t.Run("Then diagnostics are sorted by path and message", func(t *testing.T) {
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
	writeFixtureFile(t, repositoryRoot, "go.mod", "module example.com/strategic\n\ngo 1.26\n")
	writeFixtureFile(
		t,
		repositoryRoot,
		"cmd/control/main.go",
		`package main

import (
	_ "example.com/strategic/cmd/control/internal/module/orders"
	_ "example.com/strategic/internal/module/audit"
)
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"cmd/control/internal/module/orders/module.go",
		`package orders

import (
	_ "example.com/strategic/internal/library/logging"
	_ "example.com/strategic/internal/module/audit"
)
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/module/audit/module.go",
		`package audit

import _ "example.com/strategic/internal/library/logging"
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/module/cycleone/module.go",
		`package cycleone

import _ "example.com/strategic/internal/module/cycletwo"
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/module/cycletwo/module.go",
		`package cycletwo

import _ "example.com/strategic/internal/module/cycleone"
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/library/logging/logging.go",
		"package logging\n",
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/library/logging/logging_test.go",
		`package logging_test

import _ "example.com/strategic/internal/module/audit"
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/library/facade/facade.go",
		`package facade

import _ "example.com/strategic/internal/module/audit"
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/devtool/generator/main.go",
		`package main

import _ "example.com/strategic/internal/library/logging"
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
	t.Fatalf("the graph carries no component %q", identifier)
	return Component{}
}

func relationshipBetween(t *testing.T, graph Graph, source, target string) Relationship {
	t.Helper()
	for _, relationship := range graph.Relationships {
		if relationship.Source == source && relationship.Target == target {
			return relationship
		}
	}
	t.Fatalf("the graph carries no relationship from %q to %q", source, target)
	return Relationship{}
}
