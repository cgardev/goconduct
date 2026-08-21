package main

import (
	"fmt"
	"time"

	"github.com/cgardev/gokeel/conf"
)

const (
	defaultConfigurationPath = "configuration.json"
	defaultAddress           = "127.0.0.1:6062"
)

func defaultRefreshInterval() time.Duration {
	return 750 * time.Millisecond
}

func minimumRefreshInterval() time.Duration {
	return 100 * time.Millisecond
}

// ApplicationConfiguration is the external configuration document of the tool.
type ApplicationConfiguration struct {
	Server   ServerConfiguration   `json:"server,omitzero"`
	Analysis AnalysisConfiguration `json:"analysis,omitzero"`
}

// ServerConfiguration controls the optional human dashboard transport.
type ServerConfiguration struct {
	Address         string        `json:"address,omitzero"`
	RefreshInterval time.Duration `json:"refreshInterval,omitzero"`
}

// AnalysisConfiguration controls repository discovery and strategic grouping.
type AnalysisConfiguration struct {
	RepositoryRoot string                      `json:"repositoryRoot,omitzero"`
	Paths          []string                    `json:"paths,omitzero"`
	IgnoredPaths   []string                    `json:"ignoredPaths,omitzero"`
	Components     ComponentRulesConfiguration `json:"components,omitzero"`
}

// ComponentRulesConfiguration maps path templates to strategic roles.
type ComponentRulesConfiguration struct {
	Applications       []string `json:"applications,omitzero"`
	ApplicationModules []string `json:"applicationModules,omitzero"`
	SharedModules      []string `json:"sharedModules,omitzero"`
	Libraries          []string `json:"libraries,omitzero"`
	Infrastructure     []string `json:"infrastructure,omitzero"`
	DevelopmentTools   []string `json:"developmentTools,omitzero"`
}

// DefaultApplicationConfiguration returns the complete standalone configuration.
func DefaultApplicationConfiguration() ApplicationConfiguration {
	return ApplicationConfiguration{
		Server: ServerConfiguration{
			Address:         defaultAddress,
			RefreshInterval: defaultRefreshInterval(),
		},
		Analysis: AnalysisConfiguration{
			RepositoryRoot: ".",
			Paths:          []string{"."},
			IgnoredPaths:   defaultIgnoredPaths(),
			Components:     defaultComponentRulesConfiguration(),
		},
	}
}

func defaultIgnoredPaths() []string {
	return []string{".*", "_resources", "node_modules", "target", "testdata", "vendor"}
}

func defaultComponentRulesConfiguration() ComponentRulesConfiguration {
	return ComponentRulesConfiguration{
		Applications:       []string{"cmd/{application}"},
		ApplicationModules: []string{"cmd/{application}/internal/module/{component}"},
		SharedModules:      []string{"internal/module/{component}"},
		Libraries:          []string{"internal/library/{component}"},
		Infrastructure:     []string{"internal/{component}"},
		DevelopmentTools:   []string{"internal/devtool/{component}"},
	}
}

func (configuration ComponentRulesConfiguration) domainRules() ComponentRules {
	return ComponentRules{
		Applications:       configuration.Applications,
		ApplicationModules: configuration.ApplicationModules,
		SharedModules:      configuration.SharedModules,
		Libraries:          configuration.Libraries,
		Infrastructure:     configuration.Infrastructure,
		DevelopmentTools:   configuration.DevelopmentTools,
	}
}

// loadApplicationConfiguration overlays an optional JSON document on the defaults.
func loadApplicationConfiguration(path string) (ApplicationConfiguration, error) {
	configuration := DefaultApplicationConfiguration()
	if err := conf.NewLoader(conf.WithOptionalFile(path)).Load(&configuration); err != nil {
		return ApplicationConfiguration{}, fmt.Errorf("load application configuration: %w", err)
	}
	return configuration, nil
}

