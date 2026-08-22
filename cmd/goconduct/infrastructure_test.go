package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/samber/do/v2"

	"github.com/cgardev/goconduct/internal/appmodule"
	"github.com/cgardev/goconduct/plugin"
)

func TestBaseServicesComposeKernelAndApplicationScope(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	injector := do.New(newBaseServices(logger))
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
	scopes, err := do.Invoke[appmodule.ScopeResolver](injector)
	if err != nil {
		t.Fatalf("resolve application scope: %v", err)
	}
	resolvedCatalog, err := appmodule.Resolve[*plugin.Catalog](scopes, t.Context())
	if err != nil || resolvedCatalog == nil {
		t.Fatalf("resolve scoped catalog: catalog=%v, error=%v", resolvedCatalog, err)
	}
}
