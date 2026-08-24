package architecture

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	application "github.com/cgardev/goconduct/internal/application"
	querymodel "github.com/cgardev/goconduct/internal/query"
	"github.com/cgardev/goconduct/pkg/failure"
)

var errGraphCacheClose = errors.New("graph cache close failure")

var errGraphCacheIdentity = errors.New("graph cache identity failure")

type graphCacheIdentityStub struct {
	cacheKey string
	err      error
}

var _ application.GraphCacheIdentity = graphCacheIdentityStub{}

func (stub graphCacheIdentityStub) CacheKey() (string, error) {
	return stub.cacheKey, stub.err
}

type graphCacheRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip graphCacheRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

type graphCacheCloseErrorBody struct {
	io.Reader
}

func (graphCacheCloseErrorBody) Close() error {
	return errGraphCacheClose
}

func TestGraphCache_LoadCompatibleGraph(t *testing.T) {
	t.Run("Scenario: The active server has the graph for the requested analysis scope", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var identity application.GraphCacheIdentity
		var cache application.GraphCache[Graph]
		var monitor *graphMonitor
		var server *httptest.Server
		var requests atomic.Int64
		var graph Graph
		var loadError error

		if !t.Run("Given a server with one calculated graph", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			var err error
			sourceAnalyzer, err = newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			dashboard := newDashboardHandler(monitor, logger)
			server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				requests.Add(1)
				dashboard.ServeHTTP(response, request)
			}))
			t.Cleanup(server.Close)
			identity = &analyzerGraphSource{analyzer: sourceAnalyzer}
			cache = newHTTPGraphCache(
				strings.TrimPrefix(server.URL, "http://"),
				&http.Client{Timeout: time.Second},
			)
		}) {
			return
		}

		t.Run("When the CLI cache client requests the active graph", func(*testing.T) {
			graph, loadError = cache.Load(t.Context(), identity)
		})

		if !t.Run("Then the cache returns the graph without an error", func(t *testing.T) {
			if loadError != nil {
				t.Fatalf("load graph from cache: %v", loadError)
			}
			if graph.Revision != monitor.currentGraph().Revision {
				t.Fatalf("cached revision is %q, want %q", graph.Revision, monitor.currentGraph().Revision)
			}
		}) {
			return
		}

		t.Run("And the client makes exactly one server request", func(t *testing.T) {
			if requests.Load() != 1 {
				t.Errorf("graph cache requests are %d, want 1", requests.Load())
			}
		})
	})
}

func TestGraphCache_RejectInvalidIdentity(t *testing.T) {
	t.Run("Scenario: The analysis identity cannot create its cache key", func(t *testing.T) {
		var identity application.GraphCacheIdentity
		var cache application.GraphCache[Graph]
		var loadError error

		t.Run("Given a cache identity that returns an error", func(*testing.T) {
			identity = graphCacheIdentityStub{err: errGraphCacheIdentity}
			cache = newHTTPGraphCache(
				"127.0.0.1:6062",
				&http.Client{Timeout: time.Second},
			)
		})

		t.Run("When the cache loads the graph", func(*testing.T) {
			_, loadError = cache.Load(t.Context(), identity)
		})

		t.Run("Then the cache returns the identity error", func(t *testing.T) {
			if !errors.Is(loadError, errGraphCacheIdentity) {
				t.Fatalf("cache error is %v, want errGraphCacheIdentity", loadError)
			}
		})
	})
}

