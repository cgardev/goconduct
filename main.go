// Command dependencygraph analyzes Go imports, resolved calls, and coupling metrics.
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
		logger.Error("The dependency graph command fails", slog.Any("error", executionError))
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
// {"version":1,"tested_at":"2026-08-21T18:31:36Z","module_hash":"50975ef20a1a91a1c2201306194163daaeaf8322e53fea2cb29e868540ce0317","functions":[{"id":"func/main","name":"main","line":14,"end_line":26,"hash":"d4e7d93f4f0bd7f4481cc3d687567003d4047bceff6658403aa4952c9bba9974"},{"id":"func/runCommand","name":"runCommand","line":28,"end_line":37,"hash":"fbd076b8cd56220f77ed1552991082c9ce8a40c95ef07d017f2ec13e854b3031"}]}
// mutate4go-manifest-end
