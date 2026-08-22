package main

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	applicationconfiguration "github.com/cgardev/goconduct/cmd/goconduct/internal/configuration"
)

func TestApplicationEnforcesBootOrder(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := newApplication(logger)
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Shutdown(context.Background()); err != nil {
			t.Errorf("shut down application: %v", err)
		}
	})

	if _, err := app.ComposeServer(); err == nil || !strings.Contains(err.Error(), "not active") {
		t.Fatalf("compose inactive application: %v", err)
	}
	if err := app.Activate(t.Context(), applicationconfiguration.Default()); err != nil {
		t.Fatalf("activate application: %v", err)
	}
	if err := app.Activate(t.Context(), applicationconfiguration.Default()); err == nil {
		t.Fatal("second activation succeeds")
	}
	if _, err := app.ComposeServer(); err != nil {
		t.Fatalf("compose active application: %v", err)
	}
	if _, err := app.ComposeServer(); err == nil {
		t.Fatal("second server composition succeeds")
	}
}

func TestApplicationRejectsInvalidConfigurationBeforeActivation(t *testing.T) {
	app, err := newApplication(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Shutdown(context.Background()); err != nil {
			t.Errorf("shut down application: %v", err)
		}
	})
	configuration := applicationconfiguration.Default()
	configuration.Quality.Check.Plugins = []string{"missing"}
	if err := app.Activate(t.Context(), configuration); err == nil {
		t.Fatal("invalid configuration activates")
	}
}
