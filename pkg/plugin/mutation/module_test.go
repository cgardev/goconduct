package mutation

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/internal/kernel"
	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
)

// errReportSink stops the report encoder, so one test reads the write failure.
var errReportSink = errors.New("the report destination is closed")

// failingWriter rejects every report that the command writes.
type failingWriter struct{}

var _ io.Writer = failingWriter{}

func (failingWriter) Write([]byte) (int, error) { return 0, errReportSink }

func newTestInjector(t *testing.T, services ...func(do.Injector)) do.Injector {
	t.Helper()
	injector := do.New(services...)
	t.Cleanup(func() {
		if report := injector.Shutdown(); report != nil && !report.Succeed {
			t.Errorf("shut down injector: %s", report.Error())
		}
	})
	return injector
}

func newKernelInjector(t *testing.T, services ...func(do.Injector)) do.Injector {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kernelServices := []func(do.Injector){
		kernel.Module,
		func(injector do.Injector) { do.OverrideValue(injector, logger) },
	}
	return newTestInjector(t, append(kernelServices, services...)...)
}

// runMutationCommand executes the command with a deterministic runner.
func runMutationCommand(
	t *testing.T,
	runner plugin.CommandRunner,
	destination io.Writer,
	arguments ...string,
) error {
	t.Helper()
	command := newMutationCommand(runner)
	command.SilenceUsage = true
	command.SilenceErrors = true
	command.SetOut(destination)
	command.SetErr(io.Discard)
	command.SetArgs(arguments)
	return command.ExecuteContext(t.Context())
}

// decodeMutationReport reads the JSON report that the command wrote.
func decodeMutationReport(t *testing.T, payload []byte) plugin.Report {
	t.Helper()
	var report plugin.Report
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("decode the mutation report: %v", err)
	}
	if report.Plugin != "mutation" || report.SchemaVersion != plugin.ReportSchemaVersion {
		t.Fatalf("the report header is %d of %q", report.SchemaVersion, report.Plugin)
	}
	return report
}

// findingRules lists the rules of one report, in report order.
func findingRules(report plugin.Report) []string {
	rules := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		rules = append(rules, finding.Rule)
	}
	return rules
}

func TestPluginRegistersMutationEvaluatorAndCommand(t *testing.T) {
	candidate := Plugin()
	if name := candidate.Name(); name != "mutation" {
		t.Errorf("the plugin name is %q, want mutation", name)
	}
	injector := newKernelInjector(t, candidate.Services())

	if err := candidate.Activate(t.Context(), injector); err != nil {
		t.Fatalf("activate mutation plugin: %v", err)
	}

	catalog, err := do.Invoke[*plugin.Catalog](injector)
	if err != nil {
		t.Fatalf("resolve catalog: %v", err)
	}
	if names := catalog.Names(); !slices.Equal(names, []string{"mutation"}) {
		t.Fatalf("the catalog holds %v, want the mutation evaluator", names)
	}
	root := &cobra.Command{Use: "goconduct"}
	if err := candidate.RegisterCommands(injector, root); err != nil {
		t.Fatalf("register mutation command: %v", err)
	}
	command, _, err := root.Find([]string{"mutation"})
	if err != nil || command.Name() != "mutation" {
		t.Fatalf("find mutation command: command=%v, error=%v", command, err)
	}
	if err := candidate.RegisterEndpoints(injector, nil); err != nil {
		t.Errorf("the mutation plugin mounts an endpoint: %v", err)
	}
}

func TestEvaluatorInjectorSelectsTheProvidedConfiguration(t *testing.T) {
	testCases := []struct {
		name        string
		services    []func(do.Injector)
		wantCommand string
		wantExecute bool
	}{
		{name: "no configuration in the container", wantCommand: "go"},
		{
			name: "one configuration in the container",
			services: []func(do.Injector){func(injector do.Injector) {
				configuration := DefaultConfiguration()
				configuration.Command = "custom-go"
				configuration.Execute = true
				do.ProvideValue(injector, configuration)
			}},
			wantCommand: "custom-go",
			wantExecute: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			injector := newKernelInjector(t, append([]func(do.Injector){Module}, testCase.services...)...)

			evaluator, err := do.Invoke[*Evaluator](injector)

			if err != nil {
				t.Fatalf("resolve evaluator: %v", err)
			}
			if evaluator == nil {
				t.Fatal("the container resolves a nil evaluator")
			}
			if evaluator.configuration.Command != testCase.wantCommand {
				t.Errorf("the evaluator command is %q, want %q",
					evaluator.configuration.Command, testCase.wantCommand)
			}
			if evaluator.configuration.Execute != testCase.wantExecute {
				t.Errorf("the evaluator runs every mutation: %v", evaluator.configuration.Execute)
			}
		})
	}
}

