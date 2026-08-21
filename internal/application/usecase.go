package application

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrInvalidCacheMode identifies an unsupported graph cache mode.
	ErrInvalidCacheMode = errors.New("invalid graph cache mode")
	// ErrRequiredGraphCache identifies a failure from a required active cache.
	ErrRequiredGraphCache = errors.New("load active graph cache")
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
		return zeroGraph, fmt.Errorf("%w: %w", ErrRequiredGraphCache, cacheError)
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
		return fmt.Errorf(
			"%w: cache mode %q must be auto, server, or local",
			ErrInvalidCacheMode,
			mode,
		)
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T16:35:08Z","module_hash":"96f2de03ea9606372cada8b5b6def575c11f32cb929abbc54f280fa4b11249ed","functions":[{"id":"func/NewAnalyzeGraphUseCase","name":"NewAnalyzeGraphUseCase","line":41,"end_line":49,"hash":"afb8d889ef750cdd75d044a1a2211571793729cc7840b1d6bb8a6d5c927bf46c"},{"id":"func/AnalyzeGraphUseCase.Execute","name":"AnalyzeGraphUseCase.Execute","line":52,"end_line":78,"hash":"cfc695096cc2ae37d4f92e60dffb882548c0856af0f65ba0fff966eb3361b3a7"},{"id":"func/ValidateCacheMode","name":"ValidateCacheMode","line":81,"end_line":92,"hash":"1779f817f00dc805a674b48ec5b18f0b07b8721b292912e49ab38211a12b1473"}]}
// mutate4go-manifest-end
