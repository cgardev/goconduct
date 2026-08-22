package crap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cgardev/goconduct/plugin"
)

type crapRunner struct {
	output []byte
}

func (runner crapRunner) Run(context.Context, plugin.Command) (plugin.CommandResult, error) {
	return plugin.CommandResult{StandardOutput: runner.output}, nil
}

func TestEvaluatorEnforcesMaximumScore(t *testing.T) {
	evaluator, err := NewEvaluator(crapRunner{output: []byte(
		"CreateOrder orders 12 70.5% 19.2\nGetOrder orders 1 100.0% 1.0\n",
	)}, DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: "."})
	if err != nil {
		t.Fatalf("evaluate CRAP: %v", err)
	}
	if len(report.Metrics) != 6 || len(report.Findings) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Findings[0].Path != "orders.CreateOrder" {
		t.Fatalf("unexpected finding: %+v", report.Findings[0])
	}
}

func TestEvaluatorIntegrationRunsCrap4Go(t *testing.T) {
	if _, err := exec.LookPath("crap4go"); err != nil {
		t.Skip("crap4go is not installed")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/crapfixture\n\ngo 1.26.3\n"), 0o600); err != nil {
		t.Fatalf("write module: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "value.go"), []byte("package fixture\n\nfunc Value() int { return 1 }\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "value_test.go"), []byte("package fixture\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"unexpected\") } }\n"), 0o600); err != nil {
		t.Fatalf("write test: %v", err)
	}
	evaluator, err := NewEvaluator(plugin.NewCommandRunner(), DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root})
	if err != nil {
		t.Fatalf("evaluate CRAP fixture: %v", err)
	}
	if report.Plugin != "crap" || len(report.Metrics) == 0 {
		t.Fatalf("unexpected integration report: %+v", report)
	}
}
