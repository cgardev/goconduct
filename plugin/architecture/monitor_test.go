package architecture

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMonitor_StopCanceledInitialAnalysis(t *testing.T) {
	t.Run("Scenario: The monitor context ends before the initial graph analysis", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var monitorContext context.Context
		var monitorError error

		t.Run("Given a repository analyzer and a canceled monitor context", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			configuration := fixtureAnalysisConfiguration(repositoryRoot)
			configuration.Paths = []string{"absent"}
			var err error
			sourceAnalyzer, err = newAnalyzer(configuration)
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			var cancel context.CancelFunc
			monitorContext, cancel = context.WithCancel(t.Context())
			cancel()
		})

		t.Run("When the graph monitor starts its initial analysis", func(*testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			_, monitorError = newGraphMonitor(monitorContext, sourceAnalyzer, time.Second, logger)
		})

		t.Run("Then the monitor returns the context cancellation", func(t *testing.T) {
			if !errors.Is(monitorError, context.Canceled) {
				t.Fatalf("monitor error is %v, want context.Canceled", monitorError)
			}
		})
	})
}

func TestMonitor_StopCanceledRefreshPermitWait(t *testing.T) {
	t.Run("Scenario: A graph refresh waits for another active refresh", func(t *testing.T) {
		var monitor *graphMonitor
		var refreshContext context.Context
		var cancelRefresh context.CancelFunc
		var refreshResult <-chan error
		var waiting bool

		if !t.Run("Given a monitor with an occupied refresh permit", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			monitor.refreshPermit <- struct{}{}
			t.Cleanup(func() { <-monitor.refreshPermit })
			refreshContext, cancelRefresh = context.WithCancel(t.Context())
			t.Cleanup(cancelRefresh)
		}) {
			return
		}

		t.Run("When the waiting refresh context ends", func(*testing.T) {
			result := make(chan error, 1)
			refreshResult = result
			go func() {
				_, err := monitor.freshGraph(refreshContext)
				result <- err
			}()
			select {
			case <-result:
			case <-time.After(20 * time.Millisecond):
				waiting = true
			}
			cancelRefresh()
		})

		if !t.Run("Then the refresh stops waiting for the permit", func(t *testing.T) {
			if !waiting {
				t.Fatal("the refresh does not wait for the occupied permit")
			}
			select {
			case refreshError := <-refreshResult:
				if !errors.Is(refreshError, context.Canceled) {
					t.Fatalf("refresh error is %v, want context.Canceled", refreshError)
				}
			case <-time.After(time.Second):
				t.Fatal("the refresh does not stop after context cancellation")
			}
		}) {
			return
		}

		t.Run("And the active refresh keeps its permit", func(t *testing.T) {
			if len(monitor.refreshPermit) != 1 {
				t.Fatalf("refresh permit count is %d, want 1", len(monitor.refreshPermit))
			}
		})
	})
}

func TestMonitor_ReportRefreshFailure(t *testing.T) {
	t.Run("Scenario: The repository module file disappears before a refresh", func(t *testing.T) {
		var repositoryRoot string
		var logs bytes.Buffer
		var monitor *graphMonitor

		if !t.Run("Given a monitor whose module file is no longer available", func(step *testing.T) {
			repositoryRoot = newAnalyzerFixture(t)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			monitor, err = newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			if err := os.Remove(filepath.Join(repositoryRoot, "go.mod")); err != nil {
				step.Fatalf("remove module fixture: %v", err)
			}
		}) {
			return
		}

		t.Run("When the monitor refreshes the graph", func(*testing.T) {
			monitor.refresh(t.Context())
		})

		t.Run("Then the structured log reports the refresh failure", func(t *testing.T) {
			if !strings.Contains(logs.String(), "The graph monitor cannot refresh the dependency graph.") {
				t.Fatalf("refresh log does not contain the failure: %q", logs.String())
			}
		})
	})
}

