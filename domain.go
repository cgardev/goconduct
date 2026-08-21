package main

const graphSchemaVersion = 6

type componentKind string

const (
	componentKindApplication       componentKind = "application"
	componentKindApplicationModule componentKind = "application-module"
	componentKindSharedModule      componentKind = "shared-module"
	componentKindLibrary           componentKind = "library"
	componentKindInfrastructure    componentKind = "infrastructure"
	componentKindDevelopment       componentKind = "development"
)

// Graph contains the dependency data for one repository analysis.
type Graph struct {
	SchemaVersion  int            `json:"schemaVersion"`
	Revision       string         `json:"revision"`
	ModulePath     string         `json:"modulePath"`
	Scope          AnalysisScope  `json:"scope"`
	Policy         AnalysisPolicy `json:"policy"`
	Summary        GraphSummary   `json:"summary"`
	Components     []Component    `json:"components"`
	Relationships  []Relationship `json:"relationships"`
	Functions      []Function     `json:"functions"`
	FunctionCalls  []FunctionCall `json:"functionCalls"`
	FunctionCycles [][]string     `json:"functionCycles"`
	Cycles         [][]string     `json:"cycles"`
	Diagnostics    []Diagnostic   `json:"diagnostics"`
	Findings       []Finding      `json:"findings"`
}

// AnalysisScope defines the repository paths and the component classification rules.
type AnalysisScope struct {
	Paths        []string       `json:"paths"`
	IgnoredPaths []string       `json:"ignoredPaths"`
	Components   ComponentRules `json:"components"`
}

// ComponentRules defines the path templates for each component kind.
type ComponentRules struct {
	Applications       []string `json:"applications"`
	ApplicationModules []string `json:"applicationModules"`
	SharedModules      []string `json:"sharedModules"`
	Libraries          []string `json:"libraries"`
	Infrastructure     []string `json:"infrastructure"`
	DevelopmentTools   []string `json:"developmentTools"`
}

// AnalysisPolicy defines each formula and numeric limit that the calculator uses.
type AnalysisPolicy struct {
	InstabilityFormula          string                          `json:"instabilityFormula"`
	FunctionInstabilityFormula  string                          `json:"functionInstabilityFormula"`
	FunctionCouplingDefinition  string                          `json:"functionCouplingDefinition"`
	FunctionCallResolution      string                          `json:"functionCallResolution"`
	IsolatedInstability         float64                         `json:"isolatedInstability"`
	AbstractnessFormula         string                          `json:"abstractnessFormula"`
	UntypedAbstractness         float64                         `json:"untypedAbstractness"`
	MainSequenceDistanceFormula string                          `json:"mainSequenceDistanceFormula"`
	StableLowAbstraction        StableLowAbstractionPolicy      `json:"stableLowAbstraction"`
	StableDependencyPrinciple   StableDependencyPrinciplePolicy `json:"stableDependencyPrinciple"`
}

// StableLowAbstractionPolicy defines the limits for a stable component with low abstraction.
type StableLowAbstractionPolicy struct {
	MinimumAfferentCoupling int     `json:"minimumAfferentCoupling"`
	MaximumInstability      float64 `json:"maximumInstability"`
	MaximumAbstractness     float64 `json:"maximumAbstractness"`
}

// StableDependencyPrinciplePolicy defines the permitted instability direction for production imports.
type StableDependencyPrinciplePolicy struct {
	RequiredRelation string `json:"requiredRelation"`
	ProductionOnly   bool   `json:"productionOnly"`
}

// GraphSummary contains the counts for one graph.
type GraphSummary struct {
	Components                          int `json:"components"`
	Relationships                       int `json:"relationships"`
	Functions                           int `json:"functions"`
	ProductionFunctions                 int `json:"productionFunctions"`
	TestFunctions                       int `json:"testFunctions"`
	FunctionRelationships               int `json:"functionRelationships"`
	FunctionCallSites                   int `json:"functionCallSites"`
	CrossComponentFunctionCallSites     int `json:"crossComponentFunctionCallSites"`
	FunctionCycles                      int `json:"functionCycles"`
	ProductionRelationships             int `json:"productionRelationships"`
	TestOnlyRelationships               int `json:"testOnlyRelationships"`
	Applications                        int `json:"applications"`
	ApplicationModules                  int `json:"applicationModules"`
	SharedModules                       int `json:"sharedModules"`
	Libraries                           int `json:"libraries"`
	Infrastructure                      int `json:"infrastructure"`
	DevelopmentTools                    int `json:"developmentTools"`
	Cycles                              int `json:"cycles"`
	RelationshipRuleViolations          int `json:"relationshipRuleViolations"`
	StableDependencyPrincipleViolations int `json:"stableDependencyPrincipleViolations"`
	StableLowAbstractionComponents      int `json:"stableLowAbstractionComponents"`
	Findings                            int `json:"findings"`
	Errors                              int `json:"errors"`
	Warnings                            int `json:"warnings"`
}

