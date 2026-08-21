package main

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestRootCommand_DefineSafeDefaults(t *testing.T) {
	t.Run("Scenario: The dependency graph command is created without options", func(t *testing.T) {
		var sut *cobra.Command
		var commandDefaults map[string]string
		var silenceErrors bool
		var silenceUsage bool
		var defaultInterval time.Duration
		var minimumInterval time.Duration

		t.Run("Given a root command with a structured logger", func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			sut = newRootCommand(logger)
			commandDefaults = make(map[string]string)
		})

		t.Run("When the root command configuration is inspected", func(t *testing.T) {
			silenceErrors = sut.SilenceErrors
			silenceUsage = sut.SilenceUsage
			for _, name := range []string{"address", "root", "refresh-interval"} {
				flag := sut.Flags().Lookup(name)
				if flag != nil {
					commandDefaults[name] = flag.DefValue
				}
			}
			defaultInterval = defaultRefreshInterval()
			minimumInterval = minimumRefreshInterval()
		})

		t.Run("Then Cobra error and usage output is suppressed", func(t *testing.T) {
			if !silenceErrors || !silenceUsage {
				t.Fatal("the root command does not suppress Cobra error output")
			}
		})

		t.Run("And every command option has the expected default", func(t *testing.T) {
			want := map[string]string{
				"address":          "127.0.0.1:6062",
				"root":             ".",
				"refresh-interval": "750ms",
			}
			for name, value := range want {
				if commandDefaults[name] != value {
					t.Errorf("flag %q has default %q, want %q", name, commandDefaults[name], value)
				}
			}
		})

		t.Run("And refresh interval boundaries remain stable", func(t *testing.T) {
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
		name string
		args func(*testing.T) []string
		want string
	}{
		{
			name: "a refresh interval is below the minimum",
			args: func(*testing.T) []string {
				return []string{"--refresh-interval", "99ms"}
			},
			want: "at least 100ms",
		},
		{
			name: "the repository has no module file",
			args: func(t *testing.T) []string {
				return []string{"--root", t.TempDir()}
			},
			want: "module file",
		},
		{
			name: "the dashboard address is invalid",
			args: func(t *testing.T) []string {
				return []string{
					"--root", newAnalyzerFixture(t),
					"--address", "invalid address",
				}
			},
			want: "listen on invalid address",
		},
		{
			name: "a positional argument is present",
			args: func(*testing.T) []string {
				return []string{"unexpected"}
			},
			want: "unknown command",
		},
		{
			name: "the analysis view is unknown",
			args: func(*testing.T) []string {
				return []string{"analyze", "--view", "unknown"}
			},
			want: "must be report or graph",
		},
		{
			name: "the finding threshold is unknown",
			args: func(*testing.T) []string {
				return []string{"analyze", "--fail-on", "unknown"}
			},
			want: "must be none, warning, or error",
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var commandArgs []string
			var commandContext context.Context
			var commandError error
			var command interface {
				ExecuteContext(context.Context) error
			}

			t.Run("Given a root command with the invalid input", func(step *testing.T) {
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				rootCommand := newRootCommand(logger)
				commandArgs = testCase.args(t)
				rootCommand.SetArgs(commandArgs)
				commandContext = t.Context()
				command = rootCommand
			})

			t.Run("When the command is executed", func(t *testing.T) {
				commandError = command.ExecuteContext(commandContext)
			})

			t.Run("Then execution returns the expected validation error", func(t *testing.T) {
				if commandError == nil || !strings.Contains(commandError.Error(), testCase.want) {
					t.Fatalf(
						"command error is %v, want a message containing %q",
						commandError,
						testCase.want,
					)
				}
			})
		})
	}
}
func TestRootCommand_ExecuteAtMinimumInterval(t *testing.T) {
	t.Run("Scenario: The minimum refresh interval is used with a canceled context", func(t *testing.T) {
		var commandContext context.Context
		var commandError error
		var command interface {
			ExecuteContext(context.Context) error
		}

		if !t.Run("Given a valid repository command at the minimum interval", func(step *testing.T) {
			repositoryRoot := newAnalyzerFixture(t)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			rootCommand := newRootCommand(logger)
			rootCommand.SetArgs([]string{
				"--root", repositoryRoot,
				"--address", "127.0.0.1:0",
				"--refresh-interval", minimumRefreshInterval().String(),
			})
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			commandContext = ctx
			command = rootCommand
		}) {
			return
		}

		t.Run("When the command is executed", func(t *testing.T) {
			commandError = command.ExecuteContext(commandContext)
		})

		t.Run("Then the command accepts the boundary and stops cleanly", func(t *testing.T) {
			if commandError != nil {
				t.Fatalf("command failed at the minimum refresh interval: %v", commandError)
			}
		})
	})
}
