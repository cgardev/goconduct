package coverage

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/cover"

	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
	"github.com/cgardev/goconduct/pkg/policy"
)

const fixtureModulePath = "example.com/project"

// coverageRunner writes one prepared profile instead of running the Go tool.
type coverageRunner struct {
	profile       string
	removeProfile bool
	lockDirectory bool
	result        plugin.CommandResult
	runError      error
	command       plugin.Command
	calls         int
}

func (runner *coverageRunner) Run(_ context.Context, command plugin.Command) (plugin.CommandResult, error) {
	runner.calls++
	runner.command = command
	profilePath := profileArgument(command.Arguments)
	if runner.removeProfile {
		if err := os.Remove(profilePath); err != nil {
			return plugin.CommandResult{}, err
		}
		return runner.result, runner.runError
	}
	if err := os.WriteFile(profilePath, []byte(runner.profile), 0o600); err != nil {
		return plugin.CommandResult{}, err
	}
	if runner.lockDirectory {
		if err := os.Chmod(filepath.Dir(profilePath), 0o500); err != nil {
			return plugin.CommandResult{}, err
		}
	}
	return runner.result, runner.runError
}

func profileArgument(arguments []string) string {
	for _, argument := range arguments {
		if profilePath, found := strings.CutPrefix(argument, "-coverprofile="); found {
			return profilePath
		}
	}
	return ""
}

// newRepository creates one repository root that declares a Go module.
func newRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module "+fixtureModulePath+"\n\ngo 1.26.3\n")
	return root
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture %q: %v", name, err)
	}
}

// profileOf builds one coverage profile from the given block lines.
func profileOf(lines ...string) string {
	if len(lines) == 0 {
		return "mode: atomic\n"
	}
	return "mode: atomic\n" + strings.Join(lines, "\n") + "\n"
}

// evaluateProfile runs one evaluation over a prepared profile.
func evaluateProfile(t *testing.T, configuration Configuration, profile string) (plugin.Report, error) {
	t.Helper()
	runner := &coverageRunner{profile: profile}
	evaluator, err := NewEvaluator(runner, configuration)
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	return evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: newRepository(t)})
}

func metricValue(t *testing.T, report plugin.Report, path string) float64 {
	t.Helper()
	for _, metric := range report.Metrics {
		if metric.Path == path {
			return metric.Value
		}
	}
	t.Fatalf("report holds no metric for path %q: %+v", path, report.Metrics)
	return 0
}

func hasMetric(report plugin.Report, path string) bool {
	for _, metric := range report.Metrics {
		if metric.Path == path {
			return true
		}
	}
	return false
}

// nearly compares two percentages with the tolerance of a binary fraction.
func nearly(actual, want float64) bool {
	return math.Abs(actual-want) < 0.000001
}

func TestNewEvaluatorRejectsAnInvalidConfiguration(t *testing.T) {
	validPolicy := []policy.Threshold{{
		Metric: metricCoveragePercent, Comparison: policy.ComparisonMinimum,
		Value: 80, Severity: plugin.SeverityError,
	}}
	testCases := []struct {
		name          string
		runner        plugin.CommandRunner
		configuration Configuration
	}{
		{
			name:          "a nil command runner",
			runner:        nil,
			configuration: DefaultConfiguration(),
		},
		{
			name:          "an empty command",
			runner:        &coverageRunner{},
			configuration: Configuration{Command: "", Packages: []string{"./..."}},
		},
		{
			name:          "a command of blank characters",
			runner:        &coverageRunner{},
			configuration: Configuration{Command: "   ", Packages: []string{"./..."}},
		},
		{
			name:          "an empty package list",
			runner:        &coverageRunner{},
			configuration: Configuration{Command: "go"},
		},
		{
			name:   "a path policy without an identifier",
			runner: &coverageRunner{},
			configuration: Configuration{
				Command: "go", Packages: []string{"./..."},
				Policies: []policy.PathPolicy{{
					Include: []string{"**"}, Thresholds: validPolicy,
				}},
			},
		},
		{
			name:   "a coverage limit above one hundred",
			runner: &coverageRunner{},
			configuration: Configuration{
				Command: "go", Packages: []string{"./..."},
				Policies: []policy.PathPolicy{{
					ID: "wide", Include: []string{"**"},
					Thresholds: []policy.Threshold{{
						Metric: metricCoveragePercent, Comparison: policy.ComparisonMinimum,
						Value: 150, Severity: plugin.SeverityError,
					}},
				}},
			},
		},
		{
			name:   "a coverage limit below zero",
			runner: &coverageRunner{},
			configuration: Configuration{
				Command: "go", Packages: []string{"./..."},
				Policies: []policy.PathPolicy{{
					ID: "wide", Include: []string{"**"},
					Thresholds: []policy.Threshold{{
						Metric: metricCoveragePercent, Comparison: policy.ComparisonMinimum,
						Value: -1, Severity: plugin.SeverityError,
					}},
				}},
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			evaluator, err := NewEvaluator(testCase.runner, testCase.configuration)
			if evaluator != nil {
				t.Errorf("the evaluator is %+v, want no evaluator", evaluator)
			}
			if !errors.Is(err, failure.ErrValidation) {
				t.Fatalf("the failure is %v, want a validation failure", err)
			}
		})
	}
}