func TestGraphCache_RejectDifferentScope(t *testing.T) {
	t.Run("Scenario: The active server analyzes a different repository scope", func(t *testing.T) {
		var clientAnalyzer *analyzer
		var identity application.GraphCacheIdentity
		var cache application.GraphCache[Graph]
		var server *httptest.Server
		var loadError error

		if !t.Run("Given a server and client with different ignored paths", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			serverAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("build server analyzer: %v", err)
			}
			clientConfiguration := fixtureAnalysisConfiguration(repositoryRoot)
			clientConfiguration.IgnoredPaths = append(clientConfiguration.IgnoredPaths, "internal/library/logging")
			clientAnalyzer, err = newAnalyzer(clientConfiguration)
			if err != nil {
				step.Fatalf("build client analyzer: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err := newGraphMonitor(t.Context(), serverAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			server = httptest.NewServer(newDashboardHandler(monitor, logger))
			t.Cleanup(server.Close)
			identity = &analyzerGraphSource{analyzer: clientAnalyzer}
			cache = newHTTPGraphCache(
				strings.TrimPrefix(server.URL, "http://"),
				&http.Client{Timeout: time.Second},
			)
		}) {
			return
		}

		t.Run("When the client requests the graph with its scope key", func(*testing.T) {
			_, loadError = cache.Load(t.Context(), identity)
		})

		t.Run("Then the server rejects the incompatible cache", func(t *testing.T) {
			if !errors.Is(loadError, failure.ErrDataIntegrity) {
				t.Fatalf("cache error is %v, want ErrDataIntegrity", loadError)
			}
		})
	})
}

func TestGraphCache_RefreshChangedRepository(t *testing.T) {
	t.Run("Scenario: A source file changes before the next CLI query", func(t *testing.T) {
		var repositoryRoot string
		var sourceAnalyzer *analyzer
		var identity application.GraphCacheIdentity
		var cache application.GraphCache[Graph]
		var monitor *graphMonitor
		var server *httptest.Server
		var initialRevision string
		var graph Graph
		var loadError error

		if !t.Run("Given a server cache for a repository with one component", func(step *testing.T) {
			repositoryRoot = t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/cache\n\ngo 1.26\n")
			writeFixtureFile(step, repositoryRoot, "internal/library/first/first.go", "package first\n")
			var err error
			sourceAnalyzer, err = newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			initialRevision = monitor.currentGraph().Revision
			server = httptest.NewServer(newDashboardHandler(monitor, logger))
			t.Cleanup(server.Close)
			identity = &analyzerGraphSource{analyzer: sourceAnalyzer}
			cache = newHTTPGraphCache(
				strings.TrimPrefix(server.URL, "http://"),
				&http.Client{Timeout: time.Second},
			)
		}) {
			return
		}

		t.Run("When a second component is added and the CLI reads the cache", func(step *testing.T) {
			writeFixtureFile(step, repositoryRoot, "internal/library/second/second.go", "package second\n")
			graph, loadError = cache.Load(t.Context(), identity)
		})

		if !t.Run("Then the cache refresh succeeds", func(t *testing.T) {
			if loadError != nil {
				t.Fatalf("load graph from cache: %v", loadError)
			}
			if graph.Revision == initialRevision {
				t.Fatal("the cache returns the revision from before the source change")
			}
		}) {
			return
		}

		t.Run("And the graph contains the added component", func(t *testing.T) {
			if graph.Summary.Components != 2 {
				t.Errorf("cached component count is %d, want 2", graph.Summary.Components)
			}
		})
	})
}

func TestCLIQuery_UseActiveGraphCache(t *testing.T) {
	t.Run("Scenario: A summary query requires the compatible active server cache", func(t *testing.T) {
		var repositoryRoot string
		var server *httptest.Server
		var requests atomic.Int64
		var expectedRevision string
		var output bytes.Buffer
		var commandError error
		var result querymodel.SummaryResult

		if !t.Run("Given a compatible active graph server", func(step *testing.T) {
			repositoryRoot = newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err := newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			expectedRevision = monitor.currentGraph().Revision
			dashboard := newDashboardHandler(monitor, logger)
			server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				requests.Add(1)
				dashboard.ServeHTTP(response, request)
			}))
			t.Cleanup(server.Close)
		}) {
			return
		}

		t.Run("When the CLI executes the summary query in server mode", func(*testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			command := newTestRootCommand(logger)
			command.SetOut(&output)
			command.SetArgs([]string{
				"summary",
				"--root", repositoryRoot,
				"--address", strings.TrimPrefix(server.URL, "http://"),
				"--cache", string(CacheModeServer),
			})
			commandError = command.ExecuteContext(t.Context())
			if commandError == nil {
				commandError = json.Unmarshal(output.Bytes(), &result)
			}
		})

		if !t.Run("Then the CLI returns the cached revision", func(t *testing.T) {
			if commandError != nil {
				t.Fatalf("cached summary query fails: %v", commandError)
			}
			if result.Analysis.Revision != expectedRevision {
				t.Fatalf("summary revision is %q, want %q", result.Analysis.Revision, expectedRevision)
			}
		}) {
			return
		}

		t.Run("And the query reaches the server once", func(t *testing.T) {
			if requests.Load() != 1 {
				t.Errorf("summary cache requests are %d, want 1", requests.Load())
			}
		})
	})
}

