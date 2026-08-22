package architecture

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"

	"github.com/cgardev/goconduct/internal/library/connectlog"
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
		return newUnavailableError(fmt.Sprintf("listen on %s", address), err)
	}
	server := newHTTPServer(
		ctx,
		newDashboardHandler(
			monitor,
			logger,
			connect.WithInterceptors(
				connectlog.NewInterceptor(logger),
				validate.NewInterceptor(),
			),
		),
	)
	monitorContext, stopMonitor := context.WithCancel(ctx)
	defer stopMonitor()
	go monitor.run(monitorContext)

	serveErrors := make(chan error)
	go func() {
		serveErrors <- server.Serve(listener)
	}()
	logger.Info(
		"goconduct dashboard is ready",
		slog.String("url", "http://"+listener.Addr().String()),
		slog.String("repository", monitor.repositoryPath()),
	)

	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return newUnavailableError("serve dependency graph dashboard", err)
	case <-ctx.Done():
		shutdownContext, cancelShutdown := context.WithTimeout(
			context.Background(),
			dashboardShutdownTimeout(),
		)
		defer cancelShutdown()
		if shutdownError := server.Shutdown(shutdownContext); shutdownError != nil {
			closeError := server.Close()
			serveError := <-serveErrors
			if errors.Is(serveError, http.ErrServerClosed) {
				serveError = nil
			}
			return newUnavailableError(
				"shut down dashboard",
				errors.Join(shutdownError, closeError, serveError),
			)
		}
		serveError := <-serveErrors
		if !errors.Is(serveError, http.ErrServerClosed) {
			return newUnavailableError("serve dependency graph dashboard", serveError)
		}
		return nil
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"9d11b9d7e455f5dc71712f6154755cc19095e6dfd2d182bedfa64bda6a739136","functions":[{"id":"func/newHTTPServer","name":"newHTTPServer","line":19,"end_line":29,"hash":"561d3af6686eea242162aec5934aafa64ab4f3df0fcb52675e65117f4363a27f"},{"id":"func/dashboardShutdownTimeout","name":"dashboardShutdownTimeout","line":31,"end_line":33,"hash":"c377fd4829227d7213017c071237a2f59bdd8919a5bd5e9d315f97688ec33d6f"},{"id":"func/runDashboard","name":"runDashboard","line":35,"end_line":89,"hash":"e95ef0169392d3e2eacffeb815ab0046ec4dd7e0d08e591f754ca81973fa70fc"}]}
// mutate4go-manifest-end