func TestNewEvaluatorAcceptsAValidConfiguration(t *testing.T) {
	testCases := []struct {
		name          string
		configuration Configuration
	}{
		{
			name:          "one package and no path policy",
			configuration: DefaultConfiguration(),
		},
		{
			name: "a coverage limit of exactly zero",
			configuration: Configuration{
				Command: "go", Packages: []string{"./..."},
				Policies: []policy.PathPolicy{{
					ID: "wide", Include: []string{"**"},
					Thresholds: []policy.Threshold{{
						Metric: metricCoveragePercent, Comparison: policy.ComparisonMinimum,
						Value: 0, Severity: plugin.SeverityError,
					}},
				}},
			},
		},
		{
			name: "a coverage limit of exactly one hundred",
			configuration: Configuration{
				Command: "go", Packages: []string{"./..."},
				Policies: []policy.PathPolicy{{
					ID: "wide", Include: []string{"**"},
					Thresholds: []policy.Threshold{{
						Metric: metricCoveragePercent, Comparison: policy.ComparisonMinimum,
						Value: 100, Severity: plugin.SeverityError,
					}},
				}},
			},
		},
		{
			name: "another metric with a limit outside the coverage range",
			configuration: Configuration{
				Command: "go", Packages: []string{"./..."},
				Policies: []policy.PathPolicy{{
					ID: "wide", Include: []string{"**"},
					Thresholds: []policy.Threshold{{
						Metric: "duplication.percent", Comparison: policy.ComparisonMaximum,
						Value: 150, Severity: plugin.SeverityWarning,
					}},
				}},
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			evaluator, err := NewEvaluator(&coverageRunner{}, testCase.configuration)
			if err != nil {
				t.Fatalf("create evaluator: %v", err)
			}
			if evaluator.Name() != "coverage" {
				t.Errorf("the evaluator name is %q, want %q", evaluator.Name(), "coverage")
			}
		})
	}
}

// TestEvaluatorWeightsEveryStatementOfOneFile proves that a file percentage
// divides the covered statements by the total statements. It is not the mean of
// the per-block percentages and it is not the ratio of the covered blocks.
func TestEvaluatorWeightsEveryStatementOfOneFile(t *testing.T) {
	testCases := []struct {
		name    string
		blocks  []string
		want    float64
		overall float64
	}{
		{
			name:    "a fully covered file",
			blocks:  []string{"1.1,3.2 4 1"},
			want:    100,
			overall: 100,
		},
		{
			name:    "a file with no covered statement",
			blocks:  []string{"1.1,3.2 4 0"},
			want:    0,
			overall: 0,
		},
		{
			name:    "a file with no statement",
			blocks:  []string{"1.1,1.2 0 0"},
			want:    0,
			overall: 0,
		},
		{
			name:    "one large covered block and one small uncovered block",
			blocks:  []string{"1.1,6.2 5 1", "8.1,9.2 1 0"},
			want:    100 * 5 / 6.0,
			overall: 100 * 5 / 6.0,
		},
		{
			name:    "one small covered block and one large uncovered block",
			blocks:  []string{"1.1,2.2 1 1", "4.1,9.2 5 0"},
			want:    100 / 6.0,
			overall: 100 / 6.0,
		},
		{
			name:    "a hit count above one",
			blocks:  []string{"1.1,2.2 1 7", "4.1,5.2 1 0"},
			want:    50,
			overall: 50,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lines := make([]string, 0, len(testCase.blocks))
			for _, block := range testCase.blocks {
				lines = append(lines, fixtureModulePath+"/internal/order/order.go:"+block)
			}
			report, err := evaluateProfile(t, DefaultConfiguration(), profileOf(lines...))
			if err != nil {
				t.Fatalf("evaluate coverage: %v", err)
			}
			if percent := metricValue(t, report, "internal/order/order.go"); !nearly(percent, testCase.want) {
				t.Errorf("the file coverage is %v percent, want %v percent", percent, testCase.want)
			}
			if percent := metricValue(t, report, ""); !nearly(percent, testCase.overall) {
				t.Errorf("the overall coverage is %v percent, want %v percent", percent, testCase.overall)
			}
			if len(report.Findings) != 0 {
				t.Errorf("the report holds %d findings, want none", len(report.Findings))
			}
		})
	}
}

