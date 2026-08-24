package crap

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/internal/kernel"
	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
)

// errReportSink stops the report encoder, so one test reads the write failure.
var errReportSink = errors.New("the report sink is closed")

type failingWriter struct{}

var _ io.Writer = failingWriter{}

func (failingWriter) Write([]byte) (int, error) { return 0, errReportSink }

func newTestInjector(t *testing.T, packages ...func(do.Injector)) do.Injector {
	t.Helper()
	injector := do.New(packages...)
	t.Cleanup(func() {
		if report := injector.Shutdown(); report != nil && !report.Succeed {
			t.Errorf("shut down injector: %s", report.Error())
		}
	})
	return injector
}

// runCRAPCommand executes the command with a deterministic runner.
func runCRAPCommand(
	t *testing.T,
	runner plugin.CommandRunner,
	output io.Writer,
	arguments ...string,
) error {
	t.Helper()
	command := newCRAPCommand(runner)
	command.SilenceUsage = true
	command.SilenceErrors = true
	command.SetOut(output)
	command.SetErr(io.Discard)
	command.SetArgs(arguments)
	return command.ExecuteContext(t.Context())
}

func TestPluginRegistersCRAPEvaluatorAndCommand(t *testing.T) {
	candidate := Plugin()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	injector := newTestInjector(
		t,
		kernel.Module,
		func(injector do.Injector) { do.OverrideValue(injector, logger) },
		candidate.Services(),
	)
	if err := candidate.Activate(t.Context(), injector); err != nil {
		t.Fatalf("activate CRAP plugin: %v", err)
	}
	catalog, err := do.Invoke[*plugin.Catalog](injector)
	if err != nil {
		t.Fatalf("resolve catalog: %v", err)
	}
	if names := catalog.Names(); len(names) != 1 || names[0] != "crap" {
		t.Fatalf("catalog names are %v", names)
	}
	evaluator, err := do.Invoke[*Evaluator](injector)
	if err != nil {
		t.Fatalf("resolve evaluator: %v", err)
	}
	if evaluator == nil {
		t.Fatal("the injector resolves a nil evaluator")
	}
	if evaluator.configuration.Command != "go" {
		t.Errorf("the evaluator command is %q, want go", evaluator.configuration.Command)
	}
	root := &cobra.Command{Use: "goconduct"}
	if err := candidate.RegisterCommands(injector, root); err != nil {
		t.Fatalf("register CRAP command: %v", err)
	}
	command, _, err := root.Find([]string{"crap"})
	if err != nil || command.Name() != "crap" {
		t.Fatalf("find CRAP command: command=%v, error=%v", command, err)
	}
}

func TestPluginReportsItsStableName(t *testing.T) {
	if name := Plugin().Name(); name != "crap" {
		t.Errorf("the plugin name is %q, want crap", name)
	}
}

func TestPluginMountsNoEndpoint(t *testing.T) {
	if err := Plugin().RegisterEndpoints(nil, nil); err != nil {
		t.Errorf("the plugin mounts an endpoint: %v", err)
	}
}

func TestModuleUsesTheProvidedConfiguration(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.MaximumScore = 42
	injector := newTestInjector(
		t,
		func(injector do.Injector) {
			do.ProvideValue[plugin.CommandRunner](injector, plugin.NewCommandRunner())
			do.ProvideValue(injector, configuration)
		},
		Module,
	)

	evaluator, err := do.Invoke[*Evaluator](injector)

	if err != nil {
		t.Fatalf("resolve evaluator: %v", err)
	}
	if evaluator.configuration.MaximumScore != 42 {
		t.Errorf("the evaluator limit is %v, want 42", evaluator.configuration.MaximumScore)
	}
}

func TestPluginRejectsAnIncompleteInjector(t *testing.T) {
	testCases := []struct {
		name     string
		services []func(do.Injector)
		act      func(t *testing.T, injector do.Injector) error
	}{
		{
			name:     "activation without a catalog",
			services: []func(do.Injector){Plugin().Services()},
			act: func(t *testing.T, injector do.Injector) error {
				t.Helper()
				return Plugin().Activate(t.Context(), injector)
			},
		},
		{
			name: "activation without a command runner",
			services: []func(do.Injector){
				func(injector do.Injector) { do.ProvideValue(injector, plugin.NewCatalog()) },
				Plugin().Services(),
			},
			act: func(t *testing.T, injector do.Injector) error {
				t.Helper()
				return Plugin().Activate(t.Context(), injector)
			},
		},
		{
			name:     "command registration without a command runner",
			services: []func(do.Injector){Plugin().Services()},
			act: func(t *testing.T, injector do.Injector) error {
				t.Helper()
				return Plugin().RegisterCommands(injector, &cobra.Command{Use: "goconduct"})
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			injector := newTestInjector(t, testCase.services...)

			if err := testCase.act(t, injector); err == nil {
				t.Error("an incomplete injector reports no failure")
			}
		})
	}
}

