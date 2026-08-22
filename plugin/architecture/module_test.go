package architecture

import (
	"io"
	"log/slog"
	"testing"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/internal/kernel"
	goplugin "github.com/cgardev/goconduct/plugin"
)

func TestPluginRegistersEvaluatorAndCommandSurface(t *testing.T) {
	candidate := Plugin()
	if candidate.Name() != "architecture" {
		t.Fatalf("plugin name is %q", candidate.Name())
	}
	injector := do.New(
		kernel.Module(slog.New(slog.NewTextHandler(io.Discard, nil))),
		candidate.Services(),
	)
	t.Cleanup(func() {
		if report := injector.Shutdown(); report != nil && !report.Succeed {
			t.Errorf("shut down injector: %s", report.Error())
		}
	})
	if err := candidate.Activate(t.Context(), injector); err != nil {
		t.Fatalf("activate plugin: %v", err)
	}
	catalog, err := do.Invoke[*goplugin.Catalog](injector)
	if err != nil {
		t.Fatalf("resolve catalog: %v", err)
	}
	if names := catalog.Names(); len(names) != 1 || names[0] != "architecture" {
		t.Fatalf("catalog names are %v", names)
	}
	root := &cobra.Command{Use: "goconduct"}
	if err := candidate.RegisterCommands(injector, root); err != nil {
		t.Fatalf("register commands: %v", err)
	}
	for _, name := range []string{"analyze", "components", "findings", "summary"} {
		command, _, err := root.Find([]string{name})
		if err != nil || command.Name() != name {
			t.Fatalf("find %s command: command=%v, error=%v", name, command, err)
		}
	}
}

func TestEvaluatorWorksWithoutApplicationHost(t *testing.T) {
	repositoryRoot := newAnalyzerFixture(t)
	report, err := NewEvaluator(slog.New(slog.NewTextHandler(io.Discard, nil))).Evaluate(
		t.Context(),
		goplugin.Request{RepositoryRoot: repositoryRoot},
	)
	if err != nil {
		t.Fatalf("evaluate architecture: %v", err)
	}
	if report.Plugin != "architecture" || len(report.Metrics) == 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}
