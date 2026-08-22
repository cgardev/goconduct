package plugin

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
)

type hostFixturePlugin struct {
	name     string
	services func(do.Injector)
	activate func(context.Context, do.Injector) error
	options  int
}

func (candidate *hostFixturePlugin) Name() string { return candidate.name }

func (candidate *hostFixturePlugin) Services() func(do.Injector) {
	return candidate.services
}

func (candidate *hostFixturePlugin) Activate(ctx context.Context, injector do.Injector) error {
	return candidate.activate(ctx, injector)
}

func (*hostFixturePlugin) RegisterCommands(do.Injector, *cobra.Command) error { return nil }

func (candidate *hostFixturePlugin) RegisterEndpoints(
	_ do.Injector,
	_ EndpointRegistrar,
	options ...connect.HandlerOption,
) error {
	candidate.options = len(options)
	return nil
}

type hostFixtureService struct{}

type hostFixtureRegistrar struct{}

func (hostFixtureRegistrar) Handle(string, http.Handler) error { return nil }

func TestHostBuildsServicesBeforeOrderedActivation(t *testing.T) {
	order := make([]string, 0, 2)
	first := &hostFixturePlugin{
		name:     "first",
		services: func(do.Injector) {},
		activate: func(_ context.Context, injector do.Injector) error {
			if _, err := do.Invoke[hostFixtureService](injector); err != nil {
				return err
			}
			order = append(order, "first")
			return nil
		},
	}
	second := &hostFixturePlugin{
		name: "second",
		services: func(injector do.Injector) {
			do.ProvideValue(injector, hostFixtureService{})
		},
		activate: func(context.Context, do.Injector) error {
			order = append(order, "second")
			return nil
		},
	}
	host, err := NewHost(func(do.Injector) {}, first, second)
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	t.Cleanup(func() {
		if err := host.Shutdown(); err != nil {
			t.Errorf("shut down host: %v", err)
		}
	})
	if err := host.Activate(t.Context()); err != nil {
		t.Fatalf("activate host: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"first", "second"}) {
		t.Fatalf("activation order is %v", order)
	}
	if err := host.RegisterEndpoints(hostFixtureRegistrar{}, connect.WithInterceptors()); err != nil {
		t.Fatalf("register endpoints: %v", err)
	}
	if first.options != 1 || second.options != 1 {
		t.Fatalf("handler option counts are %d and %d", first.options, second.options)
	}
}

func TestHostRejectsInvalidRegistryAndNamesActivationErrors(t *testing.T) {
	if _, err := NewHost(nil); err == nil {
		t.Fatal("nil base services succeed")
	}
	if _, err := NewHost(func(do.Injector) {}, nil); err == nil {
		t.Fatal("nil plugin succeeds")
	}
	if _, err := NewHost(
		func(do.Injector) {},
		&hostFixturePlugin{name: "same", services: func(do.Injector) {}},
		&hostFixturePlugin{name: "same", services: func(do.Injector) {}},
	); err == nil {
		t.Fatal("duplicate plugin names succeed")
	}
	sentinel := errors.New("activation failed")
	host, err := NewHost(func(do.Injector) {}, &hostFixturePlugin{
		name:     "broken",
		services: func(do.Injector) {},
		activate: func(context.Context, do.Injector) error { return sentinel },
	})
	if err != nil {
		t.Fatalf("create failing host: %v", err)
	}
	if err := host.Activate(t.Context()); !errors.Is(err, sentinel) {
		t.Fatalf("activation error is %v", err)
	}
}
