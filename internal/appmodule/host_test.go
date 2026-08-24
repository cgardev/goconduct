package appmodule

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/pkg/plugin"
)

type hostTestPlugin struct {
	name      string
	services  func(do.Injector)
	activate  func(context.Context, do.Injector) error
	commands  func(do.Injector, *cobra.Command) error
	endpoints func(do.Injector, plugin.EndpointRegistrar) error
	options   func([]connect.HandlerOption)
}

func (candidate hostTestPlugin) Name() string { return candidate.name }

func (candidate hostTestPlugin) Services() func(do.Injector) {
	if candidate.services == nil {
		return func(do.Injector) {}
	}
	return candidate.services
}

func (candidate hostTestPlugin) Activate(ctx context.Context, injector do.Injector) error {
	if candidate.activate == nil {
		return nil
	}
	return candidate.activate(ctx, injector)
}

func (candidate hostTestPlugin) RegisterCommands(injector do.Injector, root *cobra.Command) error {
	if candidate.commands == nil {
		return nil
	}
	return candidate.commands(injector, root)
}

func (candidate hostTestPlugin) RegisterEndpoints(
	injector do.Injector,
	registrar plugin.EndpointRegistrar,
	options ...connect.HandlerOption,
) error {
	if candidate.options != nil {
		candidate.options(options)
	}
	if candidate.endpoints == nil {
		return nil
	}
	return candidate.endpoints(injector, registrar)
}

type hostBaseService struct{}
type hostSecondService struct{}

type endpointRecorder struct {
	patterns []string
}

func (recorder *endpointRecorder) Handle(pattern string, _ http.Handler) error {
	recorder.patterns = append(recorder.patterns, pattern)
	return nil
}

func TestHostComposesServicesBeforeOrderedActivation(t *testing.T) {
	order := make([]string, 0, 2)
	first := hostTestPlugin{
		name: "first",
		activate: func(_ context.Context, injector do.Injector) error {
			if _, err := do.Invoke[hostBaseService](injector); err != nil {
				return err
			}
			if _, err := do.Invoke[hostSecondService](injector); err != nil {
				return err
			}
			order = append(order, "first")
			return nil
		},
	}
	second := hostTestPlugin{
		name: "second",
		services: func(injector do.Injector) {
			do.ProvideValue(injector, hostSecondService{})
		},
		activate: func(context.Context, do.Injector) error {
			order = append(order, "second")
			return nil
		},
	}
	host, err := NewHost(func(injector do.Injector) {
		do.ProvideValue(injector, hostBaseService{})
	}, first, second)
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
}

func TestHostRegistersCommandsAndEndpoints(t *testing.T) {
	receivedOptions := 0
	candidate := hostTestPlugin{
		name: "coverage",
		commands: func(_ do.Injector, root *cobra.Command) error {
			root.AddCommand(&cobra.Command{Use: "coverage"})
			return nil
		},
		endpoints: func(_ do.Injector, registrar plugin.EndpointRegistrar) error {
			return registrar.Handle("/coverage.v1.CoverageService/", http.NotFoundHandler())
		},
		options: func(options []connect.HandlerOption) {
			receivedOptions = len(options)
		},
	}
	host, err := NewHost(func(do.Injector) {}, candidate)
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	t.Cleanup(func() {
		if err := host.Shutdown(); err != nil {
			t.Errorf("shut down host: %v", err)
		}
	})
	root := &cobra.Command{Use: "goconduct"}
	if err := host.RegisterCommands(root); err != nil {
		t.Fatalf("register commands: %v", err)
	}
	if command, _, err := root.Find([]string{"coverage"}); err != nil || command.Name() != "coverage" {
		t.Fatalf("find coverage command: command=%v, error=%v", command, err)
	}
	recorder := &endpointRecorder{}
	if err := host.RegisterEndpoints(recorder, connect.WithInterceptors()); err != nil {
		t.Fatalf("register endpoints: %v", err)
	}
	if !reflect.DeepEqual(recorder.patterns, []string{"/coverage.v1.CoverageService/"}) {
		t.Fatalf("endpoint patterns are %v", recorder.patterns)
	}
	if receivedOptions != 1 {
		t.Fatalf("received handler option count is %d", receivedOptions)
	}
}

func TestHostRejectsInvalidPluginsAndNamesLifecycleErrors(t *testing.T) {
	if _, err := NewHost(nil); err == nil {
		t.Fatal("expected nil base services error")
	}
	if _, err := NewHost(func(do.Injector) {}, nil); err == nil {
		t.Fatal("expected nil plugin error")
	}
	if _, err := NewHost(func(do.Injector) {}, hostTestPlugin{name: " duplicate "}); err == nil {
		t.Fatal("expected invalid name error")
	}
	if _, err := NewHost(
		func(do.Injector) {},
		hostTestPlugin{name: "duplicate"},
		hostTestPlugin{name: "duplicate"},
	); err == nil {
		t.Fatal("expected duplicate name error")
	}
	sentinel := errors.New("activation failed")
	host, err := NewHost(func(do.Injector) {}, hostTestPlugin{
		name: "coverage",
		activate: func(context.Context, do.Injector) error {
			return sentinel
		},
	})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	t.Cleanup(func() {
		if err := host.Shutdown(); err != nil {
			t.Errorf("shut down host: %v", err)
		}
	})
	err = host.Activate(t.Context())
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), `activate plugin "coverage"`) {
		t.Fatalf("unexpected activation error: %v", err)
	}
}
