package main

import (
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
			"--sort incoming-calls --limit 20",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			selectedSort, err := parseFunctionSort(sortOrder)
			if err != nil {
				return err
			}
			if err := validateQueryLimit(limit); err != nil {
				return err
			}
			graph, err := analyzeConfiguredGraph(command, options)
			if err != nil {
				return err
			}
			result := queryFunctions(graph, functionsQuery{
				component:    component,
				packagePath:  packagePath,
				sort:         selectedSort,
				includeTests: includeTests,
				limit:        limit,
			})
			return writeQueryJSON(command.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(&component, "component", "", "Select one exact component identifier.")
	command.Flags().StringVar(&packagePath, "package", "", "Select one exact repository package path.")
	command.Flags().StringVar(
		&sortOrder,
		"sort",
		string(functionSortIncomingCalls),
		"Select identifier, incoming-calls, outgoing-calls, afferent, efferent, "+
			"transitive-callers, transitive-callees, or instability.",
	)
	command.Flags().BoolVar(&includeTests, "include-tests", false, "Include test functions and test calls.")
	command.Flags().IntVar(&limit, "limit", 20, "Return at most this number of functions. Zero returns all functions.")
	return command
}

func newFunctionCommand(options *commandConfigurationOptions) *cobra.Command {
	var includeTests bool
	command := &cobra.Command{
		Use:     "function <identifier>",
		Short:   "Write one Go function with its callers, callees, metrics, and call sites as JSON.",
		Example: "  dependencygraph function internal/library/foundationdomain.NewError --include-tests",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			graph, err := analyzeConfiguredGraph(command, options)
			if err != nil {
				return err
			}
			result, err := queryFunction(graph, arguments[0], includeTests)
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
	var query functionCallsQuery
	command := &cobra.Command{
		Use:   "calls",
		Short: "Write exact resolved function calls and source locations as JSON.",
		Example: "  dependencygraph calls --source-component cmd/control " +
			"--target-component internal/library/foundationdomain",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if err := validateQueryLimit(query.limit); err != nil {
				return err
			}
			graph, err := analyzeConfiguredGraph(command, options)
			if err != nil {
				return err
			}
			return writeQueryJSON(command.OutOrStdout(), queryFunctionCalls(graph, query))
		},
	}
	command.Flags().StringVar(
		&query.sourceComponent,
		"source-component",
		"",
		"Select calls from one exact component identifier.",
	)
	command.Flags().StringVar(
		&query.targetComponent,
		"target-component",
		"",
		"Select calls to one exact component identifier.",
	)
	command.Flags().StringVar(
		&query.sourceFunction,
		"source-function",
		"",
		"Select calls from one exact function identifier.",
	)
	command.Flags().StringVar(
		&query.targetFunction,
		"target-function",
		"",
		"Select calls to one exact function identifier.",
	)
	command.Flags().BoolVar(&query.includeTests, "include-tests", false, "Include calls from test functions.")
	command.Flags().IntVar(&query.limit, "limit", 50, "Return at most this number of call relations. Zero returns all.")
	return command
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T10:42:03Z","module_hash":"47b5d55d33f989b61a4239552e59c5f1f20d9b730c5a00d0ac435e448b82d7ac","functions":[{"id":"func/newFunctionsCommand","name":"newFunctionsCommand","line":7,"end_line":53,"hash":"7927bf8761811377134c0301e90ea583bf303c3d1671cb11ee4bc31d68aed1af"},{"id":"func/newFunctionCommand","name":"newFunctionCommand","line":55,"end_line":76,"hash":"741d56ac8e30e2412e18de50561445294bfc04dd242900f2b40a15dc9a918dc1"},{"id":"func/newFunctionCallsCommand","name":"newFunctionCallsCommand","line":78,"end_line":124,"hash":"b18f734f59013aa6ff775a5704909bdedcccb52bf7edffb6af80f13f309cc0e0"}]}
// mutate4go-manifest-end
