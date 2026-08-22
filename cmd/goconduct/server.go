package main

import (
	"log/slog"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	applicationconfiguration "github.com/cgardev/goconduct/cmd/goconduct/internal/configuration"
	"github.com/cgardev/goconduct/internal/appmodule"
	"github.com/cgardev/goconduct/internal/library/httpserver"
)

func configureApplicationServer(host *appmodule.Host, root *cobra.Command) {
	root.Short = "Apply deterministic quality rules to Go software."
	root.Long = "goconduct composes deterministic Go quality plugins and serves their Protocol Buffer APIs."
	root.Args = cobra.NoArgs
	root.RunE = func(command *cobra.Command, _ []string) error {
		configuration, err := do.Invoke[applicationconfiguration.ApplicationConfiguration](host.Injector())
		if err != nil {
			return err
		}
		logger, err := do.Invoke[*slog.Logger](host.Injector())
		if err != nil {
			return err
		}
		server := httpserver.New(httpserver.Configuration{
			Address: configuration.Server.Address,
			Logger:  logger,
		})
		if err := host.RegisterEndpoints(server); err != nil {
			return err
		}
		return server.Run(command.Context())
	}
}
