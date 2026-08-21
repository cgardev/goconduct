package main

import (
	"fmt"
	"time"

	application "digginginsights.com/v3/internal/devtool/dependencygraph/internal/application"
	"github.com/cgardev/gokeel/conf"
)

const (
	defaultConfigurationPath = "configuration.json"
	defaultAddress           = "127.0.0.1:6062"
	defaultCacheTimeout      = 2 * time.Second
)

// CacheMode selects the source of a graph for CLI queries.
type CacheMode = application.CacheMode

const (
	// CacheModeAuto uses a compatible server cache and calculates the graph when no cache is available.
	CacheModeAuto = application.CacheModeAuto
	// CacheModeServer requires a compatible server cache.
	CacheModeServer = application.CacheModeServer
	// CacheModeLocal always calculates the graph in the CLI process.
	CacheModeLocal = application.CacheModeLocal
)

func defaultRefreshInterval() time.Duration {
	return 750 * time.Millisecond
}

func minimumRefreshInterval() time.Duration {
	return 100 * time.Millisecond
}

// ApplicationConfiguration defines the external configuration for the tool.
type ApplicationConfiguration struct {
	Server   ServerConfiguration   `json:"server,omitzero"`
	Analysis AnalysisConfiguration `json:"analysis,omitzero"`
	Cache    CacheConfiguration    `json:"cache,omitzero"`
}

// ServerConfiguration defines the dashboard server.
type ServerConfiguration struct {
	Address         string        `json:"address,omitzero"`
	RefreshInterval time.Duration `json:"refreshInterval,omitzero"`
}

// CacheConfiguration defines how CLI queries load a graph.
type CacheConfiguration struct {
	Mode           CacheMode     `json:"mode,omitzero"`
	RequestTimeout time.Duration `json:"requestTimeout,omitzero"`
}

// AnalysisConfiguration defines which files the tool analyzes and how it classifies components.
type AnalysisConfiguration struct {
	RepositoryRoot string                      `json:"repositoryRoot,omitzero"`
	Paths          []string                    `json:"paths,omitzero"`
	IgnoredPaths   []string                    `json:"ignoredPaths,omitzero"`
	Components     ComponentRulesConfiguration `json:"components,omitzero"`
}

// ComponentRulesConfiguration maps path templates to component roles.
type ComponentRulesConfiguration struct {
	Applications       []string                         `json:"applications,omitzero"`
	ApplicationModules []string                         `json:"applicationModules,omitzero"`
	SharedModules      []string                         `json:"sharedModules,omitzero"`
	Libraries          []string                         `json:"libraries,omitzero"`
	Infrastructure     []string                         `json:"infrastructure,omitzero"`
	DevelopmentTools   []string                         `json:"developmentTools,omitzero"`
	Taxonomy           []ComponentCategoryConfiguration `json:"taxonomy,omitzero"`
}

