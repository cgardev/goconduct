package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	query "digginginsights.com/v3/internal/devtool/dependencygraph/internal/query"

	"github.com/cgardev/gokeel/conf"
	"github.com/spf13/cobra"
)

var errArchitectureFindingThresholdReached = errors.New("architecture findings meet the failure threshold")

type analysisView string

const (
	analysisViewReport analysisView = "report"
	analysisViewGraph  analysisView = "graph"
)

type findingThreshold string

const (
	findingThresholdNone    findingThreshold = "none"
	findingThresholdWarning findingThreshold = "warning"
	findingThresholdError   findingThreshold = "error"
)

type analysisReport struct {
	SchemaVersion int            `json:"schemaVersion"`
	Revision      string         `json:"revision"`
	ModulePath    string         `json:"modulePath"`
	Scope         AnalysisScope  `json:"scope"`
	Policy        AnalysisPolicy `json:"policy"`
	Summary       GraphSummary   `json:"summary"`
	Findings      []Finding      `json:"findings"`
}

type commandConfigurationOptions struct {
	configurationPath string
	repositoryRoot    string
	analysisPaths     []string
	ignoredPaths      []string
	serverAddress     string
	cacheMode         string
	cacheTimeout      time.Duration
}

func newRootCommand(logger *slog.Logger) *cobra.Command {
	defaults := DefaultApplicationConfiguration()
	options := &commandConfigurationOptions{}
	var refreshIntervalOverride time.Duration
	command := &cobra.Command{
		Use:           "dependencygraph",
		Short:         "Analyze Go component imports and resolved function calls.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			configuration, err := options.loadConfiguration(command)
			if err != nil {
				return err
			}
			if command.Flags().Changed("refresh-interval") {
				configuration.Server.RefreshInterval = refreshIntervalOverride
			}
			if configuration.Server.RefreshInterval < minimumRefreshInterval() {
				return fmt.Errorf(
					"refresh interval must be at least %s",
					minimumRefreshInterval(),
				)
			}
			analyzer, err := newAnalyzer(configuration.Analysis)
			if err != nil {
				return err
			}
			monitor, err := newGraphMonitor(
				command.Context(),
				analyzer,
				configuration.Server.RefreshInterval,
				logger,
			)
			if err != nil {
				if isCommandContextError(command.Context(), err) {
					return nil
				}
				return err
			}
			return runDashboard(command.Context(), monitor, configuration.Server.Address, logger)
		},
	}
	command.PersistentFlags().StringVar(
		&options.serverAddress,
		"address",
		defaults.Server.Address,
		"Use this dashboard and graph cache address instead of the configured address.",
	)
	command.Flags().DurationVar(
		&refreshIntervalOverride,
		"refresh-interval",
		defaults.Server.RefreshInterval,
		"Use this file check interval instead of the configured interval.",
	)
	command.PersistentFlags().StringVar(
		&options.configurationPath,
		"configuration",
		defaultConfigurationPath,
		"Read the optional JavaScript Object Notation (JSON) configuration from this path.",
	)
	command.PersistentFlags().StringVar(
		&options.repositoryRoot,
		"root",
		defaults.Analysis.RepositoryRoot,
		"Use this repository root instead of the configured root.",
	)
	command.PersistentFlags().StringArrayVar(
		&options.analysisPaths,
		"analysis-path",
		nil,
		"Analyze this repository-relative path. Repeat this option to replace all configured paths.",
	)
	command.PersistentFlags().StringArrayVar(
		&options.ignoredPaths,
		"ignore-path",
		nil,
		"Ignore this path pattern. Repeat this option to replace all configured patterns.",
	)
	command.PersistentFlags().StringVar(
		&options.cacheMode,
		"cache",
		string(defaults.Cache.Mode),
		"Select the graph source for CLI queries. Use auto, server, or local.",
	)
	command.PersistentFlags().DurationVar(
		&options.cacheTimeout,
		"cache-timeout",
		defaults.Cache.RequestTimeout,
		"Set the maximum time for one active graph cache request.",
	)
	command.AddCommand(
		newAnalyzeCommand(options),
		newSummaryCommand(options),
		newFindingsCommand(options),
		newComponentsCommand(options),
		newComponentCommand(options),
		newFunctionsCommand(options),
		newFunctionCommand(options),
		newFunctionCallsCommand(options),
		newConfigurationSchemaCommand(),
	)
	return command
}

