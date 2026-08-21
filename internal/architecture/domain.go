// Package architecture contains the pure strategic dependency model.
// It has no transport, file system, command, or presentation dependency.
package architecture

// Role identifies the strategic responsibility that policies use.
type Role string

const (
	// RoleApplication identifies an application composition root.
	RoleApplication Role = "application"
	// RoleApplicationModule identifies a feature owned by one application.
	RoleApplicationModule Role = "application-module"
	// RoleSharedModule identifies a feature shared by applications.
	RoleSharedModule Role = "shared-module"
	// RoleLibrary identifies shared technical code.
	RoleLibrary Role = "library"
	// RoleInfrastructure identifies shared infrastructure code.
	RoleInfrastructure Role = "infrastructure"
	// RoleDevelopment identifies development-only code.
	RoleDevelopment Role = "development"
)

// ValidRole reports whether a role belongs to the closed strategic set.
func ValidRole(role Role) bool {
	switch role {
	case RoleApplication,
		RoleApplicationModule,
		RoleSharedModule,
		RoleLibrary,
		RoleInfrastructure,
		RoleDevelopment:
		return true
	default:
		return false
	}
}

// Severity identifies the effect of one architecture finding.
type Severity string

const (
	// SeverityError identifies a structural error.
	SeverityError Severity = "error"
	// SeverityWarning identifies an architecture risk.
	SeverityWarning Severity = "warning"
)

// Component contains the data that architecture rules use.
type Component struct {
	Identifier               string
	Role                     Role
	Application              string
	AfferentCoupling         int
	EfferentCoupling         int
	Instability              float64
	Abstractness             float64
	MainSequenceDistance     float64
	StableWithLowAbstraction bool
}

// Relationship identifies one dependency between two components.
type Relationship struct {
	Source   string
	Target   string
	TestOnly bool
}

// Diagnostic identifies one source analysis error.
type Diagnostic struct {
	Path    string
	Message string
}

// Graph contains the input for architecture rules.
type Graph struct {
	Components    []Component
	Relationships []Relationship
	Cycles        [][]string
	Diagnostics   []Diagnostic
}

// Finding contains one deterministic architecture rule result.
type Finding struct {
	Rule       string
	Severity   Severity
	Subject    string
	Message    string
	Source     string
	Target     string
	Components []string
	Metrics    map[string]float64
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T14:58:35Z","module_hash":"cb2971cbc40a8ce40682a50240c36bc2d0f92ec32774c6c9475931440a13e3da","functions":[{"id":"func/ValidRole","name":"ValidRole","line":24,"end_line":36,"hash":"f4fc79c953afeccb27343d77e3792e174a846f51053ddfea2f45063a75bfbeb8"}]}
// mutate4go-manifest-end
