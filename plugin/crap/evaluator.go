package crap

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

	"github.com/cgardev/goconduct/failure"
	"github.com/cgardev/goconduct/internal/library/gocomplexity"
	"github.com/cgardev/goconduct/internal/library/gocoverage"
	"github.com/cgardev/goconduct/internal/library/gosource"
	"github.com/cgardev/goconduct/plugin"
	"github.com/cgardev/goconduct/policy"
)

// functionRisk is one analyzed function with its measurements.
// The coverage profile describes no statement of a function that the selected
// build never compiled, so that function carries no coverage and no score.
type functionRisk struct {
	file       string
	name       string
	packageID  string
	complexity int
	coverage   float64
	score      float64
	measured   bool
}

// Evaluator measures the change risk of every Go function.
type Evaluator struct {
	runner        plugin.CommandRunner
	configuration Configuration
	resolver      *policy.Resolver
}

var _ plugin.Evaluator = (*Evaluator)(nil)

// NewEvaluator validates configuration and creates a CRAP evaluator.
func NewEvaluator(runner plugin.CommandRunner, configuration Configuration) (*Evaluator, error) {
	if runner == nil {
		return nil, failure.Validation("CRAP command runner is nil", nil)
	}
	if strings.TrimSpace(configuration.Command) == "" {
		return nil, failure.Validation("CRAP command is empty", nil)
	}
	if configuration.MaximumScore < 0 {
		return nil, failure.Validation(
			fmt.Sprintf("maximum CRAP score %.2f is negative", configuration.MaximumScore),
			nil,
		)
	}
	resolver, err := policy.NewResolver(configuration.Policies)
	if err != nil {
		return nil, fmt.Errorf("validate CRAP policies: %w", err)
	}
	return &Evaluator{
		runner: runner, configuration: cloneConfiguration(configuration), resolver: resolver,
	}, nil
}

// Name returns the stable evaluator identifier.
func (*Evaluator) Name() string { return "crap" }

// Evaluate measures statement coverage and cyclomatic complexity per function.
func (evaluator *Evaluator) Evaluate(
	ctx context.Context,
	request plugin.Request,
) (report plugin.Report, resultErr error) {
	root, err := filepath.Abs(cmp.Or(request.RepositoryRoot, "."))
	if err != nil {
		return plugin.Report{}, failure.Internal("resolve CRAP repository root", err)
	}
	profilePath, remove, err := newCoverageProfile()
	if err != nil {
		return plugin.Report{}, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, remove())
	}()
	if err := evaluator.runCoverage(ctx, root, profilePath); err != nil {
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
	files, err := gosource.Files(root, request.Paths)
	if err != nil {
		return plugin.Report{}, err
	}
	risks, err := analyzeFunctions(root, files, profile)
	if err != nil {
		return plugin.Report{}, err
	}
	return evaluator.report(risks)
}

// newCoverageProfile creates the profile outside the analyzed repository, so
// the analysis leaves no file behind in the tree of the caller.
func newCoverageProfile() (string, func() error, error) {
	profile, err := os.CreateTemp("", "goconduct-crap-*.out")
	if err != nil {
		return "", nil, failure.Unavailable("create CRAP coverage profile", err)
	}
	path := profile.Name()
	if err := profile.Close(); err != nil {
		return "", nil, failure.Unavailable("close CRAP coverage profile", err)
	}
	return path, func() error {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return failure.Unavailable("remove CRAP coverage profile", err)
		}
		return nil
	}, nil
}

func (evaluator *Evaluator) runCoverage(ctx context.Context, root, profilePath string) error {
	arguments := []string{"test", "-covermode=atomic", "-coverprofile=" + profilePath}
	arguments = append(arguments, slices.Clone(evaluator.configuration.Packages)...)
	result, err := evaluator.runner.Run(ctx, plugin.Command{
		Path: evaluator.configuration.Command, Arguments: arguments, Directory: root,
	})
	if err != nil {
		return fmt.Errorf(
			"run Go coverage for CRAP: %w; output: %s%s",
			err,
			strings.TrimSpace(string(result.StandardOutput)),
			strings.TrimSpace(string(result.StandardError)),
		)
	}
	return nil
}

