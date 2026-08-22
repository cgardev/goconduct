package main

import (
	"github.com/cgardev/goconduct/internal/architecture"
	"github.com/cgardev/goconduct/internal/report"
)

const graphSchemaVersion = report.SchemaVersion

// ComponentRole identifies the strategic responsibility of one component.
type ComponentRole = report.ComponentRole

type componentRole = ComponentRole

const (
	componentRoleApplication       = architecture.RoleApplication
	componentRoleApplicationModule = architecture.RoleApplicationModule
	componentRoleSharedModule      = architecture.RoleSharedModule
	componentRoleLibrary           = architecture.RoleLibrary
	componentRoleInfrastructure    = architecture.RoleInfrastructure
	componentRoleDevelopment       = architecture.RoleDevelopment
)

func validComponentRole(role componentRole) bool {
	return architecture.ValidRole(role)
}

// Graph contains one complete dependency analysis report.
type Graph = report.Graph

// AnalysisScope identifies the paths and classification rules of one analysis.
type AnalysisScope = report.AnalysisScope

// ComponentRules define the configured component path templates.
type ComponentRules = report.ComponentRules

// ComponentCategoryRule defines one custom component category.
type ComponentCategoryRule = report.ComponentCategoryRule

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

type findingSeverity = report.FindingSeverity

const (
	findingSeverityError   = architecture.SeverityError
	findingSeverityWarning = architecture.SeverityWarning
)

// Finding identifies one architecture policy result.
type Finding = report.Finding
