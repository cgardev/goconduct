package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	applicationconfiguration "github.com/cgardev/goconduct/cmd/goconduct/internal/configuration"
	goconductv1 "github.com/cgardev/goconduct/internal/protogen/v1"
	"github.com/cgardev/goconduct/internal/protogen/v1/goconductv1connect"
)

func TestApplicationServerComposesPluginEndpoints(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := newApplication(logger)
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	defer func() {
		if err := app.Shutdown(t.Context()); err != nil {
			t.Errorf("shut down application: %v", err)
		}
	}()
	if err := app.Activate(t.Context(), applicationconfiguration.Default()); err != nil {
		t.Fatalf("activate application: %v", err)
	}
	server, err := app.ComposeServer()
	if err != nil {
		t.Fatalf("compose application server: %v", err)
	}
	testServer := httptest.NewServer(server.Handler())
	defer testServer.Close()
	client := goconductv1connect.NewQualityServiceClient(testServer.Client(), testServer.URL)
	healthResponse, err := testServer.Client().Get(testServer.URL + "/healthz")
	if err != nil {
		t.Fatalf("get health endpoint: %v", err)
	}
	defer func() {
		if err := healthResponse.Body.Close(); err != nil {
			t.Errorf("close health response: %v", err)
		}
	}()
	if healthResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("health status is %d", healthResponse.StatusCode)
	}

	response, err := client.ListPlugins(
		t.Context(),
		connect.NewRequest(&goconductv1.ListPluginsRequest{}),
	)
	if err != nil {
		t.Fatalf("list plugins: %v", err)
	}
	if len(response.Msg.GetPlugins()) != 6 {
		t.Fatalf("evaluator count is %d", len(response.Msg.GetPlugins()))
	}

	_, err = client.RunCheck(
		t.Context(),
		connect.NewRequest(&goconductv1.RunCheckRequest{Paths: []string{"invalid\\path"}}),
	)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("validation code is %s: %v", connect.CodeOf(err), err)
	}

	repositoryRoot := t.TempDir()
	writeApplicationServerFixture(t, repositoryRoot, "go.mod", "module example.com/fixture\n\ngo 1.26.3\n")
	writeApplicationServerFixture(
		t,
		repositoryRoot,
		"cmd/fixture/main.go",
		"package main\n\nfunc main() {}\n",
	)
	check, err := client.RunCheck(
		t.Context(),
		connect.NewRequest(&goconductv1.RunCheckRequest{
			RepositoryRoot: repositoryRoot,
			Plugins:        []string{"architecture"},
			Paths:          []string{"cmd"},
		}),
	)
	if err != nil {
		t.Fatalf("run architecture check: %v", err)
	}
	if check.Msg.GetSummary().GetPlugins() != 1 || len(check.Msg.GetReports()) != 1 ||
		check.Msg.GetReports()[0].GetPlugin() != "architecture" {
		t.Fatalf("check response is %+v", check.Msg)
	}
}

func writeApplicationServerFixture(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}
