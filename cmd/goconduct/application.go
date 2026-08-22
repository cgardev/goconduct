package main

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"sync"

	"connectrpc.com/connect"

	applicationconfiguration "github.com/cgardev/goconduct/cmd/goconduct/internal/configuration"
	"github.com/cgardev/goconduct/failure"
	"github.com/cgardev/goconduct/internal/appmodule"
	"github.com/cgardev/goconduct/internal/library/httpserver"
)

type application struct {
	host           *appmodule.Host
	logger         *slog.Logger
	mutex          sync.Mutex
	configuration  applicationconfiguration.ApplicationConfiguration
	activated      bool
	attempted      bool
	serverComposed bool
}

func newApplication(logger *slog.Logger) (*application, error) {
	logger = cmp.Or(logger, slog.Default())
	host, err := appmodule.NewHost(newBaseServices(logger), applicationPlugins()...)
	if err != nil {
		return nil, fmt.Errorf("build plugin host: %w", err)
	}
	return &application{host: host, logger: logger}, nil
}

func (app *application) Activate(
	ctx context.Context,
	configuration applicationconfiguration.ApplicationConfiguration,
) error {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	if app.attempted {
		return failure.BusinessRule("application activation was already attempted", nil)
	}
	app.attempted = true
	if err := applicationconfiguration.Validate(configuration); err != nil {
		return fmt.Errorf("validate application configuration: %w", err)
	}
	provideRootServices(app.host.Injector(), configuration)
	if err := app.host.Activate(ctx); err != nil {
		return fmt.Errorf("activate feature plugins: %w", err)
	}
	app.configuration = configuration
	app.activated = true
	return nil
}

func (app *application) ComposeServer() (*httpserver.Server, error) {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	if !app.activated {
		return nil, failure.BusinessRule("application is not active", nil)
	}
	if app.serverComposed {
		return nil, failure.BusinessRule("application server is already composed", nil)
	}
	app.serverComposed = true
	server := newApplicationServer(app.configuration, app.logger)
	chain := buildRequestChain(app.logger)
	if err := app.host.RegisterEndpoints(
		server,
		connect.WithInterceptors(chain...),
	); err != nil {
		return nil, fmt.Errorf("register plugin endpoints: %w", err)
	}
	if err := mountHealth(server); err != nil {
		return nil, fmt.Errorf("mount health endpoint: %w", err)
	}
	return server, nil
}

func (app *application) Serve(ctx context.Context) error {
	server, err := app.ComposeServer()
	if err != nil {
		return err
	}
	return server.Run(ctx)
}

func (app *application) Shutdown(ctx context.Context) error {
	return app.host.ShutdownWithContext(ctx)
}