func TestPluginRejectsAnIncompleteContainer(t *testing.T) {
	testCases := []struct {
		name     string
		services []func(do.Injector)
		register func(t *testing.T, injector do.Injector) error
	}{
		{
			name:     "an activation without an evaluator catalog",
			services: []func(do.Injector){Module},
			register: func(t *testing.T, injector do.Injector) error {
				t.Helper()
				return Plugin().Activate(t.Context(), injector)
			},
		},
		{
			name: "an activation without a command runner",
			services: []func(do.Injector){
				Module,
				func(injector do.Injector) { do.ProvideValue(injector, plugin.NewCatalog()) },
			},
			register: func(t *testing.T, injector do.Injector) error {
				t.Helper()
				return Plugin().Activate(t.Context(), injector)
			},
		},
		{
			name:     "a command registration without a command runner",
			services: []func(do.Injector){Module},
			register: func(t *testing.T, injector do.Injector) error {
				t.Helper()
				return Plugin().RegisterCommands(injector, &cobra.Command{Use: "goconduct"})
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			injector := newTestInjector(t, testCase.services...)

			if err := testCase.register(t, injector); err == nil {
				t.Error("the plugin accepts an incomplete container")
			}
		})
	}
}

func TestMutationCommandWritesOneCompactReport(t *testing.T) {
	root := newMutationFixture(t, map[string]string{"grade.go": gradeSource})
	runner := &mutationRunner{profile: gradeProfile}
	var output bytes.Buffer

	err := runMutationCommand(t, runner, &output, "--repository", root)

	if err != nil {
		t.Fatalf("run the mutation command: %v", err)
	}
	written := output.String()
	if strings.Count(written, "\n") != 1 || !strings.HasSuffix(written, "\n") {
		t.Errorf("the command indents the report without the option: %q", written)
	}
	report := decodeMutationReport(t, output.Bytes())
	assertMetrics(t, report, "grade.go", map[string]float64{
		"mutation.total": 2, "mutation.covered": 2, "mutation.uncovered": 0,
	})
	if rules := findingRules(report); len(rules) != 0 {
		t.Errorf("the scan reports the findings %v, want none", rules)
	}
}

func TestMutationCommandIndentsTheReportOnRequest(t *testing.T) {
	root := newMutationFixture(t, map[string]string{"grade.go": gradeSource})
	runner := &mutationRunner{profile: gradeProfile}
	var output bytes.Buffer

	err := runMutationCommand(t, runner, &output, "--repository", root, "--indent")

	if err != nil {
		t.Fatalf("run the mutation command: %v", err)
	}
	if !strings.Contains(output.String(), "\n  \"schemaVersion\": 1") {
		t.Errorf("the command writes no indented report: %q", output.String())
	}
}

func TestMutationCommandRunsEveryCoveredMutationOnRequest(t *testing.T) {
	root := newMutationFixture(t, map[string]string{"grade.go": gradeSource})
	runner := &mutationRunner{profile: gradeProfile, outcomes: []error{errSuiteFails, nil}}
	var output bytes.Buffer

	err := runMutationCommand(t, runner, &output, "--repository", root, "--execute")

	if !errors.Is(err, failure.ErrBusinessRule) {
		t.Fatalf("a report with findings reports %v, want a business rule failure", err)
	}
	if !strings.Contains(err.Error(), "1 policy findings") {
		t.Errorf("the failure message is %q, want the failing finding count", err.Error())
	}
	report := decodeMutationReport(t, output.Bytes())
	assertMetrics(t, report, "grade.go", map[string]float64{
		"mutation.total": 2, "mutation.covered": 2, "mutation.uncovered": 0,
		"mutation.killed": 1, "mutation.survived": 1, "mutation.killed.percent": 50,
	})
	if !strings.Contains(output.String(), "changes > -> >= and the tests still pass") {
		t.Errorf("the command escapes the report text: %q", output.String())
	}
}

