// Package architecture analyzes Go dependencies and exposes their graph.
// The package works as a direct evaluator and as a composed goconduct plugin.
package architecture

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	goplugin "github.com/cgardev/goconduct/pkg/plugin"
)

// Module contains the architecture dependency registrations.
var Module = do.Package(
	newRuntimeInjector(),
	newEvaluatorInjector(),
	newDashboardServiceInjector(),
)

type architecturePlugin struct{}

var _ goplugin.Plugin = architecturePlugin{}

// Plugin returns the architecture lifecycle adapter.
func Plugin() goplugin.Plugin {
	return architecturePlugin{}
}

func (architecturePlugin) Name() string {
	return "architecture"
}

func (architecturePlugin) Services() func(do.Injector) {
	return Module
}

func (architecturePlugin) Activate(ctx context.Context, injector do.Injector) error {
	catalog, err := do.Invoke[*goplugin.Catalog](injector)
	if err != nil {
		return err
	}
	evaluator, err := do.Invoke[*Evaluator](injector)
	if err != nil {
		return err
	}
	if err := catalog.Register(evaluator); err != nil {
		return err
	}
	dashboard, err := do.Invoke[*dashboardService](injector)
	if err != nil {
		return err
	}
	return dashboard.Activate(ctx)
}

func (architecturePlugin) RegisterCommands(injector do.Injector, root *cobra.Command) error {
	runtime, err := do.Invoke[*dependencyGraphRuntime](injector)
	if err != nil {
		return err
	}
	command := newRootCommand(runtime)
	root.Short = command.Short
	root.Long = command.Long
	root.Args = command.Args
	root.RunE = command.RunE
	root.SilenceErrors = command.SilenceErrors
	root.SilenceUsage = command.SilenceUsage
	root.Flags().AddFlagSet(command.Flags())
	root.PersistentFlags().AddFlagSet(command.PersistentFlags())
	for _, child := range command.Commands() {
		if child.Name() == "configuration-schema" {
			continue
		}
		root.AddCommand(child)
	}
	return nil
}

func (architecturePlugin) RegisterEndpoints(
	injector do.Injector,
	registrar goplugin.EndpointRegistrar,
	options ...connect.HandlerOption,
) error {
	dashboard, err := do.Invoke[*dashboardService](injector)
	if err != nil {
		return err
	}
	if err := dashboard.ConfigureHandlerOptions(options...); err != nil {
		return err
	}
	return registrar.Handle("/", dashboard)
}

func newRuntimeInjector() func(do.Injector) {
	return do.Lazy[*dependencyGraphRuntime](func(injector do.Injector) (*dependencyGraphRuntime, error) {
		logger, err := do.Invoke[*slog.Logger](injector)
		if err != nil {
			return nil, err
		}
		return newDependencyGraphRuntime(
			analyzerGraphSourceFactory{},
			httpGraphCacheFactory{},
			logger,
		), nil
	})
}

func newEvaluatorInjector() func(do.Injector) {
	return do.Lazy[*Evaluator](func(injector do.Injector) (*Evaluator, error) {
		runtime, err := do.Invoke[*dependencyGraphRuntime](injector)
		if err != nil {
			return nil, err
		}
		configuration, err := do.Invoke[ApplicationConfiguration](injector)
		if err != nil {
			configuration = DefaultApplicationConfiguration()
		}
		return newEvaluator(runtime, configuration), nil
	})
}

func newDashboardServiceInjector() func(do.Injector) {
	return do.Lazy[*dashboardService](func(injector do.Injector) (*dashboardService, error) {
		logger, err := do.Invoke[*slog.Logger](injector)
		if err != nil {
			return nil, err
		}
		configuration, err := do.Invoke[ApplicationConfiguration](injector)
		if err != nil {
			configuration = DefaultApplicationConfiguration()
		}
		return newDashboardService(analyzerGraphSourceFactory{}, configuration, logger), nil
	})
}
