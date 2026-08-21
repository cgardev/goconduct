package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type graphMonitor struct {
	analyzer        *analyzer
	refreshInterval time.Duration
	logger          *slog.Logger

	mutex       sync.RWMutex
	graph       Graph
	snapshot    string
	subscribers map[chan string]struct{}
}

func newGraphMonitor(
	analyzer *analyzer,
	refreshInterval time.Duration,
	logger *slog.Logger,
) (*graphMonitor, error) {
	snapshot, err := analyzer.snapshot()
	if err != nil {
		return nil, err
	}
	graph, err := analyzer.analyze()
	if err != nil {
		return nil, err
	}
	return &graphMonitor{
		analyzer:        analyzer,
		refreshInterval: refreshInterval,
		logger:          logger,
		graph:           graph,
		snapshot:        snapshot,
		subscribers:     make(map[chan string]struct{}),
	}, nil
}

func (analyzer *analyzer) snapshot() (string, error) {
	paths, err := analyzer.sourcePaths()
	if err != nil {
		return "", err
	}
	paths = append(paths, filepath.Join(analyzer.repositoryRoot, "go.mod"))

	var buffer []byte
	for _, path := range paths {
		information, err := os.Stat(path)
		if err != nil {
			return "", fmt.Errorf("inspect repository source file %s: %w", path, err)
		}
		relativePath, err := filepath.Rel(analyzer.repositoryRoot, path)
		if err != nil {
			return "", fmt.Errorf("resolve repository source file %s: %w", path, err)
		}
		buffer = append(buffer, filepath.ToSlash(relativePath)...)
		buffer = append(buffer, "\x00"...)
		buffer = strconv.AppendInt(buffer, information.Size(), 10)
		buffer = append(buffer, "\x00"...)
		buffer = strconv.AppendInt(buffer, information.ModTime().UnixNano(), 10)
		buffer = append(buffer, '\n')
	}
	digest := sha256.Sum256(buffer)
	return hex.EncodeToString(digest[:]), nil
}

func (monitor *graphMonitor) run(ctx context.Context) {
	ticker := time.NewTicker(monitor.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			monitor.refresh()
		}
	}
}

func (monitor *graphMonitor) refresh() {
	snapshot, err := monitor.analyzer.snapshot()
	if err != nil {
		monitor.logger.Error("Cannot inspect repository changes", "error", err)
		return
	}

	monitor.mutex.RLock()
	unchanged := snapshot == monitor.snapshot
	monitor.mutex.RUnlock()
	if unchanged {
		return
	}

	graph, err := monitor.analyzer.analyze()
	if err != nil {
		monitor.logger.Error("Cannot refresh dependency graph", "error", err)
		return
	}

	monitor.mutex.Lock()
	monitor.snapshot = snapshot
	if graph.Revision == monitor.graph.Revision {
		monitor.mutex.Unlock()
		return
	}
	monitor.graph = graph
	for subscriber := range monitor.subscribers {
		select {
		case subscriber <- graph.Revision:
		default:
		}
	}
	monitor.mutex.Unlock()
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T09:16:30Z","module_hash":"790eb690ac80cd17cb9dd80b1d3ae090e3cc4565a07e8253c80bb6c2a35d7bb9","functions":[{"id":"func/newGraphMonitor","name":"newGraphMonitor","line":27,"end_line":48,"hash":"ec7120f2a00d34f9299c3593e1cc44f974920b46445af79ae3c1cf1624a4b361"},{"id":"func/analyzer.snapshot","name":"analyzer.snapshot","line":50,"end_line":76,"hash":"d041821b22965e54d26f7c4bc74cf3564a21be60caf676f81ab85927c3b9cd3d"},{"id":"func/graphMonitor.run","name":"graphMonitor.run","line":78,"end_line":89,"hash":"cab622474b73943c652088fe48b1e7c3ea11c9bb68fca3e380330be8096d5bab"},{"id":"func/graphMonitor.refresh","name":"graphMonitor.refresh","line":91,"end_line":125,"hash":"e8e752f5e96f826679dc1c314fb759182c2a079055f2d9e88466ebd7156d7fc8"},{"id":"func/graphMonitor.currentGraph","name":"graphMonitor.currentGraph","line":127,"end_line":131,"hash":"5939912406ef09c7d030f1aab1796dc21b0581217094bad7ee3a7077a9904c86"},{"id":"func/graphMonitor.subscribe","name":"graphMonitor.subscribe","line":133,"end_line":145,"hash":"19418b77900730437dfabb88709efe54df4d1db3474130ed47d7b316ffaf9e74"}]}
// mutate4go-manifest-end
