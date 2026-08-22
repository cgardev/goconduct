package quality

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"

	"github.com/cgardev/goconduct/internal/appmodule"
	goconductv1 "github.com/cgardev/goconduct/internal/protogen/v1"
	"github.com/cgardev/goconduct/internal/protogen/v1/goconductv1connect"
	"github.com/cgardev/goconduct/plugin"
)

func TestQualityAPIListsPluginsThroughConnect(t *testing.T) {
	catalog := plugin.NewCatalog()
	if err := catalog.Register(&staticEvaluator{name: "fixture"}); err != nil {
		t.Fatalf("register evaluator: %v", err)
	}
	injector := do.New(
		func(injector do.Injector) {
			do.ProvideValue(injector, catalog)
			do.ProvideValue(injector, Configuration{Plugins: []string{"fixture"}})
		},
		appmodule.SelfScope(),
		Module,
	)
	api, err := do.Invoke[*QualityAPI](injector)
	if err != nil {
		t.Fatalf("resolve API: %v", err)
	}
	path, handler := goconductv1connect.NewQualityServiceHandler(api)
	router := http.NewServeMux()
	router.Handle(path, handler)
	server := httptest.NewServer(router)
	defer server.Close()
	client := goconductv1connect.NewQualityServiceClient(server.Client(), server.URL)

	response, err := client.ListPlugins(
		t.Context(),
		connect.NewRequest(&goconductv1.ListPluginsRequest{}),
	)
	if err != nil {
		t.Fatalf("list plugins: %v", err)
	}
	if len(response.Msg.GetPlugins()) != 1 || response.Msg.GetPlugins()[0].GetName() != "fixture" {
		t.Fatalf("plugins are %+v", response.Msg.GetPlugins())
	}
}
