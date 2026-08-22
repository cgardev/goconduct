package mutation

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cgardev/goconduct/failure"
	"github.com/cgardev/goconduct/plugin"
)

// Evaluator inventories or executes mutations through mutate4go.
type Evaluator struct {
	runner        plugin.CommandRunner
	configuration Configuration
}

var _ plugin.Evaluator = (*Evaluator)(nil)

// NewEvaluator validates configuration and creates a mutation evaluator.
func NewEvaluator(runner plugin.CommandRunner, configuration Configuration) (*Evaluator, error) {
	if runner == nil {
		return nil, failure.Validation("mutation command runner is nil", nil)
	}
	if strings.TrimSpace(configuration.Command) == "" {
		return nil, failure.Validation("mutation command is empty", nil)
	}
	if configuration.MaxWorkers < 0 {
		return nil, failure.Validation("mutation worker count is negative", nil)
	}
	if configuration.TimeoutFactor <= 0 {
		return nil, failure.Validation("mutation timeout factor must be positive", nil)
	}
	if configuration.MaximumSurvivors < 0 || configuration.MaximumUncovered < 0 {
		return nil, failure.Validation("mutation limits must not be negative", nil)
	}
	return &Evaluator{runner: runner, configuration: cloneConfiguration(configuration)}, nil
}

// Name returns the stable evaluator identifier.
func (*Evaluator) Name() string { return "mutation" }

// Evaluate scans or mutates every selected production Go file.
func (evaluator *Evaluator) Evaluate(
	ctx context.Context,
	request plugin.Request,
) (plugin.Report, error) {
	root := request.RepositoryRoot
	if root == "" {
		root = "."
	}
	paths := request.Paths
	if len(paths) == 0 {
		paths = evaluator.configuration.Paths
	}
	files, err := mutationFiles(root, paths)
	if err != nil {
		return plugin.Report{}, err
	}
	results := make([]mutationResult, 0, len(files))
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return plugin.Report{}, err
		}
		result, err := evaluator.evaluateFile(ctx, root, file)
		if err != nil {
			return plugin.Report{}, err
		}
		results = append(results, result)
	}
	return evaluator.report(results)
}

func (evaluator *Evaluator) evaluateFile(
	ctx context.Context,
	root string,
	file string,
) (mutationResult, error) {
	arguments := []string{file}
	if !evaluator.configuration.Execute {
		arguments = append(arguments, "--scan")
	} else {
		if evaluator.configuration.ReuseCoverage {
			arguments = append(arguments, "--reuse-coverage")
		}
		if evaluator.configuration.SinceLastRun {
			arguments = append(arguments, "--since-last-run")
		}
		if evaluator.configuration.MutateAll {
			arguments = append(arguments, "--mutate-all")
		}
		if evaluator.configuration.TestCommand != "" {
			arguments = append(arguments, "--test-command", evaluator.configuration.TestCommand)
		}
		if evaluator.configuration.MaxWorkers > 0 {
			arguments = append(arguments, "--max-workers", strconv.Itoa(evaluator.configuration.MaxWorkers))
		}
		arguments = append(arguments, "--timeout-factor", strconv.Itoa(evaluator.configuration.TimeoutFactor))
	}
	result, err := evaluator.runner.Run(ctx, plugin.Command{
		Path: evaluator.configuration.Command, Arguments: arguments, Directory: root,
	})
	if err != nil {
		return mutationResult{}, fmt.Errorf(
			"run mutate4go for %q: %w; stderr: %s",
			file,
			err,
			strings.TrimSpace(string(result.StandardError)),
		)
	}
	report, err := parseMutationReport(result.StandardOutput)
	if err != nil {
		return mutationResult{}, fmt.Errorf("parse mutate4go report for %q: %w", file, err)
	}
	report.path = file
	return report, nil
}

