package main

import (
	"github.com/samber/do/v2"

	applicationconfiguration "github.com/cgardev/goconduct/cmd/goconduct/internal/configuration"
	qualitymodule "github.com/cgardev/goconduct/cmd/goconduct/internal/module/quality"
	"github.com/cgardev/goconduct/internal/appmodule"
	"github.com/cgardev/goconduct/pkg/plugin/architecture"
	"github.com/cgardev/goconduct/pkg/plugin/coverage"
	"github.com/cgardev/goconduct/pkg/plugin/crap"
	"github.com/cgardev/goconduct/pkg/plugin/duplication"
	"github.com/cgardev/goconduct/pkg/plugin/loc"
	"github.com/cgardev/goconduct/pkg/plugin/mutation"
)

func applicationPlugins() []appmodule.Plugin {
	return []appmodule.Plugin{
		architecture.Plugin(),
		coverage.Plugin(),
		crap.Plugin(),
		duplication.Plugin(),
		loc.Plugin(),
		mutation.Plugin(),
		qualitymodule.Plugin(),
	}
}

func provideRootServices(
	injector do.Injector,
	configuration applicationconfiguration.ApplicationConfiguration,
) {
	do.ProvideValue(injector, configuration)
	do.ProvideValue(injector, configuration.ArchitecturePluginConfiguration())
	do.ProvideValue(injector, configuration.Quality.Coverage)
	do.ProvideValue(injector, configuration.Quality.CRAP)
	do.ProvideValue(injector, configuration.Quality.Duplication)
	do.ProvideValue(injector, configuration.Quality.LOC)
	do.ProvideValue(injector, configuration.Quality.Mutation)
	check := configuration.CloneCheck()
	do.ProvideValue(injector, qualitymodule.Configuration{
		RepositoryRoot: configuration.Analysis.RepositoryRoot,
		Plugins:        check.Plugins,
		Paths:          check.Paths,
	})
}
