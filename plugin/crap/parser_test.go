package crap

import "testing"

func TestParseReportIgnoresTestOutputAndReadsFunctions(t *testing.T) {
	payload := []byte(`ok example.com/project 0.1s coverage: 80.0% of statements
CRAP Report
===========
Function                       Package                               CC    Cov%     CRAP
----------------------------------------------------------------------------------------
Error.Error                    failure                                5  100.0%      5.0
CreateOrder                    orders                                12   70.5%     19.2
`)
	metrics, err := parseReport(payload)
	if err != nil {
		t.Fatalf("parse CRAP report: %v", err)
	}
	if len(metrics) != 2 {
		t.Fatalf("parsed metrics are %+v", metrics)
	}
	if metrics[0].function != "Error.Error" || metrics[1].score != 19.2 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
}