// TestEvaluatorDividesTheTotalStatementsOfEverySelectedFile proves that the
// overall figure sums the statements of every file. The mean of the per-file
// percentages of this profile is 50, and the statement ratio is 10.
func TestEvaluatorDividesTheTotalStatementsOfEverySelectedFile(t *testing.T) {
	report, err := evaluateProfile(t, DefaultConfiguration(), profileOf(
		fixtureModulePath+"/internal/small/small.go:1.1,2.2 1 1",
		fixtureModulePath+"/internal/large/large.go:1.1,9.2 9 0",
	))
	if err != nil {
		t.Fatalf("evaluate coverage: %v", err)
	}
	if percent := metricValue(t, report, "internal/small/small.go"); !nearly(percent, 100) {
		t.Errorf("the small file reports %v percent, want 100 percent", percent)
	}
	if percent := metricValue(t, report, "internal/large/large.go"); !nearly(percent, 0) {
		t.Errorf("the large file reports %v percent, want 0 percent", percent)
	}
	if percent := metricValue(t, report, ""); !nearly(percent, 10) {
		t.Errorf("the overall coverage is %v percent, want 10 percent", percent)
	}
}

func TestEvaluatorAppliesCoverageByPath(t *testing.T) {
	repositoryRoot := newRepository(t)
	runner := &coverageRunner{profile: profileOf(
		fixtureModulePath+"/internal/domain/order.go:1.1,2.1 10 1",
		fixtureModulePath+"/internal/domain/order.go:3.1,4.1 10 0",
		fixtureModulePath+"/cmd/tool/main.go:1.1,2.1 5 1",
	)}
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
	if finding.Rule != ruleMinimumPathCoverage || finding.Severity != plugin.SeverityError {
		t.Errorf("the finding rule is %q with severity %q", finding.Rule, finding.Severity)
	}
	if finding.Limit == nil || *finding.Limit != 100 {
		t.Errorf("the finding limit is %v, want 100", finding.Limit)
	}
	if runner.command.Directory != repositoryRoot {
		t.Errorf("the command directory is %q, want %q", runner.command.Directory, repositoryRoot)
	}
	if runner.command.Path != "go" {
		t.Errorf("the command path is %q, want %q", runner.command.Path, "go")
	}
}

func TestEvaluatorReportsAmbiguousPathPolicies(t *testing.T) {
	report, err := evaluateProfile(t, Configuration{
		Command: "go", Packages: []string{"./..."},
		Policies: []policy.PathPolicy{
			{
				ID: "everything", Include: []string{"**"},
				Thresholds: []policy.Threshold{{
					Metric: metricCoveragePercent, Comparison: policy.ComparisonMinimum,
					Value: 80, Severity: plugin.SeverityError,
				}},
			},
			{
				ID: "domain", Include: []string{"internal/**"},
				Thresholds: []policy.Threshold{{
					Metric: metricCoveragePercent, Comparison: policy.ComparisonMinimum,
					Value: 90, Severity: plugin.SeverityError,
				}},
			},
		},
	}, profileOf(fixtureModulePath+"/internal/order/order.go:1.1,2.2 1 1"))
	if !errors.Is(err, failure.ErrValidation) {
		t.Fatalf("the failure is %v, want a validation failure", err)
	}
	if len(report.Metrics) != 0 {
		t.Errorf("the report holds %d metrics, want none", len(report.Metrics))
	}
}