// applicationSchemaDefinition documents the external configuration contract.
func applicationSchemaDefinition() conf.SchemaDefinition {
	return conf.SchemaDefinition{
		Description: "Externalized configuration of the deterministic Go dependency graph tool.",
		Fields: map[string]conf.FieldDefinition{
			"server": {
				Description: "Human dashboard transport parameters.",
			},
			"server.address": {
				Description: "Local TCP address of the dashboard listener, in host:port form.",
				Default:     defaultAddress,
			},
			"server.refreshInterval": {
				Description: "Interval used to detect changes in the selected Go source paths, in Go duration notation.",
				Default:     defaultRefreshInterval(),
			},
			"analysis": {
				Description: "Repository scope, exclusions, and strategic component layout.",
			},
			"analysis.repositoryRoot": {
				Description: "Directory containing go.mod. Relative paths resolve from the process working directory.",
				Default:     ".",
			},
			"analysis.paths": {
				Description: "Repository-relative directories or Go files inspected as source evidence.",
				Default:     []string{"."},
				Examples:    []any{"cmd", "internal/module", "internal/library/example"},
			},
			"analysis.ignoredPaths": {
				Description: "Repository-relative path patterns excluded from source discovery and dependency targets. A pattern without a slash matches any path segment; a pattern with a slash matches from the repository root.",
				Default:     defaultIgnoredPaths(),
				Examples:    []any{"vendor", "generated", "internal/library/legacy"},
			},
			"analysis.components": {
				Description: "Path templates that map repository packages to strategic roles. {component} and {application} each capture one path segment.",
			},
			"analysis.components.applications": {
				Description: "Application-root templates. Each template must contain {application}.",
				Default:     defaultComponentRulesConfiguration().Applications,
				Examples:    []any{"cmd/{application}", "services/{application}"},
			},
			"analysis.components.applicationModules": {
				Description: "Application-owned module templates. Each template must contain {application} and {component}.",
				Default:     defaultComponentRulesConfiguration().ApplicationModules,
				Examples:    []any{"cmd/{application}/internal/module/{component}"},
			},
			"analysis.components.sharedModules": {
				Description: "Shared feature-module templates. Each template must contain {component}.",
				Default:     defaultComponentRulesConfiguration().SharedModules,
				Examples:    []any{"internal/module/{component}"},
			},
			"analysis.components.libraries": {
				Description: "Shared library templates. Each template must contain {component}.",
				Default:     defaultComponentRulesConfiguration().Libraries,
				Examples:    []any{"internal/library/{component}", "packages/{component}"},
			},
			"analysis.components.infrastructure": {
				Description: "Shared infrastructure templates. Each template must contain {component}.",
				Default:     defaultComponentRulesConfiguration().Infrastructure,
				Examples:    []any{"internal/{component}"},
			},
			"analysis.components.developmentTools": {
				Description: "Development-only tool templates. Each template must contain {component}.",
				Default:     defaultComponentRulesConfiguration().DevelopmentTools,
				Examples:    []any{"internal/devtool/{component}", "tools/{component}"},
			},
		},
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T07:59:30Z","module_hash":"82c7bbdf95657b1a20d933ab8e91696217d807b40f6a9adb76ed9b4ba2122b9b","functions":[{"id":"func/defaultRefreshInterval","name":"defaultRefreshInterval","line":15,"end_line":17,"hash":"64a1f07d35ff197bf67644d4889d3f31cffa2bf3c507009de7f97c2d522b31d3"},{"id":"func/minimumRefreshInterval","name":"minimumRefreshInterval","line":19,"end_line":21,"hash":"4f2930e0810d642ebd7a786753630b3715ac62d0ddcd29ec034323084de9eebc"},{"id":"func/DefaultApplicationConfiguration","name":"DefaultApplicationConfiguration","line":54,"end_line":67,"hash":"8959b0b14ca668f368b4e3dbd9b38de6edca2700f5e9b6f19d68ba8f88a4e1fe"},{"id":"func/defaultIgnoredPaths","name":"defaultIgnoredPaths","line":69,"end_line":71,"hash":"eb1ccd39c301970fc4e6f866b369fe060c767075b0ff2b595423519b907ff623"},{"id":"func/defaultComponentRulesConfiguration","name":"defaultComponentRulesConfiguration","line":73,"end_line":82,"hash":"b7fd6194dcdff30bddf674ccf3c96c87a2a7d4c1759b8ab4d3171ab949021627"},{"id":"func/ComponentRulesConfiguration.domainRules","name":"ComponentRulesConfiguration.domainRules","line":84,"end_line":93,"hash":"dd7ed848eb5d763670ffcb4221253d3c5aa28e06d6804ba769ef4846fcb3fb4a"},{"id":"func/loadApplicationConfiguration","name":"loadApplicationConfiguration","line":96,"end_line":102,"hash":"bdb0cf615949fb4ecd8b9d4a19ec822822007dc50c3c97a02b4cc67ac5e1c9e3"},{"id":"func/applicationSchemaDefinition","name":"applicationSchemaDefinition","line":105,"end_line":172,"hash":"671a4f48452af29f9ab3f56a38d5d4ed2b541fb77aa6c0eb839707bb6544783d"}]}
// mutate4go-manifest-end
