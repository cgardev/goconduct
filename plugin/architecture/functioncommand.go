package architecture

import (
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/internal/query"
)

func newFunctionsCommand(configurationFlags *commandConfigurationFlags, analyzer graphAnalyzer) *cobra.Command {
	var component string
	var packagePath string
	var sortOrder string
	var includeTests bool
	var limit int
	command := &cobra.Command{
		Use:   "functions",
		Short: "Write filtered and sorted Go functions in JavaScript Object Notation (JSON).",
		Example: "  goconduct functions --component internal/library/logging " +
			"--sort incoming-call-sites --limit 20",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			selectedSort, err := query.ParseFunctionSort(sortOrder)
			if err != nil {
				return err
			}
			if err := validateQueryLimit(limit); err != nil {
				return err
			}
			graph, err := loadGraphForCommand(command, configurationFlags, analyzer)
			if err != nil {
				return err
			}
			result := query.Functions(graph, query.FunctionsParams{
				Component:    component,
				PackagePath:  packagePath,
				Sort:         selectedSort,
				IncludeTests: includeTests,
				Limit:        limit,
			})
			return writeQueryJSON(command.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(
		&component,
		"component",
		"",
		"Select the component with this exact identifier.",
	)
	command.Flags().StringVar(
		&packagePath,
		"package",
		"",
		"Select the package with this exact repository path.",
	)
	command.Flags().StringVar(
		&sortOrder,
		"sort",
		string(query.FunctionSortIncomingCallSites),
		"Select identifier, incoming-call-sites, outgoing-call-sites, afferent, efferent, "+
			"transitive-callers, transitive-callees, or instability.",
	)
	command.Flags().BoolVar(&includeTests, "include-tests", false, "Include test functions and test calls.")
	command.Flags().IntVar(
		&limit,
		"limit",
		20,
		"Return at most this number of functions. Zero returns all functions.",
	)
	return command
}

func newFunctionCommand(configurationFlags *commandConfigurationFlags, analyzer graphAnalyzer) *cobra.Command {
	var includeTests bool
	command := &cobra.Command{
		Use: "function <identifier>",
		Short: "Write one Go function with its caller functions, callee functions, " +
			"metrics, and call sites as JSON.",
		Example: "  goconduct function internal/library/logging.NewLogger --include-tests",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			graph, err := loadGraphForCommand(command, configurationFlags, analyzer)
			if err != nil {
				return err
			}
			result, err := query.GetFunction(graph, arguments[0], includeTests)
			if err != nil {
				return err
			}
			return writeQueryJSON(command.OutOrStdout(), result)
		},
	}
	command.Flags().BoolVar(&includeTests, "include-tests", false, "Include calls from test functions.")
	return command
}

func newFunctionCallsCommand(configurationFlags *commandConfigurationFlags, analyzer graphAnalyzer) *cobra.Command {
	var params query.FunctionCallsParams
	command := &cobra.Command{
		Use:   "calls",
		Short: "Write exact resolved function calls and source locations as JSON.",
		Example: "  goconduct calls --source-component cmd/control " +
			"--target-component internal/library/logging",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if err := validateQueryLimit(params.Limit); err != nil {
				return err
			}
			graph, err := loadGraphForCommand(command, configurationFlags, analyzer)
			if err != nil {
				return err
			}
			return writeQueryJSON(command.OutOrStdout(), query.FunctionCalls(graph, params))
		},
	}
	command.Flags().StringVar(
		&params.SourceComponent,
		"source-component",
		"",
		"Select calls from the component with this exact identifier.",
	)
	command.Flags().StringVar(
		&params.TargetComponent,
		"target-component",
		"",
		"Select calls to the component with this exact identifier.",
	)
	command.Flags().StringVar(
		&params.SourceFunction,
		"source-function",
		"",
		"Select calls from the function with this exact identifier.",
	)
	command.Flags().StringVar(
		&params.TargetFunction,
		"target-function",
		"",
		"Select calls to the function with this exact identifier.",
	)
	command.Flags().BoolVar(&params.IncludeTests, "include-tests", false, "Include calls from test functions.")
	command.Flags().IntVar(
		&params.Limit,
		"limit",
		50,
		"Return at most this number of resolved function calls. Zero returns all calls.",
	)
	return command
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"1d4d606fba951e1f20f012eb50b1dff24c0ac7a83006ad3c40da80b534723705","functions":[{"id":"func/newFunctionsCommand","name":"newFunctionsCommand","line":9,"end_line":70,"hash":"7d4d52abf06fe972a8ce7a64f3829d63905d99e6b4218045583461d96431604b"},{"id":"func/newFunctionCommand","name":"newFunctionCommand","line":72,"end_line":94,"hash":"c2873b1c646e83d9bde4c62d200abd74d0fc8296388d51e99ed01bd1c15fc47c"},{"id":"func/newFunctionCallsCommand","name":"newFunctionCallsCommand","line":96,"end_line":147,"hash":"d3095e4f4f6f304aff7683d83fc650e268cddcb0460d837c35829476d81a07f8"}]}
// mutate4go-manifest-end
