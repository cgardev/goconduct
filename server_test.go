package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/failure"
)

type notifyingResponseRecorder struct {
	*httptest.ResponseRecorder
	flushed chan struct{}
}

var errResponseWrite = errors.New("response write failure")
var errGraphRefresh = errors.New("graph refresh failure")

type failingResponseWriter struct {
	header http.Header
	status int
}

func (writer *failingResponseWriter) Header() http.Header {
	return writer.header
}

func (writer *failingResponseWriter) Write([]byte) (int, error) {
	return 0, errResponseWrite
}

func (writer *failingResponseWriter) WriteHeader(status int) {
	writer.status = status
}

type plainResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

type dashboardContextKey string

type dashboardTransportSourceStub struct {
	graph        Graph
	freshCalls   int
	freshContext context.Context
	freshError   error
}

func (stub *dashboardTransportSourceStub) currentGraph() Graph {
	return stub.graph
}

func (stub *dashboardTransportSourceStub) freshGraph(ctx context.Context) (Graph, error) {
	stub.freshCalls++
	stub.freshContext = ctx
	return stub.graph, stub.freshError
}

func (*dashboardTransportSourceStub) subscribe() (<-chan string, func()) {
	return make(chan string), func() {}
}

func (*dashboardTransportSourceStub) graphCacheKey() (string, error) {
	return "transport-cache-key", nil
}

func (writer *plainResponseWriter) Header() http.Header {
	return writer.header
}

func (writer *plainResponseWriter) Write(payload []byte) (int, error) {
	return writer.body.Write(payload)
}

func (writer *plainResponseWriter) WriteHeader(status int) {
	writer.status = status
}

func (recorder *notifyingResponseRecorder) Flush() {
	recorder.ResponseRecorder.Flush()
	select {
	case recorder.flushed <- struct{}{}:
	default:
	}
}

func TestDashboard_UseGraphSourcePort(t *testing.T) {
	t.Run("Scenario: The HTTP transport receives a graph source without an analyzer", func(t *testing.T) {
		var handler *dashboardHandler
		var source *dashboardTransportSourceStub
		var response *httptest.ResponseRecorder
		var result Graph
		var decodeError error

		t.Run("Given a graph source that implements only the transport port", func(*testing.T) {
			source = &dashboardTransportSourceStub{graph: Graph{
				SchemaVersion: graphSchemaVersion,
				Revision:      "port-revision",
			}}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler = newDashboardHandler(source, logger)
			response = httptest.NewRecorder()
		})

		t.Run("When the client requests the graph", func(*testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/graph", nil)
			handler.ServeHTTP(response, request)
			decodeError = json.Unmarshal(response.Body.Bytes(), &result)
		})

		if !t.Run("Then the transport returns the graph from the port", func(t *testing.T) {
			if decodeError != nil {
				t.Fatalf("decode graph response: %v", decodeError)
			}
			if response.Code != http.StatusOK {
				t.Fatalf("graph endpoint returns status %d", response.Code)
			}
		}) {
			return
		}

		t.Run("And the response contains the source revision", func(t *testing.T) {
			if result.Revision != "port-revision" {
				t.Errorf("graph revision is %q, want port-revision", result.Revision)
			}
			if source.freshCalls != 0 {
				t.Errorf("fresh graph calls are %d, want 0", source.freshCalls)
			}
		})
	})
}

func TestGraphEndpoint_ValidateCacheRequest(t *testing.T) {
	testCases := []struct {
		name       string
		hasRequest bool
		cacheKey   string
		protocol   string
		wantReject bool
	}{
		{name: "the request is absent"},
		{name: "the cache key is absent", hasRequest: true},
		{
			name: "the protocol is different", hasRequest: true, cacheKey: "expected", protocol: "different",
			wantReject: true,
		},
		{
			name: "the cache key is different", hasRequest: true, cacheKey: "different", protocol: "1",
			wantReject: true,
		},
		{name: "the cache contract matches", hasRequest: true, cacheKey: "expected", protocol: "1"},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var request *http.Request
			var validationError error

			t.Run("Given an optional cache request", func(*testing.T) {
				if testCase.hasRequest {
					request = cacheRequestWithHeaders(testCase.cacheKey, testCase.protocol)
				}
			})

			t.Run("When the endpoint validates the cache contract", func(*testing.T) {
				validationError = validateGraphCacheRequest(request, "expected")
			})

			t.Run("Then the endpoint returns the selected validation result", func(t *testing.T) {
				if errors.Is(validationError, failure.ErrValidation) != testCase.wantReject {
					t.Fatalf("cache rejection is %v, want %t", validationError, testCase.wantReject)
				}
				if !testCase.wantReject && validationError != nil {
					t.Fatalf("cache validation fails: %v", validationError)
				}
			})
		})
	}
}

