// Package httpserver hosts Connect and web handlers for the application.
package httpserver

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cgardev/goconduct/pkg/failure"
)

const shutdownTimeout = 15 * time.Second

// Configuration defines the HTTP listener.
type Configuration struct {
	Address string
	Logger  *slog.Logger
}

// Server owns the shared application HTTP router.
type Server struct {
	address   string
	logger    *slog.Logger
	router    *http.ServeMux
	mutex     sync.Mutex
	patterns  map[string]struct{}
	started   bool
	shutdowns []func()
}

// New creates an unstarted server.
func New(configuration Configuration) *Server {
	return &Server{
		address:  configuration.Address,
		logger:   cmp.Or(configuration.Logger, slog.Default()),
		router:   http.NewServeMux(),
		patterns: make(map[string]struct{}),
	}
}

// Handle registers one handler before the server starts.
func (server *Server) Handle(pattern string, handler http.Handler) error {
	if strings.TrimSpace(pattern) == "" {
		return failure.Validation("HTTP handler pattern is empty", nil)
	}
	if handler == nil {
		return failure.Validation(fmt.Sprintf("HTTP handler for %q is nil", pattern), nil)
	}
	server.mutex.Lock()
	defer server.mutex.Unlock()
	if server.started {
		return failure.BusinessRule("HTTP server has already started", nil)
	}
	if _, duplicate := server.patterns[pattern]; duplicate {
		return failure.Duplicate("HTTP handler pattern", pattern, nil)
	}
	server.router.Handle(pattern, handler)
	server.patterns[pattern] = struct{}{}
	return nil
}

// OnShutdown registers a hook before the server starts.
func (server *Server) OnShutdown(hook func()) error {
	if hook == nil {
		return failure.Validation("HTTP shutdown hook is nil", nil)
	}
	server.mutex.Lock()
	defer server.mutex.Unlock()
	if server.started {
		return failure.BusinessRule("HTTP server has already started", nil)
	}
	server.shutdowns = append(server.shutdowns, hook)
	return nil
}

// Handler returns the registered router for in-process tests.
func (server *Server) Handler() http.Handler {
	return server.router
}

// Run binds the configured address and serves until cancellation.
func (server *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", server.address)
	if err != nil {
		return failure.Unavailable(fmt.Sprintf("listen on %q", server.address), err)
	}
	return server.Serve(ctx, listener)
}

// Serve owns an existing listener and serves until cancellation.
func (server *Server) Serve(ctx context.Context, listener net.Listener) error {
	if listener == nil {
		return failure.Validation("HTTP listener is nil", nil)
	}
	server.mutex.Lock()
	if server.started {
		server.mutex.Unlock()
		return failure.BusinessRule("HTTP server has already started", nil)
	}
	server.started = true
	shutdowns := append([]func(){}, server.shutdowns...)
	server.mutex.Unlock()

	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	httpServer := &http.Server{
		Handler:           server.router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       75 * time.Second,
		MaxHeaderBytes:    1 << 20,
		BaseContext: func(net.Listener) context.Context {
			return context.WithoutCancel(ctx)
		},
		Protocols: &protocols,
	}
	for _, hook := range shutdowns {
		httpServer.RegisterOnShutdown(hook)
	}
	serveErrors := make(chan error, 1)
	go func() {
		server.logger.Info("HTTP server started", slog.String("address", listener.Addr().String()))
		serveErrors <- httpServer.Serve(listener)
	}()
	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return failure.Unavailable("serve HTTP", err)
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownContext); err != nil {
			closeError := httpServer.Close()
			return failure.Unavailable("shut down HTTP server", errors.Join(err, closeError))
		}
		serveError := <-serveErrors
		if !errors.Is(serveError, http.ErrServerClosed) {
			return failure.Unavailable("serve HTTP", serveError)
		}
		return nil
	}
}
