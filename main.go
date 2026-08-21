// The dependencygraph command analyzes imports between Go components.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
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
	if executionError := newRootCommand(logger).ExecuteContext(commandContext); executionError != nil {
		logger.Error("The dependency graph command fails", "error", executionError)
		os.Exit(1)
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T09:19:41Z","module_hash":"b5a944de18e5c32231713930ff254ec3c18aec25b3afb3d556e932c7a2369005","functions":[{"id":"func/main","name":"main","line":12,"end_line":24,"hash":"c1737ba8e10a6b693d81ee49612b8fc447c0c95ada37277b0d0ca6c604ea1cf3"}]}
// mutate4go-manifest-end
