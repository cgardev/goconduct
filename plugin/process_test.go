package plugin

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestCommandRunnerCapturesOutputAndExitStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture uses the POSIX shell provided by the development environment")
	}
	result, err := NewCommandRunner().Run(t.Context(), Command{
		Path:      "sh",
		Arguments: []string{"-c", "printf stdout; printf stderr >&2; exit 7"},
	})
	if err == nil {
		t.Fatal("expected process error")
	}
	if result.ExitCode != 7 || string(result.StandardOutput) != "stdout" || string(result.StandardError) != "stderr" {
		t.Fatalf("unexpected command result: %+v", result)
	}
}

func TestCommandRunnerRejectsEmptyCommandAndHonorsCancellation(t *testing.T) {
	runner := NewCommandRunner()
	if _, err := runner.Run(t.Context(), Command{}); err == nil {
		t.Fatal("expected empty command error")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := runner.Run(ctx, Command{Path: "go", Arguments: []string{"version"}})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation error, got %v", err)
	}
}

func TestCommandRunnerIntegrationExecutesGoTool(t *testing.T) {
	result, err := NewCommandRunner().Run(t.Context(), Command{Path: "go", Arguments: []string{"version"}})
	if err != nil {
		t.Fatalf("run Go tool: %v", err)
	}
	if result.ExitCode != 0 || !strings.HasPrefix(string(result.StandardOutput), "go version go") {
		t.Fatalf("unexpected Go version result: %+v", result)
	}
}
