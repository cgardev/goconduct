package duplication

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cgardev/goconduct/plugin"
)

type duplicationRunner struct {
	output []byte
}

func (runner duplicationRunner) Run(context.Context, plugin.Command) (plugin.CommandResult, error) {
	return plugin.CommandResult{StandardOutput: runner.output}, nil
}

func TestEvaluatorNormalizesDuplicateCandidates(t *testing.T) {
	evaluator, err := NewEvaluator(duplicationRunner{output: []byte(`{
  "candidates": [{
    "score": 0.91,
    "left": {"file": "internal/a.go", "start_line": 10, "end_line": 20},
    "right": {"file": "internal/b.go", "start_line": 30, "end_line": 40},
    "left_nodes": 30,
    "right_nodes": 31
  }]
}`)}, DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{})
	if err != nil {
		t.Fatalf("evaluate duplication: %v", err)
	}
	if len(report.Metrics) != 2 || len(report.Findings) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Findings[0].Path != "internal/a.go" {
		t.Fatalf("unexpected finding: %+v", report.Findings[0])
	}
}

func TestEvaluatorIntegrationRunsDry4Go(t *testing.T) {
	if _, err := exec.LookPath("dry4go"); err != nil {
		t.Skip("dry4go is not installed")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/dryfixture\n\ngo 1.26.3\n"), 0o600); err != nil {
		t.Fatalf("write module: %v", err)
	}
	source := []byte("package fixture\n\nfunc Left(value int) int {\n if value > 0 { return value + 1 }; return value - 1\n}\n\nfunc Right(value int) int {\n if value > 0 { return value + 1 }; return value - 1\n}\n")
	if err := os.WriteFile(filepath.Join(root, "duplicate.go"), source, 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	evaluator, err := NewEvaluator(plugin.NewCommandRunner(), DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root})
	if err != nil {
		t.Fatalf("evaluate duplication fixture: %v", err)
	}
	if report.Plugin != "duplication" || len(report.Metrics) == 0 {
		t.Fatalf("unexpected integration report: %+v", report)
	}
}
