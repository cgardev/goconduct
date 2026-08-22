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

var _ http.Handler = (*dashboardHandler)(nil)

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
		handler.logger.Debug(
			"The dashboard handler cannot write the health response.",
			slog.Any("error", err),
		)
	}
}

func dashboardContentSecurityPolicy() string {
	return "default-src 'self'; base-uri 'none'; connect-src 'self'; font-src 'self'; " +
		"form-action 'none'; frame-ancestors 'none'; img-src 'self' data:; object-src 'none'; " +
		"script-src 'self'; style-src 'self'"
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T18:32:05Z","module_hash":"54357e2fe968da827af83aad89a2e574f8b71662b8a5f0405d4cae58ded92c9f","functions":[{"id":"func/newDashboardHandler","name":"newDashboardHandler","line":44,"end_line":61,"hash":"bbb38d1e5b0ede416e702196f9d9e446bd3047f84afaf268b48be758ab861541"},{"id":"func/dashboardHandler.ServeHTTP","name":"dashboardHandler.ServeHTTP","line":63,"end_line":69,"hash":"2374fbb047d6a67e7a693e2a1c1f0953efb90cfc1d6a614362acfa2f2df9862a"},{"id":"func/dashboardHandler.serveDashboard","name":"dashboardHandler.serveDashboard","line":71,"end_line":73,"hash":"f4810338eaa1f719702af16b45c107419c0cf96fb690c0a3a48fa8891ad70344"},{"id":"func/dashboardHandler.serveGraph","name":"dashboardHandler.serveGraph","line":75,"end_line":77,"hash":"0fa01b9a2d7f4268bf303422e6221daf8ad0eaa5fa0d94cabc8707650741a31c"},{"id":"func/dashboardHandler.serveEvents","name":"dashboardHandler.serveEvents","line":79,"end_line":81,"hash":"1ec3b46f51646a537ac3de423a68f61321ebaa5d04ff7ffa06c6deb642e99cc2"},{"id":"func/dashboardHandler.serveHealth","name":"dashboardHandler.serveHealth","line":83,"end_line":92,"hash":"610d1b347538e022516fd2c1ccc4d117b4ab9e40f320916df9eba8934c53c9d7"},{"id":"func/dashboardContentSecurityPolicy","name":"dashboardContentSecurityPolicy","line":94,"end_line":98,"hash":"ca7c443f057c7b5035f43cb1d42c3ee9ff7e4f7f761596bcbbebad1f5b56c7a6"}]}
// mutate4go-manifest-end
