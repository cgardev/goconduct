package application

import (
	"context"
	"fmt"

	"github.com/cgardev/goconduct/pkg/failure"
)

// CacheMode selects the source for a dependency graph.
type CacheMode string

const (
	// CacheModeAuto uses the cache. CacheModeAuto uses local analysis when the cache is unavailable.
	CacheModeAuto CacheMode = "auto"
	// CacheModeServer requires the active cache.
	CacheModeServer CacheMode = "server"
	// CacheModeLocal always calculates the graph locally.
	CacheModeLocal CacheMode = "local"
)

// AnalyzeGraphUseCaseParams contains the source configuration and cache mode.
type AnalyzeGraphUseCaseParams[Configuration any] struct {
	Analysis  Configuration
	CacheMode CacheMode
}

// AnalyzeGraphUseCase selects a graph source without transport dependencies.
type AnalyzeGraphUseCase[Configuration, Graph any] struct {
	sourceFactory GraphSourceFactory[Configuration, Graph]
	graphCache    GraphCache[Graph]
}

// NewAnalyzeGraphUseCase creates the graph analysis use case.
func NewAnalyzeGraphUseCase[Configuration, Graph any](
	sourceFactory GraphSourceFactory[Configuration, Graph],
	graphCache GraphCache[Graph],
) *AnalyzeGraphUseCase[Configuration, Graph] {
	return &AnalyzeGraphUseCase[Configuration, Graph]{
		sourceFactory: sourceFactory,
		graphCache:    graphCache,
	}
}

// Execute returns a graph from the selected cache or local source.
func (useCase *AnalyzeGraphUseCase[Configuration, Graph]) Execute(
	ctx context.Context,
	params AnalyzeGraphUseCaseParams[Configuration],
) (Graph, error) {
	var zeroGraph Graph
	if err := ValidateCacheMode(params.CacheMode); err != nil {
		return zeroGraph, err
	}
	source, err := useCase.sourceFactory.NewSource(params.Analysis)
	if err != nil {
		return zeroGraph, err
	}
	if params.CacheMode == CacheModeLocal {
		return source.Analyze(ctx)
	}
	graph, cacheError := useCase.graphCache.Load(ctx, source)
	if cacheError == nil {
		return graph, nil
	}
	if params.CacheMode == CacheModeServer {
		return zeroGraph, failure.New(
			failure.ErrUnavailable,
			"load the required active graph cache",
			cacheError,
		)
	}
	if err := ctx.Err(); err != nil {
		return zeroGraph, err
	}
	return source.Analyze(ctx)
}

// ValidateCacheMode validates that the cache mode is in the supported set.
func ValidateCacheMode(mode CacheMode) error {
	switch mode {
	case CacheModeAuto, CacheModeServer, CacheModeLocal:
		return nil
	default:
		return failure.New(
			failure.ErrValidation,
			fmt.Sprintf("cache mode %q must be auto, server, or local", mode),
			nil,
		)
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"4a37216a997ea16d17d2e9d9f446af5ed734c46a185f656bec1d329318b8c8a8","functions":[{"id":"func/NewAnalyzeGraphUseCase","name":"NewAnalyzeGraphUseCase","line":35,"end_line":43,"hash":"afb8d889ef750cdd75d044a1a2211571793729cc7840b1d6bb8a6d5c927bf46c"},{"id":"func/AnalyzeGraphUseCase.Execute","name":"AnalyzeGraphUseCase.Execute","line":46,"end_line":76,"hash":"1bea5e90759be8116ea5c9e5fc292c8171dd02a234c8fe6725308a00475c6934"},{"id":"func/ValidateCacheMode","name":"ValidateCacheMode","line":79,"end_line":90,"hash":"e249cea232322bda9d1ef7f5d2d34046cabcca5e238e27d5c7887f49e76e2964"}]}
// mutate4go-manifest-end
