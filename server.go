package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type graphReader interface {
	currentGraph() Graph
}

type graphRefresher interface {
	freshGraph(ctx context.Context) (Graph, error)
}

type graphSubscriber interface {
	subscribe() (<-chan string, func())
}

type graphCacheKeyReader interface {
	graphCacheKey() (string, error)
}

type dashboardGraphSource interface {
	graphReader
	graphRefresher
	graphSubscriber
	graphCacheKeyReader
}

type dashboardHandler struct {
	assets            *dashboardAssetHandler
	graph             *graphEndpoint
	events            *graphEventStream
	logger            *slog.Logger
	router            *http.ServeMux
	keepAliveInterval time.Duration
}

func newDashboardHandler(source dashboardGraphSource, logger *slog.Logger) *dashboardHandler {
	handler := &dashboardHandler{
		assets:            newDashboardAssetHandler(logger),
		graph:             newGraphEndpoint(source, source, source, logger),
		events:            newGraphEventStream(source, source),
		logger:            logger,
		router:            http.NewServeMux(),
		keepAliveInterval: 20 * time.Second,
	}
	handler.router.HandleFunc("GET /{$}", handler.serveDashboard)
	for _, asset := range dashboardAssetDefinitions() {
		handler.router.HandleFunc("GET "+asset.requestPath, handler.assets.serve(asset))
	}
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
	handler.assets.serveAsset(response, dashboardDocumentPath, "text/html; charset=utf-8")
}

func (handler *dashboardHandler) serveGraph(response http.ResponseWriter, request *http.Request) {
	handler.graph.serve(response, request)
}

func (handler *dashboardHandler) serveEvents(response http.ResponseWriter, request *http.Request) {
	handler.events.serve(response, request, handler.keepAliveInterval)
}

func (handler *dashboardHandler) serveHealth(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := response.Write([]byte("ok\n")); err != nil {
		handler.logger.Debug("The dashboard handler cannot write the health response.", "error", err)
	}
}

func dashboardContentSecurityPolicy() string {
	return "default-src 'self'; base-uri 'none'; connect-src 'self'; font-src 'self'; " +
		"form-action 'none'; frame-ancestors 'none'; img-src 'self' data:; object-src 'none'; " +
		"script-src 'self'; style-src 'self'"
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T15:57:52Z","module_hash":"aacf9bfd3f552e05747566e3490006d70856cd37dc92473ca3b0dc2506058828","functions":[{"id":"func/newDashboardHandler","name":"newDashboardHandler","line":42,"end_line":59,"hash":"bbb38d1e5b0ede416e702196f9d9e446bd3047f84afaf268b48be758ab861541"},{"id":"func/dashboardHandler.ServeHTTP","name":"dashboardHandler.ServeHTTP","line":61,"end_line":67,"hash":"2374fbb047d6a67e7a693e2a1c1f0953efb90cfc1d6a614362acfa2f2df9862a"},{"id":"func/dashboardHandler.serveDashboard","name":"dashboardHandler.serveDashboard","line":69,"end_line":71,"hash":"f4810338eaa1f719702af16b45c107419c0cf96fb690c0a3a48fa8891ad70344"},{"id":"func/dashboardHandler.serveGraph","name":"dashboardHandler.serveGraph","line":73,"end_line":75,"hash":"0fa01b9a2d7f4268bf303422e6221daf8ad0eaa5fa0d94cabc8707650741a31c"},{"id":"func/dashboardHandler.serveEvents","name":"dashboardHandler.serveEvents","line":77,"end_line":79,"hash":"1ec3b46f51646a537ac3de423a68f61321ebaa5d04ff7ffa06c6deb642e99cc2"},{"id":"func/dashboardHandler.serveHealth","name":"dashboardHandler.serveHealth","line":81,"end_line":87,"hash":"93ce4a74aa6688b6d561e724422bc0ae989be1f9d12e3b805850e953a9edbb0c"},{"id":"func/dashboardContentSecurityPolicy","name":"dashboardContentSecurityPolicy","line":89,"end_line":93,"hash":"ca7c443f057c7b5035f43cb1d42c3ee9ff7e4f7f761596bcbbebad1f5b56c7a6"}]}
// mutate4go-manifest-end
