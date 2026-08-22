package coverage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cgardev/goconduct/plugin"
	"github.com/cgardev/goconduct/policy"
)

type coverageRunner struct {
	profile string
	command plugin.Command
}

func (runner *coverageRunner) Run(_ context.Context, command plugin.Command) (plugin.CommandResult, error) {
	runner.command = command
	for _, argument := range command.Arguments {
		if strings.HasPrefix(argument, "-coverprofile=") {
			profilePath := strings.TrimPrefix(argument, "-coverprofile=")
			if err := os.WriteFile(profilePath, []byte(runner.profile), 0o600); err != nil {
				return plugin.CommandResult{}, err
			}
		}
	}
	return plugin.CommandResult{}, nil
}

func TestEvaluatorAppliesCoverageByPath(t *testing.T) {
	repositoryRoot := t.TempDir()
	writeCoverageFixture(t, repositoryRoot, "internal/domain/order.go")
	writeCoverageFixture(t, repositoryRoot, "cmd/tool/main.go")
	runner := &coverageRunner{profile: strings.Join([]string{
		"mode: atomic",
		"example.com/project/internal/domain/order.go:1.1,2.1 10 1",
		"example.com/project/internal/domain/order.go:3.1,4.1 10 0",
		"example.com/project/cmd/tool/main.go:1.1,2.1 5 1",
	}, "\n") + "\n"}
	evaluator, err := NewEvaluator(runner, Configuration{
		Command: "go", Packages: []string{"./..."},
		Policies: []policy.PathPolicy{{
			ID: "domain", Include: []string{"internal/domain/**"},
			Thresholds: []policy.Threshold{{
				Metric: metricCoveragePercent, Comparison: policy.ComparisonMinimum,
				Value: 100, Severity: plugin.SeverityError,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: repositoryRoot})
	if err != nil {
		t.Fatalf("evaluate coverage: %v", err)
	}
	if len(report.Metrics) != 3 || len(report.Findings) != 1 {
		t.Fatalf("unexpected coverage report: %+v", report)
	}
	finding := report.Findings[0]
	if finding.Path != "internal/domain/order.go" || finding.Actual == nil || *finding.Actual != 50 {
		t.Fatalf("unexpected coverage finding: %+v", finding)
	}
	if runner.command.Directory != repositoryRoot {
		t.Fatalf("command directory is %q", runner.command.Directory)
	}
}

func TestEvaluatorIntegrationRunsGoCoverage(t *testing.T) {
	repositoryRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repositoryRoot, "go.mod"), []byte("module example.com/coveragefixture\n\ngo 1.26.3\n"), 0o600); err != nil {
		t.Fatalf("write module: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repositoryRoot, "math.go"), []byte("package fixture\n\nfunc Double(value int) int { return value * 2 }\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repositoryRoot, "math_test.go"), []byte("package fixture\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) { if Double(2) != 4 { t.Fatal(\"unexpected\") } }\n"), 0o600); err != nil {
		t.Fatalf("write test: %v", err)
	}
	evaluator, err := NewEvaluator(plugin.NewCommandRunner(), DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: repositoryRoot})
	if err != nil {
		t.Fatalf("evaluate coverage: %v", err)
	}
	if len(report.Metrics) < 2 || report.Metrics[0].Value != 100 {
		t.Fatalf("unexpected integration report: %+v", report)
	}
}

func writeCoverageFixture(t *testing.T, root, name string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte("package fixture\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}
