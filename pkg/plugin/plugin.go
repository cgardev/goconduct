// Package plugin defines the stable extension contract for goconduct plugins.
// A plugin remains a normal Go package. Consumers can use its evaluator
// directly or compose its lifecycle adapter through the application host.
package plugin

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
)

// EndpointRegistrar mounts HTTP handlers without exposing the host server.
type EndpointRegistrar interface {
	Handle(pattern string, handler http.Handler) error
}

// Plugin is one statically linked goconduct extension.
type Plugin interface {
	// Name returns the stable package identifier.
	Name() string

	// Services returns the plugin dependency registrations.
	Services() func(do.Injector)

	// Activate starts the plugin after all services are available.
	Activate(ctx context.Context, injector do.Injector) error

	// RegisterCommands adds the plugin command surface to the root command.
	RegisterCommands(injector do.Injector, root *cobra.Command) error

	// RegisterEndpoints mounts the plugin HTTP surface with the host's shared
	// Connect options. A plugin passes those options to every generated handler.
	RegisterEndpoints(
		injector do.Injector,
		registrar EndpointRegistrar,
		options ...connect.HandlerOption,
	) error
}

// Evaluator produces deterministic evidence for one quality capability.
type Evaluator interface {
	Name() string
	Evaluate(ctx context.Context, request Request) (Report, error)
}
