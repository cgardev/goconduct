package architecture

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"

	goplugin "github.com/cgardev/goconduct/pkg/plugin"
)

const architectureMetricUnit = "count"

// Evaluator adapts the dependency graph to normalized plugin evidence.
type Evaluator struct {
	runtime       graphAnalyzer
	configuration ApplicationConfiguration
}

var _ goplugin.Evaluator = (*Evaluator)(nil)

// NewEvaluator creates an independently usable architecture evaluator.
func NewEvaluator(logger *slog.Logger) *Evaluator {
	return NewEvaluatorWithConfiguration(logger, DefaultApplicationConfiguration())
}

// NewEvaluatorWithConfiguration creates an evaluator with explicit architecture rules.
func NewEvaluatorWithConfiguration(
	logger *slog.Logger,
	configuration ApplicationConfiguration,
) *Evaluator {
	return newEvaluator(newDependencyGraphRuntime(
		analyzerGraphSourceFactory{},
		httpGraphCacheFactory{},
		logger,
	), configuration)
}

func newEvaluator(runtime graphAnalyzer, configuration ApplicationConfiguration) *Evaluator {
	return &Evaluator{runtime: runtime, configuration: configuration}
}

// Name returns the stable evaluator identifier.
func (*Evaluator) Name() string {
	return "architecture"
}

// Evaluate analyzes the selected Go repository paths.
func (evaluator *Evaluator) Evaluate(
	ctx context.Context,
	request goplugin.Request,
) (goplugin.Report, error) {
	configuration := evaluator.configuration
	configuration.Cache.Mode = CacheModeLocal
	if request.RepositoryRoot != "" {
		configuration.Analysis.RepositoryRoot = request.RepositoryRoot
	}
	if len(request.Paths) != 0 {
		configuration.Analysis.Paths = slices.Clone(request.Paths)
	}
	graph, err := evaluator.runtime.analyze(ctx, configuration)
	if err != nil {
		return goplugin.Report{}, fmt.Errorf("analyze architecture: %w", err)
	}
	return normalizedArchitectureReport(graph)
}

func normalizedArchitectureReport(graph Graph) (goplugin.Report, error) {
	metrics := []goplugin.Metric{
		architectureMetric("components", graph.Summary.Components),
		architectureMetric("relationships", graph.Summary.Relationships),
		architectureMetric("functions", graph.Summary.Functions),
		architectureMetric("function-calls", graph.Summary.FunctionCalls),
		architectureMetric("cycles", graph.Summary.Cycles),
	}
	findings := make([]goplugin.Finding, 0, len(graph.Findings))
	for index, finding := range graph.Findings {
		severity := goplugin.SeverityWarning
		if finding.Severity == findingSeverityError {
			severity = goplugin.SeverityError
		}
		findingPath := finding.Subject
		if findingPath == "" {
			findingPath = finding.Source
		}
		findings = append(findings, goplugin.Finding{
			ID:   "architecture:" + finding.Rule + ":" + strconv.Itoa(index),
			Rule: finding.Rule, Path: findingPath, Severity: severity, Message: finding.Message,
		})
	}
	return goplugin.NewReport("architecture", metrics, findings)
}

func architectureMetric(name string, value int) goplugin.Metric {
	return goplugin.Metric{
		ID: "architecture:" + name, Name: "architecture." + name,
		Value: float64(value), Unit: architectureMetricUnit,
	}
}
