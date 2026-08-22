package duplication

import (
	"io"
	"log/slog"
	"testing"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/internal/kernel"
	"github.com/cgardev/goconduct/plugin"
)

func TestPluginRegistersDuplicationEvaluatorAndCommand(t *testing.T) {
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
		t.Fatalf("activate duplication plugin: %v", err)
	}
	catalog, err := do.Invoke[*plugin.Catalog](injector)
	if err != nil {
		t.Fatalf("resolve catalog: %v", err)
	}
	if names := catalog.Names(); len(names) != 1 || names[0] != "duplication" {
		t.Fatalf("catalog names are %v", names)
	}
	root := &cobra.Command{Use: "goconduct"}
	if err := candidate.RegisterCommands(injector, root); err != nil {
		t.Fatalf("register duplication command: %v", err)
	}
	command, _, err := root.Find([]string{"duplication"})
	if err != nil || command.Name() != "duplication" {
		t.Fatalf("find duplication command: command=%v, error=%v", command, err)
	}
}
