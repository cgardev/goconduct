package main

import (
	query "digginginsights.com/v3/internal/devtool/dependencygraph/internal/query"

	"github.com/spf13/cobra"
)

func newFunctionsCommand(options *commandConfigurationOptions) *cobra.Command {
	var component string
	var packagePath string
	var sortOrder string
	var includeTests bool
	var limit int
	command := &cobra.Command{
		Use:   "functions",
		Short: "Write filtered and sorted Go functions in JavaScript Object Notation (JSON).",
		Example: "  dependencygraph functions --component internal/library/foundationdomain " +
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
			graph, err := loadGraphForCommand(command, options)
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

func newFunctionCommand(options *commandConfigurationOptions) *cobra.Command {
	var includeTests bool
	command := &cobra.Command{
		Use: "function <identifier>",
		Short: "Write one Go function with its caller functions, callee functions, " +
			"metrics, and call sites as JSON.",
		Example: "  dependencygraph function internal/library/foundationdomain.NewError --include-tests",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			graph, err := loadGraphForCommand(command, options)
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

func newFunctionCallsCommand(options *commandConfigurationOptions) *cobra.Command {
	var params query.FunctionCallsParams
	command := &cobra.Command{
		Use:   "calls",
		Short: "Write exact resolved function calls and source locations as JSON.",
		Example: "  dependencygraph calls --source-component cmd/control " +
			"--target-component internal/library/foundationdomain",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if err := validateQueryLimit(params.Limit); err != nil {
				return err
			}
			graph, err := loadGraphForCommand(command, options)
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
// {"version":1,"tested_at":"2026-08-21T17:15:23Z","module_hash":"8e8813cfae282182b100419c9c30c14b4aa85c7adab3da1c12d12eee682b711f","functions":[{"id":"func/newFunctionsCommand","name":"newFunctionsCommand","line":9,"end_line":70,"hash":"77842b653d3fbb065b3ccc03667838071adf62339c43aae91ad0b3a85b80c061"},{"id":"func/newFunctionCommand","name":"newFunctionCommand","line":72,"end_line":94,"hash":"352b11cdac55ff38e9d2fa4e0fe7538e4ba665cc9846803a4a2cb8eb312b3ff7"},{"id":"func/newFunctionCallsCommand","name":"newFunctionCallsCommand","line":96,"end_line":147,"hash":"241cd802be419984ecedb533fba9daa87fa2405067746e690cca3f69f99f1b33"}]}
// mutate4go-manifest-end
