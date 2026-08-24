package query

import "github.com/cgardev/goconduct/pkg/report"

type Graph = report.Graph
type Component = report.Component
type Relationship = report.Relationship
type Function = report.Function
type FunctionCall = report.FunctionCall
type CallSite = report.CallSite
type Finding = report.Finding
type TypeDeclaration = report.TypeDeclaration
type TypeReference = report.TypeReference

const (
	graphSchemaVersion       = report.SchemaVersion
	componentRoleApplication = report.ComponentRoleApplication
	componentRoleLibrary     = report.ComponentRoleLibrary
	findingSeverityError     = report.FindingSeverityError
	findingSeverityWarning   = report.FindingSeverityWarning
	typeKindStruct           = report.TypeKindStruct
	typeKindInterface        = report.TypeKindInterface
)