func TestGraphEndpoint_ReportRefreshFailure(t *testing.T) {
	t.Run("Scenario: The active graph cannot refresh for a cache request", func(t *testing.T) {
		var source *dashboardTransportSourceStub
		var handler *dashboardHandler
		var response *httptest.ResponseRecorder
		var requestContext context.Context

		t.Run("Given a graph source that returns a refresh error", func(*testing.T) {
			source = &dashboardTransportSourceStub{freshError: errGraphRefresh}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler = newDashboardHandler(source, logger)
			response = httptest.NewRecorder()
		})

		t.Run("When a compatible cache client requests a fresh graph", func(*testing.T) {
			requestContext = context.WithValue(t.Context(), dashboardContextKey("refresh"), "value")
			request := cacheRequestWithHeaders("transport-cache-key", "1").WithContext(requestContext)
			handler.ServeHTTP(response, request)
		})

		t.Run("Then the endpoint reports that the graph is unavailable", func(t *testing.T) {
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("graph endpoint status is %d, want %d", response.Code, http.StatusServiceUnavailable)
			}
		})

		t.Run("And the endpoint requests one fresh graph", func(t *testing.T) {
			if source.freshCalls != 1 {
				t.Errorf("fresh graph calls are %d, want 1", source.freshCalls)
			}
			if source.freshContext != requestContext {
				t.Error("the graph refresh does not receive the request context")
			}
		})
	})
}

func cacheRequestWithHeaders(cacheKey, protocol string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/api/graph", nil)
	request.Header.Set(graphCacheKeyHeader, cacheKey)
	request.Header.Set(graphCacheProtocolHeader, protocol)
	return request
}