func TestCRAPCommandWritesOneCompactReport(t *testing.T) {
	root := newModuleRoot(t)
	writeFixture(t, root, "math&stats/scored.go", scoredSource)
	runner := &profileRunner{profile: scoredProfile("math&stats/scored.go", 4)}
	var output bytes.Buffer

	err := runCRAPCommand(t, runner, &output, "--repository", root, "--maximum", "1000")

	if err != nil {
		t.Fatalf("run the CRAP command: %v", err)
	}
	written := strings.TrimSuffix(output.String(), "\n")
	if written == "" {
		t.Fatal("the CRAP command writes no report")
	}
	if strings.Contains(written, "\n") {
		t.Errorf("the report is indented without the option: %q", written)
	}
	if !strings.Contains(written, `"math&stats/scored.go"`) {
		t.Errorf("the report escapes the path of a metric: %q", written)
	}
}

func TestCRAPCommandIndentsTheReportOnRequest(t *testing.T) {
	root := newModuleRoot(t)
	writeFixture(t, root, "scored.go", scoredSource)
	runner := &profileRunner{profile: scoredProfile("scored.go", 4)}
	var output bytes.Buffer

	err := runCRAPCommand(t, runner, &output, "--repository", root, "--maximum", "1000", "--indent")

	if err != nil {
		t.Fatalf("run the CRAP command: %v", err)
	}
	if !strings.Contains(output.String(), "\n  \"schemaVersion\": 1") {
		t.Errorf("the report is not indented: %q", output.String())
	}
}

func TestCRAPCommandFailsOnAPolicyFinding(t *testing.T) {
	root := newModuleRoot(t)
	writeFixture(t, root, "scored.go", scoredSource)
	runner := &profileRunner{profile: scoredProfile("scored.go", 4)}
	var output bytes.Buffer

	err := runCRAPCommand(t, runner, &output, "--repository", root, "--maximum", "0")

	if !errors.Is(err, failure.ErrBusinessRule) {
		t.Fatalf("a report with findings reports %v, want a business rule failure", err)
	}
	if !strings.Contains(err.Error(), "2 policy findings") {
		t.Errorf("the failure message is %q, want the finding count", err.Error())
	}
	if output.Len() == 0 {
		t.Error("a failing analysis writes no report")
	}
}

func TestCRAPCommandReportsAFailedEvaluation(t *testing.T) {
	root := newModuleRoot(t)
	writeFixture(t, root, "scored.go", scoredSource)
	runner := &profileRunner{failure: failure.Unavailable(`run command "go"`, nil)}
	var output bytes.Buffer

	err := runCRAPCommand(t, runner, &output, "--repository", root)

	if !errors.Is(err, failure.ErrUnavailable) {
		t.Fatalf("a failed coverage run reports %v, want an unavailable failure", err)
	}
	if output.Len() != 0 {
		t.Errorf("a failed evaluation writes the report %q", output.String())
	}
}

func TestCRAPCommandRejectsANegativeMaximum(t *testing.T) {
	err := runCRAPCommand(t, &profileRunner{}, io.Discard, "--maximum=-1")

	if !errors.Is(err, failure.ErrValidation) {
		t.Fatalf("a negative limit reports %v, want a validation failure", err)
	}
}

func TestCRAPCommandReportsAFailedWrite(t *testing.T) {
	root := newModuleRoot(t)
	writeFixture(t, root, "scored.go", scoredSource)
	runner := &profileRunner{profile: scoredProfile("scored.go", 4)}

	err := runCRAPCommand(t, runner, failingWriter{}, "--repository", root, "--maximum", "1000")

	if !errors.Is(err, failure.ErrUnavailable) {
		t.Fatalf("a closed sink reports %v, want an unavailable failure", err)
	}
	if !errors.Is(err, errReportSink) {
		t.Errorf("the failure hides its cause: %v", err)
	}
}
