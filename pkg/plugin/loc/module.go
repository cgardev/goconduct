// Package loc measures Go source lines and declared functions.
// It exposes a direct evaluator and a lifecycle adapter for goconduct hosts.
package loc

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
)

// Module contains the LOC dependency registrations.
var Module = do.Package(newEvaluatorInjector())

type locPlugin struct{}

var _ plugin.Plugin = locPlugin{}

// Plugin returns the LOC lifecycle adapter.
func Plugin() plugin.Plugin { return locPlugin{} }

func (locPlugin) Name() string { return "loc" }

func (locPlugin) Services() func(do.Injector) { return Module }

func (locPlugin) Activate(_ context.Context, injector do.Injector) error {
	catalog, err := do.Invoke[*plugin.Catalog](injector)
	if err != nil {
		return err
	}
	evaluator, err := do.Invoke[*Evaluator](injector)
	if err != nil {
		return err
	}
	return catalog.Register(evaluator)
}

func (locPlugin) RegisterCommands(injector do.Injector, root *cobra.Command) error {
	configuration, err := do.Invoke[Configuration](injector)
	if err != nil {
		configuration = DefaultConfiguration()
	}
	root.AddCommand(newLOCCommand(configuration))
	return nil
}

func (locPlugin) RegisterEndpoints(
	do.Injector,
	plugin.EndpointRegistrar,
	...connect.HandlerOption,
) error {
	return nil
}

func newEvaluatorInjector() func(do.Injector) {
	return do.Lazy[*Evaluator](func(injector do.Injector) (*Evaluator, error) {
		configuration, err := do.Invoke[Configuration](injector)
		if err != nil {
			configuration = DefaultConfiguration()
		}
		return NewEvaluator(configuration)
	})
}

func newLOCCommand(baseConfiguration Configuration) *cobra.Command {
	var repositoryRoot string
	var paths []string
	var includes []string
	var excludes []string
	var generatedPaths []string
	var generatedHeaders []string
	var forceHandwritten []string
	var standardMarker bool
	var indent bool
	command := &cobra.Command{
		Use:   "loc",
		Short: "Measure Go source lines and declared functions.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			report, err := loadLOCReport(command, baseConfiguration, locCommandOptions{
				repositoryRoot: repositoryRoot,
				paths:          paths, includes: includes, excludes: excludes,
				generatedPaths: generatedPaths, generatedHeaders: generatedHeaders,
				forceHandwritten: forceHandwritten, standardMarker: standardMarker,
			})
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(command.OutOrStdout())
			encoder.SetEscapeHTML(false)
			if indent {
				encoder.SetIndent("", "  ")
			}
			if err := encoder.Encode(report); err != nil {
				return failure.Unavailable("write LOC report", err)
			}
			if failing := plugin.FailingFindings(report.Findings); failing != 0 {
				return failure.BusinessRule(fmt.Sprintf(
					"LOC analysis has %d policy findings",
					failing,
				), nil)
			}
			return nil
		},
	}
	loadReport := func(queryCommand *cobra.Command) (plugin.Report, error) {
		return loadLOCReport(queryCommand, baseConfiguration, locCommandOptions{
			repositoryRoot: repositoryRoot,
			paths:          paths, includes: includes, excludes: excludes,
			generatedPaths: generatedPaths, generatedHeaders: generatedHeaders,
			forceHandwritten: forceHandwritten, standardMarker: standardMarker,
		})
	}
	defaults := DefaultConfiguration()
	command.PersistentFlags().StringVar(&repositoryRoot, "repository", ".", "Select the Go repository root.")
	command.PersistentFlags().StringArrayVar(
		&paths,
		"path",
		nil,
		"Select a source root. Repeat this option as needed.",
	)
	command.PersistentFlags().StringArrayVar(
		&includes,
		"include",
		nil,
		"Include a path pattern. Repeat this option as needed.",
	)
	command.PersistentFlags().StringArrayVar(
		&excludes,
		"exclude",
		nil,
		"Exclude a path pattern. Repeat this option as needed.",
	)
	command.PersistentFlags().StringArrayVar(
		&generatedPaths,
		"generated-path",
		nil,
		"Classify a path pattern as generated. Repeat this option as needed.",
	)
	command.PersistentFlags().StringArrayVar(
		&generatedHeaders,
		"generated-header",
		nil,
		"Classify an RE2 header pattern as generated. Repeat this option as needed.",
	)
	command.PersistentFlags().StringArrayVar(
		&forceHandwritten,
		"force-handwritten",
		nil,
		"Classify a path pattern as handwritten. Repeat this option as needed.",
	)
	command.PersistentFlags().BoolVar(
		&standardMarker,
		"standard-generated-marker",
		defaults.Generated.StandardMarker,
		"Recognize the standard Go generated marker.",
	)
	command.Flags().BoolVar(&indent, "indent", false, "Indent the JSON report.")
	command.AddCommand(
		newLOCSummaryCommand(loadReport),
		newLOCPackagesCommand(loadReport),
		newLOCFilesCommand(loadReport),
		newLOCFunctionsCommand(loadReport),
	)
	return command
}

