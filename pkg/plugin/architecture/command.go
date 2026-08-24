package architecture

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"time"

	"github.com/cgardev/gokeel/conf"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/internal/query"
	"github.com/cgardev/goconduct/pkg/failure"
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

// RunCommand executes the standalone architecture plugin command.
func RunCommand(ctx context.Context, logger *slog.Logger, arguments []string) error {
	runtime := newDependencyGraphRuntime(
		analyzerGraphSourceFactory{},
		httpGraphCacheFactory{},
		logger,
	)
	command := newRootCommand(runtime)
	command.SetArgs(slices.Clone(arguments))
	return command.ExecuteContext(ctx)
}

func newRootCommand(runtime commandRuntime) *cobra.Command {
	defaults := DefaultApplicationConfiguration()
	configurationFlags := &commandConfigurationFlags{}
	var refreshIntervalOverride time.Duration
	command := &cobra.Command{
		Use:           "goconduct",
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
		Example: "  goconduct analyze --configuration .goconduct.json\n" +
			"  goconduct analyze --root . --analysis-path cmd --analysis-path internal\n" +
			"  goconduct analyze --root . --view graph --indent\n" +
			"  goconduct analyze --root . --fail-on error",
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
		Example: "  goconduct findings --severity error\n" +
			"  goconduct findings --rule stable-dependency-principle\n" +
			"  goconduct findings --component internal/library/logging",
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
		Example: "  goconduct components --sort afferent --limit 10\n" +
			"  goconduct components --role library --sort importers --limit 20\n" +
			"  goconduct components --category plugin --sort afferent",
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
		Example: "  goconduct component internal/library/logging",
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
	return query.ValidateLimit(limit)
}

func writeQueryJSON(output io.Writer, queryResult any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(queryResult); err != nil {
		return failure.Unavailable("encode query result", err)
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
				return failure.Internal("generate configuration schema", err)
			}
			if _, err := command.OutOrStdout().Write(schema); err != nil {
				return failure.Unavailable("write configuration schema", err)
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
		return "", failure.Validation(
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
		return "", failure.Validation(
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
		return failure.Unavailable("encode architecture analysis", err)
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
	return failure.BusinessRule(
		fmt.Sprintf("%d findings at %s severity or higher", failures, threshold),
		nil,
	)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-23T21:34:41Z","module_hash":"a2f496d57d421a4c42c83bd468766531178d8ebcafe2be5e8138e1db4a47db41","functions":[{"id":"func/RunCommand","name":"RunCommand","line":68,"end_line":77,"hash":"7e2c39a627dc224d0c8dc9f6dabfcceb2dafb81ea5f3671a51e0c3367e627b64"},{"id":"func/newRootCommand","name":"newRootCommand","line":79,"end_line":160,"hash":"61167e4eae33a321ef9a46dd9a3b4b78cf6df090c05b667dd1c477f8e99faf51"},{"id":"func/commandConfigurationFlags.loadConfiguration","name":"commandConfigurationFlags.loadConfiguration","line":162,"end_line":192,"hash":"75165e9e3937b400e52cf5834a724cc8ee2db82a0ff3005ec9aaf52db540f23a"},{"id":"func/newAnalyzeCommand","name":"newAnalyzeCommand","line":194,"end_line":239,"hash":"64b628c90eb6a3a3cacb945628d24a17040d35e59a8a6a843ef0f5b97f64cec4"},{"id":"func/loadGraphForCommand","name":"loadGraphForCommand","line":241,"end_line":251,"hash":"0d62199532c79ca7d56efae3ec09ccfcc9e14c4025d64c7f8863f99623cb44e2"},{"id":"func/newSummaryCommand","name":"newSummaryCommand","line":253,"end_line":266,"hash":"83c1235292dc0c0f38aa64d7d0c286ca40f2b5aa9b5b3856fd0e195af249f49b"},{"id":"func/newFindingsCommand","name":"newFindingsCommand","line":268,"end_line":316,"hash":"6e7631509eb57c61351f04e83f895a07e2a33c705a8733293ff64402ccad57c3"},{"id":"func/newComponentsCommand","name":"newComponentsCommand","line":318,"end_line":381,"hash":"f3270f4a65065996cc13e8ca33bc3650707be9119b94ca2d19e64a2c9a745140"},{"id":"func/newComponentCommand","name":"newComponentCommand","line":383,"end_line":402,"hash":"cc0ef56a5793fa16992c1eb25bf7ddecdd680e2860bd2aa98fa49426ba17ed85"},{"id":"func/validateQueryLimit","name":"validateQueryLimit","line":404,"end_line":406,"hash":"e6e1f371a1cd8b544aa91303afaf79352b0d66eb80c70330d1696d7b7611773e"},{"id":"func/writeQueryJSON","name":"writeQueryJSON","line":408,"end_line":416,"hash":"9a39d3534d958093e9f736fe1ae62d4ca79c1027feed3f633d62260893b5d255"},{"id":"func/newConfigurationSchemaCommand","name":"newConfigurationSchemaCommand","line":418,"end_line":437,"hash":"6507a2e2fc057aae375987faf808bec449cf8584ca6d167730e681b204923509"},{"id":"func/parseAnalysisView","name":"parseAnalysisView","line":439,"end_line":450,"hash":"35702890cef0e4d5ba8e59c67e882c701b9ed1014e09c39ee4d58a408d2fbab1"},{"id":"func/parseFindingThreshold","name":"parseFindingThreshold","line":452,"end_line":463,"hash":"209b5941ad1354dd263c855583b6ae2292f0ec7baed73267db9a1e6aa2f2093e"},{"id":"func/writeAnalysisJSON","name":"writeAnalysisJSON","line":465,"end_line":492,"hash":"32e5cef3c9c85b37d857fd3378ccb40702adbc79bde3168c9b15055a9ebf6543"},{"id":"func/enforceFindingThreshold","name":"enforceFindingThreshold","line":494,"end_line":511,"hash":"546fa8a4b74f09b131b2db2abb19ed99d9d1d75a679052d8bc310d88bfcc1da9"}]}
// mutate4go-manifest-end
