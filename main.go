// The dependencygraph command analyzes Go imports, calls, and metrics. The command reports architecture findings.
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
		logger.Error("The dependency graph command fails", "error", executionError)
		os.Exit(1)
	}
}

func runCommand(ctx context.Context, logger *slog.Logger, arguments []string) error {
	command := newRootCommand(logger)
	command.SetArgs(slices.Clone(arguments))
	return command.ExecuteContext(ctx)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T17:15:20Z","module_hash":"64987db77a84d13c388d71c8da48c19b3375d4da015826c79988bf2e8972e5cb","functions":[{"id":"func/main","name":"main","line":13,"end_line":25,"hash":"4009d68f8c08fd8fbfec29ed92d0eda96d30b2f0b58af66ab530df83d7828cb8"},{"id":"func/runCommand","name":"runCommand","line":27,"end_line":31,"hash":"1b64d9240fad5f0b2556f66dde5d0049f50d3c7e97ffc03a77e8167995a0793c"}]}
// mutate4go-manifest-end
