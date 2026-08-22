// Command goconduct analyzes Go imports, resolved calls, and coupling metrics.
// It reports architecture findings through a CLI and an embedded web dashboard.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	applicationconfiguration "github.com/cgardev/goconduct/cmd/goconduct/internal/configuration"
	qualitymodule "github.com/cgardev/goconduct/cmd/goconduct/internal/module/quality"
	"github.com/cgardev/goconduct/internal/appmodule"
	"github.com/cgardev/goconduct/internal/kernel"
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

func runCommand(ctx context.Context, logger *slog.Logger, arguments []string) (resultErr error) {
	host, err := appmodule.NewHost(kernel.Module(logger), builtInPlugins()...)
	if err != nil {
		return err
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		resultErr = errors.Join(resultErr, host.ShutdownWithContext(shutdownContext))
	}()
	command := newRootCommand()
	if err := host.RegisterCommands(command); err != nil {
		return err
	}
	registerApplicationCommands(host, command)
	command.PersistentPreRunE = func(command *cobra.Command, _ []string) error {
		if command.Name() == "configuration-schema" {
			return nil
		}
		configurationPath, err := command.Root().PersistentFlags().GetString("configuration")
		if err != nil {
			return err
		}
		configuration, err := applicationconfiguration.Load(configurationPath)
		if err != nil {
			return err
		}
		if err := applyConfigurationOverrides(command, &configuration); err != nil {
			return err
		}
		provideConfiguration(host.Injector(), configuration)
		return host.Activate(command.Context())
	}
	command.SetArgs(slices.Clone(arguments))
	return command.ExecuteContext(ctx)
}

func applyConfigurationOverrides(
	command *cobra.Command,
	configuration *applicationconfiguration.ApplicationConfiguration,
) error {
	persistentFlags := command.Root().PersistentFlags()
	if persistentFlags.Changed("address") {
		value, err := persistentFlags.GetString("address")
		if err != nil {
			return err
		}
		configuration.Server.Address = value
	}
	if persistentFlags.Changed("root") {
		value, err := persistentFlags.GetString("root")
		if err != nil {
			return err
		}
		configuration.Analysis.RepositoryRoot = value
	}
	if persistentFlags.Changed("analysis-path") {
		value, err := persistentFlags.GetStringArray("analysis-path")
		if err != nil {
			return err
		}
		configuration.Analysis.Paths = value
	}
	if persistentFlags.Changed("ignore-path") {
		value, err := persistentFlags.GetStringArray("ignore-path")
		if err != nil {
			return err
		}
		configuration.Analysis.IgnoredPaths = value
	}
	if command.Root().Flags().Changed("refresh-interval") {
		value, err := command.Root().Flags().GetDuration("refresh-interval")
		if err != nil {
			return err
		}
		configuration.Server.RefreshInterval = value
	}
	return nil
}

func newRootCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "goconduct",
		Short:         "Apply deterministic quality rules to Go software.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
}

func provideConfiguration(
	injector do.Injector,
	configuration applicationconfiguration.ApplicationConfiguration,
) {
	do.ProvideValue(injector, configuration)
	do.ProvideValue(injector, configuration.ArchitecturePluginConfiguration())
	do.ProvideValue(injector, configuration.Quality.Coverage)
	do.ProvideValue(injector, configuration.Quality.CRAP)
	do.ProvideValue(injector, configuration.Quality.Duplication)
	do.ProvideValue(injector, configuration.Quality.Mutation)
	check := configuration.CloneCheck()
	do.ProvideValue(injector, qualitymodule.Configuration{
		RepositoryRoot: configuration.Analysis.RepositoryRoot,
		Plugins:        check.Plugins,
		Paths:          check.Paths,
	})
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"b156b82108ee7039b4d83a2d11bb22ec7968a9eaf996da69318e288ccd03cc93","functions":[{"id":"func/main","name":"main","line":14,"end_line":26,"hash":"a32421dd5066edf643ef4f18fc1a6a7f97d0e55fb9130b849c825bff3df36ce4"},{"id":"func/runCommand","name":"runCommand","line":28,"end_line":37,"hash":"fbd076b8cd56220f77ed1552991082c9ce8a40c95ef07d017f2ec13e854b3031"}]}
// mutate4go-manifest-end