func mutationFiles(root string, selectedPaths []string) ([]string, error) {
	if len(selectedPaths) == 0 {
		return nil, failure.Validation("mutation evaluation requires at least one path", nil)
	}
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return nil, failure.Internal("resolve mutation repository root", err)
	}
	files := make([]string, 0)
	for _, selectedPath := range selectedPaths {
		if filepath.IsAbs(selectedPath) {
			return nil, failure.Validation(
				fmt.Sprintf("mutation path %q is not repository-relative", selectedPath),
				nil,
			)
		}
		fullPath := filepath.Join(rootPath, filepath.Clean(selectedPath))
		relative, err := filepath.Rel(rootPath, fullPath)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, failure.Validation(
				fmt.Sprintf("mutation path %q is outside the repository", selectedPath),
				nil,
			)
		}
		information, err := os.Stat(fullPath)
		if err != nil {
			return nil, failure.Unavailable(fmt.Sprintf("inspect mutation path %q", selectedPath), err)
		}
		if !information.IsDir() {
			if validMutationFile(fullPath) {
				files = append(files, filepath.ToSlash(relative))
			}
			continue
		}
		err = filepath.WalkDir(fullPath, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path != fullPath && ignoredMutationDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !validMutationFile(path) {
				return nil
			}
			relative, err := filepath.Rel(rootPath, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(relative))
			return nil
		})
		if err != nil {
			return nil, failure.Unavailable(
				fmt.Sprintf("discover mutation files in %q", selectedPath),
				err,
			)
		}
	}
	slices.Sort(files)
	files = slices.Compact(files)
	if len(files) == 0 {
		return nil, failure.Validation("selected mutation paths contain no production Go files", nil)
	}
	return files, nil
}

func validMutationFile(path string) bool {
	return filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go")
}

func ignoredMutationDirectory(name string) bool {
	return strings.HasPrefix(name, ".") || name == "target" || name == "vendor"
}

func (evaluator *Evaluator) report(results []mutationResult) (plugin.Report, error) {
	metrics := make([]plugin.Metric, 0, len(results)*5)
	findings := make([]plugin.Finding, 0)
	for _, result := range results {
		metrics = append(metrics,
			mutationMetric(result.path, "total", result.total),
			mutationMetric(result.path, "covered", result.covered),
			mutationMetric(result.path, "uncovered", result.uncovered),
		)
		if result.executed {
			metrics = append(metrics,
				mutationMetric(result.path, "killed", result.killed),
				mutationMetric(result.path, "survived", result.survived),
				plugin.Metric{
					ID: "mutation:killed-percent:" + result.path, Path: result.path,
					Name: "mutation.killed.percent", Value: killedPercent(result), Unit: "percent",
				},
			)
		}
		if result.survived > evaluator.configuration.MaximumSurvivors {
			actual := float64(result.survived)
			limit := float64(evaluator.configuration.MaximumSurvivors)
			findings = append(findings, plugin.Finding{
				ID: "mutation:survived:" + result.path, Rule: "maximum-surviving-mutations",
				Path: result.path, Severity: plugin.SeverityError,
				Message: fmt.Sprintf(
					"%d mutations survived; the configured limit is %d",
					result.survived,
					evaluator.configuration.MaximumSurvivors,
				),
				Actual: &actual, Limit: &limit,
			})
		}
		if result.uncovered > evaluator.configuration.MaximumUncovered {
			actual := float64(result.uncovered)
			limit := float64(evaluator.configuration.MaximumUncovered)
			findings = append(findings, plugin.Finding{
				ID: "mutation:uncovered:" + result.path, Rule: "maximum-uncovered-mutations",
				Path: result.path, Severity: plugin.SeverityError,
				Message: fmt.Sprintf(
					"%d mutation sites are uncovered; the configured limit is %d",
					result.uncovered,
					evaluator.configuration.MaximumUncovered,
				),
				Actual: &actual, Limit: &limit,
			})
		}
	}
	return plugin.NewReport("mutation", metrics, findings)
}

func mutationMetric(path, name string, value int) plugin.Metric {
	return plugin.Metric{
		ID: "mutation:" + name + ":" + path, Path: path,
		Name: "mutation." + name, Value: float64(value), Unit: "count",
	}
}

func killedPercent(result mutationResult) float64 {
	tested := result.killed + result.survived
	if tested == 0 {
		return 100
	}
	return float64(result.killed) * 100 / float64(tested)
}
