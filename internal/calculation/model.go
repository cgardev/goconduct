package calculation

import (
	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/architecture"
	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/report"
)

const graphSchemaVersion = report.SchemaVersion

type componentRole = report.ComponentRole

const (
	componentRoleApplication       = architecture.RoleApplication
	componentRoleApplicationModule = architecture.RoleApplicationModule
	componentRoleSharedModule      = architecture.RoleSharedModule
	componentRoleLibrary           = architecture.RoleLibrary
	componentRoleInfrastructure    = architecture.RoleInfrastructure
	componentRoleDevelopment       = architecture.RoleDevelopment
)

type Graph = report.Graph
type AnalysisPolicy = report.AnalysisPolicy
type StableLowAbstractionPolicy = report.StableLowAbstractionPolicy
type StableDependencyPrinciplePolicy = report.StableDependencyPrinciplePolicy
type GraphSummary = report.GraphSummary
type Component = report.Component
type Relationship = report.Relationship
type ImportSite = report.ImportSite
type Function = report.Function
type FunctionCall = report.FunctionCall
type CallSite = report.CallSite
type Diagnostic = report.Diagnostic
type Finding = report.Finding

const (
	findingSeverityError   = architecture.SeverityError
	findingSeverityWarning = architecture.SeverityWarning
)
