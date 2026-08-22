package duplication

import (
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

// refusingWriter fails every write, so one test reaches the report writer.
type refusingWriter struct{}

var errRefusedWrite = errors.New("the writer refuses every report")

func (refusingWriter) Write([]byte) (int, error) { return 0, errRefusedWrite }

// newDuplicationInjector builds the dependency graph of the duplication plugin.
func newDuplicationInjector(t *testing.T, registrations ...func(do.Injector)) do.Injector {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	packages := append([]func(do.Injector){
		kernel.Module,
		func(injector do.Injector) { do.OverrideValue(injector, logger) },
		Plugin().Services(),
	}, registrations...)
	injector := do.New(packages...)
	t.Cleanup(func() {
		if report := injector.Shutdown(); report != nil && !report.Succeed {
			t.Errorf("shut down injector: %s", report.Error())
		}
	})
	return injector
}

// runDuplicationCommand executes the plugin command and captures the report.
func runDuplicationCommand(t *testing.T, output io.Writer, arguments ...string) error {
	t.Helper()
	root := &cobra.Command{Use: "goconduct", SilenceUsage: true, SilenceErrors: true}
	if err := Plugin().RegisterCommands(nil, root); err != nil {
		t.Fatalf("register duplication command: %v", err)
	}
	root.SetOut(output)
	root.SetErr(io.Discard)
	root.SetArgs(append([]string{"duplication"}, arguments...))
	return root.ExecuteContext(t.Context())
}

func decodeReport(t *testing.T, output string) plugin.Report {
	t.Helper()
	var report plugin.Report
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("decode report %q: %v", output, err)
	}
	return report
}

func TestPluginRegistersDuplicationEvaluatorAndCommand(t *testing.T) {
	candidate := Plugin()
	injector := newDuplicationInjector(t)

	if err := candidate.Activate(t.Context(), injector); err != nil {
		t.Fatalf("activate duplication plugin: %v", err)
	}

	if name := candidate.Name(); name != "duplication" {
		t.Errorf("the plugin name is %q, want %q", name, "duplication")
	}
	catalog, err := do.Invoke[*plugin.Catalog](injector)
	if err != nil {
		t.Fatalf("resolve catalog: %v", err)
	}
	if names := catalog.Names(); len(names) != 1 || names[0] != "duplication" {
		t.Fatalf("catalog names are %v", names)
	}
	root := &cobra.Command{Use: "goconduct"}
	if err := candidate.RegisterCommands(injector, root); err != nil {
		t.Fatalf("register duplication command: %v", err)
	}
	command, _, err := root.Find([]string{"duplication"})
	if err != nil || command.Name() != "duplication" {
		t.Fatalf("find duplication command: command=%v, error=%v", command, err)
	}
}

func TestPluginMountsNoEndpoint(t *testing.T) {
	if err := Plugin().RegisterEndpoints(nil, nil); err != nil {
		t.Errorf("the duplication plugin mounts an endpoint: %v", err)
	}
}

func TestPluginActivationNeedsTheSharedCatalog(t *testing.T) {
	candidate := Plugin()
	injector := do.New(candidate.Services())
	t.Cleanup(func() {
		if report := injector.Shutdown(); report != nil && !report.Succeed {
			t.Errorf("shut down injector: %s", report.Error())
		}
	})

	err := candidate.Activate(t.Context(), injector)

	if err == nil {
		t.Error("the plugin activates without the shared catalog")
	}
}

func TestPluginActivationUsesTheRegisteredConfiguration(t *testing.T) {
	candidate := Plugin()
	rejected := Configuration{Similarity: 2, MinimumLines: 4, MinimumNodes: 20}
	injector := newDuplicationInjector(t, func(injector do.Injector) {
		do.ProvideValue(injector, rejected)
	})

	err := candidate.Activate(t.Context(), injector)

	if !errors.Is(err, failure.ErrValidation) {
		t.Errorf("the plugin reports %v, want the registered configuration to fail", err)
	}
}