func TestEvaluatorSelectsTheGoPackagePatterns(t *testing.T) {
	testCases := []struct {
		name     string
		packages []string
		paths    []string
		want     []string
	}{
		{
			name:     "no selected path",
			packages: []string{"./internal/...", "./plugin/..."},
			want:     []string{"./internal/...", "./plugin/..."},
		},
		{
			name:     "one selected path",
			packages: []string{"./..."},
			paths:    []string{"internal/domain"},
			want:     []string{"./internal/domain/..."},
		},
		{
			name:     "the repository root as a selected path",
			packages: []string{"./internal/..."},
			paths:    []string{"."},
			want:     []string{"./..."},
		},
		{
			name:     "a selected path with a leading dot and slash",
			packages: []string{"./..."},
			paths:    []string{"./internal/domain"},
			want:     []string{"./internal/domain/..."},
		},
		{
			name:     "a selected path with a trailing slash and blank characters",
			packages: []string{"./..."},
			paths:    []string{"  internal/domain/  "},
			want:     []string{"./internal/domain/..."},
		},
		{
			name:     "two selected paths that name the same directory",
			packages: []string{"./..."},
			paths:    []string{"plugin", "./plugin"},
			want:     []string{"./plugin/..."},
		},
		{
			name:     "two selected paths in reverse order",
			packages: []string{"./..."},
			paths:    []string{"plugin", "internal"},
			want:     []string{"./internal/...", "./plugin/..."},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			runner := &coverageRunner{profile: profileOf()}
			evaluator, err := NewEvaluator(runner, Configuration{
				Command: "go", Packages: testCase.packages,
			})
			if err != nil {
				t.Fatalf("create evaluator: %v", err)
			}
			if _, err := evaluator.Evaluate(t.Context(), plugin.Request{
				RepositoryRoot: newRepository(t), Paths: testCase.paths,
			}); err != nil {
				t.Fatalf("evaluate coverage: %v", err)
			}
			arguments := runner.command.Arguments
			if len(arguments) < 3 {
				t.Fatalf("the command arguments are %v", arguments)
			}
			if arguments[0] != "test" || arguments[1] != "-covermode=atomic" {
				t.Errorf("the command arguments are %v", arguments)
			}
			if !strings.HasPrefix(arguments[2], "-coverprofile=") {
				t.Errorf("the third argument is %q, want a coverage profile option", arguments[2])
			}
			selected := arguments[3:]
			if len(selected) != len(testCase.want) {
				t.Fatalf("the selected packages are %v, want %v", selected, testCase.want)
			}
			for index, want := range testCase.want {
				if selected[index] != want {
					t.Errorf("package %d is %q, want %q", index, selected[index], want)
				}
			}
		})
	}
}

func TestEvaluatorRejectsAPathOutsideTheRepository(t *testing.T) {
	testCases := []struct {
		name string
		path string
	}{
		{name: "an absolute path", path: "/etc/passwd"},
		{name: "an absolute path inside the repository", path: "/workspace/project/internal"},
		{name: "a parent path", path: ".."},
		{name: "a path that leaves the repository", path: "internal/../../secret"},
		{name: "an empty path", path: ""},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			runner := &coverageRunner{profile: profileOf()}
			evaluator, err := NewEvaluator(runner, DefaultConfiguration())
			if err != nil {
				t.Fatalf("create evaluator: %v", err)
			}
			_, err = evaluator.Evaluate(t.Context(), plugin.Request{
				RepositoryRoot: newRepository(t), Paths: []string{testCase.path},
			})
			if !errors.Is(err, failure.ErrValidation) {
				t.Fatalf("the failure is %v, want a validation failure", err)
			}
			if runner.calls != 0 {
				t.Errorf("the evaluator ran the Go command %d times, want none", runner.calls)
			}
		})
	}
}

func TestEvaluatorReadsTheModuleDeclarationOfTheRepository(t *testing.T) {
	testCases := []struct {
		name     string
		prepare  func(t *testing.T, root string)
		category error
	}{
		{
			name:     "a repository without a module file",
			prepare:  func(*testing.T, string) {},
			category: failure.ErrValidation,
		},
		{
			name: "a module file that is a directory",
			prepare: func(t *testing.T, root string) {
				t.Helper()
				if err := os.Mkdir(filepath.Join(root, "go.mod"), 0o750); err != nil {
					t.Fatalf("create module directory: %v", err)
				}
			},
			category: failure.ErrUnavailable,
		},
		{
			name: "a module file without a module declaration",
			prepare: func(t *testing.T, root string) {
				t.Helper()
				writeFile(t, root, "go.mod", "go 1.26.3\n")
			},
			category: failure.ErrDataIntegrity,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			testCase.prepare(t, root)
			runner := &coverageRunner{profile: profileOf()}
			evaluator, err := NewEvaluator(runner, DefaultConfiguration())
			if err != nil {
				t.Fatalf("create evaluator: %v", err)
			}
			_, err = evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root})
			if !errors.Is(err, testCase.category) {
				t.Fatalf("the failure is %v, want %v", err, testCase.category)
			}
			if runner.calls != 0 {
				t.Errorf("the evaluator ran the Go command %d times, want none", runner.calls)
			}
		})
	}
}