func TestDashboard_ServeHTTPResources(t *testing.T) {
	t.Run("Scenario: A client requests the dashboard document", func(t *testing.T) {
		var server *httptest.Server
		var response *http.Response
		var document []byte

		if !t.Run("Given a dashboard server with a current graph", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err := newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			server = httptest.NewServer(newDashboardHandler(monitor, logger))
			t.Cleanup(server.Close)
		}) {
			return
		}

		t.Run("When the client requests the dashboard document", func(step *testing.T) {
			response, document = requestDashboardAsset(step, server.Client(), server.URL+"/")
		})

		if !t.Run("Then the server returns the self-contained document", func(t *testing.T) {
			if response.StatusCode != http.StatusOK {
				t.Fatalf("dashboard returns status %d", response.StatusCode)
			}
			if !strings.Contains(string(document), "Go component dependency map") {
				t.Fatal("the dashboard document does not contain its title")
			}
			for _, remoteReference := range []string{"https://", "http://", "//cdn"} {
				if strings.Contains(string(document), remoteReference) {
					t.Fatalf("the self-contained dashboard references %q", remoteReference)
				}
			}
		}) {
			return
		}

		t.Run("And the response contains restrictive browser security headers", func(t *testing.T) {
			securityPolicy := response.Header.Get("Content-Security-Policy")
			if securityPolicy == "" || strings.Contains(securityPolicy, "unsafe-inline") {
				t.Errorf("unexpected content security policy %q", securityPolicy)
			}
		})

		t.Run("And the dashboard contains the component count controls", func(t *testing.T) {
			for _, marker := range []string{
				`id="componentMetricView"`,
				`id="functionMetricView"`,
				`id="componentMetric"`,
				`id="functionMetric"`,
				`id="componentMetricMap"`,
				`id="functionMetricMap"`,
				"Afferent coupling",
				`id="functionUsageRanking"`,
				`id="stableLowAbstractionRanking"`,
			} {
				if !strings.Contains(string(document), marker) {
					t.Errorf("the dashboard document does not contain %q", marker)
				}
			}
		})

		t.Run("And the dashboard contains both color theme controls", func(t *testing.T) {
			for _, marker := range []string{
				`data-theme="dark"`,
				`id="lightTheme"`,
				`id="darkTheme"`,
				`aria-label="Color theme"`,
			} {
				if !strings.Contains(string(document), marker) {
					t.Errorf("the dashboard document does not contain %q", marker)
				}
			}
		})

		t.Run("And the document loads each separated presentation resource", func(t *testing.T) {
			for _, asset := range dashboardAssetDefinitions() {
				if !strings.Contains(string(document), asset.requestPath) {
					t.Errorf("the dashboard document does not load %q", asset.requestPath)
				}
			}
		})
	})

	t.Run("Scenario: A client requests all embedded dashboard assets", func(t *testing.T) {
		var server *httptest.Server
		var responses map[string]*http.Response
		var payloads map[string][]byte

		if !t.Run("Given a dashboard server with embedded assets", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err := newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			server = httptest.NewServer(newDashboardHandler(monitor, logger))
			t.Cleanup(server.Close)
		}) {
			return
		}

		t.Run("When the client requests every stylesheet and script", func(step *testing.T) {
			responses = make(map[string]*http.Response)
			payloads = make(map[string][]byte)
			for _, asset := range dashboardAssetDefinitions() {
				responses[asset.requestPath], payloads[asset.requestPath] = requestDashboardAsset(
					step,
					server.Client(),
					server.URL+asset.requestPath,
				)
			}
		})

		t.Run("Then the server returns every non-empty dashboard asset", func(t *testing.T) {
			for _, asset := range dashboardAssetDefinitions() {
				response := responses[asset.requestPath]
				payload := payloads[asset.requestPath]
				if response.StatusCode != http.StatusOK || len(payload) == 0 {
					t.Errorf(
						"asset %s response is status %d with %d bytes",
						asset.requestPath,
						response.StatusCode,
						len(payload),
					)
				}
			}
		})

		t.Run("And the assets support persistent light and dark themes", func(t *testing.T) {
			style := payloads["/assets/dashboard.css"]
			script := payloads["/assets/dashboard.js"]
			for _, marker := range []string{
				`:root[data-theme="light"]`,
				`--color-background: #ffffff`,
				`--color-background: #000000`,
			} {
				if !strings.Contains(string(style), marker) {
					t.Errorf("the dashboard stylesheet does not contain %q", marker)
				}
			}
			for _, marker := range []string{
				`dependencygraph-theme`,
				`window.localStorage.getItem`,
				`window.localStorage.setItem`,
			} {
				if !strings.Contains(string(script), marker) {
					t.Errorf("the dashboard script does not contain %q", marker)
				}
			}
		})
	})

	t.Run("Scenario: A client requests the current dependency graph", func(t *testing.T) {
		var server *httptest.Server
		var monitor *graphMonitor
		var response *http.Response
		var graph Graph
		var decodeError error

		if !t.Run("Given a dashboard server with a current graph", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			server = httptest.NewServer(newDashboardHandler(monitor, logger))
			t.Cleanup(server.Close)
		}) {
			return
		}

		t.Run("When the client requests the graph endpoint", func(step *testing.T) {
			var payload []byte
			response, payload = requestDashboardAsset(step, server.Client(), server.URL+"/api/graph")
			decodeError = json.Unmarshal(payload, &graph)
		})

		if !t.Run("Then the server returns a valid graph response", func(t *testing.T) {
			if response.StatusCode != http.StatusOK {
				t.Fatalf("graph endpoint returns status %d", response.StatusCode)
			}
			if decodeError != nil {
				t.Fatalf("decode graph response: %v", decodeError)
			}
		}) {
			return
		}

		t.Run("And the response revision matches the monitor", func(t *testing.T) {
			if graph.Revision != monitor.currentGraph().Revision {
				t.Errorf("graph response revision %q does not match the monitor", graph.Revision)
			}
		})

		t.Run("And the HTTP transport exposes the same JSON findings", func(t *testing.T) {
			if graph.Summary.Findings != 3 || len(graph.Findings) != 3 {
				t.Errorf(
					"HTTP graph has %d summary findings and %d details, want 3 and 3",
					graph.Summary.Findings,
					len(graph.Findings),
				)
			}
		})
	})

	t.Run("Scenario: A client uses an unsupported method on the graph endpoint", func(t *testing.T) {
		var server *httptest.Server
		var status int
		var requestError error

		if !t.Run("Given a dashboard server with a current graph", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err := newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			server = httptest.NewServer(newDashboardHandler(monitor, logger))
			t.Cleanup(server.Close)
		}) {
			return
		}

		t.Run("When the client sends a POST request to the graph endpoint", func(step *testing.T) {
			request, err := http.NewRequestWithContext(
				step.Context(),
				http.MethodPost,
				server.URL+"/api/graph",
				nil,
			)
			if err != nil {
				requestError = err
				return
			}
			methodResponse, err := server.Client().Do(request)
			if err != nil {
				requestError = err
				return
			}
			status = methodResponse.StatusCode
			if _, err = io.Copy(io.Discard, methodResponse.Body); err != nil {
				requestError = err
			}
			if closeError := methodResponse.Body.Close(); requestError == nil && closeError != nil {
				requestError = closeError
			}
		})

		if !t.Run("Then the request completes without a transport error", func(t *testing.T) {
			if requestError != nil {
				t.Fatalf("the unsupported method request fails: %v", requestError)
			}
		}) {
			return
		}

		t.Run("And the server reports method not allowed", func(t *testing.T) {
			if status != http.StatusMethodNotAllowed {
				t.Errorf("unsupported method returns status %d", status)
			}
		})
	})
}
func TestDashboardEventStream_PublishGraphChanges(t *testing.T) {
	t.Run("Scenario: A client observes two graph changes and cancels the stream", func(t *testing.T) {
		var repositoryRoot string
		var monitor *graphMonitor
		var handler *dashboardHandler
		var request *http.Request
		var recorder *notifyingResponseRecorder
		var cancelRequest context.CancelFunc
		var completed chan struct{}
		var readyFlushed bool
		var firstGraphFlushed bool
		var secondGraphFlushed bool
		var stopped bool
		var payload string

		if !t.Run("Given an event stream request and a monitored repository", func(step *testing.T) {
			repositoryRoot = newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			handler = newDashboardHandler(monitor, logger)
			requestContext, cancel := context.WithCancel(t.Context())
			cancelRequest = cancel
			t.Cleanup(cancelRequest)
			request = httptest.NewRequest(
				http.MethodGet,
				"/api/events",
				nil,
			).WithContext(requestContext)
			recorder = &notifyingResponseRecorder{
				ResponseRecorder: httptest.NewRecorder(),
				flushed:          make(chan struct{}, 1),
			}
			completed = make(chan struct{})
		}) {
			return
		}

		t.Run("When two source changes occur before the client cancels the request", func(step *testing.T) {
			go func() {
				handler.ServeHTTP(recorder, request)
				close(completed)
			}()
			select {
			case <-recorder.flushed:
				readyFlushed = true
			case <-time.After(time.Second):
			}
			writeFixtureFile(
				step,
				repositoryRoot,
				"cmd/control/main.go",
				"package main\n\nimport _ \"example.com/repository/internal/library/logging\"\n",
			)
			monitor.refresh(t.Context())
			select {
			case <-recorder.flushed:
				firstGraphFlushed = true
			case <-time.After(time.Second):
			}
			writeFixtureFile(
				step,
				repositoryRoot,
				"internal/library/second/second.go",
				"package second\n",
			)
			monitor.refresh(t.Context())
			select {
			case <-recorder.flushed:
				secondGraphFlushed = true
			case <-time.After(time.Second):
			}
			cancelRequest()
			select {
			case <-completed:
				stopped = true
			case <-time.After(time.Second):
			}
			payload = recorder.Body.String()
		})

		if !t.Run("Then the handler flushes every event and stops the stream", func(t *testing.T) {
			if !readyFlushed || !firstGraphFlushed || !secondGraphFlushed {
				t.Fatalf(
					"flush states are ready=%t, first=%t, second=%t",
					readyFlushed,
					firstGraphFlushed,
					secondGraphFlushed,
				)
			}
			if !stopped {
				t.Fatal("the event stream does not stop after request cancellation")
			}
			if recorder.Code != http.StatusOK {
				t.Fatalf("event stream returns status %d", recorder.Code)
			}
		}) {
			return
		}

		t.Run("And the stream contains the ready event and both graph events", func(t *testing.T) {
			if !strings.Contains(payload, "event: ready\n") ||
				!strings.Contains(payload, "data: "+monitor.currentGraph().Revision+"\n") {
				t.Errorf("the event stream does not publish the current revision: %q", payload)
			}
			if graphEvents := strings.Count(payload, "event: graph\n"); graphEvents != 2 {
				t.Errorf("event stream publishes %d graph events, want 2: %q", graphEvents, payload)
			}
		})
	})
}
func TestDashboardEventStream_PublishKeepAlive(t *testing.T) {
	t.Run("Scenario: An idle event stream remains open until cancellation", func(t *testing.T) {
		var handler *dashboardHandler
		var request *http.Request
		var recorder *notifyingResponseRecorder
		var cancelRequest context.CancelFunc
		var completed chan struct{}
		var configuredInterval time.Duration
		var flushCount int
		var stopped bool
		var payload string

		if !t.Run("Given an event stream with a short test keep-alive interval", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err := newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			handler = newDashboardHandler(monitor, logger)
			configuredInterval = handler.keepAliveInterval
			handler.keepAliveInterval = time.Millisecond
			requestContext, cancel := context.WithCancel(t.Context())
			cancelRequest = cancel
			t.Cleanup(cancelRequest)
			request = httptest.NewRequest(
				http.MethodGet,
				"/api/events",
				nil,
			).WithContext(requestContext)
			recorder = &notifyingResponseRecorder{
				ResponseRecorder: httptest.NewRecorder(),
				flushed:          make(chan struct{}, 1),
			}
			completed = make(chan struct{})
		}) {
			return
		}

		t.Run("When the stream waits for one keep-alive and the test cancels it", func(t *testing.T) {
			go func() {
				handler.ServeHTTP(recorder, request)
				close(completed)
			}()
			for flushCount < 2 {
				select {
				case <-recorder.flushed:
					flushCount++
				case <-time.After(time.Second):
					cancelRequest()
					return
				}
			}
			cancelRequest()
			select {
			case <-completed:
				stopped = true
			case <-time.After(time.Second):
			}
			payload = recorder.Body.String()
		})

		if !t.Run("Then the handler flushes the ready event and keep-alive before shutdown", func(t *testing.T) {
			if flushCount != 2 {
				t.Fatalf("event stream flushes %d events, want 2", flushCount)
			}
			if !stopped {
				t.Fatal("the event stream does not stop")
			}
		}) {
			return
		}

		t.Run("And the production interval and keep-alive payload are correct", func(t *testing.T) {
			if configuredInterval != 20*time.Second {
				t.Errorf("keep-alive interval is %s, want 20s", configuredInterval)
			}
			if !strings.Contains(payload, ": keep-alive\n\n") {
				t.Errorf("event stream has no keep-alive comment: %q", payload)
			}
		})
	})
}
func TestDashboardEventStream_RejectWriterWithoutFlush(t *testing.T) {
	t.Run("Scenario: The response writer does not support event flush operations", func(t *testing.T) {
		var handler *dashboardHandler
		var writer *plainResponseWriter
		var request *http.Request

		if !t.Run("Given an event request with a plain response writer", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err := newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			handler = newDashboardHandler(monitor, logger)
			writer = &plainResponseWriter{header: make(http.Header)}
			request = httptest.NewRequest(http.MethodGet, "/api/events", nil)
		}) {
			return
		}

		t.Run("When the event stream handles the request", func(t *testing.T) {
			handler.serveEvents(writer, request)
		})

		t.Run("Then the stream rejects a response writer without flush support", func(t *testing.T) {
			if writer.status != http.StatusInternalServerError {
				t.Fatalf("event stream returns status %d", writer.status)
			}
		})
	})
}
func TestDashboard_LogResponseWriteFailure(t *testing.T) {
	testCases := []struct {
		name string
		run  func(*dashboardHandler, http.ResponseWriter)
		want string
	}{
		{
			name: "the writer rejects an embedded dashboard asset",
			run: func(handler *dashboardHandler, writer http.ResponseWriter) {
				handler.serveDashboard(writer, nil)
			},
			want: "The dashboard asset handler cannot write the asset.",
		},
		{
			name: "the writer rejects the dependency graph",
			run: func(handler *dashboardHandler, writer http.ResponseWriter) {
				handler.serveGraph(writer, nil)
			},
			want: "The graph endpoint cannot write the dependency graph.",
		},
		{
			name: "the writer rejects the health response",
			run: func(handler *dashboardHandler, writer http.ResponseWriter) {
				handler.serveHealth(writer, nil)
			},
			want: "The dashboard handler cannot write the health response.",
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var logs bytes.Buffer
			var handler *dashboardHandler
			var writer *failingResponseWriter

			if !t.Run("Given a dashboard handler and a response writer that fails", func(step *testing.T) {
				repositoryRoot := newAnalyzerFixture(t)
				sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
				if err != nil {
					step.Fatalf("newAnalyzer fails: %v", err)
				}
				logger := slog.New(slog.NewTextHandler(
					&logs,
					&slog.HandlerOptions{Level: slog.LevelDebug},
				))
				monitor, err := newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
				if err != nil {
					step.Fatalf("newGraphMonitor fails: %v", err)
				}
				handler = newDashboardHandler(monitor, logger)
				writer = &failingResponseWriter{header: make(http.Header)}
			}) {
				return
			}

			t.Run("When the handler writes the response", func(t *testing.T) {
				testCase.run(handler, writer)
			})

			t.Run("Then the structured log records the response failure", func(t *testing.T) {
				if !strings.Contains(logs.String(), testCase.want) ||
					!strings.Contains(logs.String(), errResponseWrite.Error()) {
					t.Fatalf("the structured log omits the write failure: %q", logs.String())
				}
			})
		})
	}
}

