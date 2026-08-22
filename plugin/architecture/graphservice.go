package architecture

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"

	"github.com/cgardev/goconduct/failure"
	"github.com/cgardev/goconduct/internal/library/connecterror"
	goconductv1 "github.com/cgardev/goconduct/internal/protogen/v1"
	"github.com/cgardev/goconduct/internal/protogen/v1/goconductv1connect"
)

type graphService struct {
	goconductv1connect.UnimplementedGraphServiceHandler
	source            dashboardGraphSource
	logger            *slog.Logger
	heartbeatInterval time.Duration
}

var _ goconductv1connect.GraphServiceHandler = (*graphService)(nil)

func newGraphService(source dashboardGraphSource, logger *slog.Logger) *graphService {
	return &graphService{
		source:            source,
		logger:            logger,
		heartbeatInterval: 20 * time.Second,
	}
}

func newGraphServiceHandler(
	source dashboardGraphSource,
	logger *slog.Logger,
	options ...connect.HandlerOption,
) (string, http.Handler) {
	return goconductv1connect.NewGraphServiceHandler(
		newGraphService(source, logger),
		options...,
	)
}

func (service *graphService) GetGraph(
	ctx context.Context,
	request *connect.Request[goconductv1.GetGraphRequest],
) (*connect.Response[goconductv1.GetGraphResponse], error) {
	cacheKey, err := service.source.graphCacheKey()
	if err != nil {
		service.logger.Error(
			"The graph service cannot identify the dependency graph cache.",
			slog.Any("error", err),
		)
		return nil, connecterror.From(ctx, failure.Unavailable("identify dependency graph cache", err))
	}
	if err := validateGraphCacheContract(
		request.Msg.GetCacheKey(),
		request.Msg.GetCacheProtocol(),
		cacheKey,
	); err != nil {
		return nil, connecterror.From(ctx, err)
	}

	graph := service.source.currentGraph()
	if request.Msg.GetRefresh() {
		graph, err = service.source.freshGraph(ctx)
		if err != nil {
			service.logger.Error(
				"The graph service cannot refresh the dependency graph.",
				slog.Any("error", err),
			)
			return nil, connecterror.From(ctx, failure.Unavailable("refresh dependency graph", err))
		}
	}

	return connect.NewResponse(&goconductv1.GetGraphResponse{
		Graph:         graphToProto(graph),
		CacheKey:      cacheKey,
		CacheProtocol: graphCacheProtocolVersion,
	}), nil
}

func (service *graphService) WatchGraph(
	ctx context.Context,
	_ *connect.Request[goconductv1.WatchGraphRequest],
	stream *connect.ServerStream[goconductv1.WatchGraphResponse],
) error {
	updates, unsubscribe := service.source.subscribe()
	defer unsubscribe()

	if err := stream.Send(&goconductv1.WatchGraphResponse{
		Type:     goconductv1.GraphEventType_GRAPH_EVENT_TYPE_READY,
		Revision: service.source.currentGraph().Revision,
	}); err != nil {
		return connect.NewError(connect.CodeCanceled, fmt.Errorf("send ready graph event: %w", err))
	}

	heartbeat := time.NewTicker(service.heartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case revision, available := <-updates:
			if !available {
				return nil
			}
			if err := stream.Send(&goconductv1.WatchGraphResponse{
				Type:     goconductv1.GraphEventType_GRAPH_EVENT_TYPE_CHANGED,
				Revision: revision,
			}); err != nil {
				return connect.NewError(
					connect.CodeCanceled,
					fmt.Errorf("send changed graph event: %w", err),
				)
			}
		case <-heartbeat.C:
			if err := stream.Send(&goconductv1.WatchGraphResponse{
				Type:     goconductv1.GraphEventType_GRAPH_EVENT_TYPE_HEARTBEAT,
				Revision: service.source.currentGraph().Revision,
			}); err != nil {
				return connect.NewError(
					connect.CodeCanceled,
					fmt.Errorf("send graph heartbeat: %w", err),
				)
			}
		}
	}
}

func validateGraphCacheContract(requestedKey string, protocol uint32, expectedKey string) error {
	if requestedKey == "" {
		return nil
	}
	if protocol != graphCacheProtocolVersion {
		return failure.BusinessRule("cache protocol does not match", nil)
	}
	if requestedKey != expectedKey {
		return failure.BusinessRule("analysis scope does not match", nil)
	}
	return nil
}
