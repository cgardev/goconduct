package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cgardev/goconduct/pkg/failure"
)

var (
	errTestCache   = errors.New("test cache failure")
	errTestFactory = errors.New("test source factory failure")
)

type testGraph struct {
	revision string
}

type recordingGraphSource struct {
	graph          testGraph
	analyzeError   error
	analyzeCalls   int
	analyzeContext context.Context
	cacheKey       string
}

func (source *recordingGraphSource) Analyze(ctx context.Context) (testGraph, error) {
	source.analyzeCalls++
	source.analyzeContext = ctx
	return source.graph, source.analyzeError
}

func (source *recordingGraphSource) CacheKey() (string, error) {
	return source.cacheKey, nil
}

type recordingGraphSourceFactory struct {
	source        GraphSource[testGraph]
	createError   error
	createCalls   int
	configuration string
}

func (factory *recordingGraphSourceFactory) NewSource(
	configuration string,
) (GraphSource[testGraph], error) {
	factory.createCalls++
	factory.configuration = configuration
	return factory.source, factory.createError
}

type recordingGraphCache struct {
	graph       testGraph
	loadError   error
	loadCalls   int
	identityKey string
	afterLoad   func()
}

func (cache *recordingGraphCache) Load(
	_ context.Context,
	identity GraphCacheIdentity,
) (testGraph, error) {
	cache.loadCalls++
	key, err := identity.CacheKey()
	if err != nil {
		return testGraph{}, err
	}
	cache.identityKey = key
	if cache.afterLoad != nil {
		cache.afterLoad()
	}
	return cache.graph, cache.loadError
}

func TestAnalyzeGraphUseCase_Execute(t *testing.T) {
	testCases := []struct {
		name             string
		mode             CacheMode
		cacheGraph       testGraph
		cacheError       error
		wantGraph        testGraph
		wantCacheCalls   int
		wantAnalyzeCalls int
		wantError        error
		wantCacheError   bool
	}{
		{
			name:             "local mode calculates the graph without the cache",
			mode:             CacheModeLocal,
			wantGraph:        testGraph{revision: "local"},
			wantAnalyzeCalls: 1,
		},
		{
			name:           "automatic mode returns the compatible cached graph",
			mode:           CacheModeAuto,
			cacheGraph:     testGraph{revision: "cached"},
			wantGraph:      testGraph{revision: "cached"},
			wantCacheCalls: 1,
		},
		{
			name:             "automatic mode calculates the graph after a cache failure",
			mode:             CacheModeAuto,
			cacheError:       errTestCache,
			wantGraph:        testGraph{revision: "local"},
			wantCacheCalls:   1,
			wantAnalyzeCalls: 1,
		},
		{
			name:             "automatic mode calculates the graph after an internal cache cancellation",
			mode:             CacheModeAuto,
			cacheError:       fmt.Errorf("cache request: %w", context.Canceled),
			wantGraph:        testGraph{revision: "local"},
			wantCacheCalls:   1,
			wantAnalyzeCalls: 1,
		},
		{
			name:             "automatic mode calculates the graph after the cache request reaches its time limit",
			mode:             CacheModeAuto,
			cacheError:       fmt.Errorf("cache request: %w", context.DeadlineExceeded),
			wantGraph:        testGraph{revision: "local"},
			wantCacheCalls:   1,
			wantAnalyzeCalls: 1,
		},
		{
			name:           "server mode returns the cache failure",
			mode:           CacheModeServer,
			cacheError:     errTestCache,
			wantCacheCalls: 1,
			wantError:      failure.ErrUnavailable,
			wantCacheError: true,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var source *recordingGraphSource
			var factory *recordingGraphSourceFactory
			var cache *recordingGraphCache
			var useCase *AnalyzeGraphUseCase[string, testGraph]
			var executeContext context.Context
			var result testGraph
			var executeError error

			t.Run("Given a graph source and an active graph cache", func(*testing.T) {
				source = &recordingGraphSource{
					graph:    testGraph{revision: "local"},
					cacheKey: "scope-key",
				}
				factory = &recordingGraphSourceFactory{source: source}
				cache = &recordingGraphCache{
					graph:     testCase.cacheGraph,
					loadError: testCase.cacheError,
				}
				useCase = NewAnalyzeGraphUseCase[string, testGraph](factory, cache)
			})

			t.Run("When the use case analyzes the configured source", func(t *testing.T) {
				executeContext = t.Context()
				result, executeError = useCase.Execute(executeContext, AnalyzeGraphUseCaseParams[string]{
					Analysis:  "analysis-configuration",
					CacheMode: testCase.mode,
				})
			})

			t.Run("Then the use case returns the selected graph result", func(t *testing.T) {
				if !errors.Is(executeError, testCase.wantError) {
					t.Fatalf("execute error is %v, want %v", executeError, testCase.wantError)
				}
				if result != testCase.wantGraph {
					t.Fatalf("graph is %+v, want %+v", result, testCase.wantGraph)
				}
				if testCase.wantCacheError && !errors.Is(executeError, errTestCache) {
					t.Fatalf("execute error is %v, want errTestCache", executeError)
				}
			})

			t.Run("And the use case calls only the required ports", func(t *testing.T) {
				if factory.createCalls != 1 {
					t.Errorf("source creation calls are %d, want 1", factory.createCalls)
				}
				if factory.configuration != "analysis-configuration" {
					t.Errorf("source configuration is %q", factory.configuration)
				}
				if cache.loadCalls != testCase.wantCacheCalls {
					t.Errorf("cache calls are %d, want %d", cache.loadCalls, testCase.wantCacheCalls)
				}
				if source.analyzeCalls != testCase.wantAnalyzeCalls {
					t.Errorf(
						"analysis calls are %d, want %d",
						source.analyzeCalls,
						testCase.wantAnalyzeCalls,
					)
				}
				if source.analyzeCalls == 1 && source.analyzeContext != executeContext {
					t.Error("the graph source does not receive the execution context")
				}
				if testCase.wantCacheCalls == 1 && cache.identityKey != "scope-key" {
					t.Errorf("cache identity is %q, want scope-key", cache.identityKey)
				}
			})
		})
	}
}

