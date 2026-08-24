package calculation

import "github.com/cgardev/goconduct/pkg/report"

const graphSchemaVersion = report.SchemaVersion

type componentRole = report.ComponentRole

const (
	componentRoleApplication       = report.ComponentRoleApplication
	componentRoleApplicationModule = report.ComponentRoleApplicationModule
	componentRoleSharedModule      = report.ComponentRoleSharedModule
	componentRoleLibrary           = report.ComponentRoleLibrary
	componentRoleInfrastructure    = report.ComponentRoleInfrastructure
	componentRoleDevelopment       = report.ComponentRoleDevelopment
)

// Graph contains one complete dependency analysis report.
type Graph = report.Graph

// AnalysisPolicy records the policy values used for one analysis.
type AnalysisPolicy = report.AnalysisPolicy

// StableLowAbstractionPolicy defines the stable concrete component thresholds.
type StableLowAbstractionPolicy = report.StableLowAbstractionPolicy

// StableDependencyPrinciplePolicy defines the dependency stability tolerance.
type StableDependencyPrinciplePolicy = report.StableDependencyPrinciplePolicy

// GraphSummary contains the principal counts for one graph.
type GraphSummary = report.GraphSummary

// Component contains one classified source component and its metrics.
type Component = report.Component

// Relationship contains one directed dependency between components.
type Relationship = report.Relationship

// ImportSite identifies one source location that imports a package.
type ImportSite = report.ImportSite

// Function contains one Go function and its coupling metrics.
type Function = report.Function

// FunctionCall contains one resolved call between Go functions.
type FunctionCall = report.FunctionCall

// CallSite identifies one source location that calls a function.
type CallSite = report.CallSite

// Diagnostic identifies one source analysis problem.
type Diagnostic = report.Diagnostic

// Finding identifies one architecture policy result.
type Finding = report.Finding

const (
	findingSeverityError   = report.FindingSeverityError
	findingSeverityWarning = report.FindingSeverityWarning
)
