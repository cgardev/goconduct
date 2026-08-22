package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/internal/failure"
)

func newTestCommandRuntime(logger *slog.Logger) commandRuntime {
	return newDependencyGraphRuntime(
		analyzerGraphSourceFactory{},
		httpGraphCacheFactory{},
		logger,
	)
}

func newTestRootCommand(logger *slog.Logger) *cobra.Command {
	return newRootCommand(newTestCommandRuntime(logger))
}

type recordingCommandRuntime struct {
	graph                  Graph
	analyzeError           error
	dashboardError         error
	analyzeCalls           int
	dashboardCalls         int
	analysisConfiguration  ApplicationConfiguration
	dashboardConfiguration ApplicationConfiguration
}

func (runtime *recordingCommandRuntime) analyze(
	_ context.Context,
	configuration ApplicationConfiguration,
) (Graph, error) {
	runtime.analyzeCalls++
	runtime.analysisConfiguration = configuration
	return runtime.graph, runtime.analyzeError
}

func (runtime *recordingCommandRuntime) runDashboard(
	_ context.Context,
	configuration ApplicationConfiguration,
) error {
	runtime.dashboardCalls++
	runtime.dashboardConfiguration = configuration
	return runtime.dashboardError
}

func TestRootCommand_UseInjectedRuntime(t *testing.T) {
	t.Run("Scenario: A summary command requests a graph from the injected runtime", func(t *testing.T) {
		var runtime *recordingCommandRuntime
		var command *cobra.Command
		var output strings.Builder
		var commandError error

		t.Run("Given a command with a recording runtime", func(*testing.T) {
			runtime = &recordingCommandRuntime{graph: Graph{Revision: "injected-revision"}}
			command = newRootCommand(runtime)
			command.SetOut(&output)
			command.SetArgs([]string{"summary", "--root", "injected-root"})
		})

		t.Run("When the summary command executes", func(t *testing.T) {
			commandError = command.ExecuteContext(t.Context())
		})

		t.Run("Then the command calls only the graph analysis port", func(t *testing.T) {
			if commandError != nil {
				t.Fatalf("summary command fails: %v", commandError)
			}
			if runtime.analyzeCalls != 1 || runtime.dashboardCalls != 0 {
				t.Fatalf(
					"runtime calls are analyze=%d dashboard=%d, want 1 and 0",
					runtime.analyzeCalls,
					runtime.dashboardCalls,
				)
			}
		})

		t.Run("And the command passes configuration and writes the injected graph", func(t *testing.T) {
			if runtime.analysisConfiguration.Analysis.RepositoryRoot != "injected-root" {
				t.Errorf(
					"analysis root is %q, want injected-root",
					runtime.analysisConfiguration.Analysis.RepositoryRoot,
				)
			}
			if !strings.Contains(output.String(), `"revision": "injected-revision"`) {
				t.Errorf("summary output does not contain the injected revision: %s", output.String())
			}
		})
	})

	t.Run("Scenario: The root command starts the dashboard through the injected runtime", func(t *testing.T) {
		var runtime *recordingCommandRuntime
		var commandError error

		t.Run("Given a root command with a recording runtime", func(*testing.T) {
			runtime = &recordingCommandRuntime{}
		})

		t.Run("When the root command executes without a subcommand", func(t *testing.T) {
			command := newRootCommand(runtime)
			commandError = command.ExecuteContext(t.Context())
		})

		t.Run("Then the command calls only the dashboard port", func(t *testing.T) {
			if commandError != nil {
				t.Fatalf("root command fails: %v", commandError)
			}
			if runtime.dashboardCalls != 1 || runtime.analyzeCalls != 0 {
				t.Fatalf(
					"runtime calls are dashboard=%d analyze=%d, want 1 and 0",
					runtime.dashboardCalls,
					runtime.analyzeCalls,
				)
			}
		})

		t.Run("And the command passes the default dashboard configuration", func(t *testing.T) {
			if runtime.dashboardConfiguration.Server.Address != defaultAddress {
				t.Errorf(
					"dashboard address is %q, want %q",
					runtime.dashboardConfiguration.Server.Address,
					defaultAddress,
				)
			}
		})
	})
}

func TestRunCommand_ReturnInvalidArgumentError(t *testing.T) {
	t.Run("Scenario: The process receives an unknown command argument", func(t *testing.T) {
		var commandContext context.Context
		var logger *slog.Logger
		var commandArguments []string
		var commandError error

		t.Run("Given a command context, logger, and unknown argument", func(*testing.T) {
			commandContext = t.Context()
			logger = slog.New(slog.NewTextHandler(io.Discard, nil))
			commandArguments = []string{"unexpected"}
		})

		t.Run("When the command runner executes the argument", func(*testing.T) {
			commandError = runCommand(commandContext, logger, commandArguments)
		})

		t.Run("Then the command runner returns the argument error", func(t *testing.T) {
			if commandError == nil || !strings.Contains(commandError.Error(), "unknown command") {
				t.Fatalf("command error is %v, want an unknown command error", commandError)
			}
		})
	})
}

