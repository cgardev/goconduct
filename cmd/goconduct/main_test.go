package main

import (
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestCompositionRootRegistersBuiltInPlugins(t *testing.T) {
	plugins := applicationPlugins()
	want := []string{"architecture", "coverage", "crap", "duplication", "mutation", "quality"}
	if len(plugins) != len(want) {
		t.Fatalf("built-in plugin count is %d", len(plugins))
	}
	for index, candidate := range plugins {
		if candidate.Name() != want[index] {
			t.Fatalf("built-in plugin %d is %q, want %q", index, candidate.Name(), want[index])
		}
	}
}

func TestRunCommandReturnsArgumentErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := runCommand(t.Context(), logger, []string{"unknown-command"})
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
}

func TestRootCommandUsesStableApplicationIdentity(t *testing.T) {
	command := newRootCommand()
	if command.Use != "goconduct" || command.SilenceErrors != true || command.SilenceUsage != true {
		t.Fatalf("unexpected root command: %+v", command)
	}
}
