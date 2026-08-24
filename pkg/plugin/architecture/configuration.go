package architecture

import (
	"slices"
	"time"

	"github.com/cgardev/gokeel/conf"

	"github.com/cgardev/goconduct/internal/application"
	"github.com/cgardev/goconduct/pkg/failure"
)

const (
	defaultConfigurationPath = ".goconduct.json"
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
	Server       ServerConfiguration       `json:"server,omitzero"`
	Analysis     AnalysisConfiguration     `json:"analysis,omitzero"`
	Cache        CacheConfiguration        `json:"cache,omitzero"`
	Architecture ArchitectureConfiguration `json:"architecture,omitzero"`
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
	Role       ComponentRole `json:"role,omitzero"`
	Paths      []string      `json:"paths,omitzero"`
}

// ArchitectureConfiguration defines configurable dependency boundaries.
type ArchitectureConfiguration struct {
	Dependencies DependencyPolicyConfiguration `json:"dependencies,omitzero"`
}

// DependencyDefault defines the fallback decision for unmatched dependencies.
type DependencyDefault string

const (
	// DependencyDefaultAllow accepts unmatched dependencies.
	DependencyDefaultAllow DependencyDefault = "allow"
	// DependencyDefaultDeny rejects unmatched dependencies.
	DependencyDefaultDeny DependencyDefault = "deny"
)

// DependencyPolicyConfiguration defines default decisions and explicit rules.
type DependencyPolicyConfiguration struct {
	ProductionDefault DependencyDefault             `json:"productionDefault,omitzero"`
	TestDefault       DependencyDefault             `json:"testDefault,omitzero"`
	Allow             []DependencyRuleConfiguration `json:"allow,omitzero"`
	Deny              []DependencyRuleConfiguration `json:"deny,omitzero"`
}

// DependencyRuleConfiguration selects one directed component relationship.
type DependencyRuleConfiguration struct {
	Identifier      string                         `json:"id,omitzero"`
	From            ComponentSelectorConfiguration `json:"from,omitzero"`
	To              ComponentSelectorConfiguration `json:"to,omitzero"`
	SameApplication bool                           `json:"sameApplication,omitzero"`
	Reason          string                         `json:"reason,omitzero"`
}

// ComponentSelectorConfiguration selects components by stable attributes.
type ComponentSelectorConfiguration struct {
	Identifiers  []string        `json:"ids,omitzero"`
	Roles        []ComponentRole `json:"roles,omitzero"`
	Categories   []string        `json:"categories,omitzero"`
	Applications []string        `json:"applications,omitzero"`
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
		Architecture: ArchitectureConfiguration{Dependencies: DependencyPolicyConfiguration{
			ProductionDefault: DependencyDefaultAllow,
			TestDefault:       DependencyDefaultAllow,
		}},
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
		Libraries: []string{
			"pkg/plugin",
			"pkg/plugin/{component}",
			"pkg/{component}",
			"internal/library/{component}",
		},
		Infrastructure:   []string{"internal/{component}"},
		DevelopmentTools: []string{"internal/devtool/{component}"},
	}
}

func (configuration ComponentRulesConfiguration) domainRules() ComponentRules {
	taxonomy := make([]ComponentCategoryRule, 0, len(configuration.Taxonomy))
	for _, category := range configuration.Taxonomy {
		taxonomy = append(taxonomy, ComponentCategoryRule{
			Identifier: category.Identifier,
			Role:       category.Role,
			Paths:      slices.Clone(category.Paths),
		})
	}
	return ComponentRules{
		Applications:       slices.Clone(configuration.Applications),
		ApplicationModules: slices.Clone(configuration.ApplicationModules),
		SharedModules:      slices.Clone(configuration.SharedModules),
		Libraries:          slices.Clone(configuration.Libraries),
		Infrastructure:     slices.Clone(configuration.Infrastructure),
		DevelopmentTools:   slices.Clone(configuration.DevelopmentTools),
		Taxonomy:           taxonomy,
	}
}

func loadApplicationConfiguration(configurationPath string) (ApplicationConfiguration, error) {
	configuration := DefaultApplicationConfiguration()
	if err := conf.NewLoader(conf.WithOptionalFile(configurationPath)).Load(&configuration); err != nil {
		return ApplicationConfiguration{}, failure.Validation("load application configuration", err)
	}
	if err := ValidateApplicationConfiguration(configuration); err != nil {
		return ApplicationConfiguration{}, err
	}
	return configuration, nil
}

