package main

import (
	"cmp"
	"log/slog"
	"net/http"

	"github.com/samber/do/v2"

	applicationconfiguration "github.com/cgardev/goconduct/cmd/goconduct/internal/configuration"
	"github.com/cgardev/goconduct/internal/appmodule"
	"github.com/cgardev/goconduct/internal/kernel"
	"github.com/cgardev/goconduct/internal/library/httpserver"
)

func newBaseServices(logger *slog.Logger) func(do.Injector) {
	logger = cmp.Or(logger, slog.Default())
	return func(injector do.Injector) {
		kernel.Module(injector)
		appmodule.SelfScope()(injector)
		do.OverrideValue(injector, logger)
	}
}

func newApplicationServer(
	configuration applicationconfiguration.ApplicationConfiguration,
	logger *slog.Logger,
) *httpserver.Server {
	return httpserver.New(httpserver.Configuration{
		Address: configuration.Server.Address,
		Logger:  logger,
	})
}

func mountHealth(server *httpserver.Server) error {
	return server.Handle("GET /healthz", http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		response.WriteHeader(http.StatusNoContent)
	}))
}
