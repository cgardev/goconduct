// Package appmodule adapts public plugins to one application composition root.
// Host registers every service before activation and owns service shutdown.
// Connect handlers resolve use cases from the scope serving each request.
package appmodule

import (
	"context"

	"github.com/samber/do/v2"

	"github.com/cgardev/goconduct/pkg/plugin"
)

// ScopeResolver selects the dependency graph for one request.
// A single-scope application always returns one injector.
// A multi-scope application can select an isolated injector from the context.
type ScopeResolver interface {
	// Injector returns the dependency graph serving ctx.
	Injector(ctx context.Context) (do.Injector, error)
}

type fixedScope struct {
	injector do.Injector
}

// FixedScope creates a resolver for one application injector.
func FixedScope(injector do.Injector) ScopeResolver {
	return fixedScope{injector: injector}
}

// Injector returns the fixed application injector.
func (scope fixedScope) Injector(context.Context) (do.Injector, error) {
	return scope.injector, nil
}

// SelfScope registers a resolver that returns its owning injector.
func SelfScope() func(do.Injector) {
	return do.Lazy[ScopeResolver](func(injector do.Injector) (ScopeResolver, error) {
		return FixedScope(injector), nil
	})
}

// Resolve loads a service from the request scope.
func Resolve[Service any](scopes ScopeResolver, ctx context.Context) (Service, error) {
	injector, err := scopes.Injector(ctx)
	if err != nil {
		var zero Service
		return zero, err
	}
	return do.Invoke[Service](injector)
}

// Plugin aliases the public extension contract for composition roots.
type Plugin = plugin.Plugin
