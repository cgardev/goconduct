package main

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

func validComponentRole(role componentRole) bool {
	return architecture.ValidRole(role)
}

type Graph = report.Graph
type AnalysisScope = report.AnalysisScope
type ComponentRules = report.ComponentRules
type ComponentCategoryRule = report.ComponentCategoryRule
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

type findingSeverity = report.FindingSeverity

const (
	findingSeverityError   = architecture.SeverityError
	findingSeverityWarning = architecture.SeverityWarning
)

type Finding = report.Finding
