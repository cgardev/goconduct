// Package kernel provides services shared by every goconduct plugin.
package kernel

import (
	"cmp"
	"log/slog"

	"github.com/samber/do/v2"

	"github.com/cgardev/goconduct/internal/appmodule"
	"github.com/cgardev/goconduct/plugin"
)

// Module returns the shared dependency registrations.
func Module(logger *slog.Logger) func(do.Injector) {
	return do.Package(
		func(injector do.Injector) {
			do.ProvideValue(injector, cmp.Or(logger, slog.Default()))
		},
		do.Lazy[*plugin.Catalog](func(do.Injector) (*plugin.Catalog, error) {
			return plugin.NewCatalog(), nil
		}),
		do.Lazy[plugin.CommandRunner](func(do.Injector) (plugin.CommandRunner, error) {
			return plugin.NewCommandRunner(), nil
		}),
		appmodule.SelfScope(),
	)
}
