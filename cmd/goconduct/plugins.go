package main

import (
	"github.com/cgardev/goconduct/cmd/goconduct/internal/module/quality"
	"github.com/cgardev/goconduct/plugin"
	"github.com/cgardev/goconduct/plugin/architecture"
	"github.com/cgardev/goconduct/plugin/coverage"
	"github.com/cgardev/goconduct/plugin/crap"
	"github.com/cgardev/goconduct/plugin/duplication"
	"github.com/cgardev/goconduct/plugin/mutation"
)

func builtInPlugins() []plugin.Plugin {
	return []plugin.Plugin{
		architecture.Plugin(),
		coverage.Plugin(),
		crap.Plugin(),
		duplication.Plugin(),
		mutation.Plugin(),
		quality.Plugin(),
	}
}
