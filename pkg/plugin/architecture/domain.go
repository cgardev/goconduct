package architecture

import (
	"github.com/cgardev/goconduct/pkg/report"
)

const graphSchemaVersion = report.SchemaVersion

// ComponentRole identifies the strategic responsibility of one component.
type ComponentRole = report.ComponentRole

type componentRole = ComponentRole

const (
	componentRoleApplication       = report.ComponentRoleApplication
	componentRoleApplicationModule = report.ComponentRoleApplicationModule
	componentRoleSharedModule      = report.ComponentRoleSharedModule
	componentRoleLibrary           = report.ComponentRoleLibrary
	componentRoleInfrastructure    = report.ComponentRoleInfrastructure
	componentRoleDevelopment       = report.ComponentRoleDevelopment
)

func validComponentRole(role componentRole) bool {
	return report.ValidComponentRole(role)
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

// TypeKind identifies the declaration form of one named Go type.
type TypeKind = report.TypeKind

const (
	typeKindStruct    = report.TypeKindStruct
	typeKindInterface = report.TypeKindInterface
	typeKindAlias     = report.TypeKindAlias
	typeKindBasic     = report.TypeKindBasic
)

// TypeDeclaration describes one named Go type declared in one component.
type TypeDeclaration = report.TypeDeclaration

// TypeField describes one field of a struct type.
type TypeField = report.TypeField

// TypeMethod describes one declared method of a type.
type TypeMethod = report.TypeMethod

// TypeReference identifies one related type and its declaring component.
type TypeReference = report.TypeReference

// CallSite identifies one source location that calls a function.
type CallSite = report.CallSite

// Diagnostic identifies one source analysis problem.
type Diagnostic = report.Diagnostic

type findingSeverity = report.FindingSeverity

const (
	findingSeverityError   = report.FindingSeverityError
	findingSeverityWarning = report.FindingSeverityWarning
)

// Finding identifies one architecture policy result.
type Finding = report.Finding
