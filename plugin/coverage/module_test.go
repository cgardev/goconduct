package coverage

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/failure"
	"github.com/cgardev/goconduct/internal/kernel"
	"github.com/cgardev/goconduct/plugin"
)

// failingWriter rejects every report the command writes.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, failure.Unavailable("write to a closed destination", nil)
}

func newInjector(t *testing.T, services ...func(do.Injector)) do.Injector {
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
	all := append([]func(do.Injector){
		kernel.Module,
		func(injector do.Injector) { do.OverrideValue(injector, logger) },
	}, services...)
	return newInjector(t, all...)
}

func TestPluginRegistersCoverageEvaluatorAndCommand(t *testing.T) {
	candidate := Plugin()
	if candidate.Name() != "coverage" {
		t.Errorf("the plugin name is %q, want %q", candidate.Name(), "coverage")
	}
	injector := newKernelInjector(t, candidate.Services())
	if err := candidate.Activate(t.Context(), injector); err != nil {
		t.Fatalf("activate coverage plugin: %v", err)
	}
	catalog, err := do.Invoke[*plugin.Catalog](injector)
	if err != nil {
		t.Fatalf("resolve catalog: %v", err)
	}
	if names := catalog.Names(); len(names) != 1 || names[0] != "coverage" {
		t.Fatalf("catalog names are %v", names)
	}
	root := &cobra.Command{Use: "goconduct"}
	if err := candidate.RegisterCommands(injector, root); err != nil {
		t.Fatalf("register coverage command: %v", err)
	}
	command, _, err := root.Find([]string{"coverage"})
	if err != nil || command.Name() != "coverage" {
		t.Fatalf("find coverage command: command=%v, error=%v", command, err)
	}
	if err := candidate.RegisterEndpoints(injector, nil); err != nil {
		t.Errorf("register coverage endpoints: %v", err)
	}
}

func TestPluginReportsAnIncompleteContainer(t *testing.T) {
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
			injector := newInjector(t, testCase.services...)
			if err := testCase.register(t, injector); err == nil {
				t.Fatal("the plugin accepted an incomplete container")
			}
		})
	}
}

func TestEvaluatorInjectorSelectsTheProvidedConfiguration(t *testing.T) {
	testCases := []struct {
		name        string
		services    []func(do.Injector)
		wantCommand string
	}{
		{
			name:        "no configuration in the container",
			wantCommand: "go",
		},
		{
			name: "one configuration in the container",
			services: []func(do.Injector){func(injector do.Injector) {
				do.ProvideValue(injector, Configuration{
					Command: "custom-go", Packages: []string{"./internal/..."},
				})
			}},
			wantCommand: "custom-go",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			injector := newKernelInjector(t, append([]func(do.Injector){Module}, testCase.services...)...)
			evaluator, err := do.Invoke[*Evaluator](injector)
			if err != nil {
				t.Fatalf("resolve evaluator: %v", err)
			}
			if evaluator.configuration.Command != testCase.wantCommand {
				t.Errorf(
					"the coverage command is %q, want %q",
					evaluator.configuration.Command,
					testCase.wantCommand,
				)
			}
		})
	}
}

// runCoverageCommand executes the coverage command and captures its report.
func runCoverageCommand(
	t *testing.T,
	runner plugin.CommandRunner,
	destination io.Writer,
	arguments ...string,
) error {
	t.Helper()
	command := newCoverageCommand(runner)
	command.SetArgs(arguments)
	command.SetOut(destination)
	command.SetErr(io.Discard)
	command.SilenceUsage = true
	command.SilenceErrors = true
	return command.ExecuteContext(t.Context())
}

// newCommandRunner writes one profile with a covered file, a half covered file,
// and a file with no covered statement.
func newCommandRunner() *coverageRunner {
	return &coverageRunner{profile: profileOf(
		fixtureModulePath+"/cmd/tool/main.go:1.1,2.1 5 1",
		fixtureModulePath+"/internal/domain/order.go:1.1,2.1 10 1",
		fixtureModulePath+"/internal/domain/order.go:3.1,4.1 10 0",
		fixtureModulePath+"/internal/legacy/legacy.go:1.1,2.1 4 0",
	)}
}

// decodeReport reads the JSON report the coverage command wrote.
func decodeReport(t *testing.T, payload []byte) plugin.Report {
	t.Helper()
	var report plugin.Report
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("decode coverage report: %v", err)
	}
	if report.Plugin != "coverage" || report.SchemaVersion != plugin.ReportSchemaVersion {
		t.Fatalf("unexpected report header: %+v", report)
	}
	return report
}

