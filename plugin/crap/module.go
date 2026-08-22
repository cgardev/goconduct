// Package crap integrates crap4go as a composable quality plugin.
package crap

import (
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/plugin"
)

// Module contains the CRAP dependency registrations.
var Module = do.Package(newEvaluatorInjector())

type crapPlugin struct{}

var _ plugin.Plugin = crapPlugin{}

// Plugin returns the CRAP lifecycle adapter.
func Plugin() plugin.Plugin { return crapPlugin{} }

func (crapPlugin) Name() string { return "crap" }

func (crapPlugin) Services() func(do.Injector) { return Module }

func (crapPlugin) Activate(_ context.Context, injector do.Injector) error {
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

func (crapPlugin) RegisterCommands(injector do.Injector, root *cobra.Command) error {
	runner, err := do.Invoke[plugin.CommandRunner](injector)
	if err != nil {
		return err
	}
	root.AddCommand(newCRAPCommand(runner))
	return nil
}

func (crapPlugin) RegisterEndpoints(
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

func newCRAPCommand(runner plugin.CommandRunner) *cobra.Command {
	var repositoryRoot string
	var paths []string
	var maximum float64
	var indent bool
	command := &cobra.Command{
		Use:   "crap",
		Short: "Run crap4go and enforce a maximum function score.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			configuration := DefaultConfiguration()
			configuration.MaximumScore = maximum
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
				return fmt.Errorf("write CRAP report: %w", err)
			}
			if len(report.Findings) != 0 {
				return fmt.Errorf("CRAP analysis has %d policy findings", len(report.Findings))
			}
			return nil
		},
	}
	command.Flags().StringVar(&repositoryRoot, "repository", ".", "Select the Go repository root.")
	command.Flags().StringArrayVar(&paths, "path", nil, "Filter analyzed source paths.")
	command.Flags().Float64Var(&maximum, "maximum", 8, "Set the maximum accepted CRAP score.")
	command.Flags().BoolVar(&indent, "indent", false, "Indent the JSON report.")
	return command
}
