package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/failure"
	querymodel "digginginsights.com/v3/internal/devtool/dependencygraph/internal/query"
)

func TestAnalyzeCommand_UseConfiguredScope(t *testing.T) {
	t.Run("Scenario: The CLI analyzes a generic layout from an external document", func(t *testing.T) {
		var configurationPath string
		var output bytes.Buffer
		var commandError error
		var report analysisReport

		if !t.Run("Given a repository and a gokeel configuration document", func(step *testing.T) {
			repositoryRoot := t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/cli\n\ngo 1.26\n")
			writeFixtureFile(
				step,
				repositoryRoot,
				"services/control/features/orders/orders.go",
				`package orders

import _ "example.com/cli/packages/shared"
`,
			)
			writeFixtureFile(step, repositoryRoot, "packages/shared/shared.go", "package shared\n")
			document, err := json.Marshal(map[string]any{
				"analysis": map[string]any{
					"repositoryRoot": repositoryRoot,
					"paths":          []string{"services", "packages"},
					"ignoredPaths":   []string{"generated"},
					"components": map[string]any{
						"applications":       []string{"services/{application}"},
						"applicationModules": []string{"services/{application}/features/{component}"},
						"sharedModules":      []string{},
						"libraries":          []string{"packages/{component}"},
						"infrastructure":     []string{},
						"developmentTools":   []string{},
					},
				},
			})
			if err != nil {
				step.Fatalf("encode configuration document: %v", err)
			}
			configurationPath = filepath.Join(t.TempDir(), "configuration.json")
			writeFixtureFile(
				step,
				filepath.Dir(configurationPath),
				filepath.Base(configurationPath),
				string(document),
			)
		}) {
			return
		}

		t.Run("When the JSON analysis uses only the configuration document", func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			command := newTestRootCommand(logger)
			command.SetOut(&output)
			command.SetArgs([]string{"analyze", "--configuration", configurationPath})
			commandError = command.ExecuteContext(t.Context())
			if commandError == nil {
				commandError = json.Unmarshal(output.Bytes(), &report)
			}
		})

		if !t.Run("Then the configured JSON analysis succeeds", func(t *testing.T) {
			if commandError != nil {
				t.Fatalf("the configured analysis fails: %v", commandError)
			}
			if report.Summary.Components != 2 || report.Summary.Relationships != 1 {
				t.Fatalf("unexpected configured report: %+v", report.Summary)
			}
		}) {
			return
		}

		t.Run("And the report exposes the exact paths and generic templates", func(t *testing.T) {
			if !slices.Equal(report.Scope.Paths, []string{"packages", "services"}) ||
				!slices.Equal(
					report.Scope.Components.Libraries,
					[]string{"packages/{component}"},
				) {
				t.Errorf("unexpected configured report scope: %+v", report.Scope)
			}
		})
	})
}

func TestQueryCommands_EmitFilteredJSONWithoutPipes(t *testing.T) {
	t.Run("Scenario: An agent invokes every filtered query through native CLI arguments", func(t *testing.T) {
		var repositoryRoot string
		var outputs map[string]map[string]json.RawMessage
		var queryError error

		t.Run("Given a repository that the default component layout accepts", func(*testing.T) {
			repositoryRoot = newAnalyzerFixture(t)
			outputs = make(map[string]map[string]json.RawMessage)
		})

		t.Run("When summary, findings, component sort, and component queries execute", func(t *testing.T) {
			queries := []struct {
				name      string
				arguments []string
			}{
				{name: "summary", arguments: []string{"summary", "--root", repositoryRoot}},
				{
					name:      "findings",
					arguments: []string{"findings", "--root", repositoryRoot, "--severity", "error", "--limit", "1"},
				},
				{
					name: "components",
					arguments: []string{
						"components", "--root", repositoryRoot, "--role", "library",
						"--sort", "afferent", "--limit", "1",
					},
				},
				{
					name:      "component",
					arguments: []string{"component", "internal/library/logging", "--root", repositoryRoot},
				},
			}
			for _, query := range queries {
				var output bytes.Buffer
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				command := newTestRootCommand(logger)
				command.SetOut(&output)
				command.SetArgs(query.arguments)
				if err := command.ExecuteContext(t.Context()); err != nil {
					queryError = err
					return
				}
				var document map[string]json.RawMessage
				if err := json.Unmarshal(output.Bytes(), &document); err != nil {
					queryError = err
					return
				}
				outputs[query.name] = document
			}
		})

		if !t.Run("Then every filtered query emits valid JSON", func(t *testing.T) {
			if queryError != nil {
				t.Fatalf("the filtered query fails: %v", queryError)
			}
			if len(outputs) != 4 {
				t.Fatalf("filtered query outputs are %d, want 4", len(outputs))
			}
		}) {
			return
		}

		t.Run("And each response has its direct resource without an external filter", func(t *testing.T) {
			for query, field := range map[string]string{
				"summary":    "summary",
				"findings":   "findings",
				"components": "components",
				"component":  "component",
			} {
				if outputs[query][field] == nil {
					t.Errorf("%s query has no direct %s field", query, field)
				}
			}
		})
	})
}

