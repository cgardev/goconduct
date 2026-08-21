package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var errAnalysisWrite = errors.New("analysis write failed")

type failingAnalysisWriter struct{}

func (failingAnalysisWriter) Write([]byte) (int, error) {
	return 0, errAnalysisWrite
}

func TestAnalyzeCommand_EmitDeterministicReport(t *testing.T) {
	t.Run("Scenario: The same repository is analyzed twice through the CLI", func(t *testing.T) {
		var repositoryRoot string
		var firstOutput bytes.Buffer
		var secondOutput bytes.Buffer
		var firstError error
		var secondError error
		var report analysisReport

		t.Run("Given two machine-analysis commands for one repository", func(*testing.T) {
			repositoryRoot = newAnalyzerFixture(t)
		})

		t.Run("When both commands emit the default report view", func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			first := newRootCommand(logger)
			first.SetOut(&firstOutput)
			first.SetArgs([]string{"analyze", "--root", repositoryRoot})
			firstError = first.ExecuteContext(t.Context())

			second := newRootCommand(logger)
			second.SetOut(&secondOutput)
			second.SetArgs([]string{"analyze", "--root", repositoryRoot})
			secondError = second.ExecuteContext(t.Context())
		})

		if !t.Run("Then both executions succeed with byte-identical JSON", func(t *testing.T) {
			if firstError != nil || secondError != nil {
				t.Fatalf("analysis errors are %v and %v", firstError, secondError)
			}
			if !bytes.Equal(firstOutput.Bytes(), secondOutput.Bytes()) {
				t.Fatal("unchanged source produced different CLI reports")
			}
			if err := json.Unmarshal(firstOutput.Bytes(), &report); err != nil {
				t.Fatalf("decode CLI report: %v", err)
			}
		}) {
			return
		}

		t.Run("And the report contains the stable revision and detected risks", func(t *testing.T) {
			if report.SchemaVersion != graphSchemaVersion || report.Revision == "" {
				t.Errorf("unexpected report identity: %+v", report)
			}
			if report.Summary.Findings != 3 || len(report.Findings) != 3 {
				t.Errorf("report has %d summary findings and %d details, want 3 and 3", report.Summary.Findings, len(report.Findings))
			}
			if report.Policy.ZoneOfPain.MinimumAfferentCoupling != 1 {
				t.Errorf("report has an unexpected policy: %+v", report.Policy)
			}
			if strings.Contains(firstOutput.String(), "\n  \"") {
				t.Error("the default machine report is unexpectedly indented")
			}
		})
	})
}

func TestAnalyzeCommand_ApplyFailureThreshold(t *testing.T) {
	t.Run("Scenario: The repository has an architectural error and the error gate is active", func(t *testing.T) {
		var output bytes.Buffer
		var commandError error
		var command *cobra.Command

		t.Run("Given a machine-analysis command with fail-on error", func(*testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			command = newRootCommand(logger)
			command.SetOut(&output)
			command.SetArgs([]string{
				"analyze",
				"--root", repositoryRoot,
				"--fail-on", "error",
			})
		})

		t.Run("When the deterministic quality gate is evaluated", func(t *testing.T) {
			commandError = command.ExecuteContext(t.Context())
		})

		t.Run("Then the command returns the typed finding error", func(t *testing.T) {
			if !errors.Is(commandError, errArchitectureFindings) {
				t.Fatalf("command error is %v, want errArchitectureFindings", commandError)
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
	})
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
			command = newRootCommand(logger)
			command.SetOut(&output)
			command.SetArgs([]string{
				"analyze",
				"--root", repositoryRoot,
				"--view", "graph",
				"--pretty",
			})
		})

		t.Run("When the graph view is emitted", func(t *testing.T) {
			commandError = command.ExecuteContext(t.Context())
		})

		if !t.Run("Then the command emits valid JSON", func(t *testing.T) {
			if commandError != nil {
				t.Fatalf("graph analysis failed: %v", commandError)
			}
			if err := json.Unmarshal(output.Bytes(), &graph); err != nil {
				t.Fatalf("decode complete graph: %v", err)
			}
		}) {
			return
		}

		t.Run("And the complete view contains relationships and indentation", func(t *testing.T) {
			if len(graph.Relationships) != 10 || len(graph.Findings) != 3 {
				t.Errorf("complete graph has %d relationships and %d findings", len(graph.Relationships), len(graph.Findings))
			}
			if !strings.Contains(output.String(), "\n  \"schemaVersion\"") {
				t.Error("pretty graph output is not indented")
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

			t.Run("When the finding threshold is enforced", func(t *testing.T) {
				result = enforceFindingThreshold(findings, threshold)
			})

			t.Run("Then the gate returns the expected typed result", func(t *testing.T) {
				failed := errors.Is(result, errArchitectureFindings)
				if failed != testCase.wantFailure {
					t.Fatalf("threshold error is %v, want failure %t", result, testCase.wantFailure)
				}
			})
		})
	}
}

func TestAnalysisJSON_ReportWriteFailure(t *testing.T) {
	t.Run("Scenario: The machine output writer rejects the JSON report", func(t *testing.T) {
		var writer io.Writer
		var result error

		t.Run("Given a failing writer and an empty deterministic graph", func(t *testing.T) {
			writer = failingAnalysisWriter{}
		})

		t.Run("When the report transport encodes JSON", func(t *testing.T) {
			result = writeAnalysisJSON(writer, Graph{}, analysisViewReport, false)
		})

		t.Run("Then the underlying output error remains classified", func(t *testing.T) {
			if !errors.Is(result, errAnalysisWrite) {
				t.Fatalf("writeAnalysisJSON returned %v, want errAnalysisWrite", result)
			}
		})
	})
}

func TestAnalysisJSON_PreserveTechnicalCharacters(t *testing.T) {
	t.Run("Scenario: The module path contains characters escaped by HTML encoders", func(t *testing.T) {
		var output bytes.Buffer
		var graph Graph
		var result error

		t.Run("Given a graph with angle and ampersand characters", func(t *testing.T) {
			graph = Graph{ModulePath: "example.com/<machine>&analysis"}
		})

		t.Run("When the machine report is encoded", func(t *testing.T) {
			result = writeAnalysisJSON(&output, graph, analysisViewReport, false)
		})

		if !t.Run("Then JSON encoding succeeds", func(t *testing.T) {
			if result != nil {
				t.Fatalf("writeAnalysisJSON failed: %v", result)
			}
		}) {
			return
		}

		t.Run("And technical characters remain directly readable", func(t *testing.T) {
			if !strings.Contains(output.String(), "example.com/<machine>&analysis") {
				t.Errorf("technical characters were escaped: %s", output.String())
			}
		})
	})
}
