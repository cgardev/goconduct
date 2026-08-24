package quality

import (
	"testing"

	"github.com/samber/do/v2"

	"github.com/cgardev/goconduct/internal/appmodule"
	"github.com/cgardev/goconduct/pkg/plugin"
)

func TestPluginUsesStableNameAndServices(t *testing.T) {
	candidate := Plugin()
	if candidate.Name() != "quality" {
		t.Fatalf("plugin name is %q", candidate.Name())
	}
	if candidate.Services() == nil {
		t.Fatal("plugin services are nil")
	}
}

func TestPluginActivationResolvesEveryRequestService(t *testing.T) {
	candidate := Plugin()
	injector := do.New(
		func(injector do.Injector) {
			do.ProvideValue(injector, plugin.NewCatalog())
		},
		appmodule.SelfScope(),
		candidate.Services(),
	)
	if err := candidate.Activate(t.Context(), injector); err == nil {
		t.Fatal("activation succeeds without module configuration")
	}
	do.ProvideValue(injector, Configuration{})
	if err := candidate.Activate(t.Context(), injector); err != nil {
		t.Fatalf("activate configured plugin: %v", err)
	}
}