func TestCLIQuery_SelectCacheFailureBehavior(t *testing.T) {
	testCases := []struct {
		name      string
		mode      CacheMode
		address   string
		wantError error
	}{
		{
			name:    "automatic mode calculates locally when the server is unavailable",
			mode:    CacheModeAuto,
			address: "127.0.0.1:0",
		},
		{
			name:      "server mode reports an unavailable server",
			mode:      CacheModeServer,
			address:   "127.0.0.1:0",
			wantError: failure.ErrUnavailable,
		},
		{
			name:    "local mode does not parse the cache address",
			mode:    CacheModeLocal,
			address: "invalid address",
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var repositoryRoot string
			var output bytes.Buffer
			var commandError error

			t.Run("Given a valid local repository and the selected cache mode", func(*testing.T) {
				repositoryRoot = newAnalyzerFixture(t)
			})

			t.Run("When the CLI executes a summary query", func(*testing.T) {
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				command := newTestRootCommand(logger)
				command.SetOut(&output)
				command.SetArgs([]string{
					"summary",
					"--root", repositoryRoot,
					"--address", testCase.address,
					"--cache", string(testCase.mode),
					"--cache-timeout", "100ms",
				})
				commandError = command.ExecuteContext(t.Context())
			})

			t.Run("Then the query returns the expected cache result", func(t *testing.T) {
				if testCase.wantError != nil {
					if !errors.Is(commandError, testCase.wantError) {
						t.Fatalf("query error is %v, want %v", commandError, testCase.wantError)
					}
					return
				}
				if commandError != nil {
					t.Fatalf("summary query fails: %v", commandError)
				}
				if !json.Valid(output.Bytes()) {
					t.Fatalf("summary output is not JSON: %s", output.String())
				}
			})
		})
	}
}

func TestGraphCache_ValidateResponseContract(t *testing.T) {
	testCases := []struct {
		name         string
		status       int
		protocol     string
		key          string
		schema       string
		body         string
		wantCategory error
	}{
		{
			name: "the server status is not successful", status: http.StatusConflict,
			wantCategory: failure.ErrUnavailable,
		},
		{
			name: "the server rejects the analysis precondition", status: http.StatusPreconditionFailed,
			wantCategory: failure.ErrDataIntegrity,
		},
		{
			name: "the protocol header is absent", status: http.StatusOK,
			wantCategory: failure.ErrDataIntegrity,
		},
		{
			name: "the cache key does not match", status: http.StatusOK,
			protocol: "1", key: "other", wantCategory: failure.ErrDataIntegrity,
		},
		{
			name: "the schema header does not match", status: http.StatusOK,
			protocol: "1", key: "expected", schema: "5",
			wantCategory: failure.ErrDataIntegrity,
		},
		{
			name:     "the response body is not JSON",
			status:   http.StatusOK,
			protocol: "1",
			key:      "expected",
			schema:   strconv.Itoa(graphSchemaVersion),
			body:     "not-json",
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var response *http.Response
			var validationError error

			t.Run("Given a graph cache HTTP response", func(*testing.T) {
				response = &http.Response{
					StatusCode: testCase.status,
					Status:     http.StatusText(testCase.status),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(testCase.body)),
				}
				response.Header.Set(graphCacheProtocolHeader, testCase.protocol)
				response.Header.Set(graphCacheKeyHeader, testCase.key)
				response.Header.Set(graphCacheSchemaHeader, testCase.schema)
			})

			t.Run("When the client validates the response", func(*testing.T) {
				validationError = validateGraphCacheResponse(response, "expected")
			})

			t.Run("Then the response has the expected error category", func(t *testing.T) {
				if testCase.wantCategory != nil && !errors.Is(validationError, testCase.wantCategory) {
					t.Fatalf("validation error is %v, want %v", validationError, testCase.wantCategory)
				}
				if testCase.wantCategory == nil && validationError != nil {
					t.Fatalf("response validation fails: %v", validationError)
				}
			})
		})
	}
}