func TestFindingsCommand_DefineUnlimitedDefault(t *testing.T) {
	t.Run("Scenario: A client creates the findings command without a limit", func(t *testing.T) {
		var command *cobra.Command
		var defaultLimit string

		t.Run("Given the standard findings command", func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			command = newFindingsCommand(
				&commandConfigurationFlags{},
				newTestCommandRuntime(logger),
			)
		})

		t.Run("When the client reads the limit default", func(t *testing.T) {
			defaultLimit = command.Flag("limit").DefValue
		})

		t.Run("Then zero requests all applicable findings", func(t *testing.T) {
			if defaultLimit != "0" {
				t.Errorf("default finding limit is %q, want 0", defaultLimit)
			}
		})
	})
}

func TestQueryLimit_AcceptUnlimitedValue(t *testing.T) {
	t.Run("Scenario: A query uses zero as the unlimited value", func(t *testing.T) {
		var limit int
		var validationError error

		t.Run("Given the documented zero limit", func(t *testing.T) {
			limit = 0
		})

		t.Run("When the query validates the limit", func(t *testing.T) {
			validationError = validateQueryLimit(limit)
		})

		t.Run("Then validation accepts the unlimited value", func(t *testing.T) {
			if validationError != nil {
				t.Errorf("zero query limit fails validation: %v", validationError)
			}
		})
	})
}

func TestQueryJSON_PreserveTechnicalCharacters(t *testing.T) {
	t.Run("Scenario: A query value contains HTML control characters", func(t *testing.T) {
		var output bytes.Buffer
		var payload map[string]string
		var encodeError error

		t.Run("Given a JSON value with angle brackets and an ampersand", func(t *testing.T) {
			payload = map[string]string{
				"identifier": "example.com/<component>&analysis",
			}
		})

		t.Run("When the encoder encodes the query result", func(t *testing.T) {
			encodeError = writeQueryJSON(&output, payload)
		})

		if !t.Run("Then the JSON encoder returns no error", func(t *testing.T) {
			if encodeError != nil {
				t.Fatalf("writeQueryJSON fails: %v", encodeError)
			}
		}) {
			return
		}

		t.Run("And technical characters remain directly readable", func(t *testing.T) {
			if !strings.Contains(output.String(), `<component>&analysis`) {
				t.Errorf("the encoder escapes technical characters: %s", output.String())
			}
		})
	})
}

func TestCommandConfiguration_ApplyExplicitScopeOverrides(t *testing.T) {
	t.Run("Scenario: CLI scope flags override an external configuration document", func(t *testing.T) {
		var repositoryRoot string
		var configurationPath string
		var output bytes.Buffer
		var commandError error
		var result querymodel.SummaryResult

		t.Run("Given a document and explicit repository, analysis, and ignore flags", func(*testing.T) {
			repositoryRoot = newAnalyzerFixture(t)
			configurationPath = filepath.Join(t.TempDir(), "configuration.json")
			writeFixtureFile(
				t,
				filepath.Dir(configurationPath),
				filepath.Base(configurationPath),
				`{"analysis":{"repositoryRoot":"absent","paths":["cmd"],"ignoredPaths":["vendor"]}}`,
			)
		})

		t.Run("When the native summary command executes with every scope override", func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			command := newTestRootCommand(logger)
			command.SetOut(&output)
			command.SetArgs([]string{
				"summary",
				"--configuration", configurationPath,
				"--root", repositoryRoot,
				"--analysis-path", "internal/library/logging",
				"--ignore-path", "internal/module/audit",
			})
			commandError = command.ExecuteContext(t.Context())
			if commandError == nil {
				commandError = json.Unmarshal(output.Bytes(), &result)
			}
		})

		if !t.Run("Then the explicitly configured query succeeds", func(t *testing.T) {
			if commandError != nil {
				t.Fatalf("summary with scope overrides fails: %v", commandError)
			}
		}) {
			return
		}

		t.Run("And all scope flags replace their document values", func(t *testing.T) {
			if !slices.Equal(result.Scope.Paths, []string{"internal/library/logging"}) ||
				!slices.Equal(result.Scope.IgnoredPaths, []string{"internal/module/audit"}) ||
				result.Summary.Components != 1 {
				t.Errorf("unexpected overridden scope or summary: %+v %+v", result.Scope, result.Summary)
			}
		})
	})
}

