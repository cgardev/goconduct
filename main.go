// The dependencygraph command serves a self-contained strategic dependency map for this Go repository.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultAddress = "127.0.0.1:6062"
)

func defaultRefreshInterval() time.Duration {
	return 750 * time.Millisecond
}

func minimumRefreshInterval() time.Duration {
	return 100 * time.Millisecond
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := newRootCommand(logger).ExecuteContext(ctx); err != nil {
		logger.Error("Dependency graph command failed", "error", err)
		os.Exit(1)
	}
}

func newRootCommand(logger *slog.Logger) *cobra.Command {
	var address string
	var repositoryRoot string
	var refreshInterval time.Duration
	command := &cobra.Command{
		Use:           "dependencygraph",
		Short:         "Visualize the repository's strategic Go dependencies",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if refreshInterval < minimumRefreshInterval() {
				return fmt.Errorf(
					"refresh interval must be at least %s",
					minimumRefreshInterval(),
				)
			}
			analyzer, err := newAnalyzer(repositoryRoot)
			if err != nil {
				return err
			}
			monitor, err := newGraphMonitor(analyzer, refreshInterval, logger)
			if err != nil {
				return err
			}
			return runDashboard(command.Context(), monitor, address, logger)
		},
	}
	command.Flags().StringVar(
		&address,
		"address",
		defaultAddress,
		"local address used by the dashboard server",
	)
	command.Flags().StringVar(
		&repositoryRoot,
		"root",
		".",
		"repository root containing go.mod",
	)
	command.Flags().DurationVar(
		&refreshInterval,
		"refresh-interval",
		defaultRefreshInterval(),
		"interval used to detect source changes",
	)
	return command
}
