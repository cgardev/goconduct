package main

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/cgardev/gokeel/conf"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	applicationconfiguration "github.com/cgardev/goconduct/cmd/goconduct/internal/configuration"
	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
)

type checkSummary struct {
	Plugins  int `json:"plugins"`
	Metrics  int `json:"metrics"`
	Findings int `json:"findings"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

type checkReport struct {
	SchemaVersion int             `json:"schemaVersion"`
	Summary       checkSummary    `json:"summary"`
	Reports       []plugin.Report `json:"reports"`
}

func registerApplicationCommands(app *application, root *cobra.Command) {
	injector := app.host.Injector()
	root.AddCommand(
		newCheckCommand(injector),
		newPluginsCommand(injector),
		newApplicationConfigurationSchemaCommand(),
	)
	configureApplicationServer(app, root)
}

func newCheckCommand(injector do.Injector) *cobra.Command {
	var repositoryRoot string
	var selectedPlugins []string
	var selectedPaths []string
	var failOn string
	var indent bool
	command := &cobra.Command{
		Use:   "check",
		Short: "Run configured plugins and write one deterministic report.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			configuration, err := do.Invoke[applicationconfiguration.ApplicationConfiguration](injector)
			if err != nil {
				return err
			}
			check := configuration.CloneCheck()
			if command.Flags().Changed("plugin") {
				check.Plugins = slices.Clone(selectedPlugins)
			}
			if command.Flags().Changed("path") {
				check.Paths = slices.Clone(selectedPaths)
			}
			if command.Flags().Changed("fail-on") {
				check.FailOn = applicationconfiguration.FailureThreshold(failOn)
			}
			if err := validateCheck(check); err != nil {
				return err
			}
			root := configuration.Analysis.RepositoryRoot
			if command.Flags().Changed("repository") {
				root = repositoryRoot
			}
			catalog, err := do.Invoke[*plugin.Catalog](injector)
			if err != nil {
				return err
			}
			reports, err := catalog.Evaluate(command.Context(), check.Plugins, plugin.Request{
				RepositoryRoot: root,
				Paths:          check.Paths,
			})
			if err != nil {
				return err
			}
			report := combineReports(reports)
			if err := writeCheckReport(command.OutOrStdout(), report, indent); err != nil {
				return err
			}
			return enforceCheckThreshold(report, check.FailOn)
		},
	}
	command.Flags().StringVar(&repositoryRoot, "repository", ".", "Override the configured repository root.")
	command.Flags().StringArrayVar(&selectedPlugins, "plugin", nil, "Run this plugin. Repeat this option as needed.")
	command.Flags().StringArrayVar(&selectedPaths, "path", nil, "Select this repository path. Repeat this option as needed.")
	command.Flags().StringVar(&failOn, "fail-on", "", "Fail on none, warning, or error findings.")
	command.Flags().BoolVar(&indent, "indent", false, "Indent the JSON report.")
	return command
}

func newPluginsCommand(injector do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:   "plugins",
		Short: "List registered evaluator plugins.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			catalog, err := do.Invoke[*plugin.Catalog](injector)
			if err != nil {
				return err
			}
			return writeJSON(command.OutOrStdout(), map[string]any{
				"schemaVersion": 1,
				"plugins":       catalog.Names(),
			}, true)
		},
	}
}

func newApplicationConfigurationSchemaCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "configuration-schema",
		Short: "Write the complete configuration schema as JSON.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			schema, err := conf.GenerateSchema(
				applicationconfiguration.ApplicationConfiguration{},
				applicationconfiguration.SchemaDefinition(),
			)
			if err != nil {
				return failure.Internal("generate application configuration schema", err)
			}
			if _, err := command.OutOrStdout().Write(schema); err != nil {
				return failure.Unavailable("write application configuration schema", err)
			}
			return nil
		},
	}
}

func validateCheck(configuration applicationconfiguration.CheckConfiguration) error {
	application := applicationconfiguration.Default()
	application.Quality.Check = configuration
	return applicationconfiguration.Validate(application)
}

func combineReports(reports []plugin.Report) checkReport {
	combined := checkReport{SchemaVersion: 1, Reports: slices.Clone(reports)}
	combined.Summary.Plugins = len(reports)
	for _, report := range reports {
		combined.Summary.Metrics += len(report.Metrics)
		combined.Summary.Findings += len(report.Findings)
		for _, finding := range report.Findings {
			switch finding.Severity {
			case plugin.SeverityError:
				combined.Summary.Errors++
			case plugin.SeverityWarning:
				combined.Summary.Warnings++
			}
		}
	}
	return combined
}

func writeCheckReport(destination io.Writer, report checkReport, indent bool) error {
	return writeJSON(destination, report, indent)
}

func writeJSON(destination io.Writer, value any, indent bool) error {
	encoder := json.NewEncoder(destination)
	encoder.SetEscapeHTML(false)
	if indent {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(value); err != nil {
		return failure.Unavailable("write JSON report", err)
	}
	return nil
}

func enforceCheckThreshold(
	report checkReport,
	threshold applicationconfiguration.FailureThreshold,
) error {
	if threshold == applicationconfiguration.FailureThresholdNone {
		return nil
	}
	for _, pluginReport := range report.Reports {
		for _, finding := range pluginReport.Findings {
			fails := finding.Severity == plugin.SeverityError
			if threshold == applicationconfiguration.FailureThresholdWarning {
				fails = fails || finding.Severity == plugin.SeverityWarning
			}
			if fails {
				return failure.BusinessRule(fmt.Sprintf(
					"quality check failed on %s finding %q",
					strings.ToLower(string(finding.Severity)),
					finding.ID,
				), nil)
			}
		}
	}
	return nil
}
