package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/cgardev/gokeel/conf"
	"github.com/spf13/cobra"
)

var errArchitectureFindings = errors.New("architecture findings exceed the failure threshold")

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
}

func newRootCommand(logger *slog.Logger) *cobra.Command {
	defaults := DefaultApplicationConfiguration()
	options := &commandConfigurationOptions{}
	var addressOverride string
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
			if command.Flags().Changed("address") {
				configuration.Server.Address = addressOverride
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
				analyzer,
				configuration.Server.RefreshInterval,
				logger,
			)
			if err != nil {
				return err
			}
			return runDashboard(command.Context(), monitor, configuration.Server.Address, logger)
		},
	}
	command.Flags().StringVar(
		&addressOverride,
		"address",
		defaults.Server.Address,
		"Use this local dashboard address instead of the configured address.",
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
			graph, err := analyzeConfiguredGraph(command, options)
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

func analyzeConfiguredGraph(
	command *cobra.Command,
	options *commandConfigurationOptions,
) (Graph, error) {
	configuration, err := options.loadConfiguration(command)
	if err != nil {
		return Graph{}, err
	}
	sourceAnalyzer, err := newAnalyzer(configuration.Analysis)
	if err != nil {
		return Graph{}, err
	}
	return sourceAnalyzer.analyze()
}

func newSummaryCommand(options *commandConfigurationOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Write the configured scope, policy, and counts in JavaScript Object Notation (JSON).",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			graph, err := analyzeConfiguredGraph(command, options)
			if err != nil {
				return err
			}
			return writeQueryJSON(command.OutOrStdout(), querySummary(graph))
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
			severityFilter, err := parseFindingSeverityFilter(severity)
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
			findingQueryResult := queryFindings(graph, findingsQuery{
				severity:  severityFilter,
				rule:      rule,
				component: component,
				limit:     limit,
			})
			return writeQueryJSON(command.OutOrStdout(), findingQueryResult)
		},
	}
	command.Flags().StringVar(
		&severity,
		"severity",
		string(findingSeverityAllFilter),
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
	var kind string
	var sortOrder string
	var limit int
	command := &cobra.Command{
		Use:   "components",
		Short: "Write filtered and sorted components in JavaScript Object Notation (JSON).",
		Example: "  dependencygraph components --sort afferent --limit 10\n" +
			"  dependencygraph components --kind library --sort importers --limit 20",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			kindFilter, err := parseComponentKindFilter(kind)
			if err != nil {
				return err
			}
			selectedSort, err := parseComponentSort(sortOrder)
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
			componentQueryResult := queryComponents(graph, componentsQuery{
				kind:  kindFilter,
				sort:  selectedSort,
				limit: limit,
			})
			return writeQueryJSON(command.OutOrStdout(), componentQueryResult)
		},
	}
	command.Flags().StringVar(
		&kind,
		"kind",
		"all",
		"Select this component kind. Use all to select each kind.",
	)
	command.Flags().StringVar(
		&sortOrder,
		"sort",
		string(componentSortAfferent),
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
			graph, err := analyzeConfiguredGraph(command, options)
			if err != nil {
				return err
			}
			componentQueryResult, err := queryComponent(graph, arguments[0])
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
		errArchitectureFindings,
		failures,
		threshold,
	)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T10:49:44Z","module_hash":"bcef89c2abc4e53d267557f87b5b2545d718a155c35f688d92180ced8fb2b93f","functions":[{"id":"func/newRootCommand","name":"newRootCommand","line":49,"end_line":140,"hash":"0a3374868b6909ffe9f074a2f3678e10ba5750b71f49c96c0d511795c0167d49"},{"id":"func/commandConfigurationOptions.loadConfiguration","name":"commandConfigurationOptions.loadConfiguration","line":142,"end_line":160,"hash":"2796494987e6be9704583e4b8dc75885e61ce1fe6a56a27cc1068f0d9e26aa35"},{"id":"func/newAnalyzeCommand","name":"newAnalyzeCommand","line":162,"end_line":207,"hash":"ada712f5112c44f13ad437d3c43125220a51b0eb22802eda4079d9e6c912c19e"},{"id":"func/analyzeConfiguredGraph","name":"analyzeConfiguredGraph","line":209,"end_line":222,"hash":"ea7470ad64d5159e6ed063b3c0882df664d699fe00b3285aba442c65bd7c8362"},{"id":"func/newSummaryCommand","name":"newSummaryCommand","line":224,"end_line":237,"hash":"8a9c99f75eed6bf835e4ac30ed2366bd16272c9a9978c37f26813acb7b3f7403"},{"id":"func/newFindingsCommand","name":"newFindingsCommand","line":239,"end_line":287,"hash":"4f8f5270e4f7421ae34872927bc87620c57d2ddb12357b361ecaa9d8708a9a0b"},{"id":"func/newComponentsCommand","name":"newComponentsCommand","line":289,"end_line":343,"hash":"a17ef37cdc3d4a251b79621354c69052fc7d046b2b4d030a51e668c301d24f87"},{"id":"func/newComponentCommand","name":"newComponentCommand","line":345,"end_line":364,"hash":"96ea9420a45aedf7e9ade3b9d01dfa2ac19fefbad7e44f3421f02283a446d20a"},{"id":"func/validateQueryLimit","name":"validateQueryLimit","line":366,"end_line":371,"hash":"128deffab20785dcb9652565027ab85ad6baefb5ede20ba2baf393b2f8f0249c"},{"id":"func/writeQueryJSON","name":"writeQueryJSON","line":373,"end_line":381,"hash":"d47a0e9de8b39efbf32698714a9acc638d95981d4e13326fc140dce0f6e2b353"},{"id":"func/newConfigurationSchemaCommand","name":"newConfigurationSchemaCommand","line":383,"end_line":402,"hash":"28980aafb3906312da958b0d1f01ab0a0d1df7e5cc8aa7693d19cfa4782ca1c3"},{"id":"func/parseAnalysisView","name":"parseAnalysisView","line":404,"end_line":412,"hash":"2e911c0bcec32108ac0be9988ef64e2daf7e3aebff83847c1116747c0ef4b6f8"},{"id":"func/parseFindingThreshold","name":"parseFindingThreshold","line":414,"end_line":422,"hash":"92e8a1de6869b3365cfabdd1bde963ec1db5877f7f070beceb78fabd2719786d"},{"id":"func/writeAnalysisJSON","name":"writeAnalysisJSON","line":424,"end_line":451,"hash":"da7545861aca744e566b083bd56a90ea969f2de68e57204b8ff83526eede8ebc"},{"id":"func/enforceFindingThreshold","name":"enforceFindingThreshold","line":453,"end_line":472,"hash":"86064b18642dc0d8ce3f55626dc0ab18660bd5d2e93a462a96cacb165e5c4c51"}]}
// mutate4go-manifest-end
