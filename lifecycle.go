package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type dashboardMonitor interface {
	dashboardGraphSource
	run(context.Context)
	repositoryPath() string
}

func newHTTPServer(ctx context.Context, handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       75 * time.Second,
		MaxHeaderBytes:    1 << 20,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
}

func dashboardShutdownTimeout() time.Duration {
	return 5 * time.Second
}

func runDashboard(
	ctx context.Context,
	monitor dashboardMonitor,
	address string,
	logger *slog.Logger,
) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	server := newHTTPServer(ctx, newDashboardHandler(monitor, logger))
	monitorContext, stopMonitor := context.WithCancel(ctx)
	defer stopMonitor()
	go monitor.run(monitorContext)

	serveErrors := make(chan error)
	go func() {
		serveErrors <- server.Serve(listener)
	}()
	logger.Info(
		"Dependency graph dashboard is ready",
		"url", "http://"+listener.Addr().String(),
		"repository", monitor.repositoryPath(),
	)

	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve dependency graph dashboard: %w", err)
	case <-ctx.Done():
		shutdownContext, cancelShutdown := context.WithTimeout(
			context.Background(),
			dashboardShutdownTimeout(),
		)
		defer cancelShutdown()
		if err := server.Shutdown(shutdownContext); err != nil {
			if closeError := server.Close(); closeError != nil {
				return fmt.Errorf("shut down dashboard: %w", errors.Join(err, closeError))
			}
			return fmt.Errorf("shut down dashboard: %w", err)
		}
		serveError := <-serveErrors
		if !errors.Is(serveError, http.ErrServerClosed) {
			return fmt.Errorf("serve dependency graph dashboard: %w", serveError)
		}
		return nil
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T15:04:53Z","module_hash":"856c91345493c5ef6163fc4e0d8487b4e64de7e05faaeb30d0b26044ea9010ec","functions":[{"id":"func/newHTTPServer","name":"newHTTPServer","line":19,"end_line":29,"hash":"561d3af6686eea242162aec5934aafa64ab4f3df0fcb52675e65117f4363a27f"},{"id":"func/dashboardShutdownTimeout","name":"dashboardShutdownTimeout","line":31,"end_line":33,"hash":"c377fd4829227d7213017c071237a2f59bdd8919a5bd5e9d315f97688ec33d6f"},{"id":"func/runDashboard","name":"runDashboard","line":35,"end_line":84,"hash":"9075092798d4624ba53dd83836020218df6025264513eaa5174077a3c833bc2a"}]}
// mutate4go-manifest-end
