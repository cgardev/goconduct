// Package mutation reports mutation coverage as a composable quality plugin.
package mutation

import (
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
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

func (mutationPlugin) RegisterEndpoints(
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

func newMutationCommand(runner plugin.CommandRunner) *cobra.Command {
	var repositoryRoot string
	var execute bool
	var packages []string
	var maximumSurvivors int
	var maximumUncovered int
	var indent bool
	command := &cobra.Command{
		Use:   "mutation [path ...]",
		Short: "Report mutation sites and optionally run every covered mutation.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, paths []string) error {
			configuration := DefaultConfiguration()
			configuration.Execute = execute
			if len(packages) != 0 {
				configuration.Packages = packages
			}
			configuration.MaximumSurvivors = maximumSurvivors
			configuration.MaximumUncovered = maximumUncovered
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
				return failure.Unavailable("write mutation report", err)
			}
			if failing := plugin.FailingFindings(report.Findings); failing != 0 {
				return failure.BusinessRule(fmt.Sprintf(
					"mutation analysis has %d policy findings",
					failing,
				), nil)
			}
			return nil
		},
	}
	command.Flags().StringVar(&repositoryRoot, "repository", ".", "Select the Go repository root.")
	command.Flags().StringArrayVar(
		&packages,
		"package",
		nil,
		"Run this Go package pattern instead of the whole module. Repeat this option as needed.",
	)
	command.Flags().BoolVar(
		&execute,
		"execute",
		false,
		"Run every covered mutation instead of only reporting the sites.",
	)
	command.Flags().IntVar(
		&maximumSurvivors,
		"maximum-survivors",
		0,
		"Allow this number of surviving mutations.",
	)
	command.Flags().IntVar(
		&maximumUncovered,
		"maximum-uncovered",
		0,
		"Allow this number of uncovered mutation sites.",
	)
	command.Flags().BoolVar(&indent, "indent", false, "Indent the JSON report.")
	return command
}