type locCommandOptions struct {
	repositoryRoot   string
	paths            []string
	includes         []string
	excludes         []string
	generatedPaths   []string
	generatedHeaders []string
	forceHandwritten []string
	standardMarker   bool
}

func loadLOCReport(
	command *cobra.Command,
	baseConfiguration Configuration,
	options locCommandOptions,
) (plugin.Report, error) {
	configuration := cloneConfiguration(baseConfiguration)
	if command.Flags().Changed("path") {
		configuration.Selection.Paths = slices.Clone(options.paths)
	}
	if command.Flags().Changed("include") {
		configuration.Selection.Include = slices.Clone(options.includes)
	}
	if command.Flags().Changed("exclude") {
		configuration.Selection.Exclude = slices.Clone(options.excludes)
	}
	if command.Flags().Changed("generated-path") {
		configuration.Generated.PathPatterns = slices.Clone(options.generatedPaths)
	}
	if command.Flags().Changed("generated-header") {
		configuration.Generated.HeaderPatterns = slices.Clone(options.generatedHeaders)
	}
	if command.Flags().Changed("force-handwritten") {
		configuration.Generated.ForceHandwrittenPaths = slices.Clone(options.forceHandwritten)
	}
	configuration.Generated.StandardMarker = options.standardMarker
	evaluator, err := NewEvaluator(configuration)
	if err != nil {
		return plugin.Report{}, err
	}
	return evaluator.Evaluate(command.Context(), plugin.Request{
		RepositoryRoot: options.repositoryRoot,
	})
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-23T21:34:41Z","module_hash":"ecb794c4703ab10f54f09f17d35808a55db22bbedf39acc228d8b16d88716fb5","functions":[{"id":"func/Plugin","name":"Plugin","line":27,"end_line":27,"hash":"6983a4be479d5600ea8edca947899ba144f7023b579e9aa7bdffbbbf91cefb3d"},{"id":"func/locPlugin.Name","name":"locPlugin.Name","line":29,"end_line":29,"hash":"046c999a7be63243234e3b8f8d82b98ea4705f7be289cbe48f599c9017a35292"},{"id":"func/locPlugin.Services","name":"locPlugin.Services","line":31,"end_line":31,"hash":"4c5a5651a95403ef019ab2d7199913c3025a4e1129012c82dec97e347fef79dd"},{"id":"func/locPlugin.Activate","name":"locPlugin.Activate","line":33,"end_line":43,"hash":"98a952127d88805a0771f24275f5bacb60bda09fe1b29c836f1ad2f7e1afb213"},{"id":"func/locPlugin.RegisterCommands","name":"locPlugin.RegisterCommands","line":45,"end_line":52,"hash":"dfaf71773e8989794415973fdc773050d90f5731da77fd2820c2511c0dc4d034"},{"id":"func/locPlugin.RegisterEndpoints","name":"locPlugin.RegisterEndpoints","line":54,"end_line":60,"hash":"6f249188a5294b1a4c4d0232550a65670c11eb640ace4e1c215534b76092a008"},{"id":"func/newEvaluatorInjector","name":"newEvaluatorInjector","line":62,"end_line":70,"hash":"c28319d521d51efecc23e10b7cc14c57078edfe81b0ddee8240be9f5bdff064c"},{"id":"func/newLOCCommand","name":"newLOCCommand","line":72,"end_line":173,"hash":"d7d42bc04dd310f91cbb70f955459242f5d5046cac755800ebc6aa95fdf96a38"},{"id":"func/loadLOCReport","name":"loadLOCReport","line":186,"end_line":218,"hash":"26f988610afd8cb7f0b5afa5d849da0ff081173960dc85c69b05f8481de0ce4e"}]}
// mutate4go-manifest-end
