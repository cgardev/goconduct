package main

import (
	"context"

	application "digginginsights.com/v3/internal/devtool/dependencygraph/internal/application"
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
	sourceAnalyzer, err := newAnalyzer(configuration)
	if err != nil {
		return nil, err
	}
	return &analyzerGraphSource{analyzer: sourceAnalyzer}, nil
}

func newAnalyzeGraphUseCase(
	configuration ApplicationConfiguration,
) *application.AnalyzeGraphUseCase[AnalysisConfiguration, Graph] {
	return application.NewAnalyzeGraphUseCase[AnalysisConfiguration, Graph](
		analyzerGraphSourceFactory{},
		newHTTPGraphCache(
			configuration.Server.Address,
			configuration.Cache.RequestTimeout,
		),
	)
}

func analyzeGraph(
	ctx context.Context,
	configuration ApplicationConfiguration,
) (Graph, error) {
	useCase := newAnalyzeGraphUseCase(configuration)
	return useCase.Execute(ctx, application.AnalyzeGraphUseCaseParams[AnalysisConfiguration]{
		Analysis:  configuration.Analysis,
		CacheMode: application.CacheMode(configuration.Cache.Mode),
	})
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T14:59:38Z","module_hash":"2fce17001a96bc7b7b079b36ff32a89db557357550dec70a494c3ed1f9883944","functions":[{"id":"func/analyzerGraphSource.Analyze","name":"analyzerGraphSource.Analyze","line":15,"end_line":17,"hash":"0417f3e72aba71efd21cb41b73f40a00c10c88626ddace10562f5194edf62123"},{"id":"func/analyzerGraphSource.CacheKey","name":"analyzerGraphSource.CacheKey","line":19,"end_line":21,"hash":"2c5e9311a35c6eaf0f626d2a98e6f9ce98a75c969f661d705171717f8573b712"},{"id":"func/analyzerGraphSourceFactory.NewSource","name":"analyzerGraphSourceFactory.NewSource","line":27,"end_line":35,"hash":"0927e18fd040459a088271c05848971b5b68adc0dfecf05db571cfdb4b953572"},{"id":"func/newAnalyzeGraphUseCase","name":"newAnalyzeGraphUseCase","line":37,"end_line":47,"hash":"09cbd5d29cdde44a3aeb7e9f0236a3cb668bec191435023590377cfdb4234aa2"},{"id":"func/analyzeGraph","name":"analyzeGraph","line":49,"end_line":58,"hash":"5fe6bad449ca51faae958116c863a9aa6bb95dc081ff98c3b9e742830bce9c6c"}]}
// mutate4go-manifest-end
