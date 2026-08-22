package architecture

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDashboardServiceInitializesEmbeddedApplication(t *testing.T) {
	repositoryRoot := t.TempDir()
	writeDashboardFixture(t, filepath.Join(repositoryRoot, "go.mod"), "module example.com/dashboard\n\ngo 1.26.3\n")
	writeDashboardFixture(
		t,
		filepath.Join(repositoryRoot, "cmd", "dashboard", "main.go"),
		"package main\n\nfunc main() {}\n",
	)
	configuration := DefaultApplicationConfiguration()
	configuration.Analysis.RepositoryRoot = repositoryRoot
	configuration.Analysis.Paths = []string{"cmd"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := newDashboardService(analyzerGraphSourceFactory{}, configuration, logger)
	if err := service.Activate(t.Context()); err != nil {
		t.Fatalf("activate dashboard: %v", err)
	}
	defer func() {
		if err := service.Shutdown(); err != nil {
			t.Errorf("shut down dashboard: %v", err)
		}
	}()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	service.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status is %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("content type is %q", response.Header().Get("Content-Type"))
	}
}

func writeDashboardFixture(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}