func TestCoverageCommandWritesOneDeterministicReport(t *testing.T) {
	testCases := []struct {
		name         string
		options      []string
		wantFindings int
		wantPaths    []string
		wantIndented bool
	}{
		{
			name:         "no option",
			wantFindings: 0,
			wantIndented: false,
		},
		{
			name:         "the indent option",
			options:      []string{"--indent"},
			wantFindings: 0,
			wantIndented: true,
		},
		{
			name:         "a minimum above one file",
			options:      []string{"--minimum", "10"},
			wantFindings: 1,
			wantPaths:    []string{"internal/legacy/legacy.go"},
		},
		{
			name:         "a minimum above two files",
			options:      []string{"--minimum", "60"},
			wantFindings: 2,
			wantPaths:    []string{"internal/domain/order.go", "internal/legacy/legacy.go"},
		},
		{
			name:         "a selected path and a minimum above one file",
			options:      []string{"--minimum", "60", "--path", "internal/domain"},
			wantFindings: 1,
			wantPaths:    []string{"internal/domain/order.go"},
		},
		{
			name:         "the repository root as a selected path",
			options:      []string{"--minimum", "10", "--path", "."},
			wantFindings: 1,
			wantPaths:    []string{"internal/legacy/legacy.go"},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var output bytes.Buffer
			options := append([]string{"--repository", newRepository(t)}, testCase.options...)
			err := runCoverageCommand(t, newCommandRunner(), &output, options...)
			if testCase.wantFindings == 0 && err != nil {
				t.Fatalf("run the coverage command: %v", err)
			}
			if testCase.wantFindings != 0 && !errors.Is(err, failure.ErrBusinessRule) {
				t.Fatalf("the failure is %v, want a business rule failure", err)
			}
			report := decodeReport(t, output.Bytes())
			if len(report.Findings) != testCase.wantFindings {
				t.Fatalf("the report holds %d findings: %+v", len(report.Findings), report.Findings)
			}
			for index, wantPath := range testCase.wantPaths {
				if report.Findings[index].Path != wantPath {
					t.Errorf("finding %d names %q, want %q", index, report.Findings[index].Path, wantPath)
				}
			}
			if indented := strings.Contains(output.String(), "\n  "); indented != testCase.wantIndented {
				t.Errorf("the report is indented %t, want %t", indented, testCase.wantIndented)
			}
		})
	}
}

func TestCoverageCommandWritesTheReportWithoutHTMLEscaping(t *testing.T) {
	runner := &coverageRunner{profile: profileOf(
		fixtureModulePath + "/internal/order/a&b.go:1.1,2.1 1 1",
	)}
	var output bytes.Buffer
	if err := runCoverageCommand(t, runner, &output, "--repository", newRepository(t)); err != nil {
		t.Fatalf("run the coverage command: %v", err)
	}
	if !strings.Contains(output.String(), "internal/order/a&b.go") {
		t.Errorf("the report escapes the file path: %s", output.String())
	}
}

func TestCoverageCommandRejectsAnInvalidRequest(t *testing.T) {
	testCases := []struct {
		name     string
		options  []string
		category error
	}{
		{
			name:     "a selected path outside the repository",
			options:  []string{"--path", "/etc"},
			category: failure.ErrValidation,
		},
		{
			name:     "an empty selected path",
			options:  []string{"--path", ""},
			category: failure.ErrValidation,
		},
		{
			name:     "a minimum above one hundred",
			options:  []string{"--minimum", "150"},
			category: failure.ErrValidation,
		},
		{
			name:     "a repository without a module file",
			options:  []string{"--repository", "."},
			category: failure.ErrValidation,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			options := append([]string{"--repository", newRepository(t)}, testCase.options...)
			err := runCoverageCommand(t, newCommandRunner(), io.Discard, options...)
			if !errors.Is(err, testCase.category) {
				t.Fatalf("the failure is %v, want %v", err, testCase.category)
			}
		})
	}
}

func TestCoverageCommandReportsAWriteFailure(t *testing.T) {
	err := runCoverageCommand(t, newCommandRunner(), failingWriter{}, "--repository", newRepository(t))
	if !errors.Is(err, failure.ErrUnavailable) {
		t.Fatalf("the failure is %v, want an unavailable failure", err)
	}
}

func TestCoveragePatternsMatchTheSelectedPaths(t *testing.T) {
	testCases := []struct {
		name  string
		paths []string
		want  []string
	}{
		{name: "no selected path", want: []string{"**"}},
		{name: "one selected path", paths: []string{"internal/domain"}, want: []string{"internal/domain/**"}},
		{name: "the repository root", paths: []string{"."}, want: []string{"**"}},
		{
			name:  "a selected path with a leading dot and slash",
			paths: []string{"./internal/domain"},
			want:  []string{"internal/domain/**"},
		},
		{
			name:  "two selected paths that name the same directory",
			paths: []string{"plugin/", "./plugin"},
			want:  []string{"plugin/**"},
		},
		{
			name:  "two selected paths in reverse order",
			paths: []string{"plugin", "internal"},
			want:  []string{"internal/**", "plugin/**"},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			patterns, err := coveragePatterns(testCase.paths)
			if err != nil {
				t.Fatalf("build coverage patterns: %v", err)
			}
			if len(patterns) != len(testCase.want) {
				t.Fatalf("the patterns are %v, want %v", patterns, testCase.want)
			}
			for index, want := range testCase.want {
				if patterns[index] != want {
					t.Errorf("pattern %d is %q, want %q", index, patterns[index], want)
				}
			}
		})
	}
}

func TestCoveragePatternsRejectAPathOutsideTheRepository(t *testing.T) {
	patterns, err := coveragePatterns([]string{"internal", "../secret"})
	if !errors.Is(err, failure.ErrValidation) {
		t.Fatalf("the failure is %v, want a validation failure", err)
	}
	if patterns != nil {
		t.Errorf("the patterns are %v, want none", patterns)
	}
}
