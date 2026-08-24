package architecture

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/cgardev/goconduct/internal/application"
	"github.com/cgardev/goconduct/pkg/failure"
)

type graphCacheFactory interface {
	NewGraphCache(address string, timeout time.Duration) application.GraphCache[Graph]
}

type dependencyGraphRuntime struct {
	sourceFactory graphSourceFactory
	cacheFactory  graphCacheFactory
	logger        *slog.Logger
}

var _ commandRuntime = (*dependencyGraphRuntime)(nil)

func newDependencyGraphRuntime(
	sourceFactory graphSourceFactory,
	cacheFactory graphCacheFactory,
	logger *slog.Logger,
) *dependencyGraphRuntime {
	return &dependencyGraphRuntime{
		sourceFactory: sourceFactory,
		cacheFactory:  cacheFactory,
		logger:        logger,
	}
}

func (runtime *dependencyGraphRuntime) analyze(
	ctx context.Context,
	configuration ApplicationConfiguration,
) (Graph, error) {
	sourceFactory := newDependencyPolicyGraphSourceFactory(
		runtime.sourceFactory,
		configuration.Architecture.Dependencies,
	)
	useCase := application.NewAnalyzeGraphUseCase[AnalysisConfiguration, Graph](
		sourceFactory,
		runtime.cacheFactory.NewGraphCache(
			configuration.Server.Address,
			configuration.Cache.RequestTimeout,
		),
	)
	return useCase.Execute(ctx, application.AnalyzeGraphUseCaseParams[AnalysisConfiguration]{
		Analysis:  configuration.Analysis,
		CacheMode: application.CacheMode(configuration.Cache.Mode),
	})
}

func (runtime *dependencyGraphRuntime) runDashboard(
	ctx context.Context,
	configuration ApplicationConfiguration,
) error {
	if configuration.Server.RefreshInterval < minimumRefreshInterval() {
		return failure.Validation(
			"refresh interval must be at least "+minimumRefreshInterval().String(),
			nil,
		)
	}
	sourceFactory := newDependencyPolicyGraphSourceFactory(
		runtime.sourceFactory,
		configuration.Architecture.Dependencies,
	)
	source, err := sourceFactory.NewMonitorSource(configuration.Analysis)
	if err != nil {
		return err
	}
	monitor, err := newGraphMonitor(
		ctx,
		source,
		configuration.Server.RefreshInterval,
		runtime.logger,
	)
	if err != nil {
		if isContextError(ctx, err) {
			return nil
		}
		return err
	}
	return runDashboard(ctx, monitor, configuration.Server.Address, runtime.logger)
}

func isContextError(ctx context.Context, err error) bool {
	contextError := ctx.Err()
	return contextError != nil && errors.Is(err, contextError)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"c06ba5e9344ea82415567e42608c718d271d69f703c15840946d2e928a5edf67","functions":[{"id":"func/newDependencyGraphRuntime","name":"newDependencyGraphRuntime","line":24,"end_line":34,"hash":"e9ce501159c431610ca58060d2fce347ee07de7842bd3bb953fe4a77082a857c"},{"id":"func/dependencyGraphRuntime.analyze","name":"dependencyGraphRuntime.analyze","line":36,"end_line":51,"hash":"e142883a44fbb13edae0f3afe564cdc1d4825fbbfb8fd72b44816b3774239cfb"},{"id":"func/dependencyGraphRuntime.runDashboard","name":"dependencyGraphRuntime.runDashboard","line":53,"end_line":80,"hash":"8e12fc10f8c94c7355ece5f717e4582321ff9bf42bdb7bd4b60e0407c7095bd4"},{"id":"func/isContextError","name":"isContextError","line":82,"end_line":85,"hash":"57e15a1597b77272d920a478349c7ce7fbe65d0aecc7a69138e24bcc74fe2497"}]}
// mutate4go-manifest-end
