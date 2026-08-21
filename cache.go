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

var (
	errGraphCacheUnavailable = errors.New("graph cache unavailable")
	errGraphCacheRejected    = errors.New("graph cache rejected")
)

type httpGraphCache struct {
	address string
	client  *http.Client
}

var _ application.GraphCache[Graph] = (*httpGraphCache)(nil)

func newHTTPGraphCache(address string, timeout time.Duration) *httpGraphCache {
	return &httpGraphCache{
		address: address,
		client:  &http.Client{Timeout: timeout},
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
	client *http.Client,
) (graph Graph, returnError error) {
	request, err := newGraphCacheRequest(ctx, address, cacheKey)
	if err != nil {
		return Graph{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return Graph{}, fmt.Errorf("%w: request active graph: %w", errGraphCacheUnavailable, err)
	}
	defer func() {
		if closeError := response.Body.Close(); closeError != nil && returnError == nil {
			returnError = fmt.Errorf("close graph cache response: %w", closeError)
		}
	}()
	if err := validateGraphCacheResponse(response, cacheKey); err != nil {
		return Graph{}, err
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maximumGraphCacheBytes))
	if err := decoder.Decode(&graph); err != nil {
		return Graph{}, fmt.Errorf("decode active graph: %w", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return Graph{}, err
	}
	if graph.SchemaVersion != graphSchemaVersion {
		return Graph{}, fmt.Errorf(
			"%w: graph schema is %d, want %d",
			errGraphCacheRejected,
			graph.SchemaVersion,
			graphSchemaVersion,
		)
	}
	if graph.Revision == "" {
		return Graph{}, fmt.Errorf("%w: graph revision is empty", errGraphCacheRejected)
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
		return nil, fmt.Errorf("create graph cache request: %w", err)
	}
	request.Header.Set(graphCacheKeyHeader, cacheKey)
	request.Header.Set(graphCacheProtocolHeader, strconv.Itoa(graphCacheProtocolVersion))
	return request, nil
}

func graphCacheURL(address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("parse graph cache address %q: %w", address, err)
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
		return fmt.Errorf(
			"%w: server returns %s",
			errGraphCacheRejected,
			response.Status,
		)
	}
	protocol := response.Header.Get(graphCacheProtocolHeader)
	if protocol != strconv.Itoa(graphCacheProtocolVersion) {
		return fmt.Errorf(
			"%w: cache protocol is %q, want %d",
			errGraphCacheRejected,
			protocol,
			graphCacheProtocolVersion,
		)
	}
	if response.Header.Get(graphCacheKeyHeader) != cacheKey {
		return fmt.Errorf("%w: analysis scope does not match", errGraphCacheRejected)
	}
	if response.Header.Get(graphCacheSchemaHeader) != strconv.Itoa(graphSchemaVersion) {
		return fmt.Errorf("%w: graph schema does not match", errGraphCacheRejected)
	}
	return nil
}

func requireJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("decode active graph: response contains more than one JSON value")
		}
		return fmt.Errorf("check active graph response for another JSON value: %w", err)
	}
	return nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T16:39:29Z","module_hash":"fa7c2b25a6bff0e03e66c9b119cccac94f75d4c689b5fe6a36b2f84da659064a","functions":[{"id":"func/newHTTPGraphCache","name":"newHTTPGraphCache","line":40,"end_line":45,"hash":"8fb0cf47fcfa7986196b56d13a4f80283b9875b68c7149805052f2d2b1d0bd4d"},{"id":"func/httpGraphCache.Load","name":"httpGraphCache.Load","line":47,"end_line":56,"hash":"aee0cefbb4462dcea258e6784e26f0ccc2cd23803ad20e742cb041cb0af9955c"},{"id":"func/loadGraphFromCacheKeyWithClient","name":"loadGraphFromCacheKeyWithClient","line":58,"end_line":99,"hash":"e69d1bde378b24dd88ca869ff5c3f78d340e826955abea940fbea0b5b0396379"},{"id":"func/newGraphCacheRequest","name":"newGraphCacheRequest","line":101,"end_line":113,"hash":"23b4928fce8c70f6e2f1ad07ad890bbc9efb63b49ad99513525a87690191cbc1"},{"id":"func/graphCacheURL","name":"graphCacheURL","line":115,"end_line":127,"hash":"492eaf09163ee19aac5d750b4a4487bff1bf9c8828e9193eaa874281cde3553e"},{"id":"func/graphCacheClientHost","name":"graphCacheClientHost","line":129,"end_line":141,"hash":"2ff8bb2da1793d84546a199da9a33c72e54c141d3fe44a51bf465acab3f5e6c9"},{"id":"func/validateGraphCacheResponse","name":"validateGraphCacheResponse","line":143,"end_line":167,"hash":"39f9f1069930d9425695accf656e900dc2e3c8c24ba4b954e391c39cf4d435c6"},{"id":"func/requireJSONEnd","name":"requireJSONEnd","line":169,"end_line":178,"hash":"30e9e4256d8993a0bbf9b91551a6041fe59f7f98f528c8e1ba3029759303e175"}]}
// mutate4go-manifest-end
