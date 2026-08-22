package quality

import (
	"context"
	"testing"

	"github.com/cgardev/goconduct/plugin"
)

type staticEvaluator struct {
	name    string
	request plugin.Request
}

func (evaluator *staticEvaluator) Name() string {
	return evaluator.name
}

func (evaluator *staticEvaluator) Evaluate(
	_ context.Context,
	request plugin.Request,
) (plugin.Report, error) {
	evaluator.request = request
	return plugin.NewReport(evaluator.name, nil, nil)
}

func TestRunCheckUseCaseUsesConfiguredDefaults(t *testing.T) {
	catalog := plugin.NewCatalog()
	evaluator := &staticEvaluator{name: "architecture"}
	if err := catalog.Register(evaluator); err != nil {
		t.Fatalf("register evaluator: %v", err)
	}
	configuration := Configuration{
		RepositoryRoot: "/repository",
		Plugins:        []string{"architecture"},
		Paths:          []string{"internal"},
	}

	result, err := NewRunCheckUseCase(catalog, configuration).Execute(
		t.Context(),
		RunCheckUseCaseParams{},
	)
	if err != nil {
		t.Fatalf("run check: %v", err)
	}
	if result.Summary.Plugins != 1 {
		t.Fatalf("plugin count is %d", result.Summary.Plugins)
	}
	if evaluator.request.RepositoryRoot != "/repository" || len(evaluator.request.Paths) != 1 {
		t.Fatalf("request is %+v", evaluator.request)
	}
}

func TestListPluginsUseCaseHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := NewListPluginsUseCase(plugin.NewCatalog()).Execute(ctx, ListPluginsUseCaseParams{})
	if err == nil {
		t.Fatal("cancelled query succeeds")
	}
}