func TestGraphCacheClientHost_SelectReachableAddress(t *testing.T) {
	testCases := []struct {
		name string
		host string
		want string
	}{
		{name: "an empty listener host", host: "", want: "127.0.0.1"},
		{name: "an unspecified IPv4 listener", host: "0.0.0.0", want: "127.0.0.1"},
		{name: "an unspecified IPv6 listener", host: "::", want: "::1"},
		{name: "a named listener", host: "localhost", want: "localhost"},
		{name: "a specific listener", host: "192.0.2.1", want: "192.0.2.1"},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var result string

			t.Run("Given a configured server listener host", func(*testing.T) {})

			t.Run("When the cache client selects a connection host", func(*testing.T) {
				result = graphCacheClientHost(testCase.host)
			})

			t.Run("Then the client uses the reachable address", func(t *testing.T) {
				if result != testCase.want {
					t.Errorf("client host is %q, want %q", result, testCase.want)
				}
			})
		})
	}
}

func TestGraphCache_DetectInvalidPayload(t *testing.T) {
	testCases := []struct {
		name       string
		graph      Graph
		body       string
		suffix     string
		wantReject bool
		wantText   string
	}{
		{
			name:     "the response body is not JSON",
			body:     "not-json",
			wantText: "decode active graph",
		},
		{
			name:       "the body uses a different graph schema",
			graph:      Graph{SchemaVersion: graphSchemaVersion - 1, Revision: "revision"},
			wantReject: true,
		},
		{
			name:       "the body has no graph revision",
			graph:      Graph{SchemaVersion: graphSchemaVersion},
			wantReject: true,
		},
		{
			name:     "the body contains a second JSON value",
			graph:    Graph{SchemaVersion: graphSchemaVersion, Revision: "revision"},
			suffix:   `{}`,
			wantText: "more than one JSON value",
		},
		{
			name:     "the body ends with incomplete JSON",
			graph:    Graph{SchemaVersion: graphSchemaVersion, Revision: "revision"},
			suffix:   `{`,
			wantText: "check active graph response for another JSON value",
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var identity application.GraphCacheIdentity
			var cache application.GraphCache[Graph]
			var server *httptest.Server
			var loadError error

			if !t.Run("Given a cache server with the selected response body", func(step *testing.T) {
				const cacheKey = "analysis-scope"
				identity = graphCacheIdentityStub{cacheKey: cacheKey}
				body := testCase.body
				if body == "" {
					payload, encodeError := json.Marshal(testCase.graph)
					if encodeError != nil {
						step.Fatalf("encode cache graph: %v", encodeError)
					}
					body = string(payload) + testCase.suffix
				}
				server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
					response.Header().Set(graphCacheProtocolHeader, "1")
					response.Header().Set(graphCacheKeyHeader, cacheKey)
					response.Header().Set(graphCacheSchemaHeader, strconv.Itoa(graphSchemaVersion))
					if _, writeError := io.WriteString(response, body); writeError != nil {
						t.Errorf("write cache response: %v", writeError)
					}
				}))
				t.Cleanup(server.Close)
				cache = newHTTPGraphCache(
					strings.TrimPrefix(server.URL, "http://"),
					&http.Client{Timeout: time.Second},
				)
			}) {
				return
			}

			t.Run("When the client loads the graph", func(*testing.T) {
				_, loadError = cache.Load(t.Context(), identity)
			})

			t.Run("Then the client reports the selected invalid payload", func(t *testing.T) {
				if testCase.wantReject && !errors.Is(loadError, failure.ErrDataIntegrity) {
					t.Fatalf("cache error is %v, want ErrDataIntegrity", loadError)
				}
				if testCase.wantText != "" &&
					(loadError == nil || !strings.Contains(loadError.Error(), testCase.wantText)) {
					t.Fatalf("cache error is %v, want text %q", loadError, testCase.wantText)
				}
			})
		})
	}
}

