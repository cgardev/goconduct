package mutation

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cgardev/goconduct/internal/library/gocoverage"
	"github.com/cgardev/goconduct/internal/library/gomutation"
	"github.com/cgardev/goconduct/internal/library/gosource"
	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
)

// minimumMutationTimeout bounds a test suite that finishes almost instantly, so
// one mutation still gets a usable deadline.
const minimumMutationTimeout = 5 * time.Second

// fileResult holds the mutation measurements of one source file.
type fileResult struct {
	path      string
	total     int
	covered   int
	uncovered int
	killed    int
	survivors []gomutation.Site
	executed  bool
}

// Evaluator discovers mutation sites and, on request, runs the mutations.
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

// Evaluate reports the mutation sites of every selected file.
// It runs each covered mutation only when the configuration asks for it.
func (evaluator *Evaluator) Evaluate(
	ctx context.Context,
	request plugin.Request,
) (report plugin.Report, resultErr error) {
	root, err := filepath.Abs(cmp.Or(request.RepositoryRoot, "."))
	if err != nil {
		return plugin.Report{}, failure.Internal("resolve mutation repository root", err)
	}
	selected := request.Paths
	if len(selected) == 0 {
		selected = evaluator.configuration.Paths
	}
	files, err := gosource.Files(root, selected)
	if err != nil {
		return plugin.Report{}, err
	}
	if len(files) == 0 {
		return plugin.Report{}, failure.Validation(
			"the selected mutation paths hold no production Go file",
			nil,
		)
	}
	profilePath, remove, err := newCoverageProfile()
	if err != nil {
		return plugin.Report{}, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, remove())
	}()
	baseline, err := evaluator.runCoverage(ctx, root, profilePath)
	if err != nil {
		return plugin.Report{}, err
	}
	modulePath, err := gosource.ModulePath(root)
	if err != nil {
		return plugin.Report{}, err
	}
	profile, err := gocoverage.Load(profilePath, modulePath)
	if err != nil {
		return plugin.Report{}, err
	}
	results, err := evaluator.analyze(ctx, root, files, profile, baseline)
	if err != nil {
		return plugin.Report{}, err
	}
	return evaluator.report(results)
}

// newCoverageProfile creates the profile outside the analyzed repository.
func newCoverageProfile() (string, func() error, error) {
	profile, err := os.CreateTemp("", "goconduct-mutation-*.out")
	if err != nil {
		return "", nil, failure.Unavailable("create mutation coverage profile", err)
	}
	path := profile.Name()
	if err := profile.Close(); err != nil {
		return "", nil, failure.Unavailable("close mutation coverage profile", err)
	}
	return path, func() error {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return failure.Unavailable("remove mutation coverage profile", err)
		}
		return nil
	}, nil
}

// runCoverage measures the coverage and the duration of the unchanged suite.
// The duration bounds every mutation run.
func (evaluator *Evaluator) runCoverage(
	ctx context.Context,
	root string,
	profilePath string,
) (time.Duration, error) {
	arguments := []string{"test", "-covermode=atomic", "-coverprofile=" + profilePath}
	arguments = append(arguments, slices.Clone(evaluator.configuration.Packages)...)
	started := time.Now()
	result, err := evaluator.runner.Run(ctx, plugin.Command{
		Path: evaluator.configuration.Command, Arguments: arguments, Directory: root,
	})
	if err != nil {
		return 0, fmt.Errorf(
			"run the unchanged test suite: %w; output: %s%s",
			err,
			strings.TrimSpace(string(result.StandardOutput)),
			strings.TrimSpace(string(result.StandardError)),
		)
	}
	return time.Since(started), nil
}