func TestGraphResponse_SelectCompression(t *testing.T) {
	testCases := []struct {
		name            string
		acceptEncoding  string
		wantCompression bool
	}{
		{name: "the client accepts gzip", acceptEncoding: "gzip", wantCompression: true},
		{
			name:            "the client accepts gzip after another coding",
			acceptEncoding:  "br, gzip; q=0.5",
			wantCompression: true,
		},
		{name: "the client refuses gzip", acceptEncoding: "gzip; q=0"},
		{name: "the client requests another coding", acceptEncoding: "br"},
		{name: "the client sends no coding"},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var recorder *httptest.ResponseRecorder
			var request *http.Request
			var graph Graph
			var writeError error
			var decoded Graph
			var decodeError error

			t.Run("Given one graph and the selected Accept-Encoding value", func(*testing.T) {
				graph = Graph{SchemaVersion: graphSchemaVersion, Revision: "revision"}
				recorder = httptest.NewRecorder()
				request = httptest.NewRequest(http.MethodGet, "/api/graph", nil)
				request.Header.Set("Accept-Encoding", testCase.acceptEncoding)
			})

			t.Run("When the server writes the graph response", func(*testing.T) {
				writeError = writeGraphResponse(recorder, request, graph)
				if writeError != nil {
					return
				}
				reader := io.Reader(recorder.Body)
				if testCase.wantCompression {
					compressedReader, err := gzip.NewReader(recorder.Body)
					if err != nil {
						decodeError = err
						return
					}
					defer func() {
						if err := compressedReader.Close(); err != nil && decodeError == nil {
							decodeError = err
						}
					}()
					reader = compressedReader
				}
				decodeError = json.NewDecoder(reader).Decode(&decoded)
			})

			if !t.Run("Then the response contains the graph", func(t *testing.T) {
				if writeError != nil {
					t.Fatalf("writeGraphResponse fails: %v", writeError)
				}
				if decodeError != nil {
					t.Fatalf("decode graph response: %v", decodeError)
				}
				if decoded.Revision != graph.Revision {
					t.Fatalf("decoded revision is %q, want %q", decoded.Revision, graph.Revision)
				}
			}) {
				return
			}

			t.Run("And the content encoding matches client support", func(t *testing.T) {
				compressed := recorder.Header().Get("Content-Encoding") == "gzip"
				if compressed != testCase.wantCompression {
					t.Errorf("gzip response is %t, want %t", compressed, testCase.wantCompression)
				}
			})
		})
	}
}

