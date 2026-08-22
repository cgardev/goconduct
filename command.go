package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/cgardev/gokeel/conf"
	"github.com/spf13/cobra"

	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/query"
)

type graphAnalyzer interface {
	analyze(ctx context.Context, configuration ApplicationConfiguration) (Graph, error)
}

type dashboardRunner interface {
	runDashboard(ctx context.Context, configuration ApplicationConfiguration) error
}

type commandRuntime interface {
	graphAnalyzer
	dashboardRunner
}

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

type commandConfigurationFlags struct {
	configurationPath string
	repositoryRoot    string
	analysisPaths     []string
	ignoredPaths      []string
	serverAddress     string
	cacheMode         string
	cacheTimeout      time.Duration
}

func newRootCommand(runtime commandRuntime) *cobra.Command {
	defaults := DefaultApplicationConfiguration()
	configurationFlags := &commandConfigurationFlags{}
	var refreshIntervalOverride time.Duration
	command := &cobra.Command{
		Use:           "dependencygraph",
		Short:         "Analyze Go component imports and resolved function calls.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			configuration, err := configurationFlags.loadConfiguration(command)
			if err != nil {
				return err
			}
			if command.Flags().Changed("refresh-interval") {
				configuration.Server.RefreshInterval = refreshIntervalOverride
			}
			return runtime.runDashboard(command.Context(), configuration)
		},
	}
	command.PersistentFlags().StringVar(
		&configurationFlags.serverAddress,
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
		&configurationFlags.configurationPath,
		"configuration",
		defaultConfigurationPath,
		"Read the optional JavaScript Object Notation (JSON) configuration from this path.",
	)
	command.PersistentFlags().StringVar(
		&configurationFlags.repositoryRoot,
		"root",
		defaults.Analysis.RepositoryRoot,
		"Use this repository root instead of the configured root.",
	)
	command.PersistentFlags().StringArrayVar(
		&configurationFlags.analysisPaths,
		"analysis-path",
		nil,
		"Analyze this repository-relative path. Repeat this option to replace all configured paths.",
	)
	command.PersistentFlags().StringArrayVar(
		&configurationFlags.ignoredPaths,
		"ignore-path",
		nil,
		"Ignore this path pattern. Repeat this option to replace all configured patterns.",
	)
	command.PersistentFlags().StringVar(
		&configurationFlags.cacheMode,
		"cache",
		string(defaults.Cache.Mode),
		"Select the graph source for CLI queries. Use auto, server, or local.",
	)
	command.PersistentFlags().DurationVar(
		&configurationFlags.cacheTimeout,
		"cache-timeout",
		defaults.Cache.RequestTimeout,
		"Set the maximum time for one active graph cache request.",
	)
	command.AddCommand(
		newAnalyzeCommand(configurationFlags, runtime),
		newSummaryCommand(configurationFlags, runtime),
		newFindingsCommand(configurationFlags, runtime),
		newComponentsCommand(configurationFlags, runtime),
		newComponentCommand(configurationFlags, runtime),
		newFunctionsCommand(configurationFlags, runtime),
		newFunctionCommand(configurationFlags, runtime),
		newFunctionCallsCommand(configurationFlags, runtime),
		newConfigurationSchemaCommand(),
	)
	return command
}

func (flags *commandConfigurationFlags) loadConfiguration(
	command *cobra.Command,
) (ApplicationConfiguration, error) {
	configuration, err := loadApplicationConfiguration(flags.configurationPath)
	if err != nil {
		return ApplicationConfiguration{}, err
	}
	persistentFlags := command.Root().PersistentFlags()
	if persistentFlags.Changed("root") {
		configuration.Analysis.RepositoryRoot = flags.repositoryRoot
	}
	if persistentFlags.Changed("analysis-path") {
		configuration.Analysis.Paths = flags.analysisPaths
	}
	if persistentFlags.Changed("ignore-path") {
		configuration.Analysis.IgnoredPaths = flags.ignoredPaths
	}
	if persistentFlags.Changed("address") {
		configuration.Server.Address = flags.serverAddress
	}
	if persistentFlags.Changed("cache") {
		configuration.Cache.Mode = CacheMode(flags.cacheMode)
	}
	if persistentFlags.Changed("cache-timeout") {
		configuration.Cache.RequestTimeout = flags.cacheTimeout
	}
	if err := validateCacheConfiguration(configuration.Cache); err != nil {
		return ApplicationConfiguration{}, err
	}
	return configuration, nil
}

func newAnalyzeCommand(configurationFlags *commandConfigurationFlags, analyzer graphAnalyzer) *cobra.Command {
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
			graph, err := loadGraphForCommand(command, configurationFlags, analyzer)
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
	configurationFlags *commandConfigurationFlags,
	analyzer graphAnalyzer,
) (Graph, error) {
	configuration, err := configurationFlags.loadConfiguration(command)
	if err != nil {
		return Graph{}, err
	}
	return analyzer.analyze(command.Context(), configuration)
}

func newSummaryCommand(configurationFlags *commandConfigurationFlags, analyzer graphAnalyzer) *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Write the configured scope, policy, and counts in JavaScript Object Notation (JSON).",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			graph, err := loadGraphForCommand(command, configurationFlags, analyzer)
			if err != nil {
				return err
			}
			return writeQueryJSON(command.OutOrStdout(), query.Summary(graph))
		},
	}
}

