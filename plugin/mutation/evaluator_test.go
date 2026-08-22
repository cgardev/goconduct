package mutation

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cgardev/goconduct/plugin"
)

type mutationRunner struct {
	commands []plugin.Command
	output   []byte
}

func (runner *mutationRunner) Run(_ context.Context, command plugin.Command) (plugin.CommandResult, error) {
	runner.commands = append(runner.commands, command)
	return plugin.CommandResult{StandardOutput: runner.output}, nil
}

func TestEvaluatorReportsSurvivingAndUncoveredMutations(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "order.go"), []byte("package fixture\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	runner := &mutationRunner{output: []byte(
		"Mutation run: order.go\nTotal mutation sites: 4\nCovered mutation sites: 3\n" +
			"Uncovered mutation sites: 1\nSelected mutation sites: 3\n" +
			"Mutation Report\nKilled: 2\nSurvived: 1\nUncovered: 1\n",
	)}
	configuration := DefaultConfiguration()
	configuration.Execute = true
	evaluator, err := NewEvaluator(runner, configuration)
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{
		RepositoryRoot: root, Paths: []string{"order.go"},
	})
	if err != nil {
		t.Fatalf("evaluate mutation: %v", err)
	}
	if len(report.Findings) != 2 || len(report.Metrics) != 6 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !strings.Contains(report.Findings[0].ID+report.Findings[1].ID, "survived") {
		t.Fatalf("missing survivor finding: %+v", report.Findings)
	}
}

func TestEvaluatorIntegrationScansMutate4GoSites(t *testing.T) {
	if _, err := exec.LookPath("mutate4go"); err != nil {
		t.Skip("mutate4go is not installed")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/mutationfixture\n\ngo 1.26.3\n"), 0o600); err != nil {
		t.Fatalf("write module: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "value.go"), []byte("package fixture\n\nfunc Value(input int) int { if input > 0 { return input + 1 }; return input - 1 }\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	evaluator, err := NewEvaluator(plugin.NewCommandRunner(), DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{
		RepositoryRoot: root, Paths: []string{"value.go"},
	})
	if err != nil {
		t.Fatalf("scan mutation fixture: %v", err)
	}
	if report.Plugin != "mutation" || len(report.Metrics) == 0 {
		t.Fatalf("unexpected integration report: %+v", report)
	}
}
