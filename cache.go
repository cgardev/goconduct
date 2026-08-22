package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	application "digginginsights.com/v3/internal/devtool/dependencygraph/internal/application"
)

const (
	graphCacheProtocolVersion = 1
	maximumGraphCacheBytes    = 512 << 20
	graphCacheKeyHeader       = "X-Dependency-Graph-Cache-Key"
	graphCacheProtocolHeader  = "X-Dependency-Graph-Cache-Protocol"
	graphCacheRevisionHeader  = "X-Dependency-Graph-Revision"
	graphCacheSchemaHeader    = "X-Dependency-Graph-Schema"
)

type httpGraphCache struct {
	address string
	client  graphHTTPClient
}

var _ application.GraphCache[Graph] = (*httpGraphCache)(nil)

type graphHTTPClient interface {
	Do(request *http.Request) (*http.Response, error)
}

type httpGraphCacheFactory struct{}

var _ graphCacheFactory = httpGraphCacheFactory{}

func (httpGraphCacheFactory) NewGraphCache(
	address string,
	timeout time.Duration,
) application.GraphCache[Graph] {
	return newHTTPGraphCache(address, &http.Client{Timeout: timeout})
}

func newHTTPGraphCache(address string, client graphHTTPClient) *httpGraphCache {
	return &httpGraphCache{
		address: address,
		client:  client,
	}
}

func (cache *httpGraphCache) Load(
	ctx context.Context,
	identity application.GraphCacheIdentity,
) (Graph, error) {
	cacheKey, err := identity.CacheKey()
	if err != nil {
		return Graph{}, err
	}
	return loadGraphFromCacheKeyWithClient(ctx, cacheKey, cache.address, cache.client)
}

func loadGraphFromCacheKeyWithClient(
	ctx context.Context,
	cacheKey string,
	address string,
	client graphHTTPClient,
) (graph Graph, returnError error) {
	request, err := newGraphCacheRequest(ctx, address, cacheKey)
	if err != nil {
		return Graph{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return Graph{}, newUnavailableError("request active graph", err)
	}
	defer func() {
		if closeError := response.Body.Close(); closeError != nil && returnError == nil {
			returnError = newUnavailableError("close graph cache response", closeError)
		}
	}()
	if err := validateGraphCacheResponse(response, cacheKey); err != nil {
		return Graph{}, err
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maximumGraphCacheBytes))
	if err := decoder.Decode(&graph); err != nil {
		return Graph{}, newDataIntegrityError("decode active graph", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return Graph{}, err
	}
	if graph.SchemaVersion != graphSchemaVersion {
		return Graph{}, newDataIntegrityError(
			fmt.Sprintf(
				"graph schema is %d, want %d",
				graph.SchemaVersion,
				graphSchemaVersion,
			),
			nil,
		)
	}
	if graph.Revision == "" {
		return Graph{}, newDataIntegrityError("graph revision is empty", nil)
	}
	return graph, nil
}

func newGraphCacheRequest(ctx context.Context, address, cacheKey string) (*http.Request, error) {
	endpoint, err := graphCacheURL(address)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, newInternalError("create graph cache request", err)
	}
	request.Header.Set(graphCacheKeyHeader, cacheKey)
	request.Header.Set(graphCacheProtocolHeader, strconv.Itoa(graphCacheProtocolVersion))
	return request, nil
}

func graphCacheURL(address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", newValidationError(fmt.Sprintf("parse graph cache address %q", address), err)
	}
	host = graphCacheClientHost(host)
	endpoint := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, port),
		Path:   "/api/graph",
	}
	return endpoint.String(), nil
}

func graphCacheClientHost(host string) string {
	if host == "" {
		return "127.0.0.1"
	}
	address := net.ParseIP(strings.Trim(host, "[]"))
	if address == nil || !address.IsUnspecified() {
		return host
	}
	if address.To4() != nil {
		return "127.0.0.1"
	}
	return "::1"
}

func validateGraphCacheResponse(response *http.Response, cacheKey string) error {
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusPreconditionFailed {
			return newDataIntegrityError(
				fmt.Sprintf("graph cache server returns %s", response.Status),
				nil,
			)
		}
		return newUnavailableError(
			fmt.Sprintf("graph cache server returns %s", response.Status),
			nil,
		)
	}
	protocol := response.Header.Get(graphCacheProtocolHeader)
	if protocol != strconv.Itoa(graphCacheProtocolVersion) {
		return newDataIntegrityError(
			fmt.Sprintf(
				"cache protocol is %q, want %d",
				protocol,
				graphCacheProtocolVersion,
			),
			nil,
		)
	}
	if response.Header.Get(graphCacheKeyHeader) != cacheKey {
		return newDataIntegrityError("analysis scope does not match", nil)
	}
	if response.Header.Get(graphCacheSchemaHeader) != strconv.Itoa(graphSchemaVersion) {
		return newDataIntegrityError("graph schema does not match", nil)
	}
	return nil
}

func requireJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return newDataIntegrityError(
				"decode active graph: response contains more than one JSON value",
				nil,
			)
		}
		return newDataIntegrityError("check active graph response for another JSON value", err)
	}
	return nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T18:35:10Z","module_hash":"592c63ce286c00879393537aa3c1e4f0e032c839a3f5c00110ff22560e366f94","functions":[{"id":"func/httpGraphCacheFactory.NewGraphCache","name":"httpGraphCacheFactory.NewGraphCache","line":43,"end_line":48,"hash":"6e3abf94715cbbcc5a8fe4553b8cfe379d385298f65b73d9bb5822ba3e6f2552"},{"id":"func/newHTTPGraphCache","name":"newHTTPGraphCache","line":50,"end_line":55,"hash":"d3add52bebb7521de4e7ea887521f30406f74202df1f98c32bf359f66b6005a0"},{"id":"func/httpGraphCache.Load","name":"httpGraphCache.Load","line":57,"end_line":66,"hash":"aee0cefbb4462dcea258e6784e26f0ccc2cd23803ad20e742cb041cb0af9955c"},{"id":"func/loadGraphFromCacheKeyWithClient","name":"loadGraphFromCacheKeyWithClient","line":68,"end_line":111,"hash":"350685c4d27f1e4c0d81493f116b61b48a13a0a0583cc95d10fae9c2be8d6afd"},{"id":"func/newGraphCacheRequest","name":"newGraphCacheRequest","line":113,"end_line":125,"hash":"f3df53dce4333e29bf5e70851bfebc48772394760dec5c4ea0df45c9bf2d32e9"},{"id":"func/graphCacheURL","name":"graphCacheURL","line":127,"end_line":139,"hash":"a29293a582631d1769e2e0e285f227b53ae9cbf871296bcb5df3f665431e10b2"},{"id":"func/graphCacheClientHost","name":"graphCacheClientHost","line":141,"end_line":153,"hash":"2ff8bb2da1793d84546a199da9a33c72e54c141d3fe44a51bf465acab3f5e6c9"},{"id":"func/validateGraphCacheResponse","name":"validateGraphCacheResponse","line":155,"end_line":186,"hash":"351cb51f827539be849693615b9cb81543560e45f50a83bb5c8d27823070fd9a"},{"id":"func/requireJSONEnd","name":"requireJSONEnd","line":188,"end_line":200,"hash":"676a8ececd81473558d7cdb0ea8b644662955415d5f78718eaac572383f8c7b5"}]}
// mutate4go-manifest-end
