package loc

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/internal/kernel"
	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
	"github.com/cgardev/goconduct/pkg/policy"
)

type unavailableWriter struct{}

func (unavailableWriter) Write([]byte) (int, error) {
	return 0, failure.Unavailable("write test output", nil)
}

func newLOCInjector(t *testing.T, registrations ...func(do.Injector)) do.Injector {
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

func executeLOCCommand(t *testing.T, arguments ...string) (string, error) {
	t.Helper()
	return executeConfiguredLOCCommand(t, DefaultConfiguration(), arguments...)
}

func executeConfiguredLOCCommand(
	t *testing.T,
	configuration Configuration,
	arguments ...string,
) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "goconduct", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(newLOCCommand(configuration))
	var output strings.Builder
	root.SetOut(&output)
	root.SetErr(io.Discard)
	root.SetArgs(append([]string{"loc"}, arguments...))
	err := root.ExecuteContext(t.Context())
	return output.String(), err
}

func runLOCCommand(t *testing.T, arguments ...string) (plugin.Report, error) {
	t.Helper()
	output, err := executeLOCCommand(t, arguments...)
	if output == "" {
		return plugin.Report{}, err
	}
	var report plugin.Report
	if decodeErr := json.Unmarshal([]byte(output), &report); decodeErr != nil {
		t.Fatalf("decode LOC report %q: %v", output, decodeErr)
	}
	return report, err
}

func TestPluginRegistersLOCEvaluatorAndCommand(t *testing.T) {
	candidate := Plugin()
	injector := newLOCInjector(t)

	if err := candidate.Activate(t.Context(), injector); err != nil {
		t.Fatalf("activate LOC plugin: %v", err)
	}

	if candidate.Name() != "loc" {
		t.Errorf("the plugin name is %q", candidate.Name())
	}
	catalog, err := do.Invoke[*plugin.Catalog](injector)
	if err != nil {
		t.Fatalf("resolve plugin catalog: %v", err)
	}
	if !slices.Equal(catalog.Names(), []string{"loc"}) {
		t.Errorf("the catalog names are %v", catalog.Names())
	}
	root := &cobra.Command{Use: "goconduct"}
	if err := candidate.RegisterCommands(injector, root); err != nil {
		t.Fatalf("register LOC command: %v", err)
	}
	command, _, err := root.Find([]string{"loc"})
	if err != nil || command.Name() != "loc" {
		t.Fatalf("find LOC command: command=%v, error=%v", command, err)
	}
	var output strings.Builder
	root.SetOut(&output)
	root.SetArgs([]string{"loc", "--repository", locFixtureRoot(t)})
	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("run the registered command: %v", err)
	}
	if output.Len() == 0 {
		t.Error("the registered command produces no report")
	}
}

func TestPluginMountsNoEndpoint(t *testing.T) {
	if err := Plugin().RegisterEndpoints(nil, nil); err != nil {
		t.Errorf("the LOC plugin mounts an endpoint: %v", err)
	}
}

func TestPluginActivationUsesTheRegisteredConfiguration(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.Generated.HeaderPatterns = []string{"["}
	injector := newLOCInjector(t, func(injector do.Injector) {
		do.ProvideValue(injector, configuration)
	})

	err := Plugin().Activate(t.Context(), injector)

	if !errors.Is(err, failure.ErrValidation) {
		t.Errorf("plugin activation reports %v, want a validation failure", err)
	}
}

func TestPluginActivationRequiresTheCatalog(t *testing.T) {
	injector := do.New(Plugin().Services())
	t.Cleanup(func() {
		if report := injector.Shutdown(); report != nil && !report.Succeed {
			t.Errorf("shut down injector: %s", report.Error())
		}
	})

	err := Plugin().Activate(t.Context(), injector)

	if err == nil {
		t.Error("plugin activation accepts a missing catalog")
	}
}

func TestLOCCommandAppliesPathAndGeneratedFlags(t *testing.T) {
	report, err := runLOCCommand(
		t,
		"--repository", locFixtureRoot(t),
		"--path", "internal",
		"--include", "internal/**",
		"--exclude", "internal/skip/**",
		"--generated-path", "**/*.pb.go",
		"--force-handwritten", "manual.pb.go",
		"--indent",
	)
	if err != nil {
		t.Fatalf("run LOC command: %v", err)
	}

	if got := reportMetric(t, report, "loc.repository.files.total", "").Value; got != 1 {
		t.Errorf("the command reports %.2f files, want 1", got)
	}
	if got := reportMetric(t, report, "loc.repository.files.generated", "").Value; got != 1 {
		t.Errorf("the command reports %.2f generated files, want 1", got)
	}
}

func TestLOCCommandRejectsAnInvalidHeaderExpression(t *testing.T) {
	_, err := runLOCCommand(
		t,
		"--repository", locFixtureRoot(t),
		"--generated-header", "[",
	)

	if !errors.Is(err, failure.ErrValidation) {
		t.Errorf("the command reports %v, want a validation failure", err)
	}
}

func TestLOCCommandWritesCompactJSONWithoutHTMLEscapingByDefault(t *testing.T) {
	repositoryRoot := locFixtureRoot(t)
	oldPath := filepath.Join(repositoryRoot, "main.go")
	newPath := filepath.Join(repositoryRoot, "main&more.go")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("rename source fixture: %v", err)
	}

	output, err := executeLOCCommand(t, "--repository", repositoryRoot)
	if err != nil {
		t.Fatalf("run LOC command: %v", err)
	}
	if !strings.Contains(output, "main&more.go") || strings.Contains(output, `\u0026`) {
		t.Errorf("the JSON output escapes the source path: %q", output)
	}
	if strings.Count(output, "\n") != 1 {
		t.Errorf("the default JSON output is not compact: %q", output)
	}
}