// TestEvaluatorKeepsTheCoverageOfAFailedTestRun proves that a non-zero exit of
// the Go command never discards a profile that measures a package. The Go tool
// writes every package that compiled and ran, so the evaluator keeps that
// evidence and reports the failed run as one finding.
func TestEvaluatorKeepsTheCoverageOfAFailedTestRun(t *testing.T) {
	runner := &coverageRunner{
		profile:  profileOf(fixtureModulePath + "/internal/order/order.go:1.1,4.2 4 1"),
		runError: failure.Unavailable("run command \"go\"", nil),
		result:   plugin.CommandResult{StandardError: []byte("FAIL example.com/project/internal/order")},
	}
	evaluator, err := NewEvaluator(runner, DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: newRepository(t)})
	if err != nil {
		t.Fatalf("evaluate coverage: %v", err)
	}
	if percent := metricValue(t, report, "internal/order/order.go"); !nearly(percent, 100) {
		t.Errorf("the file coverage is %v percent, want 100 percent", percent)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("the report holds %d findings, want one", len(report.Findings))
	}
	finding := report.Findings[0]
	if finding.ID != findingTestRun || finding.Rule != ruleTestRun {
		t.Errorf("the finding is %q with rule %q", finding.ID, finding.Rule)
	}
	if finding.Severity != plugin.SeverityError {
		t.Errorf("the finding severity is %q, want %q", finding.Severity, plugin.SeverityError)
	}
	if finding.Path != "" {
		t.Errorf("the finding path is %q, want no path", finding.Path)
	}
}

func TestEvaluatorReportsNoTestRunFindingForASuccessfulRun(t *testing.T) {
	report, err := evaluateProfile(t, DefaultConfiguration(), profileOf(
		fixtureModulePath+"/internal/order/order.go:1.1,4.2 4 1",
	))
	if err != nil {
		t.Fatalf("evaluate coverage: %v", err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("the report holds %d findings, want none: %+v", len(report.Findings), report.Findings)
	}
}

func TestEvaluatorReportsTheRunFailureWhenTheProfileMeasuresNothing(t *testing.T) {
	testCases := []struct {
		name    string
		profile string
	}{
		{name: "an empty profile", profile: ""},
		{name: "a profile with a mode line only", profile: profileOf()},
		{name: "a malformed profile", profile: "this is not a coverage profile\n"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			runner := &coverageRunner{
				profile:  testCase.profile,
				runError: failure.Unavailable("run command \"go\"", nil),
				result: plugin.CommandResult{
					StandardOutput: []byte("build output"),
					StandardError:  []byte("build failed"),
				},
			}
			evaluator, err := NewEvaluator(runner, DefaultConfiguration())
			if err != nil {
				t.Fatalf("create evaluator: %v", err)
			}
			_, err = evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: newRepository(t)})
			if !errors.Is(err, failure.ErrUnavailable) {
				t.Fatalf("the failure is %v, want an unavailable failure", err)
			}
			if errors.Is(err, failure.ErrDataIntegrity) {
				t.Errorf("the failure is %v, want no data integrity category", err)
			}
			if !strings.Contains(err.Error(), "build failed") {
				t.Errorf("the failure %v does not name the captured output", err)
			}
			if !strings.Contains(err.Error(), "build output") {
				t.Errorf("the failure %v does not name the captured output", err)
			}
		})
	}
}

func TestEvaluatorReportsAnEmptyProfileOfASuccessfulRun(t *testing.T) {
	report, err := evaluateProfile(t, DefaultConfiguration(), profileOf())
	if err != nil {
		t.Fatalf("evaluate coverage: %v", err)
	}
	if len(report.Metrics) != 1 || len(report.Findings) != 0 {
		t.Fatalf("unexpected coverage report: %+v", report)
	}
	if percent := metricValue(t, report, ""); percent != 0 {
		t.Errorf("the overall coverage is %v percent, want 0 percent", percent)
	}
}

func TestEvaluatorReportsAMalformedProfileOfASuccessfulRun(t *testing.T) {
	_, err := evaluateProfile(t, DefaultConfiguration(), "this is not a coverage profile\n")
	if !errors.Is(err, failure.ErrDataIntegrity) {
		t.Fatalf("the failure is %v, want a data integrity failure", err)
	}
}

// TestEvaluatorKeepsTheParseFailureOfARemovedProfile proves that the deferred
// removal ignores a profile the Go command already deleted. The evaluation
// keeps the parse failure and adds no removal failure to it.
func TestEvaluatorKeepsTheParseFailureOfARemovedProfile(t *testing.T) {
	runner := &coverageRunner{removeProfile: true}
	evaluator, err := NewEvaluator(runner, DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	_, err = evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: newRepository(t)})
	if !errors.Is(err, failure.ErrDataIntegrity) {
		t.Fatalf("the failure is %v, want a data integrity failure", err)
	}
	if errors.Is(err, failure.ErrUnavailable) {
		t.Errorf("the failure is %v, want no removal failure", err)
	}
}

