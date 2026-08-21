package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultAddress = "127.0.0.1:6062"
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
	Policy        AnalysisPolicy `json:"policy"`
	Summary       GraphSummary   `json:"summary"`
	Findings      []Finding      `json:"findings"`
}

func defaultRefreshInterval() time.Duration {
	return 750 * time.Millisecond
}

func minimumRefreshInterval() time.Duration {
	return 100 * time.Millisecond
}

func newRootCommand(logger *slog.Logger) *cobra.Command {
	var address string
	var repositoryRoot string
	var refreshInterval time.Duration
	command := &cobra.Command{
		Use:           "dependencygraph",
		Short:         "Analyze and visualize strategic Go dependencies",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if refreshInterval < minimumRefreshInterval() {
				return fmt.Errorf(
					"refresh interval must be at least %s",
					minimumRefreshInterval(),
				)
			}
			analyzer, err := newAnalyzer(repositoryRoot)
			if err != nil {
				return err
			}
			monitor, err := newGraphMonitor(analyzer, refreshInterval, logger)
			if err != nil {
				return err
			}
			return runDashboard(command.Context(), monitor, address, logger)
		},
	}
	command.Flags().StringVar(
		&address,
		"address",
		defaultAddress,
		"local address used by the dashboard server",
	)
	command.Flags().StringVar(
		&repositoryRoot,
		"root",
		".",
		"repository root containing go.mod",
	)
	command.Flags().DurationVar(
		&refreshInterval,
		"refresh-interval",
		defaultRefreshInterval(),
		"interval used to detect source changes",
	)
	command.AddCommand(newAnalyzeCommand())
	return command
}

func newAnalyzeCommand() *cobra.Command {
	var repositoryRoot string
	var view string
	var failOn string
	var pretty bool
	command := &cobra.Command{
		Use:   "analyze",
		Short: "Emit a deterministic architectural analysis as JSON",
		Example: "  dependencygraph analyze --root .\n" +
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
			sourceAnalyzer, err := newAnalyzer(repositoryRoot)
			if err != nil {
				return err
			}
			graph, err := sourceAnalyzer.analyze()
			if err != nil {
				return err
			}
			if err := writeAnalysisJSON(command.OutOrStdout(), graph, selectedView, pretty); err != nil {
				return err
			}
			return enforceFindingThreshold(graph.Findings, threshold)
		},
	}
	command.Flags().StringVar(&repositoryRoot, "root", ".", "repository root containing go.mod")
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
