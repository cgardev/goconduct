package appmodule

import (
	"context"
	"errors"
	"testing"

	"github.com/samber/do/v2"
)

type failingScope struct {
	err error
}

func (scope failingScope) Injector(context.Context) (do.Injector, error) {
	return nil, scope.err
}

func TestFixedScopeResolvesServicesFromOneInjector(t *testing.T) {
	injector := do.New()
	do.ProvideValue(injector, "service")

	service, err := Resolve[string](FixedScope(injector), t.Context())
	if err != nil || service != "service" {
		t.Fatalf("resolve fixed service: service=%q, error=%v", service, err)
	}
}

func TestSelfScopeResolvesItsOwningInjector(t *testing.T) {
	injector := do.New(SelfScope())
	do.ProvideValue(injector, "service")
	scope, err := do.Invoke[ScopeResolver](injector)
	if err != nil {
		t.Fatalf("resolve scope: %v", err)
	}
	service, err := Resolve[string](scope, t.Context())
	if err != nil || service != "service" {
		t.Fatalf("resolve self-scoped service: service=%q, error=%v", service, err)
	}
}

func TestResolveReturnsScopeSelectionError(t *testing.T) {
	sentinel := errors.New("scope unavailable")
	service, err := Resolve[string](failingScope{err: sentinel}, t.Context())
	if service != "" || !errors.Is(err, sentinel) {
		t.Fatalf("resolve failing scope: service=%q, error=%v", service, err)
	}
}