func isCommandContextError(ctx context.Context, err error) bool {
	contextError := ctx.Err()
	return contextError != nil && errors.Is(err, contextError)
}

func (options *commandConfigurationOptions) loadConfiguration(
	command *cobra.Command,
) (ApplicationConfiguration, error) {
	configuration, err := loadApplicationConfiguration(options.configurationPath)
	if err != nil {
		return ApplicationConfiguration{}, err
	}
	flags := command.Root().PersistentFlags()
	if flags.Changed("root") {
		configuration.Analysis.RepositoryRoot = options.repositoryRoot
	}
	if flags.Changed("analysis-path") {
		configuration.Analysis.Paths = options.analysisPaths
	}
	if flags.Changed("ignore-path") {
		configuration.Analysis.IgnoredPaths = options.ignoredPaths
	}
	if flags.Changed("address") {
		configuration.Server.Address = options.serverAddress
	}
	if flags.Changed("cache") {
		configuration.Cache.Mode = CacheMode(options.cacheMode)
	}
	if flags.Changed("cache-timeout") {
		configuration.Cache.RequestTimeout = options.cacheTimeout
	}
	if err := validateCacheConfiguration(configuration.Cache); err != nil {
		return ApplicationConfiguration{}, err
	}
	return configuration, nil
}

func newAnalyzeCommand(options *commandConfigurationOptions) *cobra.Command {
	var requestedView string
	var failureThreshold string
	var indentOutput bool
	command := &cobra.Command{
		Use:   "analyze",
		Short: "Write a deterministic architecture analysis in JavaScript Object Notation (JSON).",
		Example: "  dependencygraph analyze --configuration configuration.json\n" +
			"  dependencygraph analyze --root . --analysis-path cmd --analysis-path internal\n" +
			"  dependencygraph analyze --root . --view graph --indent\n" +
			"  dependencygraph analyze --root . --fail-on error",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			selectedView, err := parseAnalysisView(requestedView)
			if err != nil {
				return err
			}
			selectedThreshold, err := parseFindingThreshold(failureThreshold)
			if err != nil {
				return err
			}
			graph, err := loadGraphForCommand(command, options)
			if err != nil {
				return err
			}
			if err := writeAnalysisJSON(command.OutOrStdout(), graph, selectedView, indentOutput); err != nil {
				return err
			}
			return enforceFindingThreshold(graph.Findings, selectedThreshold)
		},
	}
	command.Flags().StringVar(
		&requestedView,
		"view",
		string(analysisViewReport),
		"Select the JSON view. Use report or graph.",
	)
	command.Flags().StringVar(
		&failureThreshold,
		"fail-on",
		string(findingThresholdNone),
		"Exit with an error for findings at this severity. Use none, warning, or error.",
	)
	command.Flags().BoolVar(&indentOutput, "indent", false, "Indent the JSON output.")
	return command
}

func loadGraphForCommand(
	command *cobra.Command,
	options *commandConfigurationOptions,
) (Graph, error) {
	configuration, err := options.loadConfiguration(command)
	if err != nil {
		return Graph{}, err
	}
	return analyzeGraph(command.Context(), configuration)
}

func newSummaryCommand(options *commandConfigurationOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Write the configured scope, policy, and counts in JavaScript Object Notation (JSON).",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			graph, err := loadGraphForCommand(command, options)
			if err != nil {
				return err
			}
			return writeQueryJSON(command.OutOrStdout(), query.Summary(graph))
		},
	}
}

func newFindingsCommand(options *commandConfigurationOptions) *cobra.Command {
	var severity string
	var rule string
	var component string
	var limit int
	command := &cobra.Command{
		Use:   "findings",
		Short: "Write filtered architecture findings in JavaScript Object Notation (JSON).",
		Example: "  dependencygraph findings --severity error\n" +
			"  dependencygraph findings --rule stable-dependency-principle\n" +
			"  dependencygraph findings --component internal/library/foundationdomain",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			severityFilter, err := query.ParseFindingSeverity(severity)
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
			findingQueryResult := query.Findings(graph, query.FindingsParams{
				Severity:  severityFilter,
				Rule:      rule,
				Component: component,
				Limit:     limit,
			})
			return writeQueryJSON(command.OutOrStdout(), findingQueryResult)
		},
	}
	command.Flags().StringVar(
		&severity,
		"severity",
		string(query.FindingSeverityAll),
		"Select the finding severity. Use all, warning, or error.",
	)
	command.Flags().StringVar(&rule, "rule", "", "Match this exact finding rule identifier.")
	command.Flags().StringVar(&component, "component", "", "Match this exact component identifier.")
	command.Flags().IntVar(
		&limit,
		"limit",
		0,
		"Return at most this number of findings. Zero returns all findings.",
	)
	return command
}