func TestMutationCommandAppliesTheLimitOptions(t *testing.T) {
	testCases := []struct {
		name      string
		options   []string
		outcomes  []error
		wantRules []string
	}{
		{
			name:      "the default limit rejects one uncovered site",
			wantRules: []string{"maximum-uncovered-mutations"},
		},
		{
			name:    "a raised uncovered limit accepts the site",
			options: []string{"--maximum-uncovered", "1"},
		},
		{
			name:      "the default limit rejects one surviving mutation",
			options:   []string{"--execute", "--maximum-uncovered", "1"},
			outcomes:  []error{errSuiteFails, nil},
			wantRules: []string{"maximum-surviving-mutations", "surviving-mutation"},
		},
		{
			name:      "a raised survivor limit accepts the surviving mutation",
			options:   []string{"--execute", "--maximum-uncovered", "1", "--maximum-survivors", "1"},
			outcomes:  []error{errSuiteFails, nil},
			wantRules: []string{"surviving-mutation"},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newMutationFixture(t, map[string]string{
				"grade.go": gradeSource, "untested.go": untestedSource,
			})
			runner := &mutationRunner{
				profile: gradeProfile + untestedProfile, outcomes: testCase.outcomes,
			}
			var output bytes.Buffer
			options := append([]string{"--repository", root}, testCase.options...)

			err := runMutationCommand(t, runner, &output, options...)

			report := decodeMutationReport(t, output.Bytes())
			// A notice records evidence, so only an error closes the gate.
			failing := plugin.FailingFindings(report.Findings)
			if failing == 0 && err != nil {
				t.Fatalf("a report with no failing finding reports %v", err)
			}
			if failing != 0 && !errors.Is(err, failure.ErrBusinessRule) {
				t.Fatalf("a report with %d failing findings reports %v", failing, err)
			}
			if rules := findingRules(report); !slices.Equal(rules, testCase.wantRules) {
				t.Errorf("the command reports the rules %v, want %v", rules, testCase.wantRules)
			}
		})
	}
}

func TestMutationCommandSelectsTheRunPackages(t *testing.T) {
	testCases := []struct {
		name         string
		options      []string
		wantPatterns []string
	}{
		{
			name:         "the default selection runs the whole module",
			wantPatterns: []string{"./..."},
		},
		{
			name:         "one option replaces the default selection",
			options:      []string{"--package", "./alpha/..."},
			wantPatterns: []string{"./alpha/..."},
		},
		{
			name:         "two options replace the default selection",
			options:      []string{"--package", "./alpha/...", "--package", "./beta/..."},
			wantPatterns: []string{"./alpha/...", "./beta/..."},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newMutationFixture(t, map[string]string{"grade.go": gradeSource})
			runner := &mutationRunner{profile: gradeProfile}
			var output bytes.Buffer
			options := append([]string{"--repository", root}, testCase.options...)

			if err := runMutationCommand(t, runner, &output, options...); err != nil {
				t.Fatalf("run the mutation command: %v", err)
			}

			patterns := packagePatterns(runner.calls[0].command)
			if !slices.Equal(patterns, testCase.wantPatterns) {
				t.Errorf("the unchanged suite runs the packages %v, want %v",
					patterns, testCase.wantPatterns)
			}
		})
	}
}

func TestMutationCommandReportsEveryFailure(t *testing.T) {
	testCases := []struct {
		name         string
		options      []string
		emptyRoot    bool
		failingSink  bool
		wantCategory error
	}{
		{
			name:         "a rejected limit stops the command before the analysis",
			options:      []string{"--maximum-survivors", "-1"},
			wantCategory: failure.ErrValidation,
		},
		{
			name:         "a repository without a production Go file",
			emptyRoot:    true,
			wantCategory: failure.ErrValidation,
		},
		{
			name:         "a report destination that rejects the report",
			failingSink:  true,
			wantCategory: failure.ErrUnavailable,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			files := map[string]string{"grade.go": gradeSource}
			if testCase.emptyRoot {
				files = map[string]string{}
			}
			root := newMutationFixture(t, files)
			runner := &mutationRunner{profile: gradeProfile}
			var output bytes.Buffer
			var destination io.Writer = &output
			if testCase.failingSink {
				destination = failingWriter{}
			}
			options := append([]string{"--repository", root}, testCase.options...)

			err := runMutationCommand(t, runner, destination, options...)

			if !errors.Is(err, testCase.wantCategory) {
				t.Fatalf("the command reports %v, want the category %v", err, testCase.wantCategory)
			}
			if !testCase.failingSink && output.Len() != 0 {
				t.Errorf("a failed command writes the report %q", output.String())
			}
		})
	}
}
