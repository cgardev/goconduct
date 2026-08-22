// Package mutation integrates mutate4go as a composable quality plugin.
package mutation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/plugin"
)

// Module contains the mutation dependency registrations.
var Module = do.Package(newEvaluatorInjector())

type mutationPlugin struct{}

var _ plugin.Plugin = mutationPlugin{}

// Plugin returns the mutation lifecycle adapter.
func Plugin() plugin.Plugin { return mutationPlugin{} }

func (mutationPlugin) Name() string { return "mutation" }

func (mutationPlugin) Services() func(do.Injector) { return Module }

func (mutationPlugin) Activate(_ context.Context, injector do.Injector) error {
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

func (mutationPlugin) RegisterCommands(injector do.Injector, root *cobra.Command) error {
	runner, err := do.Invoke[plugin.CommandRunner](injector)
	if err != nil {
		return err
	}
	root.AddCommand(newMutationCommand(runner))
	return nil
}

func (mutationPlugin) RegisterEndpoints(do.Injector, plugin.EndpointRegistrar) error { return nil }

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

func newMutationCommand(runner plugin.CommandRunner) *cobra.Command {
	var repositoryRoot string
	var execute bool
	var mutateAll bool
	var reuseCoverage bool
	var maximumSurvivors int
	var maximumUncovered int
	var maxWorkers int
	var indent bool
	command := &cobra.Command{
		Use:   "mutation [file-or-directory ...]",
		Short: "Scan or execute mutate4go for selected source paths.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, paths []string) error {
			configuration := DefaultConfiguration()
			configuration.Execute = execute
			configuration.MutateAll = mutateAll
			configuration.ReuseCoverage = reuseCoverage
			configuration.MaximumSurvivors = maximumSurvivors
			configuration.MaximumUncovered = maximumUncovered
			configuration.MaxWorkers = maxWorkers
			evaluator, err := NewEvaluator(runner, configuration)
			if err != nil {
				return err
			}
			report, err := evaluator.Evaluate(command.Context(), plugin.Request{
				RepositoryRoot: repositoryRoot, Paths: paths,
			})
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(command.OutOrStdout())
			encoder.SetEscapeHTML(false)
			if indent {
				encoder.SetIndent("", "  ")
			}
			if err := encoder.Encode(report); err != nil {
				return fmt.Errorf("write mutation report: %w", err)
			}
			if len(report.Findings) != 0 {
				return fmt.Errorf("mutation analysis has %d policy findings", len(report.Findings))
			}
			return nil
		},
	}
	command.Flags().StringVar(&repositoryRoot, "repository", ".", "Select the Go repository root.")
	command.Flags().BoolVar(&execute, "execute", false, "Execute mutations instead of scanning sites.")
	command.Flags().BoolVar(&mutateAll, "mutate-all", false, "Ignore the mutate4go manifest selection.")
	command.Flags().BoolVar(&reuseCoverage, "reuse-coverage", false, "Reuse existing mutate4go coverage data.")
	command.Flags().IntVar(&maximumSurvivors, "maximum-survivors", 0, "Allow this number of surviving mutations.")
	command.Flags().IntVar(&maximumUncovered, "maximum-uncovered", 0, "Allow this number of uncovered mutation sites.")
	command.Flags().IntVar(&maxWorkers, "max-workers", 0, "Set the mutate4go worker count.")
	command.Flags().BoolVar(&indent, "indent", false, "Indent the JSON report.")
	return command
}
