package architecture

import (
	"context"

	"github.com/cgardev/goconduct/internal/application"
)

type analyzerGraphSource struct {
	analyzer *analyzer
}

var _ application.GraphSource[Graph] = (*analyzerGraphSource)(nil)

func (source *analyzerGraphSource) Analyze(ctx context.Context) (Graph, error) {
	return source.analyzer.analyze(ctx)
}

func (source *analyzerGraphSource) CacheKey() (string, error) {
	return source.analyzer.graphCacheKey()
}

type analyzerGraphSourceFactory struct{}

var _ application.GraphSourceFactory[AnalysisConfiguration, Graph] = analyzerGraphSourceFactory{}

func (analyzerGraphSourceFactory) NewSource(
	configuration AnalysisConfiguration,
) (application.GraphSource[Graph], error) {
	graphAnalyzer, err := newAnalyzer(configuration)
	if err != nil {
		return nil, err
	}
	return &analyzerGraphSource{analyzer: graphAnalyzer}, nil
}

func (analyzerGraphSourceFactory) NewMonitorSource(
	configuration AnalysisConfiguration,
) (graphMonitorSource, error) {
	return newAnalyzer(configuration)
}

type graphSourceFactory interface {
	application.GraphSourceFactory[AnalysisConfiguration, Graph]
	NewMonitorSource(configuration AnalysisConfiguration) (graphMonitorSource, error)
}

var _ graphSourceFactory = analyzerGraphSourceFactory{}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"e0ba943c5f6105f11d3b455b36ad2ef23d733c81657890a6b6178c7e17f887fe","functions":[{"id":"func/analyzerGraphSource.Analyze","name":"analyzerGraphSource.Analyze","line":15,"end_line":17,"hash":"e40e70e8e7bc1010362f035aecd436ce8da124d17d637d35313aed796b1b4e2e"},{"id":"func/analyzerGraphSource.CacheKey","name":"analyzerGraphSource.CacheKey","line":19,"end_line":21,"hash":"2c5e9311a35c6eaf0f626d2a98e6f9ce98a75c969f661d705171717f8573b712"},{"id":"func/analyzerGraphSourceFactory.NewSource","name":"analyzerGraphSourceFactory.NewSource","line":27,"end_line":35,"hash":"421e7cbf64e023787e25fca107a541775e62449f0cb516639510370d65d78160"},{"id":"func/analyzerGraphSourceFactory.NewMonitorSource","name":"analyzerGraphSourceFactory.NewMonitorSource","line":37,"end_line":41,"hash":"08370910c34a4bf88c920e267e05941fc43669356537bce42890267c9ed596ef"}]}
// mutate4go-manifest-end
