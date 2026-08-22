package plugin

import (
	"math"
	"reflect"
	"testing"
)

func TestNewReportValidatesAndOrdersEvidence(t *testing.T) {
	actual := 72.0
	limit := 80.0
	report, err := NewReport("coverage", []Metric{
		{ID: "z", Path: "internal/z.go", Name: "coverage", Value: 90},
		{ID: "a", Path: "internal/a.go", Name: "coverage", Value: 72},
	}, []Finding{
		{
			ID: "z", Rule: "minimum", Path: "internal/z.go", Severity: SeverityWarning,
			Message: "coverage is low", Actual: &actual, Limit: &limit,
		},
		{
			ID: "a", Rule: "minimum", Path: "internal/a.go", Severity: SeverityError,
			Message: "coverage is low", Actual: &actual, Limit: &limit,
		},
	})
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	if report.SchemaVersion != ReportSchemaVersion || report.Plugin != "coverage" {
		t.Fatalf("unexpected report header: %+v", report)
	}
	if got := []string{report.Metrics[0].ID, report.Metrics[1].ID}; !reflect.DeepEqual(got, []string{"a", "z"}) {
		t.Fatalf("metric order is %v", got)
	}
	if got := []string{report.Findings[0].ID, report.Findings[1].ID}; !reflect.DeepEqual(got, []string{"a", "z"}) {
		t.Fatalf("finding order is %v", got)
	}
}

func TestNewReportRejectsInvalidEvidence(t *testing.T) {
	tests := []struct {
		name     string
		plugin   string
		metrics  []Metric
		findings []Finding
	}{
		{name: "invalid plugin", plugin: " coverage "},
		{name: "empty metric", plugin: "coverage", metrics: []Metric{{ID: "metric", Value: 1}}},
		{name: "nonfinite metric", plugin: "coverage", metrics: []Metric{{ID: "metric", Name: "coverage", Value: math.NaN()}}},
		{name: "duplicate metric", plugin: "coverage", metrics: []Metric{{ID: "metric", Name: "coverage"}, {ID: "metric", Name: "coverage"}}},
		{name: "invalid finding", plugin: "coverage", findings: []Finding{{ID: "finding", Rule: "minimum", Severity: "fatal", Message: "failure"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewReport(test.plugin, test.metrics, test.findings); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