func TestRunCommand_UseExecutionContext(t *testing.T) {
	t.Run("Scenario: The command context ends before dashboard startup", func(t *testing.T) {
		var commandContext context.Context
		var commandArguments []string
		var commandError error

		t.Run("Given valid arguments and a canceled command context", func(*testing.T) {
			commandArguments = []string{
				"--root", newAnalyzerFixture(t),
				"--address", "127.0.0.1:0",
				"--refresh-interval", minimumRefreshInterval().String(),
			}
			var cancel context.CancelFunc
			commandContext, cancel = context.WithCancel(t.Context())
			cancel()
		})

		t.Run("When the command runner executes with the canceled context", func(*testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			commandError = runCommand(commandContext, logger, commandArguments)
		})

		t.Run("Then the command runner stops without a process error", func(t *testing.T) {
			if commandError != nil {
				t.Fatalf("command runner error is %v, want nil", commandError)
			}
		})
	})
}

func TestContextError_ClassifyExecutionFailure(t *testing.T) {
	testError := errors.New("test command failure")
	testCases := []struct {
		name           string
		cancelContext  bool
		executionError error
		want           bool
	}{
		{
			name:           "a canceled context returns its cancellation error",
			cancelContext:  true,
			executionError: context.Canceled,
			want:           true,
		},
		{
			name:           "a canceled context returns an unrelated error",
			cancelContext:  true,
			executionError: testError,
			want:           false,
		},
		{
			name:           "an active context receives a cancellation error",
			cancelContext:  false,
			executionError: context.Canceled,
			want:           false,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var commandContext context.Context
			var result bool

			t.Run("Given the command context and execution error", func(*testing.T) {
				commandContext = t.Context()
				if testCase.cancelContext {
					var cancel context.CancelFunc
					commandContext, cancel = context.WithCancel(commandContext)
					cancel()
				}
			})

			t.Run("When the classifier checks the execution error", func(*testing.T) {
				result = isContextError(commandContext, testCase.executionError)
			})

			t.Run("Then the classifier returns the expected result", func(t *testing.T) {
				if result != testCase.want {
					t.Errorf("classification is %t, want %t", result, testCase.want)
				}
			})
		})
	}
}