func newComponentsCommand(options *commandConfigurationOptions) *cobra.Command {
	var role string
	var category string
	var sortOrder string
	var limit int
	command := &cobra.Command{
		Use:   "components",
		Short: "Write filtered and sorted components in JavaScript Object Notation (JSON).",
		Example: "  dependencygraph components --sort afferent --limit 10\n" +
			"  dependencygraph components --role library --sort importers --limit 20\n" +
			"  dependencygraph components --category plugin --sort afferent",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			roleFilter, err := query.ParseComponentRole(role)
			if err != nil {
				return err
			}
			selectedSort, err := query.ParseComponentSort(sortOrder)
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
			componentQueryResult := query.Components(graph, query.ComponentsParams{
				Role:     roleFilter,
				Category: category,
				Sort:     selectedSort,
				Limit:    limit,
			})
			return writeQueryJSON(command.OutOrStdout(), componentQueryResult)
		},
	}
	command.Flags().StringVar(
		&role,
		"role",
		"all",
		"Select a component role. Use all to select all roles.",
	)
	command.Flags().StringVar(
		&category,
		"category",
		"",
		"Select the configured component category with this exact identifier.",
	)
	command.Flags().StringVar(
		&sortOrder,
		"sort",
		string(query.ComponentSortAfferent),
		"Select the sort field. Use identifier, afferent, efferent, importers, dependencies, "+
			"instability, abstractness, distance, or files.",
	)
	command.Flags().IntVar(
		&limit,
		"limit",
		20,
		"Return at most this number of components. Zero returns all components.",
	)
	return command
}

func newComponentCommand(options *commandConfigurationOptions) *cobra.Command {
	return &cobra.Command{
		Use: "component <identifier>",
		Short: "Write one component, its imports, its importing components, and its findings " +
			"in JavaScript Object Notation (JSON).",
		Example: "  dependencygraph component internal/library/foundationdomain",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			graph, err := loadGraphForCommand(command, options)
			if err != nil {
				return err
			}
			componentQueryResult, err := query.GetComponent(graph, arguments[0])
			if err != nil {
				return err
			}
			return writeQueryJSON(command.OutOrStdout(), componentQueryResult)
		},
	}
}

func validateQueryLimit(limit int) error {
	if limit < 0 {
		return fmt.Errorf("query limit must not be negative")
	}
	return nil
}

func writeQueryJSON(output io.Writer, queryResult any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(queryResult); err != nil {
		return fmt.Errorf("encode query result: %w", err)
	}
	return nil
}

func newConfigurationSchemaCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "configuration-schema",
		Short: "Write the configuration schema in JavaScript Object Notation (JSON).",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			schema, err := conf.GenerateSchema(
				ApplicationConfiguration{},
				applicationSchemaDefinition(),
			)
			if err != nil {
				return fmt.Errorf("generate configuration schema: %w", err)
			}
			if _, err := command.OutOrStdout().Write(schema); err != nil {
				return fmt.Errorf("write configuration schema: %w", err)
			}
			return nil
		},
	}
}

func parseAnalysisView(value string) (analysisView, error) {
	view := analysisView(value)
	switch view {
	case analysisViewReport, analysisViewGraph:
		return view, nil
	default:
		return "", fmt.Errorf("analysis view %q must be report or graph", value)
	}
}

func parseFindingThreshold(value string) (findingThreshold, error) {
	threshold := findingThreshold(value)
	switch threshold {
	case findingThresholdNone, findingThresholdWarning, findingThresholdError:
		return threshold, nil
	default:
		return "", fmt.Errorf("finding threshold %q must be none, warning, or error", value)
	}
}

func writeAnalysisJSON(
	output io.Writer,
	graph Graph,
	view analysisView,
	indentOutput bool,
) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if indentOutput {
		encoder.SetIndent("", "  ")
	}
	var analysisOutput any = analysisReport{
		SchemaVersion: graph.SchemaVersion,
		Revision:      graph.Revision,
		ModulePath:    graph.ModulePath,
		Scope:         graph.Scope,
		Policy:        graph.Policy,
		Summary:       graph.Summary,
		Findings:      graph.Findings,
	}
	if view == analysisViewGraph {
		analysisOutput = graph
	}
	if err := encoder.Encode(analysisOutput); err != nil {
		return fmt.Errorf("encode architecture analysis: %w", err)
	}
	return nil
}

