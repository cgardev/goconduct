package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

const (
	dashboardDocumentPath = "_resources/web/index.html"
	dashboardStylePath    = "_resources/web/app.css"
	dashboardScriptPath   = "_resources/web/app.js"
)

//go:embed _resources/web/index.html _resources/web/app.css _resources/web/app.js
var dashboardAssets embed.FS

type dashboardHandler struct {
	monitor           *graphMonitor
	logger            *slog.Logger
	mux               *http.ServeMux
	keepAliveInterval time.Duration
}

func newDashboardHandler(monitor *graphMonitor, logger *slog.Logger) *dashboardHandler {
	handler := &dashboardHandler{
		monitor:           monitor,
		logger:            logger,
		mux:               http.NewServeMux(),
		keepAliveInterval: 20 * time.Second,
	}
	handler.mux.HandleFunc("GET /{$}", handler.serveDashboard)
	handler.mux.HandleFunc("GET /assets/app.css", handler.serveStyle)
	handler.mux.HandleFunc("GET /assets/app.js", handler.serveScript)
	handler.mux.HandleFunc("GET /api/graph", handler.serveGraph)
	handler.mux.HandleFunc("GET /api/events", handler.serveEvents)
	handler.mux.HandleFunc("GET /healthz", handler.serveHealth)
	return handler
}

func (handler *dashboardHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Security-Policy", dashboardContentSecurityPolicy())
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("X-Frame-Options", "DENY")
	handler.mux.ServeHTTP(response, request)
}

func (handler *dashboardHandler) serveDashboard(response http.ResponseWriter, _ *http.Request) {
	handler.serveEmbeddedAsset(response, dashboardDocumentPath, "text/html; charset=utf-8")
}

func (handler *dashboardHandler) serveStyle(response http.ResponseWriter, _ *http.Request) {
	handler.serveEmbeddedAsset(response, dashboardStylePath, "text/css; charset=utf-8")
}

func (handler *dashboardHandler) serveScript(response http.ResponseWriter, _ *http.Request) {
	handler.serveEmbeddedAsset(response, dashboardScriptPath, "text/javascript; charset=utf-8")
}

func (handler *dashboardHandler) serveEmbeddedAsset(
	response http.ResponseWriter,
	path string,
	contentType string,
) {
	payload, err := dashboardAssets.ReadFile(path)
	if err != nil {
		handler.logger.Error("Cannot read embedded dashboard asset", "path", path, "error", err)
		http.Error(response, "embedded asset unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Content-Type", contentType)
	if _, err := response.Write(payload); err != nil {
		handler.logger.Debug("Cannot write dashboard asset", "path", path, "error", err)
	}
}

func (handler *dashboardHandler) serveGraph(response http.ResponseWriter, _ *http.Request) {
	payload, err := json.Marshal(handler.monitor.currentGraph())
	if err != nil {
		handler.logger.Error("Cannot encode dependency graph", "error", err)
		http.Error(response, "dependency graph unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := response.Write(payload); err != nil {
		handler.logger.Debug("Cannot write dependency graph", "error", err)
	}
}

func (handler *dashboardHandler) serveEvents(response http.ResponseWriter, request *http.Request) {
	flusher, supported := response.(http.Flusher)
	if !supported {
		http.Error(response, "event streaming unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("X-Accel-Buffering", "no")

	updates, unsubscribe := handler.monitor.subscribe()
	defer unsubscribe()
	if err := writeServerEvent(response, "ready", handler.monitor.currentGraph().Revision); err != nil {
		return
	}
	flusher.Flush()

	keepAlive := time.NewTicker(handler.keepAliveInterval)
	defer keepAlive.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case revision := <-updates:
			if err := writeServerEvent(response, "graph", revision); err != nil {
				return
			}
			flusher.Flush()
		case <-keepAlive.C:
			if _, err := fmt.Fprint(response, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeServerEvent(response http.ResponseWriter, event, data string) error {
	if _, err := fmt.Fprintf(response, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return fmt.Errorf("write server event: %w", err)
	}
	return nil
}

func (handler *dashboardHandler) serveHealth(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := response.Write([]byte("ok\n")); err != nil {
		handler.logger.Debug("Cannot write health response", "error", err)
	}
}

func dashboardContentSecurityPolicy() string {
	return "default-src 'self'; base-uri 'none'; connect-src 'self'; font-src 'self'; " +
		"form-action 'none'; frame-ancestors 'none'; img-src 'self' data:; object-src 'none'; " +
		"script-src 'self'; style-src 'self'"
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
	monitor *graphMonitor,
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
		"repository", monitor.analyzer.repositoryRoot,
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