func TestRootCommand_DefineSafeDefaults(t *testing.T) {
	t.Run("Scenario: The test creates the dependency graph command without options", func(t *testing.T) {
		var rootCommand *cobra.Command
		var commandDefaults map[string]string
		var silenceErrors bool
		var silenceUsage bool
		var defaultInterval time.Duration
		var minimumInterval time.Duration

		t.Run("Given a root command with a structured logger", func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			rootCommand = newTestRootCommand(logger)
			commandDefaults = make(map[string]string)
		})

		t.Run("When the test inspects the root command configuration", func(t *testing.T) {
			silenceErrors = rootCommand.SilenceErrors
			silenceUsage = rootCommand.SilenceUsage
			for _, name := range []string{"refresh-interval"} {
				flag := rootCommand.Flags().Lookup(name)
				if flag != nil {
					commandDefaults[name] = flag.DefValue
				}
			}
			for _, name := range []string{
				"address",
				"analysis-path",
				"cache",
				"cache-timeout",
				"configuration",
				"ignore-path",
				"root",
			} {
				flag := rootCommand.PersistentFlags().Lookup(name)
				if flag != nil {
					commandDefaults[name] = flag.DefValue
				}
			}
			defaultInterval = defaultRefreshInterval()
			minimumInterval = minimumRefreshInterval()
		})

		t.Run("Then the root command suppresses Cobra error and usage output", func(t *testing.T) {
			if !silenceErrors || !silenceUsage {
				t.Fatal("the root command does not suppress Cobra error output")
			}
		})

		t.Run("And each command option has the expected default", func(t *testing.T) {
			want := map[string]string{
				"address":          "127.0.0.1:6062",
				"cache":            "auto",
				"cache-timeout":    "2s",
				"configuration":    ".goconduct.json",
				"root":             ".",
				"refresh-interval": "750ms",
				"analysis-path":    "[]",
				"ignore-path":      "[]",
			}
			for name, value := range want {
				if commandDefaults[name] != value {
					t.Errorf("flag %q has default %q, want %q", name, commandDefaults[name], value)
				}
			}
		})

		t.Run("And the refresh interval boundaries have the expected values", func(t *testing.T) {
			if defaultInterval != 750*time.Millisecond {
				t.Errorf("unexpected default refresh interval %s", defaultInterval)
			}
			if minimumInterval != 100*time.Millisecond {
				t.Errorf("unexpected minimum refresh interval %s", minimumInterval)
			}
		})
	})
}
func TestRootCommand_RejectInvalidArguments(t *testing.T) {
	testCases := []struct {
		name         string
		arguments    func(*testing.T) []string
		want         string
		wantCategory error
	}{
		{
			name: "a refresh interval is below the minimum",
			arguments: func(*testing.T) []string {
				return []string{"--refresh-interval", "99ms"}
			},
			want:         "at least 100ms",
			wantCategory: failure.ErrValidation,
		},
		{
			name: "the repository has no module file",
			arguments: func(t *testing.T) []string {
				return []string{"--root", t.TempDir()}
			},
			want:         "module file",
			wantCategory: failure.ErrValidation,
		},
		{
			name: "the dashboard address is invalid",
			arguments: func(t *testing.T) []string {
				return []string{
					"--root", newAnalyzerFixture(t),
					"--address", "invalid address",
				}
			},
			want:         "listen on invalid address",
			wantCategory: failure.ErrUnavailable,
		},
		{
			name: "a positional argument is present",
			arguments: func(*testing.T) []string {
				return []string{"unexpected"}
			},
			want: "unknown command",
		},
		{
			name: "the analysis view is unknown",
			arguments: func(*testing.T) []string {
				return []string{"analyze", "--view", "unknown"}
			},
			want:         "must be report or graph",
			wantCategory: failure.ErrValidation,
		},
		{
			name: "the finding threshold is unknown",
			arguments: func(*testing.T) []string {
				return []string{"analyze", "--fail-on", "unknown"}
			},
			want:         "must be none, warning, or error",
			wantCategory: failure.ErrValidation,
		},
		{
			name: "the filtered finding severity is unknown",
			arguments: func(*testing.T) []string {
				return []string{"findings", "--severity", "critical"}
			},
			want:         "must be all, warning, or error",
			wantCategory: failure.ErrValidation,
		},
		{
			name: "the filtered component role is unknown",
			arguments: func(*testing.T) []string {
				return []string{"components", "--role", "service"}
			},
			want:         "component role",
			wantCategory: failure.ErrValidation,
		},
		{
			name: "the filtered component sort is unknown",
			arguments: func(*testing.T) []string {
				return []string{"components", "--sort", "weight"}
			},
			want:         "component sort",
			wantCategory: failure.ErrValidation,
		},
		{
			name: "the filtered function sort is unknown",
			arguments: func(*testing.T) []string {
				return []string{"functions", "--sort", "weight"}
			},
			want:         "function sort",
			wantCategory: failure.ErrValidation,
		},
		{
			name: "a filtered query limit is negative",
			arguments: func(*testing.T) []string {
				return []string{"findings", "--limit", "-1"}
			},
			want:         "must not be negative",
			wantCategory: failure.ErrValidation,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var commandArguments []string
			var commandContext context.Context
			var commandError error
			var command interface {
				ExecuteContext(context.Context) error
			}

			t.Run("Given a root command with the invalid input", func(step *testing.T) {
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				rootCommand := newTestRootCommand(logger)
				commandArguments = testCase.arguments(t)
				rootCommand.SetArgs(commandArguments)
				commandContext = t.Context()
				command = rootCommand
			})

			t.Run("When the test executes the command", func(t *testing.T) {
				commandError = command.ExecuteContext(commandContext)
			})

			t.Run("Then execution returns the expected categorized error", func(t *testing.T) {
				if commandError == nil || !strings.Contains(commandError.Error(), testCase.want) {
					t.Fatalf(
						"command error is %v, want a message that contains %q",
						commandError,
						testCase.want,
					)
				}
				if testCase.wantCategory != nil && !errors.Is(commandError, testCase.wantCategory) {
					t.Fatalf("command error is %v, want category %v", commandError, testCase.wantCategory)
				}
			})
		})
	}
}
func TestRootCommand_ExecuteAtMinimumInterval(t *testing.T) {
	t.Run("Scenario: The test cancels the context and uses the minimum refresh interval", func(t *testing.T) {
		var commandContext context.Context
		var commandError error
		var command interface {
			ExecuteContext(context.Context) error
		}

		if !t.Run("Given a valid repository command at the minimum interval", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			rootCommand := newTestRootCommand(logger)
			rootCommand.SetArgs([]string{
				"--root", repositoryRoot,
				"--address", "127.0.0.1:0",
				"--refresh-interval", minimumRefreshInterval().String(),
			})
			canceledContext, cancel := context.WithCancel(t.Context())
			cancel()
			commandContext = canceledContext
			command = rootCommand
		}) {
			return
		}

		t.Run("When the test executes the command", func(t *testing.T) {
			commandError = command.ExecuteContext(commandContext)
		})

		t.Run("Then the command accepts the boundary and stops cleanly", func(t *testing.T) {
			if commandError != nil {
				t.Fatalf("the command fails at the minimum refresh interval: %v", commandError)
			}
		})
	})
}
