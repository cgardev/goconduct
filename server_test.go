package main

import (
	"bytes"
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
)

type notifyingResponseRecorder struct {
	*httptest.ResponseRecorder
	flushed chan struct{}
}

var errResponseWrite = errors.New("response write failed")

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

func TestDashboard_ServeHTTPResources(t *testing.T) {
	t.Run("Scenario: A client requests the dashboard document", func(t *testing.T) {
		var server *httptest.Server
		var response *http.Response
		var document []byte

		if !t.Run("Given a dashboard server with a current graph", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(repositoryRoot)
			if err != nil {
				step.Fatalf("newAnalyzer failed: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err := newGraphMonitor(sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor failed: %v", err)
			}
			server = httptest.NewServer(newDashboardHandler(monitor, logger))
			t.Cleanup(server.Close)
		}) {
			return
		}

		t.Run("When the dashboard document is requested", func(step *testing.T) {
			response, document = requestDashboardResource(step, server.Client(), server.URL+"/")
		})

		if !t.Run("Then the self-contained document is returned", func(t *testing.T) {
			if response.StatusCode != http.StatusOK {
				t.Fatalf("dashboard returned status %d", response.StatusCode)
			}
			if !strings.Contains(string(document), "Strategic design map") {
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

		t.Run("And restrictive browser security headers are present", func(t *testing.T) {
			securityPolicy := response.Header.Get("Content-Security-Policy")
			if securityPolicy == "" || strings.Contains(securityPolicy, "unsafe-inline") {
				t.Errorf("unexpected content security policy %q", securityPolicy)
			}
		})
	})

	t.Run("Scenario: A client requests both embedded static assets", func(t *testing.T) {
		var server *httptest.Server
		var styleResponse *http.Response
		var scriptResponse *http.Response
		var style []byte
		var script []byte

		if !t.Run("Given a dashboard server with embedded assets", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(repositoryRoot)
			if err != nil {
				step.Fatalf("newAnalyzer failed: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err := newGraphMonitor(sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor failed: %v", err)
			}
			server = httptest.NewServer(newDashboardHandler(monitor, logger))
			t.Cleanup(server.Close)
		}) {
			return
		}

		t.Run("When the stylesheet and script are requested", func(step *testing.T) {
			styleResponse, style = requestDashboardResource(
				step,
				server.Client(),
				server.URL+"/assets/app.css",
			)
			scriptResponse, script = requestDashboardResource(
				step,
				server.Client(),
				server.URL+"/assets/app.js",
			)
		})

		t.Run("Then both non-empty assets are returned", func(t *testing.T) {
			if styleResponse.StatusCode != http.StatusOK || len(style) == 0 {
				t.Errorf(
					"style response is status %d with %d bytes",
					styleResponse.StatusCode,
					len(style),
				)
			}
			if scriptResponse.StatusCode != http.StatusOK || len(script) == 0 {
				t.Errorf(
					"script response is status %d with %d bytes",
					scriptResponse.StatusCode,
					len(script),
				)
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
			sourceAnalyzer, err := newAnalyzer(repositoryRoot)
			if err != nil {
				step.Fatalf("newAnalyzer failed: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor failed: %v", err)
			}
			server = httptest.NewServer(newDashboardHandler(monitor, logger))
			t.Cleanup(server.Close)
		}) {
			return
		}

		t.Run("When the graph endpoint is requested", func(step *testing.T) {
			var payload []byte
			response, payload = requestDashboardResource(step, server.Client(), server.URL+"/api/graph")
			decodeError = json.Unmarshal(payload, &graph)
		})

		if !t.Run("Then a valid graph response is returned", func(t *testing.T) {
			if response.StatusCode != http.StatusOK {
				t.Fatalf("graph endpoint returned status %d", response.StatusCode)
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
	})

	t.Run("Scenario: A client uses an unsupported method on the graph endpoint", func(t *testing.T) {
		var server *httptest.Server
		var status int
		var requestError error

		if !t.Run("Given a dashboard server with a current graph", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(repositoryRoot)
			if err != nil {
				step.Fatalf("newAnalyzer failed: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err := newGraphMonitor(sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor failed: %v", err)
			}
			server = httptest.NewServer(newDashboardHandler(monitor, logger))
			t.Cleanup(server.Close)
		}) {
			return
		}

		t.Run("When a POST request is sent to the graph endpoint", func(step *testing.T) {
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
				t.Fatalf("unsupported method request failed: %v", requestError)
			}
		}) {
			return
		}

		t.Run("And the server reports method not allowed", func(t *testing.T) {
			if status != http.StatusMethodNotAllowed {
				t.Errorf("unsupported method returned status %d", status)
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
			sourceAnalyzer, err := newAnalyzer(repositoryRoot)
			if err != nil {
				step.Fatalf("newAnalyzer failed: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor failed: %v", err)
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

		t.Run("When two source changes occur before the request is canceled", func(step *testing.T) {
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
				"package main\n\nimport _ \"example.com/strategic/internal/library/logging\"\n",
			)
			monitor.refresh()
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
			monitor.refresh()
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

		if !t.Run("Then every event is flushed and the stream stops", func(t *testing.T) {
			if !readyFlushed || !firstGraphFlushed || !secondGraphFlushed {
				t.Fatalf(
					"flush states are ready=%t, first=%t, second=%t",
					readyFlushed,
					firstGraphFlushed,
					secondGraphFlushed,
				)
			}
			if !stopped {
				t.Fatal("the event stream did not stop after request cancellation")
			}
			if recorder.Code != http.StatusOK {
				t.Fatalf("event stream returned status %d", recorder.Code)
			}
		}) {
			return
		}

		t.Run("And the stream contains the ready event and both graph events", func(t *testing.T) {
			if !strings.Contains(payload, "event: ready\n") ||
				!strings.Contains(payload, "data: "+monitor.currentGraph().Revision+"\n") {
				t.Errorf("event stream did not publish the current revision: %q", payload)
			}
			if graphEvents := strings.Count(payload, "event: graph\n"); graphEvents != 2 {
				t.Errorf("event stream published %d graph events, want 2: %q", graphEvents, payload)
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
			sourceAnalyzer, err := newAnalyzer(repositoryRoot)
			if err != nil {
				step.Fatalf("newAnalyzer failed: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err := newGraphMonitor(sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor failed: %v", err)
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

		t.Run("When the stream waits for one keep-alive and is canceled", func(t *testing.T) {
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

		if !t.Run("Then the ready event and keep-alive are flushed before shutdown", func(t *testing.T) {
			if flushCount != 2 {
				t.Fatalf("event stream flushed %d events, want 2", flushCount)
			}
			if !stopped {
				t.Fatal("the event stream did not stop")
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
	t.Run("Scenario: The response writer does not support event flushing", func(t *testing.T) {
		var handler *dashboardHandler
		var writer *plainResponseWriter
		var request *http.Request

		if !t.Run("Given an event request with a plain response writer", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(repositoryRoot)
			if err != nil {
				step.Fatalf("newAnalyzer failed: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err := newGraphMonitor(sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor failed: %v", err)
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

		t.Run("Then the stream reports that event streaming is unavailable", func(t *testing.T) {
			if writer.status != http.StatusInternalServerError {
				t.Fatalf("event stream returned status %d", writer.status)
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
			name: "an embedded asset cannot be written",
			run: func(handler *dashboardHandler, writer http.ResponseWriter) {
				handler.serveDashboard(writer, nil)
			},
			want: "Cannot write dashboard asset",
		},
		{
			name: "the graph cannot be written",
			run: func(handler *dashboardHandler, writer http.ResponseWriter) {
				handler.serveGraph(writer, nil)
			},
			want: "Cannot write dependency graph",
		},
		{
			name: "the health response cannot be written",
			run: func(handler *dashboardHandler, writer http.ResponseWriter) {
				handler.serveHealth(writer, nil)
			},
			want: "Cannot write health response",
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var logs bytes.Buffer
			var handler *dashboardHandler
			var writer *failingResponseWriter

			if !t.Run("Given a dashboard handler and a failing response writer", func(step *testing.T) {
				repositoryRoot := newAnalyzerFixture(t)
				sourceAnalyzer, err := newAnalyzer(repositoryRoot)
				if err != nil {
					step.Fatalf("newAnalyzer failed: %v", err)
				}
				logger := slog.New(slog.NewTextHandler(
					&logs,
					&slog.HandlerOptions{Level: slog.LevelDebug},
				))
				monitor, err := newGraphMonitor(sourceAnalyzer, time.Second, logger)
				if err != nil {
					step.Fatalf("newGraphMonitor failed: %v", err)
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
					t.Fatalf("write failure was not logged: %q", logs.String())
				}
			})
		})
	}
}
func TestDashboard_ReportUnavailableAsset(t *testing.T) {
	t.Run("Scenario: A requested embedded asset does not exist", func(t *testing.T) {
		var logs bytes.Buffer
		var handler *dashboardHandler
		var recorder *httptest.ResponseRecorder

		t.Run("Given a dashboard handler and a missing asset path", func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			handler = &dashboardHandler{logger: logger}
			recorder = httptest.NewRecorder()
		})

		t.Run("When the embedded asset is served", func(t *testing.T) {
			handler.serveEmbeddedAsset(recorder, "_resources/web/missing.txt", "text/plain")
		})

		t.Run("Then the response reports an internal server error", func(t *testing.T) {
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("missing asset returned status %d", recorder.Code)
			}
		})

		t.Run("And the missing asset is recorded in the structured log", func(t *testing.T) {
			if !strings.Contains(logs.String(), "Cannot read embedded dashboard asset") {
				t.Errorf("missing asset was not logged: %q", logs.String())
			}
		})
	})
}
func TestHTTPServer_ConfigureSafetyLimits(t *testing.T) {
	t.Run("Scenario: The dashboard HTTP server is constructed", func(t *testing.T) {
		var parentContext context.Context
		var handler *http.ServeMux
		var server *http.Server
		var baseContext context.Context
		var shutdownTimeout time.Duration

		t.Run("Given a parent context and an HTTP handler", func(t *testing.T) {
			parentContext = context.WithValue(t.Context(), dashboardContextKey("test"), "value")
			handler = http.NewServeMux()
		})

		t.Run("When the HTTP server configuration is created", func(t *testing.T) {
			server = newHTTPServer(parentContext, handler)
			baseContext = server.BaseContext(nil)
			shutdownTimeout = dashboardShutdownTimeout()
		})

		t.Run("Then bounded header and idle limits are configured", func(t *testing.T) {
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
	t.Run("Scenario: The server context is canceled before dashboard startup", func(t *testing.T) {
		var monitor *graphMonitor
		var logger *slog.Logger
		var serverContext context.Context
		var result error

		if !t.Run("Given a graph monitor and an already canceled context", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(repositoryRoot)
			if err != nil {
				step.Fatalf("newAnalyzer failed: %v", err)
			}
			logger = slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor failed: %v", err)
			}
			ctx, cancelServer := context.WithCancel(t.Context())
			cancelServer()
			serverContext = ctx
		}) {
			return
		}

		t.Run("When the dashboard server runs on an ephemeral port", func(t *testing.T) {
			result = runDashboard(serverContext, monitor, "127.0.0.1:0", logger)
		})

		t.Run("Then dashboard shutdown completes without an error", func(t *testing.T) {
			if result != nil {
				t.Fatalf("runDashboard failed during a normal shutdown: %v", result)
			}
		})
	})
}
func requestDashboardResource(
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
		t.Fatalf("request dashboard resource: %v", err)
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
