package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type graphMonitorSource interface {
	analyze(context.Context) (Graph, error)
	snapshot(context.Context) (string, error)
	graphCacheKey() (string, error)
	repositoryPath() string
}

type graphMonitor struct {
	source          graphMonitorSource
	refreshInterval time.Duration
	logger          *slog.Logger

	refreshPermit chan struct{}
	mutex         sync.RWMutex
	graph         Graph
	snapshot      string
	subscribers   map[chan string]struct{}
}

var _ dashboardMonitor = (*graphMonitor)(nil)

func newGraphMonitor(
	ctx context.Context,
	source graphMonitorSource,
	refreshInterval time.Duration,
	logger *slog.Logger,
) (*graphMonitor, error) {
	snapshot, err := source.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	graph, err := source.analyze(ctx)
	if err != nil {
		return nil, err
	}
	return &graphMonitor{
		source:          source,
		refreshInterval: refreshInterval,
		logger:          logger,
		refreshPermit:   make(chan struct{}, 1),
		graph:           graph,
		snapshot:        snapshot,
		subscribers:     make(map[chan string]struct{}),
	}, nil
}

func (monitor *graphMonitor) run(ctx context.Context) {
	ticker := time.NewTicker(monitor.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			monitor.refresh(ctx)
		}
	}
}

func (monitor *graphMonitor) refresh(ctx context.Context) {
	if _, err := monitor.freshGraph(ctx); err != nil {
		monitor.logger.Error(
			"The graph monitor cannot refresh the dependency graph.",
			slog.Any("error", err),
		)
	}
}

func (monitor *graphMonitor) freshGraph(ctx context.Context) (Graph, error) {
	if err := monitor.acquireRefreshPermit(ctx); err != nil {
		return Graph{}, err
	}
	defer monitor.releaseRefreshPermit()

	snapshot, err := monitor.source.snapshot(ctx)
	if err != nil {
		return Graph{}, fmt.Errorf("inspect repository changes: %w", err)
	}

	monitor.mutex.RLock()
	unchanged := snapshot == monitor.snapshot
	current := monitor.graph
	monitor.mutex.RUnlock()
	if unchanged {
		return current, nil
	}

	graph, err := monitor.source.analyze(ctx)
	if err != nil {
		return Graph{}, fmt.Errorf("calculate dependency graph: %w", err)
	}

	monitor.mutex.Lock()
	monitor.snapshot = snapshot
	if graph.Revision == monitor.graph.Revision {
		current = monitor.graph
		monitor.mutex.Unlock()
		return current, nil
	}
	monitor.graph = graph
	for subscriber := range monitor.subscribers {
		select {
		case subscriber <- graph.Revision:
		default:
		}
	}
	monitor.mutex.Unlock()
	return graph, nil
}

func (monitor *graphMonitor) acquireRefreshPermit(ctx context.Context) error {
	select {
	case monitor.refreshPermit <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (monitor *graphMonitor) releaseRefreshPermit() {
	<-monitor.refreshPermit
}

func (monitor *graphMonitor) currentGraph() Graph {
	monitor.mutex.RLock()
	defer monitor.mutex.RUnlock()
	return monitor.graph
}

func (monitor *graphMonitor) subscribe() (<-chan string, func()) {
	updates := make(chan string, 1)
	monitor.mutex.Lock()
	monitor.subscribers[updates] = struct{}{}
	monitor.mutex.Unlock()

	unsubscribe := func() {
		monitor.mutex.Lock()
		delete(monitor.subscribers, updates)
		monitor.mutex.Unlock()
	}
	return updates, unsubscribe
}

func (monitor *graphMonitor) graphCacheKey() (string, error) {
	return monitor.source.graphCacheKey()
}

func (monitor *graphMonitor) repositoryPath() string {
	return monitor.source.repositoryPath()
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"68abb58211e9235bab02df1866404acf88e48d93f9637ddbdf0e1a2aa43d4731","functions":[{"id":"func/newGraphMonitor","name":"newGraphMonitor","line":32,"end_line":55,"hash":"82dca6600c2357e4d642ba06e91e78c554fce6a43eb18638ab6e2222c5dd7f03"},{"id":"func/graphMonitor.run","name":"graphMonitor.run","line":57,"end_line":68,"hash":"9f1ae09533866845261f585fbcff002a1693266e1d316febfef9ac3ee18913cd"},{"id":"func/graphMonitor.refresh","name":"graphMonitor.refresh","line":70,"end_line":77,"hash":"8e0be8fd1d5d44910cc842092cfad974941239dbe7aa1a4552bfb16f2ca0d8e0"},{"id":"func/graphMonitor.freshGraph","name":"graphMonitor.freshGraph","line":79,"end_line":119,"hash":"a8054dd96da7cfa4d18f5d7b277fff4a91b8aa2f12987ad3cccc7ebdaacf3d78"},{"id":"func/graphMonitor.acquireRefreshPermit","name":"graphMonitor.acquireRefreshPermit","line":121,"end_line":128,"hash":"2968a66e85a98dcb4b8db19402f0800406a7249b8f1bcb6f25cba78b97e02821"},{"id":"func/graphMonitor.releaseRefreshPermit","name":"graphMonitor.releaseRefreshPermit","line":130,"end_line":132,"hash":"bf77787869b8a3e49236441b744bd6ba716dffa4c362b9ba86652cc93590450a"},{"id":"func/graphMonitor.currentGraph","name":"graphMonitor.currentGraph","line":134,"end_line":138,"hash":"5939912406ef09c7d030f1aab1796dc21b0581217094bad7ee3a7077a9904c86"},{"id":"func/graphMonitor.subscribe","name":"graphMonitor.subscribe","line":140,"end_line":152,"hash":"19418b77900730437dfabb88709efe54df4d1db3474130ed47d7b316ffaf9e74"},{"id":"func/graphMonitor.graphCacheKey","name":"graphMonitor.graphCacheKey","line":154,"end_line":156,"hash":"0154b443b120fb5911fe750ed3b3bae439db3e796b6aed6ee1da752395b89624"},{"id":"func/graphMonitor.repositoryPath","name":"graphMonitor.repositoryPath","line":158,"end_line":160,"hash":"631b710830bd42b122ac7e472eaae302e2d35b5ad029c54c580f140956b5fcce"}]}
// mutate4go-manifest-end