func TestDuplicationCommandReportsEveryCandidateWithTheDefaultBudget(t *testing.T) {
	var output strings.Builder

	err := runDuplicationCommand(t, &output, "--repository", newDuplicationFixture(t))

	if !errors.Is(err, failure.ErrBusinessRule) {
		t.Fatalf("the command reports %v, want a business rule failure", err)
	}
	written := output.String()
	report := decodeReport(t, written)
	candidates := metricValue(t, report, "duplication:candidates")
	if candidates != 1 {
		t.Fatalf("the fixture reports %v candidates, want 1", candidates)
	}
	if float64(len(report.Findings)) != candidates {
		t.Errorf("the default budget keeps %d of %v candidates", len(report.Findings), candidates)
	}
	if lines := strings.Count(strings.TrimSpace(written), "\n"); lines != 0 {
		t.Errorf("the report holds %d line breaks, want one line", lines)
	}
	if !strings.Contains(written, fixtureDirectory) {
		t.Errorf("the report escapes the path %q: %s", fixtureDirectory, written)
	}
}

func TestDuplicationCommandForgivesEveryCandidateInsideTheBudget(t *testing.T) {
	var output strings.Builder

	err := runDuplicationCommand(
		t, &output, "--repository", newDuplicationFixture(t), "--maximum", "5",
	)

	if err != nil {
		t.Fatalf("run the duplication command: %v", err)
	}
	report := decodeReport(t, output.String())
	if len(report.Findings) != 0 {
		t.Errorf("the budget of five keeps %d findings", len(report.Findings))
	}
	if candidates := metricValue(t, report, "duplication:candidates"); candidates != 1 {
		t.Errorf("the report counts %v candidates, want 1", candidates)
	}
}

func TestDuplicationCommandIndentsTheReportOnRequest(t *testing.T) {
	var output strings.Builder

	err := runDuplicationCommand(
		t, &output, "--repository", newDuplicationFixture(t), "--maximum", "5", "--indent",
	)

	if err != nil {
		t.Fatalf("run the duplication command: %v", err)
	}
	if lines := strings.Count(strings.TrimSpace(output.String()), "\n"); lines == 0 {
		t.Errorf("the indented report holds one line: %s", output.String())
	}
}

func TestDuplicationCommandAppliesTheSimilarityFlag(t *testing.T) {
	var output strings.Builder

	err := runDuplicationCommand(
		t, &output,
		"--repository", newDuplicationFixture(t),
		"--maximum", "5",
		"--similarity", "0.5",
	)

	if err != nil {
		t.Fatalf("run the duplication command: %v", err)
	}
	report := decodeReport(t, output.String())
	if candidates := metricValue(t, report, "duplication:candidates"); candidates != 2 {
		t.Errorf("the lower threshold reports %v candidates, want 2", candidates)
	}
}

func TestDuplicationCommandRejectsAnUnusableRequest(t *testing.T) {
	testCases := []struct {
		name      string
		arguments []string
	}{
		{name: "a similarity outside its range", arguments: []string{"--similarity", "2"}},
		{name: "a negative candidate budget", arguments: []string{"--maximum", "-1"}},
		{name: "a path that does not exist", arguments: []string{"--path", "does-not-exist"}},
		{name: "a path outside the repository", arguments: []string{"--path", ".."}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var output strings.Builder
			arguments := append([]string{"--repository", newDuplicationFixture(t)}, testCase.arguments...)

			err := runDuplicationCommand(t, &output, arguments...)

			if !errors.Is(err, failure.ErrValidation) {
				t.Errorf("the command reports %v, want a validation failure", err)
			}
			if output.Len() != 0 {
				t.Errorf("the rejected request writes the report %q", output.String())
			}
		})
	}
}

func TestDuplicationCommandClassifiesARefusedWrite(t *testing.T) {
	err := runDuplicationCommand(
		t, refusingWriter{}, "--repository", newDuplicationFixture(t), "--maximum", "5",
	)

	if !errors.Is(err, failure.ErrUnavailable) {
		t.Fatalf("the command reports %v, want an unavailable dependency", err)
	}
	if !errors.Is(err, errRefusedWrite) {
		t.Errorf("the command drops the cause of the refused write: %v", err)
	}
}