func enforceFindingThreshold(findings []Finding, threshold findingThreshold) error {
	if threshold == findingThresholdNone {
		return nil
	}
	failures := 0
	for _, finding := range findings {
		if threshold == findingThresholdWarning || finding.Severity == findingSeverityError {
			failures++
		}
	}
	if failures == 0 {
		return nil
	}
	return fmt.Errorf(
		"%w: %d findings at %s severity or higher",
		errArchitectureFindingThresholdReached,
		failures,
		threshold,
	)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T17:18:02Z","module_hash":"dbc0e6bacf40db868fc146ca64828e9616bca9c32acca94cce425e05486b2526","functions":[{"id":"func/newRootCommand","name":"newRootCommand","line":55,"end_line":158,"hash":"756884de1ae0233fa025aed2137786d7f5c2c258a2ac5c390c9330065b8f947e"},{"id":"func/isCommandContextError","name":"isCommandContextError","line":160,"end_line":163,"hash":"cb42632c8af285e4d142b63705c8842eb0e26bce3fe0cd3564990805bbe6bf55"},{"id":"func/commandConfigurationOptions.loadConfiguration","name":"commandConfigurationOptions.loadConfiguration","line":165,"end_line":195,"hash":"8b89961e60397e18b983531b1f07994ff31eb87368d04d1a27274ea67c9b407e"},{"id":"func/newAnalyzeCommand","name":"newAnalyzeCommand","line":197,"end_line":242,"hash":"f531efa9c9ca7f024793775a5c86c4a017ed69603848f5214ddc14e083e13b4d"},{"id":"func/loadGraphForCommand","name":"loadGraphForCommand","line":244,"end_line":253,"hash":"4bbb5eff180aaab5ca68381a5a6093f2f333be4ae1a7acbafd9ff0d178d654aa"},{"id":"func/newSummaryCommand","name":"newSummaryCommand","line":255,"end_line":268,"hash":"31c0e7cd13482e38320e829533147246bf3c290cec55544a51be58f9d5f734f1"},{"id":"func/newFindingsCommand","name":"newFindingsCommand","line":270,"end_line":318,"hash":"f4d91f39ef5bc330fd16503f088dfb2388086838e7256c94c7793e0d697bf544"},{"id":"func/newComponentsCommand","name":"newComponentsCommand","line":320,"end_line":383,"hash":"e7a4a3082bef2cce86c526fea971e0d6a22d2839326db1f66a3b375ba19c611c"},{"id":"func/newComponentCommand","name":"newComponentCommand","line":385,"end_line":404,"hash":"9e3fd3cde87f3789ef1aedf7d9c36879365c6066cf68ae962e00558a81d6fd4e"},{"id":"func/validateQueryLimit","name":"validateQueryLimit","line":406,"end_line":411,"hash":"128deffab20785dcb9652565027ab85ad6baefb5ede20ba2baf393b2f8f0249c"},{"id":"func/writeQueryJSON","name":"writeQueryJSON","line":413,"end_line":421,"hash":"d47a0e9de8b39efbf32698714a9acc638d95981d4e13326fc140dce0f6e2b353"},{"id":"func/newConfigurationSchemaCommand","name":"newConfigurationSchemaCommand","line":423,"end_line":442,"hash":"28980aafb3906312da958b0d1f01ab0a0d1df7e5cc8aa7693d19cfa4782ca1c3"},{"id":"func/parseAnalysisView","name":"parseAnalysisView","line":444,"end_line":452,"hash":"2e911c0bcec32108ac0be9988ef64e2daf7e3aebff83847c1116747c0ef4b6f8"},{"id":"func/parseFindingThreshold","name":"parseFindingThreshold","line":454,"end_line":462,"hash":"92e8a1de6869b3365cfabdd1bde963ec1db5877f7f070beceb78fabd2719786d"},{"id":"func/writeAnalysisJSON","name":"writeAnalysisJSON","line":464,"end_line":491,"hash":"da7545861aca744e566b083bd56a90ea969f2de68e57204b8ff83526eede8ebc"},{"id":"func/enforceFindingThreshold","name":"enforceFindingThreshold","line":493,"end_line":512,"hash":"a83bfa7a7f8a290abc28eb54cea09477b17b6c99de081093066c72cf4169ffb6"}]}
// mutate4go-manifest-end
