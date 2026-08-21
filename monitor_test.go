package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestMonitor_PublishChangedRevision(t *testing.T) {
	t.Run("Scenario: A production import is added while the monitor runs", func(t *testing.T) {
		var repositoryRoot string
		var monitor *graphMonitor
		var updates <-chan string
		var initialRevision string
		var revision string
		var graph Graph
		var received bool
		var err error

		if !t.Run("Given a running monitor with one subscriber", func(step *testing.T) {
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
				step.Fatalf("newAnalyzer failed: %v", analyzerError)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(sourceAnalyzer, 10*time.Millisecond, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor failed: %v", err)
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
				t.Fatal("the monitor did not publish the source change")
			}
			if revision == initialRevision {
				t.Fatal("the monitor published the unchanged revision")
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

		if !t.Run("Given a subscribed monitor and a pending source change", func(step *testing.T) {
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
				step.Fatalf("newAnalyzer failed: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor failed: %v", err)
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
			monitor.refresh()
			select {
			case revision = <-updates:
				received = true
			default:
			}
		})

		if !t.Run("Then the update remains buffered for the subscriber", func(t *testing.T) {
			if !received {
				t.Fatal("the monitor did not buffer the update")
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

	t.Run("Scenario: A subscriber is removed before a source refresh", func(t *testing.T) {
		var monitor *graphMonitor
		var updates <-chan string
		var received bool
		var revision string

		if !t.Run("Given an unsubscribed monitor and a pending source change", func(step *testing.T) {
			repositoryRoot := t.TempDir()
			writeFixtureFile(step, repositoryRoot, "go.mod", "module example.com/live\n\ngo 1.26\n")
			writeFixtureFile(step, repositoryRoot, "cmd/control/main.go", "package main\n")
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer failed: %v", err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			monitor, err = newGraphMonitor(sourceAnalyzer, time.Second, logger)
			if err != nil {
				step.Fatalf("newGraphMonitor failed: %v", err)
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
			monitor.refresh()
			select {
			case revision = <-updates:
				received = true
			default:
			}
		})

		t.Run("Then the removed subscriber receives no revision", func(t *testing.T) {
			if received {
				t.Fatalf("the removed subscriber received revision %q", revision)
			}
		})
	})
}
