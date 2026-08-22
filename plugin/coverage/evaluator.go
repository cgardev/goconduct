package coverage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/cover"

	"github.com/cgardev/goconduct/plugin"
	"github.com/cgardev/goconduct/policy"
)

type fileCoverage struct {
	path       string
	statements int64
	covered    int64
}

// Evaluator calculates Go statement coverage and applies path policies.
type Evaluator struct {
	runner        plugin.CommandRunner
	configuration Configuration
	resolver      *policy.Resolver
}

var _ plugin.Evaluator = (*Evaluator)(nil)

// NewEvaluator validates configuration and creates a coverage evaluator.
func NewEvaluator(runner plugin.CommandRunner, configuration Configuration) (*Evaluator, error) {
	if runner == nil {
		return nil, fmt.Errorf("coverage command runner is nil")
	}
	if strings.TrimSpace(configuration.Command) == "" {
		return nil, fmt.Errorf("coverage command is empty")
	}
	if len(configuration.Packages) == 0 {
		return nil, fmt.Errorf("coverage package list is empty")
	}
	resolver, err := policy.NewResolver(configuration.Policies)
	if err != nil {
		return nil, fmt.Errorf("validate coverage policies: %w", err)
	}
	for _, candidate := range configuration.Policies {
		for _, threshold := range candidate.Thresholds {
			if threshold.Metric == metricCoveragePercent && (threshold.Value < 0 || threshold.Value > 100) {
				return nil, fmt.Errorf(
					"coverage policy %q limit %.2f is outside 0 through 100",
					candidate.ID,
					threshold.Value,
				)
			}
		}
	}
	return &Evaluator{
		runner: runner, configuration: cloneConfiguration(configuration), resolver: resolver,
	}, nil
}

// Name returns the stable evaluator identifier.
func (*Evaluator) Name() string {
	return "coverage"
}

// Evaluate runs Go tests and converts their coverage profile to evidence.
func (evaluator *Evaluator) Evaluate(
	ctx context.Context,
	request plugin.Request,
) (report plugin.Report, resultErr error) {
	repositoryRoot := request.RepositoryRoot
	if repositoryRoot == "" {
		repositoryRoot = "."
	}
	profile, err := os.CreateTemp("", "goconduct-coverage-*.out")
	if err != nil {
		return plugin.Report{}, fmt.Errorf("create coverage profile: %w", err)
	}
	profilePath := profile.Name()
	if err := profile.Close(); err != nil {
		return plugin.Report{}, fmt.Errorf("close coverage profile: %w", err)
	}
	defer func() {
		if err := os.Remove(profilePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove coverage profile: %w", err))
		}
	}()
	packages := evaluator.packages(request.Paths)
	arguments := []string{"test", "-covermode=atomic", "-coverprofile=" + profilePath}
	arguments = append(arguments, packages...)
	result, err := evaluator.runner.Run(ctx, plugin.Command{
		Path: evaluator.configuration.Command, Arguments: arguments, Directory: repositoryRoot,
	})
	if err != nil {
		return plugin.Report{}, fmt.Errorf(
			"run Go coverage: %w; stderr: %s",
			err,
			strings.TrimSpace(string(result.StandardError)),
		)
	}
	profiles, err := cover.ParseProfiles(profilePath)
	if err != nil {
		return plugin.Report{}, fmt.Errorf("parse coverage profile: %w", err)
	}
	files, err := summarizeProfiles(repositoryRoot, profiles)
	if err != nil {
		return plugin.Report{}, err
	}
	return evaluator.report(files)
}

func (evaluator *Evaluator) packages(requestPaths []string) []string {
	if len(requestPaths) == 0 {
		return slices.Clone(evaluator.configuration.Packages)
	}
	packages := make([]string, 0, len(requestPaths))
	for _, requestedPath := range requestPaths {
		cleaned := filepath.ToSlash(filepath.Clean(requestedPath))
		cleaned = strings.TrimPrefix(cleaned, "./")
		if cleaned == "." || cleaned == "" {
			packages = append(packages, "./...")
			continue
		}
		packages = append(packages, "./"+cleaned+"/...")
	}
	slices.Sort(packages)
	return slices.Compact(packages)
}

func summarizeProfiles(repositoryRoot string, profiles []*cover.Profile) ([]fileCoverage, error) {
	files := make([]fileCoverage, 0, len(profiles))
	for _, profile := range profiles {
		relativePath, err := resolveProfilePath(repositoryRoot, profile.FileName)
		if err != nil {
			return nil, err
		}
		entry := fileCoverage{path: relativePath}
		for _, block := range profile.Blocks {
			entry.statements += int64(block.NumStmt)
			if block.Count > 0 {
				entry.covered += int64(block.NumStmt)
			}
		}
		files = append(files, entry)
	}
	slices.SortFunc(files, func(left, right fileCoverage) int {
		return strings.Compare(left.path, right.path)
	})
	return files, nil
}

func resolveProfilePath(repositoryRoot, profilePath string) (string, error) {
	if filepath.IsAbs(profilePath) {
		relative, err := filepath.Rel(repositoryRoot, profilePath)
		if err != nil {
			return "", fmt.Errorf("resolve coverage path %q: %w", profilePath, err)
		}
		return filepath.ToSlash(relative), nil
	}
	slashPath := filepath.ToSlash(profilePath)
	segments := strings.Split(slashPath, "/")
	for index := range segments {
		candidate := filepath.Join(append([]string{repositoryRoot}, segments[index:]...)...)
		information, err := os.Stat(candidate)
		if err == nil && !information.IsDir() {
			return strings.Join(segments[index:], "/"), nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect coverage path %q: %w", candidate, err)
		}
	}
	return "", fmt.Errorf("coverage path %q is outside repository %q", profilePath, repositoryRoot)
}

func (evaluator *Evaluator) report(files []fileCoverage) (plugin.Report, error) {
	metrics := make([]plugin.Metric, 0, len(files)+1)
	findings := make([]plugin.Finding, 0)
	var totalStatements int64
	var totalCovered int64
	for _, file := range files {
		percent := coveragePercent(file.covered, file.statements)
		totalStatements += file.statements
		totalCovered += file.covered
		metrics = append(metrics, plugin.Metric{
			ID: "coverage:" + file.path, Path: file.path, Name: metricCoveragePercent,
			Value: percent, Unit: "percent",
		})
		threshold, found, err := evaluator.resolver.Resolve(file.path, metricCoveragePercent)
		if err != nil {
			return plugin.Report{}, err
		}
		if !found || threshold.Passes(percent) {
			continue
		}
		actual := percent
		limit := threshold.Value
		findings = append(findings, plugin.Finding{
			ID:   "coverage:" + threshold.PolicyID + ":" + file.path,
			Rule: "minimum-path-coverage", Path: file.path, Severity: threshold.Severity,
			Message: fmt.Sprintf(
				"statement coverage %.2f%% is below the %.2f%% limit from policy %q",
				actual,
				limit,
				threshold.PolicyID,
			),
			Actual: &actual, Limit: &limit,
		})
	}
	metrics = append(metrics, plugin.Metric{
		ID: "coverage:overall", Name: metricCoveragePercent,
		Value: coveragePercent(totalCovered, totalStatements), Unit: "percent",
	})
	return plugin.NewReport("coverage", metrics, findings)
}

func coveragePercent(covered, statements int64) float64 {
	if statements == 0 {
		return 100
	}
	return float64(covered) * 100 / float64(statements)
}
