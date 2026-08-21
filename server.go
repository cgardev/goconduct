package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type dashboardHandler struct {
	monitor           *graphMonitor
	logger            *slog.Logger
	router            *http.ServeMux
	keepAliveInterval time.Duration
}

func newDashboardHandler(monitor *graphMonitor, logger *slog.Logger) *dashboardHandler {
	handler := &dashboardHandler{
		monitor:           monitor,
		logger:            logger,
		router:            http.NewServeMux(),
		keepAliveInterval: 20 * time.Second,
	}
	handler.router.HandleFunc("GET /{$}", handler.serveDashboard)
	handler.router.HandleFunc("GET /assets/dashboard.css", handler.serveStyle)
	handler.router.HandleFunc("GET /assets/dashboard.js", handler.serveScript)
	handler.router.HandleFunc("GET /api/graph", handler.serveGraph)
	handler.router.HandleFunc("GET /api/events", handler.serveEvents)
	handler.router.HandleFunc("GET /healthz", handler.serveHealth)
	return handler
}

func (handler *dashboardHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Security-Policy", dashboardContentSecurityPolicy())
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("X-Frame-Options", "DENY")
	handler.router.ServeHTTP(response, request)
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T09:16:52Z","module_hash":"f6d254074550b4a01c22da5db4aac334f7585ccf138e57a392c6031e3ccddedb","functions":[{"id":"func/newDashboardHandler","name":"newDashboardHandler","line":21,"end_line":35,"hash":"08e8dbcea67431289267e1bbaab0a829a943eeb7ab5cef7ceda8d2d9a69ad5aa"},{"id":"func/dashboardHandler.ServeHTTP","name":"dashboardHandler.ServeHTTP","line":37,"end_line":43,"hash":"2374fbb047d6a67e7a693e2a1c1f0953efb90cfc1d6a614362acfa2f2df9862a"},{"id":"func/dashboardHandler.serveDashboard","name":"dashboardHandler.serveDashboard","line":45,"end_line":47,"hash":"83dda4230e250d7b1152e9f1fc035505caffeb3d282bc132b741671f0643546b"},{"id":"func/dashboardHandler.serveStyle","name":"dashboardHandler.serveStyle","line":49,"end_line":51,"hash":"c609a4534a3746af4592c5c6b1de5ff51a31d78967b97511d0bdc0eb58b790c0"},{"id":"func/dashboardHandler.serveScript","name":"dashboardHandler.serveScript","line":53,"end_line":55,"hash":"4fd16509b7d53e5368bd3784de04021c31e59e55d4ff047a98ff76a826a00989"},{"id":"func/dashboardHandler.serveEmbeddedAsset","name":"dashboardHandler.serveEmbeddedAsset","line":57,"end_line":73,"hash":"e80daf0219a2774250ff3bd346fa5c65aa502c5e4d41c68629640649b16f0cfa"},{"id":"func/dashboardHandler.serveGraph","name":"dashboardHandler.serveGraph","line":75,"end_line":87,"hash":"326cf30998d56d21c6b9907cdcca86c9b85b7609ab04041451a50e4e4b34fb93"},{"id":"func/dashboardHandler.serveEvents","name":"dashboardHandler.serveEvents","line":89,"end_line":125,"hash":"dcda3cfd832d47227148aef5df1e33b55334bcd8b1c98de43507c92b20f46577"},{"id":"func/writeServerEvent","name":"writeServerEvent","line":127,"end_line":132,"hash":"3f41198f8da8187b8969223241cdc8c29324d59411785db59dd212d6db0fde62"},{"id":"func/dashboardHandler.serveHealth","name":"dashboardHandler.serveHealth","line":134,"end_line":140,"hash":"d329f635419b99fe62415f10ece1cf3c3bf76e502ea1b27fe87475402a3e6195"},{"id":"func/dashboardContentSecurityPolicy","name":"dashboardContentSecurityPolicy","line":142,"end_line":146,"hash":"ca7c443f057c7b5035f43cb1d42c3ee9ff7e4f7f761596bcbbebad1f5b56c7a6"},{"id":"func/newHTTPServer","name":"newHTTPServer","line":148,"end_line":158,"hash":"561d3af6686eea242162aec5934aafa64ab4f3df0fcb52675e65117f4363a27f"},{"id":"func/dashboardShutdownTimeout","name":"dashboardShutdownTimeout","line":160,"end_line":162,"hash":"c377fd4829227d7213017c071237a2f59bdd8919a5bd5e9d315f97688ec33d6f"},{"id":"func/runDashboard","name":"runDashboard","line":164,"end_line":213,"hash":"39834d01dd9f593d7484c6f4d68e40c39f98d288e3ffb05bb40597a6bd06eef8"}]}
// mutate4go-manifest-end
