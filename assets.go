package main

import (
	"log/slog"
	"net/http"
)

type dashboardAsset struct {
	requestPath  string
	embeddedPath string
	contentType  string
}

type dashboardAssetHandler struct {
	logger *slog.Logger
}

func newDashboardAssetHandler(logger *slog.Logger) *dashboardAssetHandler {
	return &dashboardAssetHandler{logger: logger}
}

func dashboardAssetDefinitions() []dashboardAsset {
	return []dashboardAsset{
		{
			requestPath:  "/assets/dashboard.css",
			embeddedPath: dashboardStylePath,
			contentType:  "text/css; charset=utf-8",
		},
		{
			requestPath:  "/assets/graph.css",
			embeddedPath: dashboardGraphStylePath,
			contentType:  "text/css; charset=utf-8",
		},
		{
			requestPath:  "/assets/insights.css",
			embeddedPath: dashboardInsightsStylePath,
			contentType:  "text/css; charset=utf-8",
		},
		{
			requestPath:  "/assets/responsive.css",
			embeddedPath: dashboardResponsiveStylePath,
			contentType:  "text/css; charset=utf-8",
		},
		{
			requestPath:  "/assets/dashboard.js",
			embeddedPath: dashboardScriptPath,
			contentType:  "text/javascript; charset=utf-8",
		},
		{
			requestPath:  "/assets/maps.js",
			embeddedPath: dashboardMapsScriptPath,
			contentType:  "text/javascript; charset=utf-8",
		},
		{
			requestPath:  "/assets/insights.js",
			embeddedPath: dashboardInsightsScriptPath,
			contentType:  "text/javascript; charset=utf-8",
		},
		{
			requestPath:  "/assets/runtime.js",
			embeddedPath: dashboardRuntimeScriptPath,
			contentType:  "text/javascript; charset=utf-8",
		},
	}
}

func (handler *dashboardAssetHandler) serve(asset dashboardAsset) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		handler.serveAsset(response, asset.embeddedPath, asset.contentType)
	}
}

func (handler *dashboardAssetHandler) serveAsset(
	response http.ResponseWriter,
	path string,
	contentType string,
) {
	payload, err := dashboardAssets.ReadFile(path)
	if err != nil {
		handler.logger.Error(
			"The dashboard asset handler cannot read the embedded asset.",
			slog.String("path", path),
			slog.Any("error", err),
		)
		http.Error(response, "embedded asset unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Content-Type", contentType)
	if _, err := response.Write(payload); err != nil {
		handler.logger.Debug(
			"The dashboard asset handler cannot write the asset.",
			slog.String("path", path),
			slog.Any("error", err),
		)
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"bf64e4dd25481b246d7c1776e9caa9735ea2b748d9e7a17158370be801d51482","functions":[{"id":"func/newDashboardAssetHandler","name":"newDashboardAssetHandler","line":18,"end_line":20,"hash":"dccfdb5364fb2829ea3113c62de2adbbd596bf1275f6bf75a92233daf641a52a"},{"id":"func/dashboardAssetDefinitions","name":"dashboardAssetDefinitions","line":22,"end_line":65,"hash":"71be4a653e047c031aa4e3c8b70ef513794aebcb88b3b131b33f1bfae193efbd"},{"id":"func/dashboardAssetHandler.serve","name":"dashboardAssetHandler.serve","line":67,"end_line":71,"hash":"7e1b5d055ecdb04c54f611ab60d0ac82daf22e3083abbcc48e66343ad804f77c"},{"id":"func/dashboardAssetHandler.serveAsset","name":"dashboardAssetHandler.serveAsset","line":73,"end_line":97,"hash":"fdb595c60205deade29b3a579d3f2d8225a6eb7bf83ae6cdf419bad9ec03864e"}]}
// mutate4go-manifest-end