// ValidateApplicationConfiguration checks architecture and cache settings.
func ValidateApplicationConfiguration(configuration ApplicationConfiguration) error {
	if err := validateCacheConfiguration(configuration.Cache); err != nil {
		return err
	}
	return validateDependencyPolicy(configuration.Architecture.Dependencies)
}

func validateCacheConfiguration(configuration CacheConfiguration) error {
	if err := application.ValidateCacheMode(configuration.Mode); err != nil {
		return err
	}
	if configuration.RequestTimeout <= 0 {
		return failure.Validation("cache request timeout must be greater than zero", nil)
	}
	return nil
}

func applicationSchemaDefinition() conf.SchemaDefinition {
	return conf.SchemaDefinition{
		Description: "External configuration for deterministic Go dependency analysis.",
		Fields: map[string]conf.FieldDefinition{
			"architecture": {
				Description: "Deterministic dependency boundaries for classified components.",
			},
			"architecture.dependencies": {
				Description: "Default dependency decisions and explicit allow or deny rules.",
			},
			"architecture.dependencies.productionDefault": {
				Description: "Decision for unmatched production dependencies.",
				Default:     string(DependencyDefaultAllow),
				Enum:        []any{string(DependencyDefaultAllow), string(DependencyDefaultDeny)},
			},
			"architecture.dependencies.testDefault": {
				Description: "Decision for unmatched test-only dependencies.",
				Default:     string(DependencyDefaultAllow),
				Enum:        []any{string(DependencyDefaultAllow), string(DependencyDefaultDeny)},
			},
			"architecture.dependencies.allow": {
				Description: "Explicit dependency grants. Matching grants form a union.",
				Default:     []DependencyRuleConfiguration{},
			},
			"architecture.dependencies.deny": {
				Description: "Mandatory dependency prohibitions. A prohibition overrides every grant.",
				Default:     []DependencyRuleConfiguration{},
			},
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
					"The {component} and {application} placeholders each match one path segment. " +
					"A literal template classifies one exact package.",
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
				Description: "Templates for shared libraries. A template can select one exact package " +
					"or contain {component}.",
				Default:  defaultComponentRulesConfiguration().Libraries,
				Examples: []any{"plugin", "internal/library/{component}", "packages/{component}"},
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

// SchemaDefinition returns the architecture configuration metadata.
func SchemaDefinition() conf.SchemaDefinition {
	return applicationSchemaDefinition()
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"51c30d80db32a358830c59877432bdf8c25664bfe25d06a504184a2ee804b3f2","functions":[{"id":"func/defaultRefreshInterval","name":"defaultRefreshInterval","line":30,"end_line":32,"hash":"64a1f07d35ff197bf67644d4889d3f31cffa2bf3c507009de7f97c2d522b31d3"},{"id":"func/minimumRefreshInterval","name":"minimumRefreshInterval","line":34,"end_line":36,"hash":"4f2930e0810d642ebd7a786753630b3715ac62d0ddcd29ec034323084de9eebc"},{"id":"func/DefaultApplicationConfiguration","name":"DefaultApplicationConfiguration","line":84,"end_line":101,"hash":"6ddb537e3c885e9a636a56dd09cf5209b49c329a88e239f1968604add127c1d8"},{"id":"func/defaultIgnoredPaths","name":"defaultIgnoredPaths","line":103,"end_line":105,"hash":"eb1ccd39c301970fc4e6f866b369fe060c767075b0ff2b595423519b907ff623"},{"id":"func/defaultComponentRulesConfiguration","name":"defaultComponentRulesConfiguration","line":107,"end_line":116,"hash":"b7fd6194dcdff30bddf674ccf3c96c87a2a7d4c1759b8ab4d3171ab949021627"},{"id":"func/ComponentRulesConfiguration.domainRules","name":"ComponentRulesConfiguration.domainRules","line":118,"end_line":136,"hash":"5e39ed32ce08bcb3a6120199d9fd61157808ba3eb494041d161ebad3d0fb2472"},{"id":"func/loadApplicationConfiguration","name":"loadApplicationConfiguration","line":138,"end_line":144,"hash":"6d941d6b5b58e93f0c454bc4db4254de4dfe82595ae6978ab3b9d137380a8e9f"},{"id":"func/validateCacheConfiguration","name":"validateCacheConfiguration","line":146,"end_line":154,"hash":"3d6511a5387d4ff613428f436ecf6a5e3700827045dd0d7209a4189cdc8f5bec"},{"id":"func/applicationSchemaDefinition","name":"applicationSchemaDefinition","line":156,"end_line":256,"hash":"16af5eb2b6d63c4a0276ebb2661e9353fa50e0b395bf0126d888341187b53ebb"}]}
// mutate4go-manifest-end
