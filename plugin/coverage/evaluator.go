package coverage

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/cover"

	"github.com/cgardev/goconduct/failure"
	"github.com/cgardev/goconduct/plugin"
	"github.com/cgardev/goconduct/policy"
)

const (
	ruleMinimumPathCoverage = "minimum-path-coverage"
	ruleTestRun             = "go-test-run"
	findingTestRun          = "coverage:test-run"
	messageTestRun          = "the Go test run failed, so this report omits every package that did not run"
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
		return nil, failure.Validation("coverage command runner is nil", nil)
	}
	if strings.TrimSpace(configuration.Command) == "" {
		return nil, failure.Validation("coverage command is empty", nil)
	}
	if len(configuration.Packages) == 0 {
		return nil, failure.Validation("coverage package list is empty", nil)
	}
	resolver, err := policy.NewResolver(configuration.Policies)
	if err != nil {
		return nil, fmt.Errorf("validate coverage policies: %w", err)
	}
	for _, candidate := range configuration.Policies {
		for _, threshold := range candidate.Thresholds {
			if threshold.Metric == metricCoveragePercent && (threshold.Value < 0 || threshold.Value > 100) {
				return nil, failure.Validation(fmt.Sprintf(
					"coverage policy %q limit %.2f is outside 0 through 100",
					candidate.ID,
					threshold.Value,
				), nil)
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
	repositoryRoot, err := filepath.Abs(cmp.Or(request.RepositoryRoot, "."))
	if err != nil {
		return plugin.Report{}, failure.Internal("resolve coverage repository root", err)
	}
	modulePath, err := repositoryModulePath(repositoryRoot)
	if err != nil {
		return plugin.Report{}, err
	}
	packages, err := evaluator.packages(request.Paths)
	if err != nil {
		return plugin.Report{}, err
	}
	profilePath, err := createProfileFile()
	if err != nil {
		return plugin.Report{}, err
	}
	defer func() {
		if err := os.Remove(profilePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, failure.Unavailable("remove coverage profile", err))
		}
	}()
	arguments := []string{"test", "-covermode=atomic", "-coverprofile=" + profilePath}
	arguments = append(arguments, packages...)
	result, runErr := evaluator.runner.Run(ctx, plugin.Command{
		Path: evaluator.configuration.Command, Arguments: arguments, Directory: repositoryRoot,
	})
	profiles, parseErr := cover.ParseProfiles(profilePath)
	if parseErr != nil {
		if runErr != nil {
			return plugin.Report{}, runFailure(runErr, result)
		}
		return plugin.Report{}, failure.DataIntegrity("parse coverage profile", parseErr)
	}
	// The Go tool writes a complete profile for every package that compiled and
	// ran, so a failed run still measures them. The evaluator keeps that
	// measurement and reports the failed run as one finding. It returns the run
	// failure only when the profile measures nothing.
	if len(profiles) == 0 && runErr != nil {
		return plugin.Report{}, runFailure(runErr, result)
	}
	files := summarizeProfiles(repositoryRoot, modulePath, profiles)
	return evaluator.report(files, runErr != nil)
}

// createProfileFile reserves one profile path outside the analyzed repository.
func createProfileFile() (string, error) {
	profile, err := os.CreateTemp("", "goconduct-coverage-*.out")
	if err != nil {
		return "", failure.Unavailable("create coverage profile", err)
	}
	if err := profile.Close(); err != nil {
		return "", failure.Unavailable("close coverage profile", err)
	}
	return profile.Name(), nil
}

// runFailure keeps both output streams of the failed Go command.
func runFailure(cause error, result plugin.CommandResult) error {
	return fmt.Errorf(
		"run Go coverage: %w; output: %s%s",
		cause,
		strings.TrimSpace(string(result.StandardOutput)),
		strings.TrimSpace(string(result.StandardError)),
	)
}

// packages selects the Go package patterns of one request.
func (evaluator *Evaluator) packages(requestPaths []string) ([]string, error) {
	if len(requestPaths) == 0 {
		return slices.Clone(evaluator.configuration.Packages), nil
	}
	packages := make([]string, 0, len(requestPaths))
	for _, requestedPath := range requestPaths {
		selected, err := normalizeSelectedPath(requestedPath)
		if err != nil {
			return nil, err
		}
		if selected == "" {
			packages = append(packages, "./...")
			continue
		}
		packages = append(packages, "./"+selected+"/...")
	}
	slices.Sort(packages)
	return slices.Compact(packages), nil
}

// normalizeSelectedPath converts one selected path to a repository-relative path
// with forward slashes. The empty result names the repository root. An absolute
// path or a path that leaves the repository is rejected, because the policy
// patterns and the Go package patterns both need a repository-relative path.
func normalizeSelectedPath(selectedPath string) (string, error) {
	trimmed := strings.TrimSpace(selectedPath)
	if trimmed == "" {
		return "", failure.Validation("coverage path is empty", nil)
	}
	cleaned := filepath.ToSlash(filepath.Clean(trimmed))
	if cleaned == "." {
		return "", nil
	}
	if !filepath.IsLocal(filepath.FromSlash(cleaned)) {
		return "", failure.Validation(fmt.Sprintf(
			"coverage path %q is not repository-relative",
			selectedPath,
		), nil)
	}
	return cleaned, nil
}

// summarizeProfiles weights every block by its statement count.
// It drops one entry that names a file outside the repository, so a foreign
// entry never discards the files the run measured correctly.
func summarizeProfiles(
	repositoryRoot string,
	modulePath string,
	profiles []*cover.Profile,
) []fileCoverage {
	files := make([]fileCoverage, 0, len(profiles))
	for _, profile := range profiles {
		relativePath, inRepository := resolveProfilePath(repositoryRoot, modulePath, profile.FileName)
		if !inRepository {
			continue
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
	return files
}

// repositoryModulePath reads the module declaration of the repository root.
// A coverage profile names each file with the module path, so the evaluator
// needs that prefix to recover the repository-relative path.
func repositoryModulePath(repositoryRoot string) (string, error) {
	payload, err := os.ReadFile(filepath.Join(repositoryRoot, "go.mod"))
	if errors.Is(err, os.ErrNotExist) {
		return "", failure.Validation(fmt.Sprintf(
			"repository %q holds no go.mod file",
			repositoryRoot,
		), err)
	}
	if err != nil {
		return "", failure.Unavailable("read repository module file", err)
	}
	modulePath := modfile.ModulePath(payload)
	if modulePath == "" {
		return "", failure.DataIntegrity("module declaration not found in go.mod", nil)
	}
	return modulePath, nil
}

// resolveProfilePath recovers the repository-relative path of one profile entry.
// The Go tool names every file with the module path, so the evaluator strips
// that prefix instead of searching the file system for a matching suffix. It
// reports false for an entry that names no file of this repository.
func resolveProfilePath(repositoryRoot, modulePath, profilePath string) (string, bool) {
	candidate, trimmed := strings.CutPrefix(filepath.ToSlash(profilePath), modulePath+"/")
	if !trimmed {
		relative, err := filepath.Rel(repositoryRoot, profilePath)
		if err != nil {
			return "", false
		}
		candidate = filepath.ToSlash(relative)
	}
	if !filepath.IsLocal(filepath.FromSlash(candidate)) || candidate == "." {
		return "", false
	}
	return candidate, true
}

// report converts the measured files to metrics and policy findings.
// A failed test run adds one finding, so an incomplete profile never reports a
// silent success.
func (evaluator *Evaluator) report(files []fileCoverage, testRunFailed bool) (plugin.Report, error) {
	metrics := make([]plugin.Metric, 0, len(files))
	findings := make([]plugin.Finding, 0)
	if testRunFailed {
		findings = append(findings, plugin.Finding{
			ID: findingTestRun, Rule: ruleTestRun,
			Severity: plugin.SeverityError, Message: messageTestRun,
		})
	}
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
			Rule: ruleMinimumPathCoverage, Path: file.path, Severity: threshold.Severity,
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

// coveragePercent reports zero for an empty statement count, as crap4go does.
// An unmeasurable denominator must not satisfy a minimum coverage limit.
func coveragePercent(covered, statements int64) float64 {
	if statements == 0 {
		return 0
	}
	return float64(covered) * 100 / float64(statements)
}
