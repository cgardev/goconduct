package architecture

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
)

const dashboardIndexFile = "index.html"

type dashboardAssetHandler struct {
	assets     fs.FS
	fileServer http.Handler
	logger     *slog.Logger
}

var _ http.Handler = (*dashboardAssetHandler)(nil)

func newDashboardAssetHandler(logger *slog.Logger) *dashboardAssetHandler {
	assets, err := fs.Sub(dashboardAssets, dashboardWebRoot)
	if err != nil {
		panic("open embedded dashboard assets: " + err.Error())
	}
	return newDashboardAssetHandlerFromFS(assets, logger)
}

func newDashboardAssetHandlerFromFS(assets fs.FS, logger *slog.Logger) *dashboardAssetHandler {
	return &dashboardAssetHandler{
		assets:     assets,
		fileServer: http.FileServerFS(assets),
		logger:     logger,
	}
}

func (handler *dashboardAssetHandler) ServeHTTP(
	response http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(path.Clean("/"+request.URL.Path), "/")
	if name == "" || name == "." {
		name = dashboardIndexFile
	}
	if hasHiddenPathSegment(name) {
		http.NotFound(response, request)
		return
	}
	if dashboardAssetExists(handler.assets, name) {
		handler.setCachePolicy(response, name)
		handler.fileServer.ServeHTTP(response, request)
		return
	}
	if path.Ext(name) != "" {
		http.NotFound(response, request)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	handler.serveIndex(response, request)
}

func (handler *dashboardAssetHandler) setCachePolicy(response http.ResponseWriter, name string) {
	if name == dashboardIndexFile {
		response.Header().Set("Cache-Control", "no-store")
		return
	}
	response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
}

func (handler *dashboardAssetHandler) serveIndex(
	response http.ResponseWriter,
	request *http.Request,
) {
	payload, err := fs.ReadFile(handler.assets, dashboardIndexFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.NotFound(response, request)
			return
		}
		handler.logger.Error(
			"The dashboard cannot read its embedded application shell.",
			slog.Any("error", err),
		)
		http.Error(response, "embedded dashboard unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if request.Method == http.MethodHead {
		return
	}
	if _, err := response.Write(payload); err != nil {
		handler.logger.Debug(
			"The dashboard cannot write its application shell.",
			slog.Any("error", err),
		)
	}
}

func hasHiddenPathSegment(name string) bool {
	for segment := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

func dashboardAssetExists(assets fs.FS, name string) bool {
	info, err := fs.Stat(assets, name)
	return err == nil && !info.IsDir()
}