// TestEvaluatorReportsAProfileItCannotRemove proves that the deferred removal
// adds its own failure to the result of one complete evaluation.
func TestEvaluatorReportsAProfileItCannotRemove(t *testing.T) {
	temporaryRoot := t.TempDir()
	t.Setenv("TMPDIR", temporaryRoot)
	t.Cleanup(func() {
		if err := os.Chmod(temporaryRoot, 0o750); err != nil {
			t.Errorf("restore the temporary directory: %v", err)
		}
	})
	runner := &coverageRunner{
		profile:       profileOf(fixtureModulePath + "/internal/order/order.go:1.1,2.2 1 1"),
		lockDirectory: true,
	}
	evaluator, err := NewEvaluator(runner, DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: newRepository(t)})
	if !errors.Is(err, failure.ErrUnavailable) {
		t.Fatalf("the failure is %v, want an unavailable failure", err)
	}
	if percent := metricValue(t, report, "internal/order/order.go"); !nearly(percent, 100) {
		t.Errorf("the file coverage is %v percent, want 100 percent", percent)
	}
}

func TestEvaluatorReportsAProfileItCannotCreate(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))
	runner := &coverageRunner{profile: profileOf()}
	evaluator, err := NewEvaluator(runner, DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	_, err = evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: newRepository(t)})
	if !errors.Is(err, failure.ErrUnavailable) {
		t.Fatalf("the failure is %v, want an unavailable failure", err)
	}
	if runner.calls != 0 {
		t.Errorf("the evaluator ran the Go command %d times, want none", runner.calls)
	}
}

func TestEvaluatorUsesTheWorkingDirectoryWithoutARepositoryRoot(t *testing.T) {
	t.Chdir(newRepository(t))
	runner := &coverageRunner{profile: profileOf()}
	evaluator, err := NewEvaluator(runner, DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	if _, err := evaluator.Evaluate(t.Context(), plugin.Request{}); err != nil {
		t.Fatalf("evaluate coverage: %v", err)
	}
	working, err := os.Getwd()
	if err != nil {
		t.Fatalf("read working directory: %v", err)
	}
	if runner.command.Directory != working {
		t.Errorf("the command directory is %q, want %q", runner.command.Directory, working)
	}
}

func TestResolveProfilePathStripsTheModulePath(t *testing.T) {
	repositoryRoot := filepath.Join(string(filepath.Separator), "repository", "project")
	testCases := []struct {
		name         string
		profilePath  string
		want         string
		inRepository bool
	}{
		{
			name:         "a module path segment that also names a directory",
			profilePath:  "example.com/tools/x/x.go",
			want:         "x/x.go",
			inRepository: true,
		},
		{
			name:         "a file in the repository root",
			profilePath:  "example.com/tools/root.go",
			want:         "root.go",
			inRepository: true,
		},
		{
			name:         "a repeated package directory name",
			profilePath:  "example.com/tools/a/b/a/x.go",
			want:         "a/b/a/x.go",
			inRepository: true,
		},
		{
			name:         "an absolute path inside the repository",
			profilePath:  filepath.Join(repositoryRoot, "internal", "order.go"),
			want:         "internal/order.go",
			inRepository: true,
		},
		{
			name:        "an absolute path outside the repository",
			profilePath: filepath.Join(string(filepath.Separator), "other", "order.go"),
		},
		{
			name:        "an entry of another module",
			profilePath: "other.example.com/library/order.go",
		},
		{
			name:        "an entry that names the repository root",
			profilePath: repositoryRoot,
		},
		{
			name:        "an entry that leaves the module root",
			profilePath: "example.com/tools/../secret.go",
		},
		{
			name:        "an entry that names the module without a file",
			profilePath: "example.com/tools/",
		},
		{
			name:        "a relative entry of an unknown module",
			profilePath: "internal/order.go",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resolved, inRepository := resolveProfilePath(
				repositoryRoot,
				"example.com/tools",
				testCase.profilePath,
			)
			if inRepository != testCase.inRepository {
				t.Fatalf("the entry reports %t, want %t", inRepository, testCase.inRepository)
			}
			if resolved != testCase.want {
				t.Errorf("the resolved path is %q, want %q", resolved, testCase.want)
			}
		})
	}
}