func TestGraphCacheURL_NormalizeListenerAddress(t *testing.T) {
	testCases := []struct {
		name    string
		address string
		want    string
		wantErr bool
	}{
		{name: "an empty host", address: ":6062", want: "http://127.0.0.1:6062/api/graph"},
		{name: "an unspecified IPv4 host", address: "0.0.0.0:6062", want: "http://127.0.0.1:6062/api/graph"},
		{name: "an unspecified IPv6 host", address: "[::]:6062", want: "http://[::1]:6062/api/graph"},
		{name: "an invalid listener address", address: "invalid", wantErr: true},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var endpoint string
			var parseError error

			t.Run("Given a configured dashboard listener address", func(*testing.T) {})

			t.Run("When the cache client creates its endpoint", func(*testing.T) {
				endpoint, parseError = graphCacheURL(testCase.address)
			})

			t.Run("Then the endpoint has the expected result", func(t *testing.T) {
				if (parseError != nil) != testCase.wantErr {
					t.Fatalf("endpoint error is %v, want error %t", parseError, testCase.wantErr)
				}
				if !testCase.wantErr && endpoint != testCase.want {
					t.Errorf("cache endpoint is %q, want %q", endpoint, testCase.want)
				}
			})
		})
	}
}

func TestGraphCacheKey_IncludeGoExecutionContext(t *testing.T) {
	t.Run("Scenario: The Go build flags change for the same analysis scope", func(t *testing.T) {
		var identity application.GraphCacheIdentity
		var initialKey string
		var changedKey string
		var keyError error

		if !t.Run("Given one analyzer and no Go build flags", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			var err error
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			identity = &analyzerGraphSource{analyzer: sourceAnalyzer}
			t.Setenv("GOFLAGS", "")
		}) {
			return
		}

		t.Run("When the key is calculated before and after a build tag change", func(*testing.T) {
			initialKey, keyError = identity.CacheKey()
			if keyError != nil {
				return
			}
			t.Setenv("GOFLAGS", "-tags=cachetest")
			changedKey, keyError = identity.CacheKey()
		})

		if !t.Run("Then both key calculations succeed", func(t *testing.T) {
			if keyError != nil {
				t.Fatalf("graphCacheKey fails: %v", keyError)
			}
		}) {
			return
		}

		t.Run("And the build context changes the cache identity", func(t *testing.T) {
			if initialKey == changedKey {
				t.Errorf("cache key remains %q after a Go build flag change", initialKey)
			}
		})
	})
}

func TestGraphCache_ReportResponseCloseFailure(t *testing.T) {
	t.Run("Scenario: The cache response closes with an error after valid JSON is read", func(t *testing.T) {
		var identity application.GraphCacheIdentity
		var cache application.GraphCache[Graph]
		var client *http.Client
		var loadError error

		if !t.Run("Given a valid cache response body that fails during close", func(step *testing.T) {
			const cacheKey = "analysis-scope"
			identity = graphCacheIdentityStub{cacheKey: cacheKey}
			payload, err := json.Marshal(Graph{
				SchemaVersion: graphSchemaVersion,
				Revision:      "revision",
			})
			if err != nil {
				step.Fatalf("encode graph response: %v", err)
			}
			client = &http.Client{Transport: graphCacheRoundTripper(func(*http.Request) (*http.Response, error) {
				header := make(http.Header)
				header.Set(graphCacheProtocolHeader, "1")
				header.Set(graphCacheKeyHeader, cacheKey)
				header.Set(graphCacheSchemaHeader, strconv.Itoa(graphSchemaVersion))
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     header,
					Body:       graphCacheCloseErrorBody{Reader: bytes.NewReader(payload)},
				}, nil
			})}
			cache = &httpGraphCache{address: "127.0.0.1:6062", client: client}
		}) {
			return
		}

		t.Run("When the client loads and closes the graph response", func(*testing.T) {
			_, loadError = cache.Load(t.Context(), identity)
		})

		t.Run("Then the client returns the close error", func(t *testing.T) {
			if !errors.Is(loadError, errGraphCacheClose) {
				t.Fatalf("cache error is %v, want errGraphCacheClose", loadError)
			}
		})
	})
}
