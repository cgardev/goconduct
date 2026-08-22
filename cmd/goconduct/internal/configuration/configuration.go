// Package configuration owns the goconduct application configuration.
package configuration

import (
	"fmt"
	"maps"
	"slices"

	"github.com/cgardev/gokeel/conf"

	"github.com/cgardev/goconduct/failure"
	"github.com/cgardev/goconduct/plugin/architecture"
	"github.com/cgardev/goconduct/plugin/coverage"
	"github.com/cgardev/goconduct/plugin/crap"
	"github.com/cgardev/goconduct/plugin/duplication"
	"github.com/cgardev/goconduct/plugin/mutation"
)

// FailureThreshold selects which finding severity fails a combined check.
type FailureThreshold string

const (
	// FailureThresholdNone never fails after successful evaluation.
	FailureThresholdNone FailureThreshold = "none"
	// FailureThresholdWarning fails on warnings and errors.
	FailureThresholdWarning FailureThreshold = "warning"
	// FailureThresholdError fails only on errors.
	FailureThresholdError FailureThreshold = "error"
)

// CheckConfiguration defines the default combined verification request.
type CheckConfiguration struct {
	Plugins []string         `json:"plugins"`
	Paths   []string         `json:"paths,omitempty"`
	FailOn  FailureThreshold `json:"failOn"`
}

// QualityConfiguration configures built-in quality plugins.
type QualityConfiguration struct {
	Check       CheckConfiguration        `json:"check"`
	Coverage    coverage.Configuration    `json:"coverage"`
	CRAP        crap.Configuration        `json:"crap"`
	Duplication duplication.Configuration `json:"duplication"`
	Mutation    mutation.Configuration    `json:"mutation"`
}

// ApplicationConfiguration contains architecture and quality settings.
type ApplicationConfiguration struct {
	Server       architecture.ServerConfiguration       `json:"server,omitzero"`
	Analysis     architecture.AnalysisConfiguration     `json:"analysis,omitzero"`
	Cache        architecture.CacheConfiguration        `json:"cache,omitzero"`
	Architecture architecture.ArchitectureConfiguration `json:"architecture,omitzero"`
	Quality      QualityConfiguration                   `json:"quality,omitzero"`
}

// Default returns the complete standalone application configuration.
func Default() ApplicationConfiguration {
	architectureDefaults := architecture.DefaultApplicationConfiguration()
	return ApplicationConfiguration{
		Server:       architectureDefaults.Server,
		Analysis:     architectureDefaults.Analysis,
		Cache:        architectureDefaults.Cache,
		Architecture: architectureDefaults.Architecture,
		Quality: QualityConfiguration{
			Check: CheckConfiguration{
				Plugins: []string{"architecture", "coverage"},
				FailOn:  FailureThresholdError,
			},
			Coverage:    coverage.DefaultConfiguration(),
			CRAP:        crap.DefaultConfiguration(),
			Duplication: duplication.DefaultConfiguration(),
			Mutation:    mutation.DefaultConfiguration(),
		},
	}
}

// Load reads one optional JSON document over the defaults.
func Load(path string) (ApplicationConfiguration, error) {
	configuration := Default()
	if err := conf.NewLoader(conf.WithOptionalFile(path)).Load(&configuration); err != nil {
		return ApplicationConfiguration{}, failure.Validation("load application configuration", err)
	}
	if err := Validate(configuration); err != nil {
		return ApplicationConfiguration{}, err
	}
	return configuration, nil
}

// Validate checks cross-plugin application settings.
func Validate(configuration ApplicationConfiguration) error {
	if err := architecture.ValidateApplicationConfiguration(configuration.ArchitecturePluginConfiguration()); err != nil {
		return err
	}
	if len(configuration.Quality.Check.Plugins) == 0 {
		return failure.Validation("quality check plugin list is empty", nil)
	}
	knownPlugins := map[string]struct{}{
		"architecture": {}, "coverage": {}, "crap": {}, "duplication": {}, "mutation": {},
	}
	seen := make(map[string]struct{}, len(configuration.Quality.Check.Plugins))
	for _, name := range configuration.Quality.Check.Plugins {
		if _, known := knownPlugins[name]; !known {
			return failure.Validation(fmt.Sprintf("quality check plugin %q is unknown", name), nil)
		}
		if _, duplicate := seen[name]; duplicate {
			return failure.Validation(fmt.Sprintf("quality check plugin %q is duplicated", name), nil)
		}
		seen[name] = struct{}{}
	}
	switch configuration.Quality.Check.FailOn {
	case FailureThresholdNone, FailureThresholdWarning, FailureThresholdError:
		return nil
	default:
		return failure.Validation(fmt.Sprintf(
			"quality failure threshold %q is invalid",
			configuration.Quality.Check.FailOn,
		), nil)
	}
}

// ArchitecturePluginConfiguration returns the architecture plugin settings.
func (configuration ApplicationConfiguration) ArchitecturePluginConfiguration() architecture.ApplicationConfiguration {
	return architecture.ApplicationConfiguration{
		Server: configuration.Server, Analysis: configuration.Analysis, Cache: configuration.Cache,
		Architecture: configuration.Architecture,
	}
}

