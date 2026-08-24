package main

import (
	"bytes"
	"encoding/json"
	"testing"

	applicationconfiguration "github.com/cgardev/goconduct/cmd/goconduct/internal/configuration"
	"github.com/cgardev/goconduct/pkg/plugin"
)

func TestCombineReportsCountsEvidence(t *testing.T) {
	reports := []plugin.Report{
		{
			Plugin:  "coverage",
			Metrics: []plugin.Metric{{ID: "metric", Name: "coverage.percent"}},
			Findings: []plugin.Finding{{
				ID: "warning", Rule: "minimum", Severity: plugin.SeverityWarning, Message: "warning",
			}},
		},
		{
			Plugin: "mutation",
			Findings: []plugin.Finding{{
				ID: "error", Rule: "survived", Severity: plugin.SeverityError, Message: "error",
			}},
		},
	}
	combined := combineReports(reports)
	if combined.Summary.Plugins != 2 || combined.Summary.Metrics != 1 || combined.Summary.Findings != 2 {
		t.Fatalf("unexpected summary: %+v", combined.Summary)
	}
	if combined.Summary.Errors != 1 || combined.Summary.Warnings != 1 {
		t.Fatalf("unexpected severity counts: %+v", combined.Summary)
	}
}

func TestCheckThresholdUsesConfiguredSeverity(t *testing.T) {
	report := checkReport{Reports: []plugin.Report{{
		Plugin: "coverage",
		Findings: []plugin.Finding{{
			ID: "warning", Rule: "minimum", Severity: plugin.SeverityWarning, Message: "warning",
		}},
	}}}
	if err := enforceCheckThreshold(report, applicationconfiguration.FailureThresholdError); err != nil {
		t.Fatalf("error threshold rejects warning: %v", err)
	}
	if err := enforceCheckThreshold(report, applicationconfiguration.FailureThresholdWarning); err == nil {
		t.Fatal("warning threshold accepts warning")
	}
}

func TestWriteCheckReportProducesMachineReadableJSON(t *testing.T) {
	var output bytes.Buffer
	if err := writeCheckReport(&output, checkReport{SchemaVersion: 1}, true); err != nil {
		t.Fatalf("write report: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if document["schemaVersion"] != float64(1) {
		t.Fatalf("schema version is %v", document["schemaVersion"])
	}
}
