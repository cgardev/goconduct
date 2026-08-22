package architecture

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

var errGraphRefresh = errors.New("graph refresh failure")

type dashboardContextKey string

type dashboardTransportSourceStub struct {
	mutex        sync.RWMutex
	graph        Graph
	freshResult  Graph
	freshCalls   int
	freshContext context.Context
	freshError   error
	cacheKey     string
	cacheError   error
	updates      chan string
}

func newDashboardTransportSourceStub(graph Graph) *dashboardTransportSourceStub {
	return &dashboardTransportSourceStub{
		graph:    graph,
		cacheKey: strings.Repeat("a", 64),
		updates:  make(chan string, 4),
	}
}

func (stub *dashboardTransportSourceStub) currentGraph() Graph {
	stub.mutex.RLock()
	defer stub.mutex.RUnlock()
	return stub.graph
}

func (stub *dashboardTransportSourceStub) freshGraph(ctx context.Context) (Graph, error) {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	stub.freshCalls++
	stub.freshContext = ctx
	if stub.freshError != nil {
		return Graph{}, stub.freshError
	}
	if stub.freshResult.SchemaVersion != 0 {
		stub.graph = stub.freshResult
	}
	return stub.graph, nil
}

func (stub *dashboardTransportSourceStub) subscribe() (<-chan string, func()) {
	return stub.updates, func() {}
}

func (stub *dashboardTransportSourceStub) graphCacheKey() (string, error) {
	stub.mutex.RLock()
	defer stub.mutex.RUnlock()
	return stub.cacheKey, stub.cacheError
}

func (stub *dashboardTransportSourceStub) publish(revision string) {
	stub.updates <- revision
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDashboardHandler_ServesAngularApplication(t *testing.T) {
	source := newDashboardTransportSourceStub(Graph{
		SchemaVersion: graphSchemaVersion,
		Revision:      strings.Repeat("1", 64),
		ModulePath:    "example.com/repository",
	})
	server := httptest.NewServer(newDashboardHandler(source, discardLogger()))
	t.Cleanup(server.Close)

	response, payload := requestDashboardResource(t, server.Client(), server.URL+"/")

	if response.StatusCode != http.StatusOK {
		t.Fatalf("dashboard status is %d, want %d", response.StatusCode, http.StatusOK)
	}
	document := string(payload)
	for _, marker := range []string{
		"<title>goconduct — Go component dependency map</title>",
		"main-",
		"styles-",
	} {
		if !strings.Contains(document, marker) {
			t.Errorf("dashboard document does not contain %q", marker)
		}
	}
	if strings.Contains(document, "dashboard.js") {
		t.Error("dashboard document still references the retired frontend")
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("dashboard cache policy is %q", response.Header.Get("Cache-Control"))
	}
	policy := response.Header.Get("Content-Security-Policy")
	for _, directive := range []string{
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"script-src 'self'",
	} {
		if !strings.Contains(policy, directive) {
			t.Errorf("content security policy does not contain %q", directive)
		}
	}
	if response.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("frame policy is %q", response.Header.Get("X-Frame-Options"))
	}
}

func TestDashboardHandler_ServesHealthAndLegacyGraph(t *testing.T) {
	revision := strings.Repeat("2", 64)
	source := newDashboardTransportSourceStub(Graph{
		SchemaVersion: graphSchemaVersion,
		Revision:      revision,
	})
	server := httptest.NewServer(newDashboardHandler(source, discardLogger()))
	t.Cleanup(server.Close)

	healthResponse, healthPayload := requestDashboardResource(
		t,
		server.Client(),
		server.URL+"/healthz",
	)
	if healthResponse.StatusCode != http.StatusOK || string(healthPayload) != "ok\n" {
		t.Fatalf("health response is status %d with payload %q", healthResponse.StatusCode, healthPayload)
	}

	graphResponse, graphPayload := requestDashboardResource(
		t,
		server.Client(),
		server.URL+"/api/graph",
	)
	if graphResponse.StatusCode != http.StatusOK {
		t.Fatalf("legacy graph status is %d, want %d", graphResponse.StatusCode, http.StatusOK)
	}
	var graph Graph
	if err := json.Unmarshal(graphPayload, &graph); err != nil {
		t.Fatalf("decode legacy graph: %v", err)
	}
	if graph.Revision != revision {
		t.Errorf("legacy graph revision is %q, want %q", graph.Revision, revision)
	}
}

func TestDashboardHandler_RejectsUnsupportedLegacyGraphMethod(t *testing.T) {
	source := newDashboardTransportSourceStub(Graph{SchemaVersion: graphSchemaVersion})
	handler := newDashboardHandler(source, discardLogger())
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/graph", nil)

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Errorf("unsupported graph method returns status %d", response.Code)
	}
}

func TestHTTPServer_ConfigureSafetyLimits(t *testing.T) {
	parentContext := context.WithValue(t.Context(), dashboardContextKey("test"), "value")
	handler := http.NewServeMux()

	server := newHTTPServer(parentContext, handler)

	if server.Handler != handler || server.ReadHeaderTimeout != 5*time.Second ||
		server.IdleTimeout != 75*time.Second || server.MaxHeaderBytes != 1<<20 {
		t.Fatalf("unexpected HTTP server configuration: %+v", server)
	}
	if server.BaseContext(nil) != parentContext {
		t.Error("HTTP connections do not inherit the parent context")
	}
	if dashboardShutdownTimeout() != 5*time.Second {
		t.Errorf("shutdown timeout is %s, want 5s", dashboardShutdownTimeout())
	}
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
	payload, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		t.Fatalf("read dashboard response: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close dashboard response: %v", closeErr)
	}
	return response, payload
}
