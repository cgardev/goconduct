// The dependencygraph command analyzes and presents strategic Go dependencies.
//
// domain.go defines the stable architectural model. calculation.go contains only
// deterministic mathematical rules. analyzer.go adapts repository source files
// into that calculation model. command.go and server.go are transport adapters.
// presentation.go owns the embedded human interface.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := newRootCommand(logger).ExecuteContext(ctx); err != nil {
		logger.Error("Dependency graph command failed", "error", err)
		os.Exit(1)
	}
}