func TestAnalyzeGraphUseCase_StopAutoFallbackAfterRequestCancellation(t *testing.T) {
	t.Run("Scenario: The request ends while the automatic cache request fails", func(t *testing.T) {
		var source *recordingGraphSource
		var useCase *AnalyzeGraphUseCase[string, testGraph]
		var executeContext context.Context
		var cancel context.CancelFunc
		var executeError error

		t.Run("Given a cache that cancels the request and returns a different failure", func(*testing.T) {
			source = &recordingGraphSource{graph: testGraph{revision: "local"}, cacheKey: "scope-key"}
			executeContext, cancel = context.WithCancel(t.Context())
			cache := &recordingGraphCache{
				loadError: errTestCache,
				afterLoad: cancel,
			}
			factory := &recordingGraphSourceFactory{source: source}
			useCase = NewAnalyzeGraphUseCase[string, testGraph](factory, cache)
		})

		t.Run("When automatic mode receives the cache failure", func(*testing.T) {
			_, executeError = useCase.Execute(executeContext, AnalyzeGraphUseCaseParams[string]{
				CacheMode: CacheModeAuto,
			})
		})

		t.Run("Then the use case returns the request cancellation", func(t *testing.T) {
			if !errors.Is(executeError, context.Canceled) {
				t.Fatalf("execute error is %v, want context.Canceled", executeError)
			}
		})

		t.Run("And the use case does not start local analysis", func(t *testing.T) {
			if source.analyzeCalls != 0 {
				t.Errorf("analysis calls are %d, want 0", source.analyzeCalls)
			}
		})
	})
}

func TestAnalyzeGraphUseCase_RejectInvalidMode(t *testing.T) {
	t.Run("Scenario: The request contains an unknown cache mode", func(t *testing.T) {
		var factory *recordingGraphSourceFactory
		var useCase *AnalyzeGraphUseCase[string, testGraph]
		var executeError error

		t.Run("Given a use case with valid graph ports", func(*testing.T) {
			factory = &recordingGraphSourceFactory{source: &recordingGraphSource{}}
			useCase = NewAnalyzeGraphUseCase[string, testGraph](factory, &recordingGraphCache{})
		})

		t.Run("When the use case receives the unknown cache mode", func(t *testing.T) {
			_, executeError = useCase.Execute(t.Context(), AnalyzeGraphUseCaseParams[string]{
				CacheMode: CacheMode("remote"),
			})
		})

		t.Run("Then the use case rejects the mode before source creation", func(t *testing.T) {
			if !errors.Is(executeError, failure.ErrValidation) {
				t.Fatalf("execute error is %v, want ErrValidation", executeError)
			}
			if !strings.Contains(executeError.Error(), `"remote"`) {
				t.Fatalf("execute error is %v, want the invalid mode", executeError)
			}
		})

		t.Run("And the source factory remains unused", func(t *testing.T) {
			if factory.createCalls != 0 {
				t.Errorf("source creation calls are %d, want 0", factory.createCalls)
			}
		})
	})
}

func TestAnalyzeGraphUseCase_ReturnSourceFactoryFailure(t *testing.T) {
	t.Run("Scenario: The graph source factory cannot create the configured source", func(t *testing.T) {
		var factory *recordingGraphSourceFactory
		var cache *recordingGraphCache
		var useCase *AnalyzeGraphUseCase[string, testGraph]
		var executeError error

		t.Run("Given a source factory that returns a typed failure", func(*testing.T) {
			factory = &recordingGraphSourceFactory{createError: errTestFactory}
			cache = &recordingGraphCache{}
			useCase = NewAnalyzeGraphUseCase[string, testGraph](factory, cache)
		})

		t.Run("When the use case starts local analysis", func(t *testing.T) {
			_, executeError = useCase.Execute(t.Context(), AnalyzeGraphUseCaseParams[string]{
				CacheMode: CacheModeLocal,
			})
		})

		t.Run("Then the use case returns the source factory failure", func(t *testing.T) {
			if !errors.Is(executeError, errTestFactory) {
				t.Fatalf("execute error is %v, want errTestFactory", executeError)
			}
		})

		t.Run("And the cache remains unused", func(t *testing.T) {
			if cache.loadCalls != 0 {
				t.Errorf("cache calls are %d, want 0", cache.loadCalls)
			}
		})
	})
}