func TestLOCCommandReturnsEvaluationAndOutputFailures(t *testing.T) {
	t.Run("an evaluation failure", func(t *testing.T) {
		_, err := executeLOCCommand(
			t,
			"--repository", locFixtureRoot(t),
			"--path", "absent",
		)

		if !errors.Is(err, failure.ErrValidation) {
			t.Errorf("the missing path reports %v, want a validation failure", err)
		}
	})

	t.Run("an output failure", func(t *testing.T) {
		root := &cobra.Command{Use: "goconduct", SilenceErrors: true, SilenceUsage: true}
		root.AddCommand(newLOCCommand(DefaultConfiguration()))
		root.SetOut(unavailableWriter{})
		root.SetErr(io.Discard)
		root.SetArgs([]string{"loc", "--repository", locFixtureRoot(t)})

		err := root.ExecuteContext(t.Context())

		if !errors.Is(err, failure.ErrUnavailable) {
			t.Errorf("the output failure reports %v, want an unavailable failure", err)
		}
	})
}

func TestLOCCommandReturnsFailingConfiguredPolicies(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.Policies = []policy.PathPolicy{{
		ID: "small-main-file", Include: []string{"main.go"},
		Thresholds: []policy.Threshold{{
			Metric: "loc.file.lines.code", Comparison: policy.ComparisonMaximum,
			Value: 0, Severity: plugin.SeverityError,
		}},
	}}

	output, err := executeConfiguredLOCCommand(
		t,
		configuration,
		"--repository", locFixtureRoot(t),
		"--path", "main.go",
	)

	if output == "" {
		t.Error("the failing policy produces no report")
	}
	if !errors.Is(err, failure.ErrBusinessRule) {
		t.Errorf("the failing policy reports %v, want a business rule failure", err)
	}
}

func TestLOCQueryCommandsReturnFocusedJSONWithoutOutputPipes(t *testing.T) {
	repositoryRoot := locFixtureRoot(t)
	configuration := fixtureConfiguration()

	t.Run("the summary returns repository evidence", func(t *testing.T) {
		output, err := executeConfiguredLOCCommand(
			t,
			configuration,
			"summary",
			"--repository",
			repositoryRoot,
		)
		if err != nil {
			t.Fatalf("run LOC summary command: %v", err)
		}
		var result SummaryResult
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("decode LOC summary %q: %v", output, err)
		}
		if result.Summary.Files.Total != 7 || result.Summary.Lines.Total != 25 {
			t.Errorf("the LOC summary is %+v", result.Summary)
		}
	})

	t.Run("the package query sorts and limits before encoding", func(t *testing.T) {
		output, err := executeConfiguredLOCCommand(
			t,
			configuration,
			"packages",
			"--repository",
			repositoryRoot,
			"--sort",
			"total",
			"--limit",
			"1",
		)
		if err != nil {
			t.Fatalf("run LOC package command: %v", err)
		}
		var result PackagesResult
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("decode LOC packages %q: %v", output, err)
		}
		if result.Matched != 4 || result.Returned != 1 || result.Packages[0].Path != "." {
			t.Errorf("the LOC package result is %+v", result)
		}
	})

	t.Run("the file query excludes tests and generated sources by default", func(t *testing.T) {
		output, err := executeConfiguredLOCCommand(
			t,
			configuration,
			"files",
			"--repository",
			repositoryRoot,
			"--limit",
			"1",
		)
		if err != nil {
			t.Fatalf("run LOC file command: %v", err)
		}
		var result FilesResult
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("decode LOC files %q: %v", output, err)
		}
		if result.Matched != 3 || result.Returned != 1 || result.Files[0].Path != "main.go" {
			t.Errorf("the LOC file result is %+v", result)
		}
	})

	t.Run("the function query returns the largest handwritten function", func(t *testing.T) {
		output, err := executeConfiguredLOCCommand(
			t,
			configuration,
			"functions",
			"--repository",
			repositoryRoot,
			"--sort",
			"code",
			"--limit",
			"1",
		)
		if err != nil {
			t.Fatalf("run LOC function command: %v", err)
		}
		var result FunctionsResult
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("decode LOC functions %q: %v", output, err)
		}
		if result.Matched != 3 || result.Returned != 1 || result.Functions[0].Name != "Main" {
			t.Errorf("the LOC function result is %+v", result)
		}
	})
}

func TestLOCQueryCommandsValidateArgumentsBeforeEvaluation(t *testing.T) {
	tests := [][]string{
		{"packages", "--sort", "other"},
		{"files", "--kind", "other"},
		{"functions", "--limit", "-1"},
	}
	for _, arguments := range tests {
		_, err := executeLOCCommand(t, arguments...)
		if !errors.Is(err, failure.ErrValidation) {
			t.Errorf("arguments %v report %v, want a validation failure", arguments, err)
		}
	}
}

func TestLOCQueryCommandReturnsOutputFailures(t *testing.T) {
	root := &cobra.Command{Use: "goconduct", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(newLOCCommand(DefaultConfiguration()))
	root.SetOut(unavailableWriter{})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"loc", "summary", "--repository", locFixtureRoot(t)})

	err := root.ExecuteContext(t.Context())

	if !errors.Is(err, failure.ErrUnavailable) {
		t.Errorf("the query output failure reports %v, want an unavailable failure", err)
	}
}
