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
	paths, err := goSourcePaths(analyzer.repositoryRoot)
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
