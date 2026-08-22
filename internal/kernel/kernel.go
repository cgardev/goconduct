// Package kernel provides services shared by every goconduct plugin.
package kernel

import (
	"log/slog"

	"github.com/samber/do/v2"

	"github.com/cgardev/goconduct/plugin"
)

// Module contains services shared by every goconduct plugin.
var Module = do.Package(
	newLoggerInjector(),
	newCatalogInjector(),
	newCommandRunnerInjector(),
)

func newLoggerInjector() func(do.Injector) {
	return do.Lazy[*slog.Logger](func(do.Injector) (*slog.Logger, error) {
		return slog.Default(), nil
	})
}

func newCatalogInjector() func(do.Injector) {
	return do.Lazy[*plugin.Catalog](func(do.Injector) (*plugin.Catalog, error) {
		return plugin.NewCatalog(), nil
	})
}

func newCommandRunnerInjector() func(do.Injector) {
	return do.Lazy[plugin.CommandRunner](func(do.Injector) (plugin.CommandRunner, error) {
		return plugin.NewCommandRunner(), nil
	})
}