var errAnalysisWrite = errors.New("analysis write failure")

type failingAnalysisWriter struct{}

func (failingAnalysisWriter) Write([]byte) (int, error) {
	return 0, errAnalysisWrite
}

func TestAnalyzeCommand_EmitDeterministicReport(t *testing.T) {
	t.Run("Scenario: The CLI analyzes the same repository twice", func(t *testing.T) {
		var repositoryRoot string
		var firstOutput bytes.Buffer
		var secondOutput bytes.Buffer
		var firstError error
		var secondError error
		var report analysisReport

		t.Run("Given two JSON analysis commands for one repository", func(*testing.T) {
			repositoryRoot = newAnalyzerFixture(t)
		})

		t.Run("When both commands emit the default report view", func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			first := newTestRootCommand(logger)
			first.SetOut(&firstOutput)
			first.SetArgs([]string{"analyze", "--root", repositoryRoot})
			firstError = first.ExecuteContext(t.Context())

			second := newTestRootCommand(logger)
			second.SetOut(&secondOutput)
			second.SetArgs([]string{"analyze", "--root", repositoryRoot})
			secondError = second.ExecuteContext(t.Context())
		})

		if !t.Run("Then both executions succeed with byte-identical JSON", func(t *testing.T) {
			if firstError != nil || secondError != nil {
				t.Fatalf("analysis errors are %v and %v", firstError, secondError)
			}
			if !bytes.Equal(firstOutput.Bytes(), secondOutput.Bytes()) {
				t.Fatal("unchanged source produces different CLI reports")
			}
			if err := json.Unmarshal(firstOutput.Bytes(), &report); err != nil {
				t.Fatalf("decode CLI report: %v", err)
			}
		}) {
			return
		}

		t.Run("And the report contains the stable revision and findings", func(t *testing.T) {
			if report.SchemaVersion != graphSchemaVersion || report.Revision == "" {
				t.Errorf("unexpected report identity: %+v", report)
			}
			if report.Summary.Findings != 3 || len(report.Findings) != 3 {
				t.Errorf(
					"report has %d summary findings and %d details, want 3 and 3",
					report.Summary.Findings,
					len(report.Findings),
				)
			}
			if report.Policy.StableLowAbstraction.MinimumAfferentCoupling != 1 {
				t.Errorf("report has an unexpected policy: %+v", report.Policy)
			}
			if strings.Contains(firstOutput.String(), "\n  \"") {
				t.Error("the default JSON report has unexpected indentation")
			}
		})
	})
}

func TestAnalyzeCommand_ApplyFailureThreshold(t *testing.T) {
	t.Run(
		"Scenario: The repository has an architecture error and the failure threshold is active",
		func(t *testing.T) {
			var output bytes.Buffer
			var commandError error
			var command *cobra.Command

			t.Run("Given a JSON analysis command with fail-on error", func(*testing.T) {
				repositoryRoot := newAnalyzerFixture(t)
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				command = newTestRootCommand(logger)
				command.SetOut(&output)
				command.SetArgs([]string{
					"analyze",
					"--root", repositoryRoot,
					"--fail-on", "error",
				})
			})

			t.Run("When the command evaluates the deterministic failure threshold", func(t *testing.T) {
				commandError = command.ExecuteContext(t.Context())
			})

			t.Run("Then the command returns the business rule error category", func(t *testing.T) {
				if !errors.Is(commandError, failure.ErrBusinessRule) {
					t.Fatalf("command error is %v, want ErrBusinessRule", commandError)
				}
			})

			t.Run("And the JSON report remains available for automated diagnosis", func(t *testing.T) {
				var report analysisReport
				if err := json.Unmarshal(output.Bytes(), &report); err != nil {
					t.Fatalf("decode gated CLI report: %v", err)
				}
				if report.Summary.Errors != 1 {
					t.Errorf("error findings are %d, want 1", report.Summary.Errors)
				}
			})
		},
	)
}

