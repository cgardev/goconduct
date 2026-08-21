package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

	refreshMutex sync.Mutex
	mutex        sync.RWMutex
	graph        Graph
	snapshot     string
	subscribers  map[chan string]struct{}
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
	moduleSumPath := filepath.Join(analyzer.repositoryRoot, "go.sum")
	if _, err := os.Stat(moduleSumPath); err == nil {
		paths = append(paths, moduleSumPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect repository module sum: %w", err)
	}

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
	if _, err := monitor.freshGraph(); err != nil {
		monitor.logger.Error("Cannot refresh dependency graph", "error", err)
	}
}

func (monitor *graphMonitor) freshGraph() (Graph, error) {
	monitor.refreshMutex.Lock()
	defer monitor.refreshMutex.Unlock()

	snapshot, err := monitor.analyzer.snapshot()
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

	graph, err := monitor.analyzer.analyze()
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
// {"version":1,"tested_at":"2026-08-21T11:41:22Z","module_hash":"32f587a6815aa65b4e01761f3547ef6eccc4ab4a112c9886657ba50ea8e0ee99","functions":[{"id":"func/newGraphMonitor","name":"newGraphMonitor","line":29,"end_line":50,"hash":"ec7120f2a00d34f9299c3593e1cc44f974920b46445af79ae3c1cf1624a4b361"},{"id":"func/analyzer.snapshot","name":"analyzer.snapshot","line":52,"end_line":84,"hash":"4ce2aa8085350031b42fb1ce4bc98c380d866a442aa26a843443c557010f7e0b"},{"id":"func/graphMonitor.run","name":"graphMonitor.run","line":86,"end_line":97,"hash":"cab622474b73943c652088fe48b1e7c3ea11c9bb68fca3e380330be8096d5bab"},{"id":"func/graphMonitor.refresh","name":"graphMonitor.refresh","line":99,"end_line":103,"hash":"8989d24d3f7fa609bf5b8699bd5a6a871c478f8f8ed52544a15c1cf2ad992b49"},{"id":"func/graphMonitor.freshGraph","name":"graphMonitor.freshGraph","line":105,"end_line":143,"hash":"55f7ecf22248516757431afa3daca921162b73400df87658000b487a0cf5cf0c"},{"id":"func/graphMonitor.currentGraph","name":"graphMonitor.currentGraph","line":145,"end_line":149,"hash":"5939912406ef09c7d030f1aab1796dc21b0581217094bad7ee3a7077a9904c86"},{"id":"func/graphMonitor.subscribe","name":"graphMonitor.subscribe","line":151,"end_line":163,"hash":"19418b77900730437dfabb88709efe54df4d1db3474130ed47d7b316ffaf9e74"}]}
// mutate4go-manifest-end
