package main

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	applicationconfiguration "github.com/cgardev/goconduct/cmd/goconduct/internal/configuration"
	"github.com/cgardev/goconduct/internal/appmodule"
	"github.com/cgardev/goconduct/internal/kernel"
	"github.com/cgardev/goconduct/internal/library/httpserver"
	goconductv1 "github.com/cgardev/goconduct/internal/protogen/v1"
	"github.com/cgardev/goconduct/internal/protogen/v1/goconductv1connect"
)

func TestApplicationServerComposesPluginEndpoints(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	host, err := appmodule.NewHost(kernel.Module(logger), builtInPlugins()...)
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	defer func() {
		if err := host.Shutdown(); err != nil {
			t.Errorf("shut down host: %v", err)
		}
	}()
	provideConfiguration(host.Injector(), applicationconfiguration.Default())
	if err := host.Activate(t.Context()); err != nil {
		t.Fatalf("activate host: %v", err)
	}
	server := httpserver.New(httpserver.Configuration{Address: "127.0.0.1:0", Logger: logger})
	if err := host.RegisterEndpoints(server); err != nil {
		t.Fatalf("register endpoints: %v", err)
	}
	testServer := httptest.NewServer(server.Handler())
	defer testServer.Close()
	client := goconductv1connect.NewQualityServiceClient(testServer.Client(), testServer.URL)

	response, err := client.ListPlugins(
		t.Context(),
		connect.NewRequest(&goconductv1.ListPluginsRequest{}),
	)
	if err != nil {
		t.Fatalf("list plugins: %v", err)
	}
	if len(response.Msg.GetPlugins()) != 5 {
		t.Fatalf("evaluator count is %d", len(response.Msg.GetPlugins()))
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