// ComponentCategoryConfiguration maps custom path templates to one strategic role.
type ComponentCategoryConfiguration struct {
	Identifier string        `json:"id,omitzero"`
	Role       componentRole `json:"role,omitzero"`
	Paths      []string      `json:"paths,omitzero"`
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
		Cache: CacheConfiguration{
			Mode:           CacheModeAuto,
			RequestTimeout: defaultCacheTimeout,
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
	taxonomy := make([]ComponentCategoryRule, 0, len(configuration.Taxonomy))
	for _, category := range configuration.Taxonomy {
		taxonomy = append(taxonomy, ComponentCategoryRule{
			Identifier: category.Identifier,
			Role:       category.Role,
			Paths:      category.Paths,
		})
	}
	return ComponentRules{
		Applications:       configuration.Applications,
		ApplicationModules: configuration.ApplicationModules,
		SharedModules:      configuration.SharedModules,
		Libraries:          configuration.Libraries,
		Infrastructure:     configuration.Infrastructure,
		DevelopmentTools:   configuration.DevelopmentTools,
		Taxonomy:           taxonomy,
	}
}

func loadApplicationConfiguration(configurationPath string) (ApplicationConfiguration, error) {
	configuration := DefaultApplicationConfiguration()
	if err := conf.NewLoader(conf.WithOptionalFile(configurationPath)).Load(&configuration); err != nil {
		return ApplicationConfiguration{}, fmt.Errorf("load application configuration: %w", err)
	}
	return configuration, nil
}

func validateCacheConfiguration(configuration CacheConfiguration) error {
	if err := application.ValidateCacheMode(configuration.Mode); err != nil {
		return err
	}
	if configuration.RequestTimeout <= 0 {
		return fmt.Errorf("cache request timeout must be greater than zero")
	}
	return nil
}

func applicationSchemaDefinition() conf.SchemaDefinition {
	return conf.SchemaDefinition{
		Description: "External configuration for deterministic Go dependency analysis.",
		Fields: map[string]conf.FieldDefinition{
			"cache": {
				Description: "Parameters for CLI access to the active graph cache.",
			},
			"cache.mode": {
				Description: "Graph source for CLI queries. Auto uses the server cache. " +
					"Auto uses local analysis when the cache is unavailable.",
				Default: string(CacheModeAuto),
				Enum: []any{
					string(CacheModeAuto),
					string(CacheModeServer),
					string(CacheModeLocal),
				},
			},
			"cache.requestTimeout": {
				Description: "Maximum time for one request to the active graph cache, in Go duration notation.",
				Default:     defaultCacheTimeout,
			},
			"server": {
				Description: "Parameters for the dashboard server.",
			},
			"server.address": {
				Description: "Local network address for the dashboard server, in host:port form.",
				Default:     defaultAddress,
			},
			"server.refreshInterval": {
				Description: "Time between checks for changes in the selected Go files, in Go duration notation.",
				Default:     defaultRefreshInterval(),
			},
			"analysis": {
				Description: "Repository paths, exclusion patterns, and component classification rules.",
			},
			"analysis.repositoryRoot": {
				Description: "Directory containing go.mod. Relative paths resolve from the process working directory.",
				Default:     ".",
			},
			"analysis.paths": {
				Description: "Repository-relative directories or Go files that the analyzer reads.",
				Default:     []string{"."},
				Examples:    []any{"cmd", "internal/module", "internal/library/example"},
			},
			"analysis.ignoredPaths": {
				Description: "Repository-relative patterns that exclude files. " +
					"A pattern with no slash matches one path segment. " +
					"A pattern with a slash matches from the repository root.",
				Default:  defaultIgnoredPaths(),
				Examples: []any{"vendor", "generated", "internal/library/legacy"},
			},
			"analysis.components": {
				Description: "Path templates that classify repository packages. " +
					"The {component} and {application} placeholders each match one path segment.",
			},
			"analysis.components.applications": {
				Description: "Templates for application root paths. Each template must contain {application}.",
				Default:     defaultComponentRulesConfiguration().Applications,
				Examples:    []any{"cmd/{application}", "services/{application}"},
			},
			"analysis.components.applicationModules": {
				Description: "Templates for modules in one application. " +
					"Each template must contain {application} and {component}.",
				Default:  defaultComponentRulesConfiguration().ApplicationModules,
				Examples: []any{"cmd/{application}/internal/module/{component}"},
			},
			"analysis.components.sharedModules": {
				Description: "Templates for shared modules. Each template must contain {component}.",
				Default:     defaultComponentRulesConfiguration().SharedModules,
				Examples:    []any{"internal/module/{component}"},
			},
			"analysis.components.libraries": {
				Description: "Templates for shared libraries. Each template must contain {component}.",
				Default:     defaultComponentRulesConfiguration().Libraries,
				Examples:    []any{"internal/library/{component}", "packages/{component}"},
			},
			"analysis.components.infrastructure": {
				Description: "Templates for shared infrastructure. Each template must contain {component}.",
				Default:     defaultComponentRulesConfiguration().Infrastructure,
				Examples:    []any{"internal/{component}"},
			},
			"analysis.components.developmentTools": {
				Description: "Templates for development tools. Each template must contain {component}.",
				Default:     defaultComponentRulesConfiguration().DevelopmentTools,
				Examples:    []any{"internal/devtool/{component}", "tools/{component}"},
			},
			"analysis.components.taxonomy": {
				Description: "Custom component categories. Each category has an id, " +
					"a closed strategic role, and path templates.",
				Default: []ComponentCategoryConfiguration{},
				Examples: []any{
					ComponentCategoryConfiguration{
						Identifier: "plugin",
						Role:       componentRoleLibrary,
						Paths:      []string{"plugins/{component}"},
					},
				},
			},
		},
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T17:15:20Z","module_hash":"6f8f43f5ada2748b01eebfd40ad5e5f2896cc22ea7a4391189912517bdd1f423","functions":[{"id":"func/defaultRefreshInterval","name":"defaultRefreshInterval","line":29,"end_line":31,"hash":"64a1f07d35ff197bf67644d4889d3f31cffa2bf3c507009de7f97c2d522b31d3"},{"id":"func/minimumRefreshInterval","name":"minimumRefreshInterval","line":33,"end_line":35,"hash":"4f2930e0810d642ebd7a786753630b3715ac62d0ddcd29ec034323084de9eebc"},{"id":"func/DefaultApplicationConfiguration","name":"DefaultApplicationConfiguration","line":83,"end_line":100,"hash":"6ddb537e3c885e9a636a56dd09cf5209b49c329a88e239f1968604add127c1d8"},{"id":"func/defaultIgnoredPaths","name":"defaultIgnoredPaths","line":102,"end_line":104,"hash":"eb1ccd39c301970fc4e6f866b369fe060c767075b0ff2b595423519b907ff623"},{"id":"func/defaultComponentRulesConfiguration","name":"defaultComponentRulesConfiguration","line":106,"end_line":115,"hash":"b7fd6194dcdff30bddf674ccf3c96c87a2a7d4c1759b8ab4d3171ab949021627"},{"id":"func/ComponentRulesConfiguration.domainRules","name":"ComponentRulesConfiguration.domainRules","line":117,"end_line":135,"hash":"6f2efe3f0d63b275ad303dcfc2ae3c9c02195966c3577fbff5e71ccd61d51502"},{"id":"func/loadApplicationConfiguration","name":"loadApplicationConfiguration","line":137,"end_line":143,"hash":"1db003044865f83678db2220b8f87b9f55fa552aed7693838ec80caf6ad2dd2e"},{"id":"func/validateCacheConfiguration","name":"validateCacheConfiguration","line":145,"end_line":153,"hash":"e83f0aa82c4388f3f49d35f0d59f8438a6ca75f2637b9e6388eb79abaf6b9e0f"},{"id":"func/applicationSchemaDefinition","name":"applicationSchemaDefinition","line":155,"end_line":255,"hash":"16af5eb2b6d63c4a0276ebb2661e9353fa50e0b395bf0126d888341187b53ebb"}]}
// mutate4go-manifest-end