func TestMonitor_PublishChangedRevision(t *testing.T) {
	t.Run("Scenario: A source change adds a production import while the monitor runs", func(t *testing.T) {
		var repositoryRoot string
		var monitor *graphMonitor
		var updates <-chan string
		var initialRevision string
		var revision string
		var graph Graph
		var received bool
		var err error

		if !t.Run("Given an active monitor with one subscriber", func(step *testing.T) {
			repositoryRoot = t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/live\n\ngo 1.26\n")
			writeFixtureFile(step, repositoryRoot, "cmd/control/main.go", "package main\n")
			writeFixtureFile(
				step,
				repositoryRoot,
				"internal/library/logging/logging.go",
				"package logging\n",
			)
			sourceAnalyzer, analyzerError := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if analyzerError != nil {
				step.Fatalf("newAnalyzer fails: %v", analyzerError)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(t.Context(), sourceAnalyzer, 10*time.Millisecond, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			initialRevision = monitor.currentGraph().Revision
			var unsubscribe func()
			updates, unsubscribe = monitor.subscribe()
			t.Cleanup(unsubscribe)
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			go monitor.run(ctx)
		}) {
			return
		}

		t.Run("When a source change adds a production import", func(step *testing.T) {
			writeFixtureFile(
				step,
				repositoryRoot,
				"cmd/control/main.go",
				`package main

import _ "example.com/live/internal/library/logging"
`,
			)
			select {
			case revision = <-updates:
				received = true
				graph = monitor.currentGraph()
			case <-time.After(3 * time.Second):
			}
		})

		if !t.Run("Then the subscriber receives a new revision", func(t *testing.T) {
			if !received {
				t.Fatal("the monitor does not publish the source change")
			}
			if revision == initialRevision {
				t.Fatal("the monitor publishes the unchanged revision")
			}
		}) {
			return
		}

		t.Run("And the current graph contains the new production relationship", func(t *testing.T) {
			if graph.Revision != revision || graph.Summary.ProductionRelationships != 1 {
				t.Errorf("unexpected refreshed graph: revision %q, summary %+v", graph.Revision, graph.Summary)
			}
		})
	})
}
func TestMonitor_ManageSubscriptionDelivery(t *testing.T) {
	t.Run("Scenario: A subscriber reads after a synchronous refresh", func(t *testing.T) {
		var monitor *graphMonitor
		var updates <-chan string
		var revision string
		var received bool

		if !t.Run("Given a subscribed monitor and a source change before refresh", func(step *testing.T) {
			repositoryRoot := t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/live\n\ngo 1.26\n")
			writeFixtureFile(step, repositoryRoot, "cmd/control/main.go", "package main\n")
			writeFixtureFile(
				step,
				repositoryRoot,
				"internal/library/logging/logging.go",
				"package logging\n",
			)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			var unsubscribe func()
			updates, unsubscribe = monitor.subscribe()
			t.Cleanup(unsubscribe)
			writeFixtureFile(
				step,
				repositoryRoot,
				"cmd/control/main.go",
				"package main\n\nimport _ \"example.com/live/internal/library/logging\"\n",
			)
		}) {
			return
		}

		t.Run("When the monitor refreshes before the subscriber reads", func(t *testing.T) {
			monitor.refresh(t.Context())
			select {
			case revision = <-updates:
				received = true
			default:
			}
		})

		if !t.Run("Then the update remains buffered for the subscriber", func(t *testing.T) {
			if !received {
				t.Fatal("the monitor does not buffer the update")
			}
		}) {
			return
		}

		t.Run("And the buffered revision matches the current graph", func(t *testing.T) {
			if revision != monitor.currentGraph().Revision {
				t.Errorf(
					"buffered revision is %q, want %q",
					revision,
					monitor.currentGraph().Revision,
				)
			}
		})
	})

	t.Run("Scenario: The test removes a subscriber before a source refresh", func(t *testing.T) {
		var monitor *graphMonitor
		var updates <-chan string
		var received bool
		var revision string

		if !t.Run("Given a monitor with no subscriber and a source change before refresh", func(step *testing.T) {
			repositoryRoot := t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/live\n\ngo 1.26\n")
			writeFixtureFile(step, repositoryRoot, "cmd/control/main.go", "package main\n")
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(t.Context(), sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor fails: %v", err)
			}
			var unsubscribe func()
			updates, unsubscribe = monitor.subscribe()
			unsubscribe()
			writeFixtureFile(
				step,
				repositoryRoot,
				"internal/library/second/second.go",
				"package second\n",
			)
		}) {
			return
		}

		t.Run("When the monitor refreshes after unsubscription", func(t *testing.T) {
			monitor.refresh(t.Context())
			select {
			case revision = <-updates:
				received = true
			default:
			}
		})

		t.Run("Then the former subscriber receives no revision", func(t *testing.T) {
			if received {
				t.Fatalf("the former subscriber receives revision %q", revision)
			}
		})
	})
}