// TestEvaluatorKeepsTheFilesOfTheRepositoryBesideAForeignEntry proves that one
// entry the evaluator cannot attribute never discards a measured file.
func TestEvaluatorKeepsTheFilesOfTheRepositoryBesideAForeignEntry(t *testing.T) {
	report, err := evaluateProfile(t, DefaultConfiguration(), profileOf(
		"other.example.com/library/helper.go:1.1,2.2 4 0",
		fixtureModulePath+"/internal/order/order.go:1.1,2.2 1 1",
	))
	if err != nil {
		t.Fatalf("evaluate coverage: %v", err)
	}
	if len(report.Metrics) != 2 {
		t.Fatalf("the report holds %d metrics, want two: %+v", len(report.Metrics), report.Metrics)
	}
	if hasMetric(report, "library/helper.go") {
		t.Errorf("the report measures a file of another module: %+v", report.Metrics)
	}
	if percent := metricValue(t, report, ""); !nearly(percent, 100) {
		t.Errorf("the overall coverage is %v percent, want 100 percent", percent)
	}
}

func TestSummarizeProfilesOrdersFilesByRepositoryPath(t *testing.T) {
	repositoryRoot := filepath.Join(string(filepath.Separator), "repository", "project")
	profile := profileOf(
		filepath.ToSlash(filepath.Join(repositoryRoot, "zebra.go"))+":1.1,2.2 1 1",
		"example.com/tools/alpha.go:1.1,2.2 1 1",
	)
	profilePath := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(profilePath, []byte(profile), 0o600); err != nil {
		t.Fatalf("write coverage profile: %v", err)
	}
	parsed, err := cover.ParseProfiles(profilePath)
	if err != nil {
		t.Fatalf("parse coverage profile: %v", err)
	}
	files := summarizeProfiles(repositoryRoot, "example.com/tools", parsed)
	want := []string{"alpha.go", "zebra.go"}
	if len(files) != len(want) {
		t.Fatalf("the summary holds %d files, want %d", len(files), len(want))
	}
	for index, name := range want {
		if files[index].path != name {
			t.Errorf("file %d is %q, want %q", index, files[index].path, name)
		}
	}
}

func TestCoveragePercentDividesTheCoveredStatements(t *testing.T) {
	testCases := []struct {
		name       string
		covered    int64
		statements int64
		want       float64
	}{
		{name: "a file with no statement", covered: 0, statements: 0, want: 0},
		{name: "one covered statement", covered: 1, statements: 1, want: 100},
		{name: "half the statements covered", covered: 1, statements: 2, want: 50},
		{name: "every statement covered", covered: 4, statements: 4, want: 100},
		{name: "no covered statement", covered: 0, statements: 4, want: 0},
		{name: "three of four statements covered", covered: 3, statements: 4, want: 75},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			percent := coveragePercent(testCase.covered, testCase.statements)
			if !nearly(percent, testCase.want) {
				t.Errorf("the coverage is %v percent, want %v percent", percent, testCase.want)
			}
		})
	}
}

// fixturePackage holds one Go package of the integration repository.
type fixturePackage struct {
	name       string
	source     string
	sourceTest string
}

// integrationPackages describe every measurable shape of one Go repository.
// The comments state the statement counts the Go language defines, so the
// expected percentages do not depend on how the Go tool groups the blocks.
var integrationPackages = []fixturePackage{
	{
		// One statement, covered.
		name:   "full",
		source: "package full\n\nfunc Double(value int) int { return value * 2 }\n",
		sourceTest: "package full\n\nimport \"testing\"\n\n" +
			"func TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"unexpected\")\n\t}\n}\n",
	},
	{
		// Six statements, two covered.
		name: "partial",
		source: "package partial\n\nfunc Classify(value int) string {\n" +
			"\tif value < 0 {\n\t\treturn \"negative\"\n\t}\n" +
			"\tif value == 0 {\n\t\treturn \"zero\"\n\t}\n\treturn \"positive\"\n}\n\n" +
			"func Unused(value int) int {\n\treturn value + 1\n}\n",
		sourceTest: "package partial\n\nimport \"testing\"\n\n" +
			"func TestClassify(t *testing.T) {\n\tif Classify(-1) != \"negative\" {\n" +
			"\t\tt.Fatal(\"unexpected\")\n\t}\n}\n",
	},
	{
		// One statement, none covered.
		name:       "zero",
		source:     "package zero\n\nfunc Never(value int) int {\n\treturn value\n}\n",
		sourceTest: "package zero\n\nimport \"testing\"\n\nfunc TestNothing(*testing.T) {}\n",
	},
	{
		// No statement at all.
		name: "declaration",
		source: "package declaration\n\ntype Kind string\n\n" +
			"const KindAlpha Kind = \"alpha\"\n\nvar Table = map[Kind]int{KindAlpha: 1}\n",
		sourceTest: "package declaration\n\nimport \"testing\"\n\n" +
			"func TestTable(t *testing.T) {\n\tif Table[KindAlpha] != 1 {\n" +
			"\t\tt.Fatal(\"unexpected\")\n\t}\n}\n",
	},
	{
		// Three statements, none covered, and no test file.
		name: "untested",
		source: "package untested\n\nfunc Absolute(value int) int {\n" +
			"\tif value > 0 {\n\t\treturn value\n\t}\n\treturn -value\n}\n",
	},
}

