package main

import "embed"

const (
	dashboardDocumentPath = "_resources/web/index.html"
	dashboardStylePath    = "_resources/web/app.css"
	dashboardScriptPath   = "_resources/web/app.js"
)

//go:embed _resources/web/index.html _resources/web/app.css _resources/web/app.js
var dashboardAssets embed.FS
