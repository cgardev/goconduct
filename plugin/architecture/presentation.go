package architecture

import "embed"

const dashboardWebRoot = "_resources/web"

// dashboardAssets contains the compiled Angular application.
//
//go:embed all:_resources/web
var dashboardAssets embed.FS
