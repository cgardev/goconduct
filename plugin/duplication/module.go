// Package duplication integrates dry4go as a composable quality plugin.
package duplication

import (
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/failure"
	"github.com/cgardev/goconduct/plugin"
)

// Module contains the duplication dependency registrations.
var Module = do.Package(newEvaluatorInjector())

type duplicationPlugin struct{}

var _ plugin.Plugin = duplicationPlugin{}

// Plugin returns the duplication lifecycle adapter.
func Plugin() plugin.Plugin { return duplicationPlugin{} }

func (duplicationPlugin) Name() string { return "duplication" }

func (duplicationPlugin) Services() func(do.Injector) { return Module }

func (duplicationPlugin) Activate(_ context.Context, injector do.Injector) error {
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

func (duplicationPlugin) RegisterCommands(injector do.Injector, root *cobra.Command) error {
	runner, err := do.Invoke[plugin.CommandRunner](injector)
	if err != nil {
		return err
	}
	root.AddCommand(newDuplicationCommand(runner))
	return nil
}

func (duplicationPlugin) RegisterEndpoints(
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

func newDuplicationCommand(runner plugin.CommandRunner) *cobra.Command {
	var repositoryRoot string
	var paths []string
	var maximum int
	var similarity float64
	var indent bool
	command := &cobra.Command{
		Use:   "duplication",
		Short: "Run dry4go and enforce a duplicate candidate limit.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			configuration := DefaultConfiguration()
			configuration.MaximumCandidates = maximum
			configuration.Similarity = similarity
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
				return failure.Unavailable("write duplication report", err)
			}
			if len(report.Findings) != 0 {
				return failure.BusinessRule(fmt.Sprintf(
					"duplication analysis has %d policy findings",
					len(report.Findings),
				), nil)
			}
			return nil
		},
	}
	command.Flags().StringVar(&repositoryRoot, "repository", ".", "Select the Go repository root.")
	command.Flags().StringArrayVar(&paths, "path", nil, "Select source paths for dry4go.")
	command.Flags().IntVar(&maximum, "maximum", 0, "Allow this number of duplicate candidates.")
	command.Flags().Float64Var(&similarity, "similarity", 0.82, "Set the dry4go similarity threshold.")
	command.Flags().BoolVar(&indent, "indent", false, "Indent the JSON report.")
	return command
}
