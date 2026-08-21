// Package application selects the source for one dependency graph analysis.
package application

import "context"

// GraphCacheIdentity supplies the deterministic identity for one analysis scope.
type GraphCacheIdentity interface {
	CacheKey() (string, error)
}

// GraphSource calculates a graph and identifies its analysis scope.
type GraphSource[Graph any] interface {
	GraphCacheIdentity
	Analyze(ctx context.Context) (Graph, error)
}

// GraphSourceFactory creates a graph source for one analysis configuration.
type GraphSourceFactory[Configuration, Graph any] interface {
	NewSource(configuration Configuration) (GraphSource[Graph], error)
}

// GraphCache loads a compatible graph from an active cache.
type GraphCache[Graph any] interface {
	Load(ctx context.Context, identity GraphCacheIdentity) (Graph, error)
}
