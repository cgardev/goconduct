package main

import (
	"compress/gzip"
	"encoding/json"
	"errors"
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
		endpoint.logger.Error(
			"The graph endpoint cannot identify the dependency graph cache.",
			slog.Any("error", err),
		)
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
			endpoint.logger.Error(
				"The graph endpoint cannot refresh the dependency graph cache.",
				slog.Any("error", err),
			)
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
		endpoint.logger.Debug(
			"The graph endpoint cannot write the dependency graph.",
			slog.Any("error", err),
		)
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
		return newValidationError("cache protocol does not match", nil)
	}
	if requestedKey != cacheKey {
		return newValidationError("analysis scope does not match", nil)
	}
	return nil
}

func writeGraphResponse(response http.ResponseWriter, request *http.Request, graph Graph) error {
	if request == nil || !acceptsGzip(request.Header.Get("Accept-Encoding")) {
		if err := json.NewEncoder(response).Encode(graph); err != nil {
			return newUnavailableError("encode dependency graph", err)
		}
		return nil
	}
	response.Header().Set("Content-Encoding", "gzip")
	response.Header().Add("Vary", "Accept-Encoding")
	compressor := gzip.NewWriter(response)
	if err := json.NewEncoder(compressor).Encode(graph); err != nil {
		closeError := compressor.Close()
		return newUnavailableError(
			"encode dependency graph",
			errors.Join(err, closeError),
		)
	}
	if err := compressor.Close(); err != nil {
		return newUnavailableError("close dependency graph compressor", err)
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
// {"version":1,"tested_at":"2026-08-21T18:29:26Z","module_hash":"3a4acd3a3b75ab8d908cfc7cc13497899b1eef58fcd4774fb7ae9bce2b3e54ec","functions":[{"id":"func/newGraphEndpoint","name":"newGraphEndpoint","line":20,"end_line":32,"hash":"61e4d3999c323bc19a3998d9f35e92a9245c9f4554390e391b9c1a44b3b79192"},{"id":"func/graphEndpoint.serve","name":"graphEndpoint.serve","line":34,"end_line":72,"hash":"32e6593aba216c6362e97d47f01842c3c45db456cc036fd6e72f60de28514480"},{"id":"func/validateGraphCacheRequest","name":"validateGraphCacheRequest","line":74,"end_line":89,"hash":"b61c7ff3793014ccb5a027e89d394be14598e9892680581bb201205324f4e018"},{"id":"func/writeGraphResponse","name":"writeGraphResponse","line":91,"end_line":112,"hash":"960c77e1d137330457606c0c1bec5876043731f1ca531575b818fa658188c213"},{"id":"func/acceptsGzip","name":"acceptsGzip","line":114,"end_line":122,"hash":"52a2204173ba0ecd2214eda1419ea35433f8b6162a72696398e167cc2d6374bf"},{"id":"func/gzipQuality","name":"gzipQuality","line":124,"end_line":136,"hash":"2fb50fc55492f665d63aff76f9eb767ebb73404b1cfe15c321e1a6728d5cf83e"}]}
// mutate4go-manifest-end
