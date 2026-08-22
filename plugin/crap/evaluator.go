package crap

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/cgardev/goconduct/plugin"
	"github.com/cgardev/goconduct/policy"
)

// Evaluator executes crap4go and normalizes function risk metrics.
type Evaluator struct {
	runner        plugin.CommandRunner
	configuration Configuration
	resolver      *policy.Resolver
}

var _ plugin.Evaluator = (*Evaluator)(nil)

// NewEvaluator validates configuration and creates a CRAP evaluator.
func NewEvaluator(runner plugin.CommandRunner, configuration Configuration) (*Evaluator, error) {
	if runner == nil {
		return nil, fmt.Errorf("CRAP command runner is nil")
	}
	if strings.TrimSpace(configuration.Command) == "" {
		return nil, fmt.Errorf("CRAP command is empty")
	}
	if configuration.MaximumScore < 0 {
		return nil, fmt.Errorf("maximum CRAP score %.2f is negative", configuration.MaximumScore)
	}
	if configuration.MaxWorkers < 0 {
		return nil, fmt.Errorf("CRAP worker count %d is negative", configuration.MaxWorkers)
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

// Evaluate runs crap4go for the selected repository paths.
func (evaluator *Evaluator) Evaluate(
	ctx context.Context,
	request plugin.Request,
) (plugin.Report, error) {
	root := request.RepositoryRoot
	if root == "" {
		root = "."
	}
	arguments := make([]string, 0, len(request.Paths)+4)
	if evaluator.configuration.MaxWorkers > 0 {
		arguments = append(arguments, "--max-workers", strconv.Itoa(evaluator.configuration.MaxWorkers))
	}
	if evaluator.configuration.TestCommand != "" {
		arguments = append(arguments, "--test-command", evaluator.configuration.TestCommand)
	}
	arguments = append(arguments, slices.Clone(request.Paths)...)
	result, err := evaluator.runner.Run(ctx, plugin.Command{
		Path: evaluator.configuration.Command, Arguments: arguments, Directory: root,
	})
	if err != nil {
		return plugin.Report{}, fmt.Errorf(
			"run crap4go: %w; stderr: %s",
			err,
			strings.TrimSpace(string(result.StandardError)),
		)
	}
	metrics, err := parseReport(result.StandardOutput)
	if err != nil {
		return plugin.Report{}, err
	}
	return evaluator.report(metrics)
}

func (evaluator *Evaluator) report(functions []functionMetric) (plugin.Report, error) {
	metrics := make([]plugin.Metric, 0, len(functions)*3)
	findings := make([]plugin.Finding, 0)
	for index, function := range functions {
		identifier := function.packageID + "." + function.function
		ordinal := strconv.Itoa(index)
		metrics = append(metrics,
			plugin.Metric{ID: "crap:score:" + ordinal, Path: identifier, Name: metricCRAPScore, Value: function.score},
			plugin.Metric{ID: "crap:complexity:" + ordinal, Path: identifier, Name: "complexity.cyclomatic", Value: float64(function.complexity)},
			plugin.Metric{ID: "crap:coverage:" + ordinal, Path: identifier, Name: "coverage.percent", Value: function.coverage, Unit: "percent"},
		)
		limit := evaluator.configuration.MaximumScore
		severity := plugin.SeverityError
		policyID := "default"
		threshold, found, err := evaluator.resolver.Resolve(function.packageID, metricCRAPScore)
		if err != nil {
			return plugin.Report{}, err
		}
		if found {
			limit = threshold.Value
			severity = threshold.Severity
			policyID = threshold.PolicyID
		}
		if function.score <= limit {
			continue
		}
		actual := function.score
		findings = append(findings, plugin.Finding{
			ID: "crap:" + policyID + ":" + ordinal, Rule: "maximum-crap-score",
			Path: identifier, Severity: severity,
			Message: fmt.Sprintf(
				"CRAP score %.2f exceeds the %.2f limit from policy %q",
				actual,
				limit,
				policyID,
			),
			Actual: &actual, Limit: &limit,
		})
	}
	return plugin.NewReport("crap", metrics, findings)
}
