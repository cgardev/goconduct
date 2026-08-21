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

var errArchitectureFindings = errors.New("architecture findings meet the failure threshold")

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
		Short:         "Analyze and visualize strategic Go dependencies",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			configuration, err := options.load(command)
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
		"local dashboard address, overriding the configuration document",
	)
	command.Flags().DurationVar(
		&refreshIntervalOverride,
		"refresh-interval",
		defaults.Server.RefreshInterval,
		"source refresh interval, overriding the configuration document",
	)
	command.PersistentFlags().StringVar(
		&options.configurationPath,
		"configuration",
		defaultConfigurationPath,
		"path to the optional JSON configuration document",
	)
	command.PersistentFlags().StringVar(
		&options.repositoryRoot,
		"root",
		defaults.Analysis.RepositoryRoot,
		"repository root, overriding the configuration document",
	)
	command.PersistentFlags().StringArrayVar(
		&options.analysisPaths,
		"analysis-path",
		nil,
		"repository-relative analysis path; repeat to replace configured paths",
	)
	command.PersistentFlags().StringArrayVar(
		&options.ignoredPaths,
		"ignore-path",
		nil,
		"ignored path pattern; repeat to replace configured exclusions",
	)
	command.AddCommand(
		newAnalyzeCommand(options),
		newSummaryCommand(options),
		newFindingsCommand(options),
		newComponentsCommand(options),
		newComponentCommand(options),
		newConfigurationSchemaCommand(),
	)
	return command
}

func (options *commandConfigurationOptions) load(command *cobra.Command) (ApplicationConfiguration, error) {
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
	var view string
	var failOn string
	var pretty bool
	command := &cobra.Command{
		Use:   "analyze",
		Short: "Emit a deterministic architectural analysis as JSON",
		Example: "  dependencygraph analyze --configuration configuration.json\n" +
			"  dependencygraph analyze --root . --analysis-path cmd --analysis-path internal\n" +
			"  dependencygraph analyze --root . --view graph --pretty\n" +
			"  dependencygraph analyze --root . --fail-on error",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			selectedView, err := parseAnalysisView(view)
			if err != nil {
				return err
			}
			threshold, err := parseFindingThreshold(failOn)
			if err != nil {
				return err
			}
			graph, err := analyzeConfiguredGraph(command, options)
			if err != nil {
				return err
			}
			if err := writeAnalysisJSON(command.OutOrStdout(), graph, selectedView, pretty); err != nil {
				return err
			}
			return enforceFindingThreshold(graph.Findings, threshold)
		},
	}
	command.Flags().StringVar(&view, "view", string(analysisViewReport), "JSON view: report or graph")
	command.Flags().StringVar(
		&failOn,
		"fail-on",
		string(findingThresholdNone),
		"return a failure for findings at this severity: none, warning, or error",
	)
	command.Flags().BoolVar(&pretty, "pretty", false, "indent JSON output for human inspection")
	return command
}

func analyzeConfiguredGraph(
	command *cobra.Command,
	options *commandConfigurationOptions,
) (Graph, error) {
	configuration, err := options.load(command)
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
		Short: "Emit the configured scope, policy, and aggregate metrics as JSON",
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
		Short: "Emit filtered architectural findings as JSON",
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
			result := queryFindings(graph, findingsQuery{
				severity:  severityFilter,
				rule:      rule,
				component: component,
				limit:     limit,
			})
			return writeQueryJSON(command.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(
		&severity,
		"severity",
		string(findingSeverityAllFilter),
		"finding severity: all, warning, or error",
	)
	command.Flags().StringVar(&rule, "rule", "", "exact finding rule identifier")
	command.Flags().StringVar(&component, "component", "", "exact related component identifier")
	command.Flags().IntVar(&limit, "limit", 0, "maximum findings returned; zero returns all")
	return command
}

func newComponentsCommand(options *commandConfigurationOptions) *cobra.Command {
	var kind string
	var sortOrder string
	var limit int
	command := &cobra.Command{
		Use:   "components",
		Short: "Emit filtered and ranked architectural components as JSON",
		Example: "  dependencygraph components --sort afferent --limit 10\n" +
			"  dependencygraph components --kind library --sort dependants --limit 20",
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
			result := queryComponents(graph, componentsQuery{
				kind:  kindFilter,
				sort:  selectedSort,
				limit: limit,
			})
			return writeQueryJSON(command.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(&kind, "kind", "all", "component kind or all")
	command.Flags().StringVar(
		&sortOrder,
		"sort",
		string(componentSortAfferent),
		"ranking: identifier, afferent, efferent, dependants, dependencies, instability, abstractness, distance, or files",
	)
	command.Flags().IntVar(&limit, "limit", 20, "maximum components returned; zero returns all")
	return command
}

func newComponentCommand(options *commandConfigurationOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "component <identifier>",
		Short:   "Emit one component with dependencies, dependants, and findings as JSON",
		Example: "  dependencygraph component internal/library/foundationdomain",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			graph, err := analyzeConfiguredGraph(command, options)
			if err != nil {
				return err
			}
			result, err := queryComponent(graph, args[0])
			if err != nil {
				return err
			}
			return writeQueryJSON(command.OutOrStdout(), result)
		},
	}
}

func validateQueryLimit(limit int) error {
	if limit < 0 {
		return fmt.Errorf("query limit must not be negative")
	}
	return nil
}

func writeQueryJSON(output io.Writer, payload any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("encode query result: %w", err)
	}
	return nil
}

func newConfigurationSchemaCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "configuration-schema",
		Short: "Print the JSON Schema of the configuration document",
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
	pretty bool,
) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	var payload any = analysisReport{
		SchemaVersion: graph.SchemaVersion,
		Revision:      graph.Revision,
		ModulePath:    graph.ModulePath,
		Scope:         graph.Scope,
		Policy:        graph.Policy,
		Summary:       graph.Summary,
		Findings:      graph.Findings,
	}
	if view == analysisViewGraph {
		payload = graph
	}
	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("encode architectural analysis: %w", err)
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
// {"version":1,"tested_at":"2026-08-21T08:01:10Z","module_hash":"6533aa92e1c08fd976a38cea72701b8c704270d89372bd3a90ec71cae0e54273","functions":[{"id":"func/newRootCommand","name":"newRootCommand","line":49,"end_line":137,"hash":"a178ab3d5b4d3b3a0ce41fda352fc3efc0577d5b22ae4736f5d1e4073da38c4e"},{"id":"func/commandConfigurationOptions.load","name":"commandConfigurationOptions.load","line":139,"end_line":155,"hash":"b2b9ce48e6baedb3b5b0d47bf9dada80032b19acc7c56b02d4ca90288a36b5c5"},{"id":"func/newAnalyzeCommand","name":"newAnalyzeCommand","line":157,"end_line":197,"hash":"6d6debaac553e2f19056449e41c6e6e8637ed2c4dd16382f87e166bd70ea0029"},{"id":"func/analyzeConfiguredGraph","name":"analyzeConfiguredGraph","line":199,"end_line":212,"hash":"7ebf67bca0d8907d779d099fa2c584e547847d1caf8a817ce175edf8f41c7811"},{"id":"func/newSummaryCommand","name":"newSummaryCommand","line":214,"end_line":227,"hash":"5b98b0d570bf5da1ed01dd9fe3242cc0c95d260f15d8771bf9b1730fac7a82c5"},{"id":"func/newFindingsCommand","name":"newFindingsCommand","line":229,"end_line":272,"hash":"d5730ceae01df82947bbda241ce11dab771fc887fe6d824c760628289781aeed"},{"id":"func/newComponentsCommand","name":"newComponentsCommand","line":274,"end_line":317,"hash":"cefbd522df307465eb68eec56905f26ddb67e5a4fbef3f1896adae4de1210c7b"},{"id":"func/newComponentCommand","name":"newComponentCommand","line":319,"end_line":337,"hash":"817edb93cf5ba5902dabd855ece8cea384e2a40117460a3470e1962958bc6af3"},{"id":"func/validateQueryLimit","name":"validateQueryLimit","line":339,"end_line":344,"hash":"128deffab20785dcb9652565027ab85ad6baefb5ede20ba2baf393b2f8f0249c"},{"id":"func/writeQueryJSON","name":"writeQueryJSON","line":346,"end_line":354,"hash":"1af2f35a1e4667e2d9ec88d962d527e05757e148e989af9f5411c7b81edd619f"},{"id":"func/newConfigurationSchemaCommand","name":"newConfigurationSchemaCommand","line":356,"end_line":375,"hash":"8995331c5b7d501117c06e1145bca6530860151bc924124ac4ce2f4655ac47c9"},{"id":"func/parseAnalysisView","name":"parseAnalysisView","line":377,"end_line":385,"hash":"2e911c0bcec32108ac0be9988ef64e2daf7e3aebff83847c1116747c0ef4b6f8"},{"id":"func/parseFindingThreshold","name":"parseFindingThreshold","line":387,"end_line":395,"hash":"92e8a1de6869b3365cfabdd1bde963ec1db5877f7f070beceb78fabd2719786d"},{"id":"func/writeAnalysisJSON","name":"writeAnalysisJSON","line":397,"end_line":424,"hash":"ac47518ffcaa0936ca658aa041b0911d8483711e1b57aa79cdc26436aea0f485"},{"id":"func/enforceFindingThreshold","name":"enforceFindingThreshold","line":426,"end_line":445,"hash":"86064b18642dc0d8ce3f55626dc0ab18660bd5d2e93a462a96cacb165e5c4c51"}]}
// mutate4go-manifest-end
