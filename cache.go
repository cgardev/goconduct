package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
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

type graphCacheIdentity struct {
	ProtocolVersion   int           `json:"protocolVersion"`
	SchemaVersion     int           `json:"schemaVersion"`
	RepositoryRoot    string        `json:"repositoryRoot"`
	Scope             AnalysisScope `json:"scope"`
	GoVersion         string        `json:"goVersion"`
	GoOperatingSystem string        `json:"goOperatingSystem"`
	GoArchitecture    string        `json:"goArchitecture"`
	GoFlags           string        `json:"goFlags"`
}

func (analyzer *analyzer) graphCacheKey() (string, error) {
	payload, err := json.Marshal(graphCacheIdentity{
		ProtocolVersion:   graphCacheProtocolVersion,
		SchemaVersion:     graphSchemaVersion,
		RepositoryRoot:    analyzer.repositoryRoot,
		Scope:             analyzer.scope,
		GoVersion:         runtime.Version(),
		GoOperatingSystem: runtime.GOOS,
		GoArchitecture:    runtime.GOARCH,
		GoFlags:           os.Getenv("GOFLAGS"),
	})
	if err != nil {
		return "", fmt.Errorf("encode graph cache identity: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func loadGraphFromCache(
	ctx context.Context,
	analyzer *analyzer,
	address string,
	timeout time.Duration,
) (Graph, error) {
	client := &http.Client{Timeout: timeout}
	return loadGraphFromCacheWithClient(ctx, analyzer, address, client)
}

func loadGraphFromCacheWithClient(
	ctx context.Context,
	analyzer *analyzer,
	address string,
	client *http.Client,
) (graph Graph, returnError error) {
	cacheKey, err := analyzer.graphCacheKey()
	if err != nil {
		return Graph{}, err
	}
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
		return fmt.Errorf("decode active graph end: %w", err)
	}
	return nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T11:38:57Z","module_hash":"a30d5fb3e47a9d390a069af7f9a360ba49d4601a2320022203ed049b4a1d69c2","functions":[{"id":"func/analyzer.graphCacheKey","name":"analyzer.graphCacheKey","line":46,"end_line":62,"hash":"cfaba175605e61592d366b05d1a98e4dd22cc6b1f414eef35b689e83ff5e2c40"},{"id":"func/loadGraphFromCache","name":"loadGraphFromCache","line":64,"end_line":72,"hash":"9b67bde4c9016f1b119de2b28031a58a20e48ebaeb1981aa5fde5c075133fa79"},{"id":"func/loadGraphFromCacheWithClient","name":"loadGraphFromCacheWithClient","line":74,"end_line":119,"hash":"e139d9b34b3972d0cad3a5cdcd9d7e42785c25cb16ff683e298ae36fc4c2cbce"},{"id":"func/newGraphCacheRequest","name":"newGraphCacheRequest","line":121,"end_line":133,"hash":"23b4928fce8c70f6e2f1ad07ad890bbc9efb63b49ad99513525a87690191cbc1"},{"id":"func/graphCacheURL","name":"graphCacheURL","line":135,"end_line":147,"hash":"492eaf09163ee19aac5d750b4a4487bff1bf9c8828e9193eaa874281cde3553e"},{"id":"func/graphCacheClientHost","name":"graphCacheClientHost","line":149,"end_line":161,"hash":"2ff8bb2da1793d84546a199da9a33c72e54c141d3fe44a51bf465acab3f5e6c9"},{"id":"func/validateGraphCacheResponse","name":"validateGraphCacheResponse","line":163,"end_line":187,"hash":"39f9f1069930d9425695accf656e900dc2e3c8c24ba4b954e391c39cf4d435c6"},{"id":"func/requireJSONEnd","name":"requireJSONEnd","line":189,"end_line":198,"hash":"5500f604babea14339a2abf42a512ffb3a1acd3d39fb8e929bb37e6e2bb7fd37"}]}
// mutate4go-manifest-end
