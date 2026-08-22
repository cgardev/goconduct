// Command goconduct analyzes Go imports, resolved calls, and coupling metrics.
// It reports architecture findings through a CLI and an embedded web dashboard.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"syscall"
)

func main() {
	commandContext, stopSignalNotifications := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignalNotifications()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if executionError := runCommand(commandContext, logger, os.Args[1:]); executionError != nil {
		logger.Error("The goconduct command fails", slog.Any("error", executionError))
		os.Exit(1)
	}
}

func runCommand(ctx context.Context, logger *slog.Logger, arguments []string) error {
	runtime := newDependencyGraphRuntime(
		analyzerGraphSourceFactory{},
		httpGraphCacheFactory{},
		logger,
	)
	command := newRootCommand(runtime)
	command.SetArgs(slices.Clone(arguments))
	return command.ExecuteContext(ctx)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"b156b82108ee7039b4d83a2d11bb22ec7968a9eaf996da69318e288ccd03cc93","functions":[{"id":"func/main","name":"main","line":14,"end_line":26,"hash":"a32421dd5066edf643ef4f18fc1a6a7f97d0e55fb9130b849c825bff3df36ce4"},{"id":"func/runCommand","name":"runCommand","line":28,"end_line":37,"hash":"fbd076b8cd56220f77ed1552991082c9ce8a40c95ef07d017f2ec13e854b3031"}]}
// mutate4go-manifest-end
