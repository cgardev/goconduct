package appmodule

import (
	"github.com/samber/do/v2"

	"github.com/cgardev/goconduct/plugin"
)

// Host aliases the public plugin host for application composition roots.
type Host = plugin.Host

// NewHost builds the public plugin host behind the application boundary.
func NewHost(baseServices func(do.Injector), plugins ...Plugin) (*Host, error) {
	return plugin.NewHost(baseServices, plugins...)
}
