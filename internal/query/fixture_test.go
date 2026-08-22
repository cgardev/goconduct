package query

import (
	"github.com/cgardev/goconduct/internal/architecture"
	"github.com/cgardev/goconduct/internal/report"
)

type Graph = report.Graph
type Component = report.Component
type Relationship = report.Relationship
type Function = report.Function
type FunctionCall = report.FunctionCall
type CallSite = report.CallSite
type Finding = report.Finding

const (
	graphSchemaVersion       = report.SchemaVersion
	componentRoleApplication = architecture.RoleApplication
	componentRoleLibrary     = architecture.RoleLibrary
	findingSeverityError     = architecture.SeverityError
	findingSeverityWarning   = architecture.SeverityWarning
)
