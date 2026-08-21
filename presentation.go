package main

import "embed"

const (
	dashboardDocumentPath        = "_resources/web/index.html"
	dashboardStylePath           = "_resources/web/dashboard.css"
	dashboardGraphStylePath      = "_resources/web/graph.css"
	dashboardInsightsStylePath   = "_resources/web/insights.css"
	dashboardResponsiveStylePath = "_resources/web/responsive.css"
	dashboardScriptPath          = "_resources/web/dashboard.js"
	dashboardMapsScriptPath      = "_resources/web/maps.js"
	dashboardInsightsScriptPath  = "_resources/web/insights.js"
	dashboardRuntimeScriptPath   = "_resources/web/runtime.js"
)

//go:embed _resources/web/*
var dashboardAssets embed.FS
