package architecture

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"

	"connectrpc.com/connect"

	"github.com/cgardev/goconduct/failure"
)

type dashboardService struct {
	sourceFactory graphSourceFactory
	configuration ApplicationConfiguration
	logger        *slog.Logger

	mutex   sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	handler http.Handler
	options []connect.HandlerOption
}

// ConfigureHandlerOptions sets the shared Connect options before the first request.
func (service *dashboardService) ConfigureHandlerOptions(options ...connect.HandlerOption) error {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	if service.handler != nil {
		return failure.BusinessRule("architecture dashboard handler is already initialized", nil)
	}
	service.options = slices.Clone(options)
	return nil
}

var _ http.Handler = (*dashboardService)(nil)

func newDashboardService(
	sourceFactory graphSourceFactory,
	configuration ApplicationConfiguration,
	logger *slog.Logger,
) *dashboardService {
	return &dashboardService{
		sourceFactory: sourceFactory,
		configuration: configuration,
		logger:        logger,
	}
}

func (service *dashboardService) Activate(ctx context.Context) error {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	if service.ctx != nil {
		return failure.BusinessRule("architecture dashboard is already active", nil)
	}
	service.ctx, service.cancel = context.WithCancel(ctx)
	return nil
}

func (service *dashboardService) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	handler, err := service.initialize(request.Context())
	if err != nil {
		service.logger.Error("The architecture dashboard cannot initialize.", slog.Any("error", err))
		http.Error(response, "architecture dashboard unavailable", http.StatusServiceUnavailable)
		return
	}
	handler.ServeHTTP(response, request)
}

func (service *dashboardService) initialize(requestContext context.Context) (http.Handler, error) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	if service.handler != nil {
		return service.handler, nil
	}
	if service.ctx == nil {
		return nil, failure.BusinessRule("architecture dashboard is not active", nil)
	}
	if err := service.ctx.Err(); err != nil {
		return nil, fmt.Errorf("architecture dashboard context: %w", err)
	}
	if service.configuration.Server.RefreshInterval < minimumRefreshInterval() {
		return nil, failure.Validation(
			"refresh interval must be at least "+minimumRefreshInterval().String(),
			nil,
		)
	}
	sourceFactory := newDependencyPolicyGraphSourceFactory(
		service.sourceFactory,
		service.configuration.Architecture.Dependencies,
	)
	source, err := sourceFactory.NewMonitorSource(service.configuration.Analysis)
	if err != nil {
		return nil, err
	}
	monitor, err := newGraphMonitor(
		requestContext,
		source,
		service.configuration.Server.RefreshInterval,
		service.logger,
	)
	if err != nil {
		return nil, err
	}
	go monitor.run(service.ctx)
	service.handler = newDashboardHandler(monitor, service.logger, service.options...)
	return service.handler, nil
}

func (service *dashboardService) Shutdown() error {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	if service.cancel != nil {
		service.cancel()
		service.cancel = nil
	}
	return nil
}
