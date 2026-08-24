package quality

import (
	"testing"

	"github.com/cgardev/goconduct/pkg/plugin"
)

func TestNewCheckResultCountsEverySeverity(t *testing.T) {
	result := newCheckResult([]plugin.Report{{
		Plugin:  "fixture",
		Metrics: []plugin.Metric{{ID: "metric", Name: "fixture.value"}},
		Findings: []plugin.Finding{
			{ID: "notice", Rule: "rule", Severity: plugin.SeverityNotice, Message: "notice"},
			{ID: "warning", Rule: "rule", Severity: plugin.SeverityWarning, Message: "warning"},
			{ID: "error", Rule: "rule", Severity: plugin.SeverityError, Message: "error"},
		},
	}})

	want := CheckSummary{Plugins: 1, Metrics: 1, Findings: 3, Notices: 1, Warnings: 1, Errors: 1}
	if result.Summary != want {
		t.Fatalf("summary is %+v, want %+v", result.Summary, want)
	}
}