func newFindingsCommand(configurationFlags *commandConfigurationFlags, analyzer graphAnalyzer) *cobra.Command {
	var severity string
	var rule string
	var component string
	var limit int
	command := &cobra.Command{
		Use:   "findings",
		Short: "Write filtered architecture findings in JavaScript Object Notation (JSON).",
		Example: "  dependencygraph findings --severity error\n" +
			"  dependencygraph findings --rule stable-dependency-principle\n" +
			"  dependencygraph findings --component internal/library/logging",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			severityFilter, err := query.ParseFindingSeverity(severity)
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

func newComponentsCommand(configurationFlags *commandConfigurationFlags, analyzer graphAnalyzer) *cobra.Command {
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
			graph, err := loadGraphForCommand(command, configurationFlags, analyzer)
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

func newComponentCommand(configurationFlags *commandConfigurationFlags, analyzer graphAnalyzer) *cobra.Command {
	return &cobra.Command{
		Use: "component <identifier>",
		Short: "Write one component, its imports, its importing components, and its findings " +
			"in JavaScript Object Notation (JSON).",
		Example: "  dependencygraph component internal/library/logging",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			graph, err := loadGraphForCommand(command, configurationFlags, analyzer)
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
		return newValidationError("query limit must not be negative", nil)
	}
	return nil
}

func writeQueryJSON(output io.Writer, queryResult any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(queryResult); err != nil {
		return newUnavailableError("encode query result", err)
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
				return newInternalError("generate configuration schema", err)
			}
			if _, err := command.OutOrStdout().Write(schema); err != nil {
				return newUnavailableError("write configuration schema", err)
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
		return "", newValidationError(
			fmt.Sprintf("analysis view %q must be report or graph", value),
			nil,
		)
	}
}

func parseFindingThreshold(value string) (findingThreshold, error) {
	threshold := findingThreshold(value)
	switch threshold {
	case findingThresholdNone, findingThresholdWarning, findingThresholdError:
		return threshold, nil
	default:
		return "", newValidationError(
			fmt.Sprintf("finding threshold %q must be none, warning, or error", value),
			nil,
		)
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
		return newUnavailableError("encode architecture analysis", err)
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
	return newBusinessRuleError(
		fmt.Sprintf("%d findings at %s severity or higher", failures, threshold),
		nil,
	)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T19:47:25Z","module_hash":"c540eefefcae6c00f9788b09097e1f2c983302493d26f0865fe73bb2e61fb4c5","functions":[{"id":"func/newRootCommand","name":"newRootCommand","line":64,"end_line":145,"hash":"580bf74dab4b8041b7580af6d7c20b06a82adbe66cb00cbbfdffbab0e8660639"},{"id":"func/commandConfigurationFlags.loadConfiguration","name":"commandConfigurationFlags.loadConfiguration","line":147,"end_line":177,"hash":"75165e9e3937b400e52cf5834a724cc8ee2db82a0ff3005ec9aaf52db540f23a"},{"id":"func/newAnalyzeCommand","name":"newAnalyzeCommand","line":179,"end_line":224,"hash":"b4de892c331a2988566fd2a63d7bcb30999403373c02c8d5a1728611091d0398"},{"id":"func/loadGraphForCommand","name":"loadGraphForCommand","line":226,"end_line":236,"hash":"0d62199532c79ca7d56efae3ec09ccfcc9e14c4025d64c7f8863f99623cb44e2"},{"id":"func/newSummaryCommand","name":"newSummaryCommand","line":238,"end_line":251,"hash":"83c1235292dc0c0f38aa64d7d0c286ca40f2b5aa9b5b3856fd0e195af249f49b"},{"id":"func/newFindingsCommand","name":"newFindingsCommand","line":253,"end_line":301,"hash":"7c6b2769600b3757afe60939b9875e913160cae28d78a8131cf2df9769c82ed5"},{"id":"func/newComponentsCommand","name":"newComponentsCommand","line":303,"end_line":366,"hash":"6acc1e42b3b99b2e992612b0c46c895825f690384bc43d6296eb90c73d07d779"},{"id":"func/newComponentCommand","name":"newComponentCommand","line":368,"end_line":387,"hash":"84b321bb745884ed66a7114204027f4af7a6c5e2123eda3ab628083ac44677d8"},{"id":"func/validateQueryLimit","name":"validateQueryLimit","line":389,"end_line":394,"hash":"137d5d4d8d0710499af12a630b0990318d216aeebdf8b59aef9d7934df5e2ca6"},{"id":"func/writeQueryJSON","name":"writeQueryJSON","line":396,"end_line":404,"hash":"de1792654e01679c030b556a26b7287c525dbe2a439201b2a6e95597b305b10b"},{"id":"func/newConfigurationSchemaCommand","name":"newConfigurationSchemaCommand","line":406,"end_line":425,"hash":"31b549dd67a25898f82e845dff6aa287470d2e2cab2d33f4ec0421807e521228"},{"id":"func/parseAnalysisView","name":"parseAnalysisView","line":427,"end_line":438,"hash":"2f0bbbd831be7a7e6bc5813d6bd9df6a7d6e07db5c56ebeaa42957ae493a67cc"},{"id":"func/parseFindingThreshold","name":"parseFindingThreshold","line":440,"end_line":451,"hash":"815da9041742486570baf886be5ea5d4f7f63efe39bd519df5e5402d90242018"},{"id":"func/writeAnalysisJSON","name":"writeAnalysisJSON","line":453,"end_line":480,"hash":"573670eab51b89384b49d2eecb7ab5e17b73fc72a2182bbcbbe027e0be1d7653"},{"id":"func/enforceFindingThreshold","name":"enforceFindingThreshold","line":482,"end_line":499,"hash":"c2d30c81317ebc6885a3e010128235cdf069cd8a71e3733ae209931c90060d88"}]}
// mutate4go-manifest-end