// Component describes one component and its coupling metrics.
type Component struct {
	Identifier                    string        `json:"id"`
	Name                          string        `json:"name"`
	Kind                          componentKind `json:"kind"`
	Application                   string        `json:"application,omitempty"`
	Packages                      int           `json:"packages"`
	SourceFiles                   int           `json:"sourceFiles"`
	ProductionFiles               int           `json:"productionFiles"`
	TestFiles                     int           `json:"testFiles"`
	DirectDependencies            int           `json:"directDependencies"`
	ProductionDependencies        int           `json:"productionDependencies"`
	TestOnlyDependencies          int           `json:"testOnlyDependencies"`
	DirectImportingComponents     int           `json:"directImportingComponents"`
	ProductionImportingComponents int           `json:"productionImportingComponents"`
	TestOnlyImportingComponents   int           `json:"testOnlyImportingComponents"`
	TransitiveDependencies        int           `json:"transitiveDependencies"`
	TransitiveImportingComponents int           `json:"transitiveImportingComponents"`
	ImporterPackages              int           `json:"importerPackages"`
	ProductionImporterPackages    int           `json:"productionImporterPackages"`
	TestImporterPackages          int           `json:"testImporterPackages"`
	UsingApplicationCount         int           `json:"usingApplicationCount"`
	UsingApplications             []string      `json:"usingApplications"`
	AfferentCoupling              int           `json:"afferentCoupling"`
	EfferentCoupling              int           `json:"efferentCoupling"`
	Instability                   float64       `json:"instability"`
	AbstractTypes                 int           `json:"abstractTypes"`
	ConcreteTypes                 int           `json:"concreteTypes"`
	Abstractness                  float64       `json:"abstractness"`
	MainSequenceDistance          float64       `json:"mainSequenceDistance"`
	IsStableWithLowAbstraction    bool          `json:"isStableWithLowAbstraction"`
	InCycle                       bool          `json:"inCycle"`
	ProductionFunctions           int           `json:"productionFunctions"`
	TestFunctions                 int           `json:"testFunctions"`
	ProductionIncomingCallSites   int           `json:"productionIncomingCallSites"`
	ProductionOutgoingCallSites   int           `json:"productionOutgoingCallSites"`
	TestIncomingCallSites         int           `json:"testIncomingCallSites"`
	TestOutgoingCallSites         int           `json:"testOutgoingCallSites"`
}

// Relationship describes imports between two components.
type Relationship struct {
	Source                            string       `json:"source"`
	Target                            string       `json:"target"`
	ProductionReferences              int          `json:"productionReferences"`
	TestReferences                    int          `json:"testReferences"`
	SourcePackages                    []string     `json:"sourcePackages"`
	TargetPackages                    []string     `json:"targetPackages"`
	ImportSites                       []ImportSite `json:"importSites"`
	ProductionFunctionCallSites       int          `json:"productionFunctionCallSites"`
	TestFunctionCallSites             int          `json:"testFunctionCallSites"`
	CallingFunctions                  int          `json:"callingFunctions"`
	CalledFunctions                   int          `json:"calledFunctions"`
	TestOnly                          bool         `json:"testOnly"`
	ViolatesStableDependencyPrinciple bool         `json:"violatesStableDependencyPrinciple"`
	RuleViolations                    []string     `json:"ruleViolations"`
}

// ImportSite identifies one internal package import declaration.
type ImportSite struct {
	SourcePackage string `json:"sourcePackage"`
	TargetPackage string `json:"targetPackage"`
	Path          string `json:"path"`
	Line          int    `json:"line"`
	Alias         string `json:"alias,omitempty"`
	Test          bool   `json:"test"`
}

// Function contains the static dependency metrics for one Go function or method.
type Function struct {
	Identifier                    string   `json:"id"`
	Name                          string   `json:"name"`
	Package                       string   `json:"package"`
	Component                     string   `json:"component"`
	Path                          string   `json:"path,omitempty"`
	Line                          int      `json:"line,omitempty"`
	Receiver                      string   `json:"receiver,omitempty"`
	Method                        bool     `json:"method"`
	Exported                      bool     `json:"exported"`
	Test                          bool     `json:"test"`
	Synthetic                     bool     `json:"synthetic"`
	InAnalysisScope               bool     `json:"inAnalysisScope"`
	AfferentCoupling              int      `json:"afferentCoupling"`
	EfferentCoupling              int      `json:"efferentCoupling"`
	TestAfferentCoupling          int      `json:"testAfferentCoupling"`
	TestEfferentCoupling          int      `json:"testEfferentCoupling"`
	IncomingCallSites             int      `json:"incomingCallSites"`
	OutgoingCallSites             int      `json:"outgoingCallSites"`
	TestIncomingCallSites         int      `json:"testIncomingCallSites"`
	TestOutgoingCallSites         int      `json:"testOutgoingCallSites"`
	CrossComponentCallerFunctions int      `json:"crossComponentCallerFunctions"`
	CrossComponentCalledFunctions int      `json:"crossComponentCalledFunctions"`
	TransitiveCallerFunctions     int      `json:"transitiveCallerFunctions"`
	TransitiveCalledFunctions     int      `json:"transitiveCalledFunctions"`
	UsingApplicationCount         int      `json:"usingApplicationCount"`
	UsingApplications             []string `json:"usingApplications"`
	InCycle                       bool     `json:"inCycle"`
	Instability                   float64  `json:"instability"`
}

// FunctionCall describes one resolved static call between two Go functions.
type FunctionCall struct {
	Source          string     `json:"source"`
	Target          string     `json:"target"`
	SourceComponent string     `json:"sourceComponent"`
	TargetComponent string     `json:"targetComponent"`
	CallSites       []CallSite `json:"callSites"`
	Calls           int        `json:"calls"`
	TestOnly        bool       `json:"testOnly"`
	CrossComponent  bool       `json:"crossComponent"`
}

// CallSite identifies the source location of one resolved static function call.
type CallSite struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// Diagnostic reports an error that the analyzer found in a source file.
type Diagnostic struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type findingSeverity string

const (
	findingSeverityError   findingSeverity = "error"
	findingSeverityWarning findingSeverity = "warning"
)

// Finding reports one deterministic architecture rule violation.
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
