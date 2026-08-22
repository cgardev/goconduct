package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/spf13/cobra"

	querymodel "github.com/cgardev/goconduct/internal/query"
)

func TestFunctionCommands_EmitFocusedJSONWithoutPipes(t *testing.T) {
	t.Run("Scenario: An automated client requests function data through native CLI filters", func(t *testing.T) {
		var repositoryRoot string
		var functions querymodel.FunctionsResult
		var function querymodel.FunctionResult
		var calls querymodel.FunctionCallsResult
		var commandError error

		t.Run("Given a repository with resolved calls between two components", func(*testing.T) {
			repositoryRoot = newFunctionAnalysisFixture(t)
		})

		t.Run("When the functions, function, and calls commands execute", func(step *testing.T) {
			queries := []struct {
				arguments []string
				result    any
			}{
				{
					arguments: []string{
						"functions", "--root", repositoryRoot,
						"--component", "internal/library/telemetry",
						"--sort", "incoming-call-sites", "--include-tests", "--limit", "3",
					},
					result: &functions,
				},
				{
					arguments: []string{
						"function", "internal/library/telemetry.Recorder.Record",
						"--root", repositoryRoot, "--include-tests",
					},
					result: &function,
				},
				{
					arguments: []string{
						"calls", "--root", repositoryRoot,
						"--source-component", "internal/module/orders",
						"--target-component", "internal/library/telemetry",
						"--include-tests",
					},
					result: &calls,
				},
			}
			for _, query := range queries {
				var output bytes.Buffer
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				command := newTestRootCommand(logger)
				command.SetOut(&output)
				command.SetArgs(query.arguments)
				if err := command.ExecuteContext(step.Context()); err != nil {
					commandError = err
					return
				}
				if err := json.Unmarshal(output.Bytes(), query.result); err != nil {
					commandError = err
					return
				}
			}
		})

		if !t.Run("Then every native query succeeds with its direct resource", func(t *testing.T) {
			if commandError != nil {
				t.Fatalf("native function query fails: %v", commandError)
			}
			if functions.Returned != 3 || function.Function.Identifier == "" || calls.Returned == 0 {
				t.Fatalf("unexpected native query results: %+v %+v %+v", functions, function, calls)
			}
		}) {
			return
		}

		t.Run("And the filtered call result contains exact source locations", func(t *testing.T) {
			if calls.CallSites != 8 {
				t.Errorf("call sites are %d, want 8", calls.CallSites)
			}
			if len(function.IncomingCalls) != 2 || len(function.IncomingCalls[0].CallSites) == 0 {
				t.Errorf("unexpected exact function callers: %+v", function.IncomingCalls)
			}
		})
	})
}

func TestFunctionCommands_DefaultToProductionScope(t *testing.T) {
	t.Run("Scenario: A client does not request test calls", func(t *testing.T) {
		var commands map[string]*cobra.Command
		var defaults map[string][2]string

		t.Run("Given each function query command with its default flags", func(*testing.T) {
			configurationFlags := &commandConfigurationFlags{}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			analyzer := newTestCommandRuntime(logger)
			commands = map[string]*cobra.Command{
				"functions": newFunctionsCommand(configurationFlags, analyzer),
				"function":  newFunctionCommand(configurationFlags, analyzer),
				"calls":     newFunctionCallsCommand(configurationFlags, analyzer),
			}
		})

		t.Run("When the include-tests flags are read", func(step *testing.T) {
			defaults = make(map[string][2]string, len(commands))
			for name, command := range commands {
				flag := command.Flags().Lookup("include-tests")
				if flag == nil {
					step.Fatalf("%s command has no include-tests flag", name)
				}
				defaults[name] = [2]string{flag.DefValue, flag.Value.String()}
			}
		})

		t.Run("Then every command excludes test data by default", func(t *testing.T) {
			for name, values := range defaults {
				if values[0] != "false" || values[1] != "false" {
					t.Errorf(
						"%s include-tests defaults are declaration=%q value=%q",
						name,
						values[0],
						values[1],
					)
				}
			}
		})
	})
}