// newIntegrationRepository writes one real Go module in a temporary directory.
func newIntegrationRepository(t *testing.T) string {
	t.Helper()
	root := newRepository(t)
	for _, fixture := range integrationPackages {
		writeFile(t, root, fixture.name+"/"+fixture.name+".go", fixture.source)
		if fixture.sourceTest == "" {
			continue
		}
		writeFile(t, root, fixture.name+"/"+fixture.name+"_test.go", fixture.sourceTest)
	}
	return root
}

// TestEvaluatorIntegrationRunsGoCoverage measures one real Go module with the
// real Go tool. It proves the statement arithmetic against the Go tool itself.
func TestEvaluatorIntegrationRunsGoCoverage(t *testing.T) {
	evaluator, err := NewEvaluator(plugin.NewCommandRunner(), DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{
		RepositoryRoot: newIntegrationRepository(t),
	})
	if err != nil {
		t.Fatalf("evaluate coverage: %v", err)
	}
	testCases := []struct {
		name string
		path string
		want float64
	}{
		{name: "a fully covered file", path: "full/full.go", want: 100},
		{name: "a partially covered file", path: "partial/partial.go", want: 100 * 2 / 6.0},
		{name: "a file with no covered statement", path: "zero/zero.go", want: 0},
		{name: "a package with no test file", path: "untested/untested.go", want: 0},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if percent := metricValue(t, report, testCase.path); !nearly(percent, testCase.want) {
				t.Errorf("the file coverage is %v percent, want %v percent", percent, testCase.want)
			}
		})
	}
	t.Run("a package with no statement", func(t *testing.T) {
		if hasMetric(report, "declaration/declaration.go") {
			t.Errorf("the report measures a file with no statement: %+v", report.Metrics)
		}
	})
	t.Run("the overall coverage", func(t *testing.T) {
		if percent := metricValue(t, report, ""); !nearly(percent, 100*3/11.0) {
			t.Errorf("the overall coverage is %v percent, want %v percent", percent, 100*3/11.0)
		}
	})
}

// TestEvaluatorIntegrationKeepsTheCoverageOfAFailingSuite proves that the Go
// tool writes a complete profile for a failing suite, and that the evaluator
// keeps it.
func TestEvaluatorIntegrationKeepsTheCoverageOfAFailingSuite(t *testing.T) {
	root := newIntegrationRepository(t)
	writeFile(t, root, "failing/failing.go", "package failing\n\nfunc Half(value int) int {\n\treturn value / 2\n}\n")
	writeFile(t, root, "failing/failing_test.go", "package failing\n\nimport \"testing\"\n\n"+
		"func TestHalf(t *testing.T) {\n\tif Half(4) != 3 {\n\t\tt.Fatal(\"unexpected\")\n\t}\n}\n")
	evaluator, err := NewEvaluator(plugin.NewCommandRunner(), DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root})
	if err != nil {
		t.Fatalf("evaluate coverage: %v", err)
	}
	if percent := metricValue(t, report, "failing/failing.go"); !nearly(percent, 100) {
		t.Errorf("the failing file reports %v percent, want 100 percent", percent)
	}
	if percent := metricValue(t, report, "full/full.go"); !nearly(percent, 100) {
		t.Errorf("the file of another package reports %v percent, want 100 percent", percent)
	}
	if len(report.Findings) != 1 || report.Findings[0].ID != findingTestRun {
		t.Fatalf("the report holds %d findings: %+v", len(report.Findings), report.Findings)
	}
}

// TestEvaluatorIntegrationSelectsOnePath proves the Go package pattern of one
// selected path reaches the Go tool.
func TestEvaluatorIntegrationSelectsOnePath(t *testing.T) {
	evaluator, err := NewEvaluator(plugin.NewCommandRunner(), DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{
		RepositoryRoot: newIntegrationRepository(t), Paths: []string{"partial"},
	})
	if err != nil {
		t.Fatalf("evaluate coverage: %v", err)
	}
	if hasMetric(report, "full/full.go") {
		t.Errorf("the report measures a package outside the selected path: %+v", report.Metrics)
	}
	if percent := metricValue(t, report, "partial/partial.go"); !nearly(percent, 100*2/6.0) {
		t.Errorf("the selected file reports %v percent, want %v percent", percent, 100*2/6.0)
	}
}
