// Package appmodule composes public plugins inside one application process.
package appmodule

import (
	"context"

	"github.com/samber/do/v2"

	"github.com/cgardev/goconduct/plugin"
)

// ScopeResolver selects the dependency graph for one request.
type ScopeResolver interface {
	Injector(ctx context.Context) (do.Injector, error)
}

type fixedScope struct {
	injector do.Injector
}

// FixedScope creates a resolver for one application injector.
func FixedScope(injector do.Injector) ScopeResolver {
	return fixedScope{injector: injector}
}

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

// Plugin aliases the public contract for composition roots.
type Plugin = plugin.Plugin
