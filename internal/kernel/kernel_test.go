package kernel

import (
	"io"
	"log/slog"
	"testing"

	"github.com/samber/do/v2"

	"github.com/cgardev/goconduct/internal/appmodule"
	"github.com/cgardev/goconduct/plugin"
)

func TestModuleProvidesSharedServices(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	injector := do.New(Module(logger))
	t.Cleanup(func() {
		if report := injector.Shutdown(); report != nil && !report.Succeed {
			t.Errorf("shut down injector: %s", report.Error())
		}
	})
	resolvedLogger, err := do.Invoke[*slog.Logger](injector)
	if err != nil || resolvedLogger != logger {
		t.Fatalf("resolve logger: logger=%v, error=%v", resolvedLogger, err)
	}
	if _, err := do.Invoke[*plugin.Catalog](injector); err != nil {
		t.Fatalf("resolve catalog: %v", err)
	}
	if _, err := do.Invoke[plugin.CommandRunner](injector); err != nil {
		t.Fatalf("resolve command runner: %v", err)
	}
	scope, err := do.Invoke[appmodule.ScopeResolver](injector)
	if err != nil {
		t.Fatalf("resolve scope: %v", err)
	}
	resolvedCatalog, err := appmodule.Resolve[*plugin.Catalog](scope, t.Context())
	if err != nil || resolvedCatalog == nil {
		t.Fatalf("resolve scoped catalog: catalog=%v, error=%v", resolvedCatalog, err)
	}
}