func (evaluator *Evaluator) analyze(
	ctx context.Context,
	root string,
	files []string,
	profile *gocoverage.Profile,
	baseline time.Duration,
) ([]fileResult, error) {
	results := make([]fileResult, 0, len(files))
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result, err := evaluator.analyzeFile(ctx, root, file, profile, baseline)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (evaluator *Evaluator) analyzeFile(
	ctx context.Context,
	root string,
	file string,
	profile *gocoverage.Profile,
	baseline time.Duration,
) (fileResult, error) {
	source := filepath.Join(root, filepath.FromSlash(file))
	sites, err := gomutation.Sites(source)
	if err != nil {
		return fileResult{}, err
	}
	covered := make([]gomutation.Site, 0, len(sites))
	for _, site := range sites {
		if measurable(profile, file, site) {
			covered = append(covered, site)
		}
	}
	result := fileResult{
		path: file, total: len(sites), covered: len(covered),
		uncovered: len(sites) - len(covered),
	}
	if !evaluator.configuration.Execute {
		return result, nil
	}
	result.executed = true
	result.killed, result.survivors, err = evaluator.runMutations(ctx, root, source, covered, baseline)
	return result, err
}

// measurable reports whether the analysis can run one mutation site.
// A Go coverage profile describes function bodies only, so it never reaches a
// package level expression. The Go runtime evaluates such an expression when it
// loads the package, so every test run reaches it.
func measurable(profile *gocoverage.Profile, file string, site gomutation.Site) bool {
	if site.Function == "" {
		return true
	}
	return profile.CoversLine(file, site.Line)
}

// runMutations applies each covered mutation, runs the suite, and restores the
// source. A test failure means the suite detects the changed behavior.
func (evaluator *Evaluator) runMutations(
	ctx context.Context,
	root string,
	source string,
	sites []gomutation.Site,
	baseline time.Duration,
) (killed int, survivors []gomutation.Site, resultErr error) {
	if len(sites) == 0 {
		return 0, nil, nil
	}
	original, err := os.ReadFile(source)
	if err != nil {
		return 0, nil, failure.Unavailable(fmt.Sprintf("read Go source %q", source), err)
	}
	defer func() {
		if err := os.WriteFile(source, original, 0o600); err != nil {
			resultErr = errors.Join(resultErr, failure.Unavailable(
				fmt.Sprintf("restore Go source %q", source), err,
			))
		}
	}()
	survivors = make([]gomutation.Site, 0)
	timeout := max(baseline*time.Duration(evaluator.configuration.TimeoutFactor), minimumMutationTimeout)
	for _, site := range sites {
		detected, err := evaluator.runMutation(ctx, root, source, string(original), site, timeout)
		if err != nil {
			return killed, survivors, err
		}
		if detected {
			killed++
			continue
		}
		survivors = append(survivors, site)
	}
	return killed, survivors, nil
}

// runMutation reports whether the test suite detects one changed expression.
func (evaluator *Evaluator) runMutation(
	ctx context.Context,
	root string,
	source string,
	original string,
	site gomutation.Site,
	timeout time.Duration,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := os.WriteFile(source, []byte(site.Apply(original)), 0o600); err != nil {
		return false, failure.Unavailable(fmt.Sprintf("write mutation into %q", source), err)
	}
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	arguments := append([]string{"test"}, slices.Clone(evaluator.configuration.Packages)...)
	_, err := evaluator.runner.Run(bounded, plugin.Command{
		Path: evaluator.configuration.Command, Arguments: arguments, Directory: root,
	})
	if err != nil && ctx.Err() != nil {
		return false, ctx.Err()
	}
	return err != nil, nil
}

func (evaluator *Evaluator) report(results []fileResult) (plugin.Report, error) {
	metrics := make([]plugin.Metric, 0, len(results)*6)
	findings := make([]plugin.Finding, 0)
	for _, result := range results {
		metrics = append(metrics, mutationMetrics(result)...)
		findings = append(findings, survivorFindings(result)...)
		findings = append(findings, evaluator.thresholdFindings(result)...)
	}
	return plugin.NewReport("mutation", metrics, findings)
}

// mutationMetrics reports only the counts the evaluation measured.
func mutationMetrics(result fileResult) []plugin.Metric {
	metrics := []plugin.Metric{
		mutationMetric(result.path, "total", result.total),
		mutationMetric(result.path, "covered", result.covered),
		mutationMetric(result.path, "uncovered", result.uncovered),
	}
	if !result.executed {
		return metrics
	}
	metrics = append(metrics,
		mutationMetric(result.path, "killed", result.killed),
		mutationMetric(result.path, "survived", len(result.survivors)),
	)
	tested := result.killed + len(result.survivors)
	if tested == 0 {
		return metrics
	}
	return append(metrics, plugin.Metric{
		ID: "mutation:killed-percent:" + result.path, Path: result.path,
		Name: "mutation.killed.percent", Value: float64(result.killed) * 100 / float64(tested),
		Unit: "percent",
	})
}

// survivorFindings names every mutation the test suite did not detect.
// They carry no limit, because the configured limit compares their count.
func survivorFindings(result fileResult) []plugin.Finding {
	findings := make([]plugin.Finding, 0, len(result.survivors))
	for _, site := range result.survivors {
		findings = append(findings, plugin.Finding{
			ID: "mutation:survivor:" + result.path + ":" + strconv.Itoa(site.Line) +
				":" + strconv.Itoa(site.Index),
			Rule: "surviving-mutation", Path: result.path, Severity: plugin.SeverityNotice,
			Message: fmt.Sprintf(
				"%s line %d changes %s and the tests still pass",
				site.Function,
				site.Line,
				site.Describe(),
			),
		})
	}
	return findings
}

// thresholdFindings applies the configured mutation limits.
// Only an execution measures survivors, so a scan reports none.
func (evaluator *Evaluator) thresholdFindings(result fileResult) []plugin.Finding {
	findings := make([]plugin.Finding, 0, 2)
	if result.uncovered > evaluator.configuration.MaximumUncovered {
		findings = append(findings, mutationThresholdFinding(
			"uncovered",
			"maximum-uncovered-mutations",
			result.path,
			fmt.Sprintf(
				"%d mutation sites are uncovered; the configured limit is %d",
				result.uncovered,
				evaluator.configuration.MaximumUncovered,
			),
			result.uncovered,
			evaluator.configuration.MaximumUncovered,
		))
	}
	if result.executed && len(result.survivors) > evaluator.configuration.MaximumSurvivors {
		findings = append(findings, mutationThresholdFinding(
			"survived",
			"maximum-surviving-mutations",
			result.path,
			fmt.Sprintf(
				"%d mutations survived; the configured limit is %d",
				len(result.survivors),
				evaluator.configuration.MaximumSurvivors,
			),
			len(result.survivors),
			evaluator.configuration.MaximumSurvivors,
		))
	}
	return findings
}

func mutationThresholdFinding(
	kind string,
	rule string,
	path string,
	message string,
	actualCount int,
	limitCount int,
) plugin.Finding {
	actual := float64(actualCount)
	limit := float64(limitCount)
	return plugin.Finding{
		ID: "mutation:" + kind + ":" + path, Rule: rule,
		Path: path, Severity: plugin.SeverityError, Message: message,
		Actual: &actual, Limit: &limit,
	}
}

func mutationMetric(path, name string, value int) plugin.Metric {
	return plugin.Metric{
		ID: "mutation:" + name + ":" + path, Path: path,
		Name: "mutation." + name, Value: float64(value), Unit: "count",
	}
}