func TestGraphCompression_ParseQuality(t *testing.T) {
	testCases := []struct {
		name       string
		parameters string
		want       float64
	}{
		{name: "no parameter is present", want: 1},
		{name: "an unrelated parameter is present", parameters: "level=0.5", want: 1},
		{name: "a valid quality is present", parameters: "q=0.25", want: 0.25},
		{name: "the quality has surrounding space", parameters: " q = 0.5 ", want: 0.5},
		{name: "the quality is invalid", parameters: "q=invalid", want: 1},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var quality float64

			t.Run("Given content coding parameters", func(*testing.T) {})

			t.Run("When the server reads the gzip quality", func(*testing.T) {
				quality = gzipQuality(testCase.parameters)
			})

			t.Run("Then the server returns the expected quality", func(t *testing.T) {
				if quality != testCase.want {
					t.Errorf("gzip quality is %v, want %v", quality, testCase.want)
				}
			})
		})
	}
}
func TestDashboard_ReportUnavailableAsset(t *testing.T) {
	t.Run("Scenario: A client requests an embedded dashboard asset that does not exist", func(t *testing.T) {
		var logs bytes.Buffer
		var handler *dashboardAssetHandler
		var recorder *httptest.ResponseRecorder

		t.Run("Given a dashboard handler and an absent asset path", func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			handler = newDashboardAssetHandler(logger)
			recorder = httptest.NewRecorder()
		})

		t.Run("When the handler serves the embedded dashboard asset", func(t *testing.T) {
			handler.serveAsset(recorder, "_resources/web/missing.txt", "text/plain")
		})

		t.Run("Then the response reports an internal server error", func(t *testing.T) {
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("missing asset returns status %d", recorder.Code)
			}
		})

		t.Run("And the structured log records the absent dashboard asset", func(t *testing.T) {
			if !strings.Contains(logs.String(), "The dashboard asset handler cannot read the embedded asset.") {
				t.Errorf("the structured log omits the missing dashboard asset: %q", logs.String())
			}
		})
	})
}
func TestHTTPServer_ConfigureSafetyLimits(t *testing.T) {
	t.Run("Scenario: The constructor creates the dashboard HTTP server", func(t *testing.T) {
		var parentContext context.Context
		var handler *http.ServeMux
		var server *http.Server
		var baseContext context.Context
		var shutdownTimeout time.Duration

		t.Run("Given a parent context and an HTTP handler", func(t *testing.T) {
			parentContext = context.WithValue(t.Context(), dashboardContextKey("test"), "value")
			handler = http.NewServeMux()
		})

		t.Run("When the constructor creates the HTTP server configuration", func(t *testing.T) {
			server = newHTTPServer(parentContext, handler)
			baseContext = server.BaseContext(nil)
			shutdownTimeout = dashboardShutdownTimeout()
		})

		t.Run("Then the server contains bounded header and idle limits", func(t *testing.T) {
			if server.Handler != handler || server.ReadHeaderTimeout != 5*time.Second ||
				server.IdleTimeout != 75*time.Second || server.MaxHeaderBytes != 1<<20 {
				t.Fatalf("unexpected HTTP server configuration: %+v", server)
			}
		})

		t.Run("And connections inherit the parent context and shutdown limit", func(t *testing.T) {
			if baseContext != parentContext {
				t.Error("HTTP connections do not inherit the parent context")
			}
			if shutdownTimeout != 5*time.Second {
				t.Errorf("shutdown timeout is %s, want 5s", shutdownTimeout)
			}
		})
	})
}
func TestDashboard_RunWithCanceledContext(t *testing.T) {
	t.Run("Scenario: The caller cancels the server context before dashboard startup", func(t *testing.T) {
		var monitor *graphMonitor
		var logger *slog.Logger
		var serverContext context.Context
		var result error

		if !t.Run("Given a graph monitor and a context with an active cancellation", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger = slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			ctx, cancelServer := context.WithCancel(t.Context())
			cancelServer()
			serverContext = ctx
		}) {
			return
		}

		t.Run("When the dashboard server runs on a temporary port", func(t *testing.T) {
			result = runDashboard(serverContext, monitor, "127.0.0.1:0", logger)
		})

		t.Run("Then dashboard shutdown completes without an error", func(t *testing.T) {
			if result != nil {
				t.Fatalf("runDashboard fails during a normal shutdown: %v", result)
			}
		})
	})
}
func requestDashboardAsset(
	t *testing.T,
	client *http.Client,
	address string,
) (*http.Response, []byte) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, nil)
	if err != nil {
		t.Fatalf("create dashboard request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("request dashboard asset: %v", err)
	}
	payload, readError := io.ReadAll(response.Body)
	closeError := response.Body.Close()
	if readError != nil {
		t.Fatalf("read dashboard response: %v", readError)
	}
	if closeError != nil {
		t.Fatalf("close dashboard response: %v", closeError)
	}
	return response, payload
}
