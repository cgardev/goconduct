package main

import "embed"

const (
	dashboardDocumentPath = "_resources/web/index.html"
	dashboardStylePath    = "_resources/web/dashboard.css"
	dashboardScriptPath   = "_resources/web/dashboard.js"
)

//go:embed _resources/web/index.html _resources/web/dashboard.css _resources/web/dashboard.js
var dashboardAssets embed.FS