func TestAnalyzeCommand_EmitCompleteGraph(t *testing.T) {
	t.Run("Scenario: An automated client requests the complete indented graph", func(t *testing.T) {
		var output bytes.Buffer
		var command *cobra.Command
		var commandError error
		var graph Graph

		t.Run("Given a graph-view command with human-readable indentation", func(*testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			command = newTestRootCommand(logger)
			command.SetOut(&output)
			command.SetArgs([]string{
				"analyze",
				"--root", repositoryRoot,
				"--view", "graph",
				"--indent",
			})
		})

		t.Run("When the command emits the graph view", func(t *testing.T) {
			commandError = command.ExecuteContext(t.Context())
		})

		if !t.Run("Then the command emits valid JSON", func(t *testing.T) {
			if commandError != nil {
				t.Fatalf("graph analysis fails: %v", commandError)
			}
			if err := json.Unmarshal(output.Bytes(), &graph); err != nil {
				t.Fatalf("decode complete graph: %v", err)
			}
		}) {
			return
		}

		t.Run("And the complete view contains relationships and indentation", func(t *testing.T) {
			if len(graph.Relationships) != 10 || len(graph.Findings) != 3 {
				t.Errorf(
					"complete graph has %d relationships and %d findings",
					len(graph.Relationships),
					len(graph.Findings),
				)
			}
			if !strings.Contains(output.String(), "\n  \"schemaVersion\"") {
				t.Error("indented graph output is not indented")
			}
		})
	})
}

func TestFindingThreshold_FilterSeverity(t *testing.T) {
	testCases := []struct {
		name        string
		findings    []Finding
		threshold   findingThreshold
		wantFailure bool
	}{
		{
			name:        "a warning reaches the warning threshold",
			findings:    []Finding{{Severity: findingSeverityWarning}},
			threshold:   findingThresholdWarning,
			wantFailure: true,
		},
		{
			name:      "a warning remains below the error threshold",
			findings:  []Finding{{Severity: findingSeverityWarning}},
			threshold: findingThresholdError,
		},
		{
			name:        "an error reaches the error threshold",
			findings:    []Finding{{Severity: findingSeverityError}},
			threshold:   findingThresholdError,
			wantFailure: true,
		},
		{
			name:      "the disabled threshold accepts an error",
			findings:  []Finding{{Severity: findingSeverityError}},
			threshold: findingThresholdNone,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var findings []Finding
			var threshold findingThreshold
			var result error

			t.Run("Given findings and a deterministic severity threshold", func(t *testing.T) {
				findings = testCase.findings
				threshold = testCase.threshold
			})

			t.Run("When the function enforces the finding threshold", func(t *testing.T) {
				result = enforceFindingThreshold(findings, threshold)
			})

			t.Run("Then the failure threshold returns the expected typed result", func(t *testing.T) {
				hasFailure := errors.Is(result, failure.ErrBusinessRule)
				if hasFailure != testCase.wantFailure {
					t.Fatalf("threshold error is %v, want failure %t", result, testCase.wantFailure)
				}
			})
		})
	}
}

func TestAnalysisJSON_ReportWriteFailure(t *testing.T) {
	t.Run("Scenario: The JSON output writer rejects the JSON report", func(t *testing.T) {
		var writer io.Writer
		var result error

		t.Run("Given a writer that fails and an empty deterministic graph", func(t *testing.T) {
			writer = failingAnalysisWriter{}
		})

		t.Run("When the report transport encodes JSON", func(t *testing.T) {
			result = writeAnalysisJSON(writer, Graph{}, analysisViewReport, false)
		})

		t.Run("Then the result retains the output error category", func(t *testing.T) {
			if !errors.Is(result, errAnalysisWrite) {
				t.Fatalf("writeAnalysisJSON returns %v, want errAnalysisWrite", result)
			}
		})
	})
}

func TestAnalysisJSON_PreserveTechnicalCharacters(t *testing.T) {
	t.Run("Scenario: HTML encoders escape characters in the module path", func(t *testing.T) {
		var output bytes.Buffer
		var graph Graph
		var result error

		t.Run("Given a graph with angle and ampersand characters", func(t *testing.T) {
			graph = Graph{ModulePath: "example.com/<component>&analysis"}
		})

		t.Run("When the encoder encodes the JSON report", func(t *testing.T) {
			result = writeAnalysisJSON(&output, graph, analysisViewReport, false)
		})

		if !t.Run("Then the JSON encoder returns no error", func(t *testing.T) {
			if result != nil {
				t.Fatalf("writeAnalysisJSON fails: %v", result)
			}
		}) {
			return
		}

		t.Run("And technical characters remain directly readable", func(t *testing.T) {
			if !strings.Contains(output.String(), "example.com/<component>&analysis") {
				t.Errorf("the encoder escapes technical characters: %s", output.String())
			}
		})
	})
}
