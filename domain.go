package main

const graphSchemaVersion = 3

type componentKind string

const (
	componentKindApplication       componentKind = "application"
	componentKindApplicationModule componentKind = "application-module"
	componentKindSharedModule      componentKind = "shared-module"
	componentKindLibrary           componentKind = "library"
	componentKindInfrastructure    componentKind = "infrastructure"
	componentKindDevelopment       componentKind = "development"
)

// Graph is a stable representation of the repository's architectural dependencies.
type Graph struct {
	SchemaVersion int            `json:"schemaVersion"`
	Revision      string         `json:"revision"`
	ModulePath    string         `json:"modulePath"`
	Policy        AnalysisPolicy `json:"policy"`
	Summary       GraphSummary   `json:"summary"`
	Components    []Component    `json:"components"`
	Relationships []Relationship `json:"relationships"`
	Cycles        [][]string     `json:"cycles"`
	Diagnostics   []Diagnostic   `json:"diagnostics"`
	Findings      []Finding      `json:"findings"`
}

// AnalysisPolicy declares every formula and classification boundary used by the calculator.
type AnalysisPolicy struct {
	InstabilityFormula          string                 `json:"instabilityFormula"`
	IsolatedInstability         float64                `json:"isolatedInstability"`
	AbstractnessFormula         string                 `json:"abstractnessFormula"`
	UntypedAbstractness         float64                `json:"untypedAbstractness"`
	MainSequenceDistanceFormula string                 `json:"mainSequenceDistanceFormula"`
	ZoneOfPain                  ZoneOfPainPolicy       `json:"zoneOfPain"`
	StableDependency            StableDependencyPolicy `json:"stableDependency"`
}

// ZoneOfPainPolicy declares the inclusive stability and abstraction limits.
type ZoneOfPainPolicy struct {
	MinimumAfferentCoupling int     `json:"minimumAfferentCoupling"`
	MaximumInstability      float64 `json:"maximumInstability"`
	MaximumAbstractness     float64 `json:"maximumAbstractness"`
}

// StableDependencyPolicy declares the valid instability direction for production imports.
type StableDependencyPolicy struct {
	RequiredRelation string `json:"requiredRelation"`
	ProductionOnly   bool   `json:"productionOnly"`
}

// GraphSummary contains aggregate values shown before the detailed graph.
type GraphSummary struct {
	Components                 int `json:"components"`
	Relationships              int `json:"relationships"`
	ProductionRelationships    int `json:"productionRelationships"`
	TestOnlyRelationships      int `json:"testOnlyRelationships"`
	Applications               int `json:"applications"`
	ApplicationModules         int `json:"applicationModules"`
	SharedModules              int `json:"sharedModules"`
	Libraries                  int `json:"libraries"`
	Infrastructure             int `json:"infrastructure"`
	DevelopmentTools           int `json:"developmentTools"`
	Cycles                     int `json:"cycles"`
	Concerns                   int `json:"concerns"`
	StableDependencyViolations int `json:"stableDependencyViolations"`
	ZonesOfPain                int `json:"zonesOfPain"`
	Findings                   int `json:"findings"`
	Errors                     int `json:"errors"`
	Warnings                   int `json:"warnings"`
}

// Component describes one architectural unit and its coupling metrics.
type Component struct {
	Identifier                 string        `json:"id"`
	Name                       string        `json:"name"`
	Kind                       componentKind `json:"kind"`
	Application                string        `json:"application,omitempty"`
	Packages                   int           `json:"packages"`
	SourceFiles                int           `json:"sourceFiles"`
	ProductionFiles            int           `json:"productionFiles"`
	TestFiles                  int           `json:"testFiles"`
	DirectDependencies         int           `json:"directDependencies"`
	ProductionDependencies     int           `json:"productionDependencies"`
	TestOnlyDependencies       int           `json:"testOnlyDependencies"`
	DirectDependants           int           `json:"directDependants"`
	ProductionDependants       int           `json:"productionDependants"`
	TestOnlyDependants         int           `json:"testOnlyDependants"`
	TransitiveDependencies     int           `json:"transitiveDependencies"`
	TransitiveDependants       int           `json:"transitiveDependants"`
	ImporterPackages           int           `json:"importerPackages"`
	ProductionImporterPackages int           `json:"productionImporterPackages"`
	TestImporterPackages       int           `json:"testImporterPackages"`
	ApplicationReach           int           `json:"applicationReach"`
	Applications               []string      `json:"applications"`
	AfferentCoupling           int           `json:"afferentCoupling"`
	EfferentCoupling           int           `json:"efferentCoupling"`
	Instability                float64       `json:"instability"`
	AbstractTypes              int           `json:"abstractTypes"`
	ConcreteTypes              int           `json:"concreteTypes"`
	Abstractness               float64       `json:"abstractness"`
	MainSequenceDistance       float64       `json:"mainSequenceDistance"`
	InZoneOfPain               bool          `json:"inZoneOfPain"`
	InCycle                    bool          `json:"inCycle"`
}

// Relationship describes imports between two architectural units.
type Relationship struct {
	Source                    string   `json:"source"`
	Target                    string   `json:"target"`
	ProductionReferences      int      `json:"productionReferences"`
	TestReferences            int      `json:"testReferences"`
	SourcePackages            []string `json:"sourcePackages"`
	TargetPackages            []string `json:"targetPackages"`
	TestOnly                  bool     `json:"testOnly"`
	StableDependencyViolation bool     `json:"stableDependencyViolation"`
	Concerns                  []string `json:"concerns"`
}

// Diagnostic reports a source file that could not be fully inspected.
type Diagnostic struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type findingSeverity string

const (
	findingSeverityError   findingSeverity = "error"
	findingSeverityWarning findingSeverity = "warning"
)

// Finding is one deterministic architectural rule violation.
type Finding struct {
	Rule       string             `json:"rule"`
	Severity   findingSeverity    `json:"severity"`
	Subject    string             `json:"subject"`
	Message    string             `json:"message"`
	Source     string             `json:"source,omitempty"`
	Target     string             `json:"target,omitempty"`
	Components []string           `json:"components,omitempty"`
	Metrics    map[string]float64 `json:"metrics,omitempty"`
}