// analyzeFunctions measures the change risk of every function of every file.
func analyzeFunctions(
	root string,
	files []string,
	profile *gocoverage.Profile,
) ([]functionRisk, error) {
	risks := make([]functionRisk, 0, len(files))
	for _, file := range files {
		functions, err := gocomplexity.Functions(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return nil, err
		}
		for _, function := range functions {
			risk := functionRisk{
				file: file, name: function.Name, packageID: function.Package,
				complexity: function.Complexity,
			}
			risk.coverage, risk.measured = profile.Fraction(file, function.StartLine, function.EndLine)
			if risk.measured {
				risk.score = gocomplexity.Score(function.Complexity, risk.coverage)
			}
			risks = append(risks, risk)
		}
	}
	slices.SortFunc(risks, compareFunctionRisk)
	return risks, nil
}

func compareFunctionRisk(left, right functionRisk) int {
	if comparison := strings.Compare(left.file, right.file); comparison != 0 {
		return comparison
	}
	return strings.Compare(left.name, right.name)
}

func (evaluator *Evaluator) report(risks []functionRisk) (plugin.Report, error) {
	metrics := make([]plugin.Metric, 0, len(risks)*3)
	findings := make([]plugin.Finding, 0)
	for index, risk := range risks {
		identity := risk.file + ":" + risk.name + ":" + strconv.Itoa(index)
		metrics = append(metrics, functionMetrics(risk, identity)...)
		finding, err := evaluator.scoreFinding(risk, identity)
		if err != nil {
			return plugin.Report{}, err
		}
		if finding != nil {
			findings = append(findings, *finding)
		}
	}
	return plugin.NewReport("crap", metrics, findings)
}

// functionMetrics reports the measurements the analysis made for one function.
func functionMetrics(risk functionRisk, identity string) []plugin.Metric {
	metrics := []plugin.Metric{{
		ID: "crap:complexity:" + identity, Path: risk.file,
		Name: "complexity.cyclomatic", Value: float64(risk.complexity),
	}}
	if !risk.measured {
		return metrics
	}
	return append(metrics,
		plugin.Metric{
			ID: "crap:coverage:" + identity, Path: risk.file,
			Name: "coverage.percent", Value: risk.coverage, Unit: "percent",
		},
		plugin.Metric{
			ID: "crap:score:" + identity, Path: risk.file,
			Name: metricCRAPScore, Value: risk.score,
		},
	)
}

// scoreFinding reports one function over its limit, or one function that the
// coverage profile does not describe. An unmeasured function keeps a risk that
// no limit can see, so the report names it instead of dropping it.
func (evaluator *Evaluator) scoreFinding(
	risk functionRisk,
	identity string,
) (*plugin.Finding, error) {
	if !risk.measured {
		return &plugin.Finding{
			ID: "crap:unmeasured:" + identity, Rule: "indeterminate-crap-score",
			Path: risk.file, Severity: plugin.SeverityWarning,
			Message: fmt.Sprintf(
				"the coverage profile describes no statement of %s.%s, so it has no CRAP score",
				risk.packageID,
				risk.name,
			),
		}, nil
	}
	threshold, err := evaluator.resolveScoreLimit(risk.file)
	if err != nil {
		return nil, err
	}
	if threshold.Passes(risk.score) {
		return nil, nil
	}
	actual := risk.score
	limit := threshold.Value
	return &plugin.Finding{
		ID: "crap:" + threshold.PolicyID + ":" + identity, Rule: "maximum-crap-score",
		Path: risk.file, Severity: threshold.Severity,
		Message: fmt.Sprintf(
			"%s.%s has a CRAP score of %.2f, outside the %s limit %.2f from policy %q",
			risk.packageID,
			risk.name,
			actual,
			threshold.Comparison,
			limit,
			threshold.PolicyID,
		),
		Actual: &actual, Limit: &limit,
	}, nil
}

// resolveScoreLimit returns the policy threshold for one file, or the
// configured maximum when no policy selects it.
func (evaluator *Evaluator) resolveScoreLimit(file string) (policy.ResolvedThreshold, error) {
	threshold, found, err := evaluator.resolver.Resolve(file, metricCRAPScore)
	if err != nil {
		return policy.ResolvedThreshold{}, err
	}
	if found {
		return threshold, nil
	}
	return policy.ResolvedThreshold{
		PolicyID: "default",
		Threshold: policy.Threshold{
			Metric:     metricCRAPScore,
			Comparison: policy.ComparisonMaximum,
			Value:      evaluator.configuration.MaximumScore,
			Severity:   plugin.SeverityError,
		},
	}, nil
}
