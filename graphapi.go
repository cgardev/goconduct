package main

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

type graphEndpoint struct {
	reader    graphReader
	refresher graphRefresher
	cacheKey  graphCacheKeyReader
	logger    *slog.Logger
}

func newGraphEndpoint(
	reader graphReader,
	refresher graphRefresher,
	cacheKey graphCacheKeyReader,
	logger *slog.Logger,
) *graphEndpoint {
	return &graphEndpoint{
		reader:    reader,
		refresher: refresher,
		cacheKey:  cacheKey,
		logger:    logger,
	}
}

func (endpoint *graphEndpoint) serve(response http.ResponseWriter, request *http.Request) {
	cacheKey, err := endpoint.cacheKey.graphCacheKey()
	if err != nil {
		endpoint.logger.Error("The graph endpoint cannot identify the dependency graph cache.", "error", err)
		http.Error(response, "dependency graph unavailable", http.StatusInternalServerError)
		return
	}
	if err := validateGraphCacheRequest(request, cacheKey); err != nil {
		http.Error(response, err.Error(), http.StatusPreconditionFailed)
		return
	}
	graph := endpoint.reader.currentGraph()
	if request != nil && request.Header.Get(graphCacheKeyHeader) != "" {
		graph, err = endpoint.refresher.freshGraph(request.Context())
		if err != nil {
			endpoint.logger.Error("The graph endpoint cannot refresh the dependency graph cache.", "error", err)
			http.Error(response, "dependency graph unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set(graphCacheKeyHeader, cacheKey)
	response.Header().Set(graphCacheProtocolHeader, strconv.Itoa(graphCacheProtocolVersion))
	response.Header().Set(graphCacheRevisionHeader, graph.Revision)
	response.Header().Set(graphCacheSchemaHeader, strconv.Itoa(graph.SchemaVersion))
	if err := writeGraphResponse(response, request, graph); err != nil {
		endpoint.logger.Debug("The graph endpoint cannot write the dependency graph.", "error", err)
	}
}

func validateGraphCacheRequest(request *http.Request, cacheKey string) error {
	if request == nil {
		return nil
	}
	requestedKey := request.Header.Get(graphCacheKeyHeader)
	if requestedKey == "" {
		return nil
	}
	if request.Header.Get(graphCacheProtocolHeader) != strconv.Itoa(graphCacheProtocolVersion) {
		return fmt.Errorf("%w: cache protocol does not match", errGraphCacheRejected)
	}
	if requestedKey != cacheKey {
		return fmt.Errorf("%w: analysis scope does not match", errGraphCacheRejected)
	}
	return nil
}

func writeGraphResponse(response http.ResponseWriter, request *http.Request, graph Graph) error {
	if request == nil || !acceptsGzip(request.Header.Get("Accept-Encoding")) {
		if err := json.NewEncoder(response).Encode(graph); err != nil {
			return fmt.Errorf("encode dependency graph: %w", err)
		}
		return nil
	}
	response.Header().Set("Content-Encoding", "gzip")
	response.Header().Add("Vary", "Accept-Encoding")
	compressor := gzip.NewWriter(response)
	if err := json.NewEncoder(compressor).Encode(graph); err != nil {
		closeError := compressor.Close()
		return errors.Join(fmt.Errorf("encode dependency graph: %w", err), closeError)
	}
	if err := compressor.Close(); err != nil {
		return fmt.Errorf("close dependency graph compressor: %w", err)
	}
	return nil
}

func acceptsGzip(header string) bool {
	for value := range strings.SplitSeq(header, ",") {
		coding, parameters, _ := strings.Cut(value, ";")
		if strings.TrimSpace(coding) == "gzip" && gzipQuality(parameters) > 0 {
			return true
		}
	}
	return false
}

func gzipQuality(parameters string) float64 {
	for parameter := range strings.SplitSeq(parameters, ";") {
		name, value, found := strings.Cut(parameter, "=")
		if !found || strings.TrimSpace(name) != "q" {
			continue
		}
		quality, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil {
			return quality
		}
	}
	return 1
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T15:58:00Z","module_hash":"5fa42934748d25e5241d38a0175e665f8d344affbb2cd25bb0edaee8f8be1eaf","functions":[{"id":"func/newGraphEndpoint","name":"newGraphEndpoint","line":21,"end_line":33,"hash":"61e4d3999c323bc19a3998d9f35e92a9245c9f4554390e391b9c1a44b3b79192"},{"id":"func/graphEndpoint.serve","name":"graphEndpoint.serve","line":35,"end_line":64,"hash":"6230cb38d38fa4a35ec710d23d2260853ff8f723f52a6de81d56b2b93c69cf4e"},{"id":"func/validateGraphCacheRequest","name":"validateGraphCacheRequest","line":66,"end_line":81,"hash":"3b2ee5c79fe5c50ca67a15731afdd939f75e7f5e15e1df5b349a006f72c55053"},{"id":"func/writeGraphResponse","name":"writeGraphResponse","line":83,"end_line":101,"hash":"1c70911489427a36cfb6240b00126b8171b97970acba756ef44bc2dc824243a4"},{"id":"func/acceptsGzip","name":"acceptsGzip","line":103,"end_line":111,"hash":"52a2204173ba0ecd2214eda1419ea35433f8b6162a72696398e167cc2d6374bf"},{"id":"func/gzipQuality","name":"gzipQuality","line":113,"end_line":125,"hash":"2fb50fc55492f665d63aff76f9eb767ebb73404b1cfe15c321e1a6728d5cf83e"}]}
// mutate4go-manifest-end