// SchemaDefinition returns metadata for the complete application document.
func SchemaDefinition() conf.SchemaDefinition {
	architectureDefinition := architecture.SchemaDefinition()
	fields := maps.Clone(architectureDefinition.Fields)
	fields["quality"] = conf.FieldDefinition{
		Description: "Deterministic checks and external quality tool settings.",
	}
	fields["quality.check"] = conf.FieldDefinition{
		Description: "Defaults for the combined check command.",
	}
	fields["quality.check.plugins"] = conf.FieldDefinition{
		Description: "Plugins executed by the combined check command.",
		Default:     []string{"architecture", "coverage"},
		Enum:        []any{"architecture", "coverage", "crap", "duplication", "mutation"},
	}
	fields["quality.check.paths"] = conf.FieldDefinition{
		Description: "Optional repository-relative paths supplied to selected plugins.",
		Examples:    []any{"cmd", "internal", "plugin"},
	}
	fields["quality.check.failOn"] = conf.FieldDefinition{
		Description: "Finding severity that fails the combined check.",
		Default:     string(FailureThresholdError),
		Enum:        []any{string(FailureThresholdNone), string(FailureThresholdWarning), string(FailureThresholdError)},
	}
	fields["quality.coverage"] = conf.FieldDefinition{Description: "Go statement coverage settings."}
	fields["quality.coverage.command"] = conf.FieldDefinition{Description: "Go executable used for coverage.", Default: "go"}
	fields["quality.coverage.packages"] = conf.FieldDefinition{Description: "Go package patterns passed to go test.", Default: []string{"./..."}}
	fields["quality.coverage.pathPolicies"] = conf.FieldDefinition{Description: "Path-specific coverage thresholds."}
	fields["quality.crap"] = conf.FieldDefinition{Description: "Coverage run and CRAP score limits."}
	fields["quality.crap.command"] = conf.FieldDefinition{Description: "Go executable used for coverage.", Default: "go"}
	fields["quality.crap.packages"] = conf.FieldDefinition{Description: "Go package patterns passed to go test.", Default: []string{"./..."}}
	fields["quality.crap.maximumScore"] = conf.FieldDefinition{Description: "Maximum accepted CRAP score.", Default: 8.0, Minimum: conf.Pointer(0.0)}
	fields["quality.crap.pathPolicies"] = conf.FieldDefinition{Description: "Path-specific CRAP score thresholds."}
	fields["quality.duplication"] = conf.FieldDefinition{Description: "Structural duplicate limits."}
	fields["quality.duplication.similarity"] = conf.FieldDefinition{Description: "Minimum reported structural similarity.", Default: 0.82, Minimum: conf.Pointer(0.0), Maximum: conf.Pointer(1.0)}
	fields["quality.duplication.minimumLines"] = conf.FieldDefinition{Description: "Minimum source lines per duplicate candidate.", Default: 4, Minimum: conf.Pointer(1.0)}
	fields["quality.duplication.minimumNodes"] = conf.FieldDefinition{Description: "Minimum syntax nodes per duplicate candidate.", Default: 20, Minimum: conf.Pointer(1.0)}
	fields["quality.duplication.maximumCandidates"] = conf.FieldDefinition{Description: "Maximum accepted duplicate candidates.", Default: 0, Minimum: conf.Pointer(0.0)}
	fields["quality.mutation"] = conf.FieldDefinition{Description: "Coverage run and mutation limits."}
	fields["quality.mutation.command"] = conf.FieldDefinition{Description: "Go executable used for coverage.", Default: "go"}
	fields["quality.mutation.packages"] = conf.FieldDefinition{Description: "Go package patterns passed to go test.", Default: []string{"./..."}}
	fields["quality.mutation.paths"] = conf.FieldDefinition{Description: "Production Go paths selected for mutation."}
	fields["quality.mutation.execute"] = conf.FieldDefinition{Description: "Run every covered mutation instead of only reporting the sites.", Default: false}
	fields["quality.mutation.timeoutFactor"] = conf.FieldDefinition{Description: "Mutation timeout multiplier.", Default: 10, Minimum: conf.Pointer(1.0)}
	fields["quality.mutation.maximumSurvivors"] = conf.FieldDefinition{Description: "Maximum accepted surviving mutations.", Default: 0, Minimum: conf.Pointer(0.0)}
	fields["quality.mutation.maximumUncovered"] = conf.FieldDefinition{Description: "Maximum accepted uncovered mutation sites.", Default: 0, Minimum: conf.Pointer(0.0)}
	return conf.SchemaDefinition{
		Description: "External configuration for deterministic Go quality checks.",
		Fields:      fields,
	}
}

// CloneCheck returns a defensive copy for command execution.
func (configuration ApplicationConfiguration) CloneCheck() CheckConfiguration {
	return CheckConfiguration{
		Plugins: slices.Clone(configuration.Quality.Check.Plugins),
		Paths:   slices.Clone(configuration.Quality.Check.Paths),
		FailOn:  configuration.Quality.Check.FailOn,
	}
}
