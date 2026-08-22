// Package coverage measures Go statement coverage and applies path limits.
package coverage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/failure"
	"github.com/cgardev/goconduct/plugin"
	"github.com/cgardev/goconduct/policy"
)

// Module contains the coverage dependency registrations.
var Module = do.Package(newEvaluatorInjector())

type coveragePlugin struct{}

var _ plugin.Plugin = coveragePlugin{}

// Plugin returns the coverage lifecycle adapter.
func Plugin() plugin.Plugin {
	return coveragePlugin{}
}

func (coveragePlugin) Name() string { return "coverage" }

func (coveragePlugin) Services() func(do.Injector) { return Module }

func (coveragePlugin) Activate(_ context.Context, injector do.Injector) error {
	catalog, err := do.Invoke[*plugin.Catalog](injector)
	if err != nil {
		return err
	}
	evaluator, err := do.Invoke[*Evaluator](injector)
	if err != nil {
		return err
	}
	return catalog.Register(evaluator)
}

func (coveragePlugin) RegisterCommands(injector do.Injector, root *cobra.Command) error {
	runner, err := do.Invoke[plugin.CommandRunner](injector)
	if err != nil {
		return err
	}
	root.AddCommand(newCoverageCommand(runner))
	return nil
}

func (coveragePlugin) RegisterEndpoints(
	do.Injector,
	plugin.EndpointRegistrar,
	...connect.HandlerOption,
) error {
	return nil
}

func newEvaluatorInjector() func(do.Injector) {
	return do.Lazy[*Evaluator](func(injector do.Injector) (*Evaluator, error) {
		runner, err := do.Invoke[plugin.CommandRunner](injector)
		if err != nil {
			return nil, err
		}
		configuration, err := do.Invoke[Configuration](injector)
		if err != nil {
			configuration = DefaultConfiguration()
		}
		return NewEvaluator(runner, configuration)
	})
}

func newCoverageCommand(runner plugin.CommandRunner) *cobra.Command {
	var repositoryRoot string
	var paths []string
	var minimum float64
	var indent bool
	command := &cobra.Command{
		Use:   "coverage",
		Short: "Measure Go statement coverage and apply path limits.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			patterns, err := coveragePatterns(paths)
			if err != nil {
				return err
			}
			configuration := DefaultConfiguration()
			configuration.Policies = []policy.PathPolicy{{
				ID: "command-line", Include: patterns,
				Thresholds: []policy.Threshold{{
					Metric: metricCoveragePercent, Comparison: policy.ComparisonMinimum,
					Value: minimum, Severity: plugin.SeverityError,
				}},
			}}
			evaluator, err := NewEvaluator(runner, configuration)
			if err != nil {
				return err
			}
			report, err := evaluator.Evaluate(command.Context(), plugin.Request{
				RepositoryRoot: repositoryRoot, Paths: slices.Clone(paths),
			})
			if err != nil {
				return err
			}
			if err := writeReport(command.OutOrStdout(), report, indent); err != nil {
				return err
			}
			if failing := plugin.FailingFindings(report.Findings); failing != 0 {
				return failure.BusinessRule(
					fmt.Sprintf("coverage has %d policy findings", failing),
					nil,
				)
			}
			return nil
		},
	}
	command.Flags().StringVar(&repositoryRoot, "repository", ".", "Select the Go repository root.")
	command.Flags().StringArrayVar(&paths, "path", nil, "Select a repository path. Repeat this option as needed.")
	command.Flags().Float64Var(&minimum, "minimum", 0, "Require this minimum statement coverage percentage.")
	command.Flags().BoolVar(&indent, "indent", false, "Indent the JSON report.")
	return command
}

// coveragePatterns converts the selected paths to policy include patterns.
// It normalizes every path exactly as the Go package selection does, so one
// path never reaches the package list and fails the policy validation.
func coveragePatterns(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return []string{"**"}, nil
	}
	patterns := make([]string, 0, len(paths))
	for _, selectedPath := range paths {
		selected, err := normalizeSelectedPath(selectedPath)
		if err != nil {
			return nil, err
		}
		pattern := "**"
		if selected != "" {
			pattern = selected + "/**"
		}
		patterns = append(patterns, pattern)
	}
	slices.Sort(patterns)
	return slices.Compact(patterns), nil
}

func writeReport(destination io.Writer, report plugin.Report, indent bool) error {
	encoder := json.NewEncoder(destination)
	encoder.SetEscapeHTML(false)
	if indent {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(report); err != nil {
		return failure.Unavailable("write coverage report", err)
	}
	return nil
}
