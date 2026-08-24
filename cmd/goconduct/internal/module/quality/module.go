// Package quality composes evaluator plugins and exposes their normalized
// reports through Connect RPC. The composition root resolves QualityAPI.
package quality

import (
	"context"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/internal/protogen/v1/goconductv1connect"
	"github.com/cgardev/goconduct/pkg/plugin"
)

// Module contains the quality module dependency registrations.
var Module = do.Package(
	newListPluginsUseCaseInjector(),
	newRunCheckUseCaseInjector(),
	newQualityAPIInjector(),
)

type qualityPlugin struct{}

var _ plugin.Plugin = qualityPlugin{}

// Plugin returns the application quality module.
func Plugin() plugin.Plugin {
	return qualityPlugin{}
}

func (qualityPlugin) Name() string {
	return "quality"
}

func (qualityPlugin) Services() func(do.Injector) {
	return Module
}

func (qualityPlugin) Activate(_ context.Context, injector do.Injector) error {
	if _, err := do.Invoke[*ListPluginsUseCase](injector); err != nil {
		return err
	}
	if _, err := do.Invoke[*RunCheckUseCase](injector); err != nil {
		return err
	}
	_, err := do.Invoke[*QualityAPI](injector)
	return err
}

func (qualityPlugin) RegisterCommands(do.Injector, *cobra.Command) error {
	return nil
}

func (qualityPlugin) RegisterEndpoints(
	injector do.Injector,
	registrar plugin.EndpointRegistrar,
	options ...connect.HandlerOption,
) error {
	api, err := do.Invoke[*QualityAPI](injector)
	if err != nil {
		return err
	}
	path, handler := goconductv1connect.NewQualityServiceHandler(
		api,
		options...,
	)
	return registrar.Handle(path, handler)
}
