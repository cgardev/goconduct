package coverage

import (
	"io"
	"log/slog"
	"testing"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/internal/kernel"
	"github.com/cgardev/goconduct/plugin"
)

func TestPluginRegistersCoverageEvaluatorAndCommand(t *testing.T) {
	candidate := Plugin()
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
		t.Fatalf("activate coverage plugin: %v", err)
	}
	catalog, err := do.Invoke[*plugin.Catalog](injector)
	if err != nil {
		t.Fatalf("resolve catalog: %v", err)
	}
	if names := catalog.Names(); len(names) != 1 || names[0] != "coverage" {
		t.Fatalf("catalog names are %v", names)
	}
	root := &cobra.Command{Use: "goconduct"}
	if err := candidate.RegisterCommands(injector, root); err != nil {
		t.Fatalf("register coverage command: %v", err)
	}
	command, _, err := root.Find([]string{"coverage"})
	if err != nil || command.Name() != "coverage" {
		t.Fatalf("find coverage command: command=%v, error=%v", command, err)
	}
}
