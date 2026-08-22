package mutation

import "testing"

func TestParseMutationReportsScanAndExecution(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		executed bool
		killed   int
		survived int
	}{
		{
			name: "scan",
			payload: "Mutation scan: internal/order.go\n" +
				"Total mutation sites: 9\nChanged mutation sites: 0\nManifest exists: true\n",
		},
		{
			name: "execution",
			payload: "Mutation run: internal/order.go\nTotal mutation sites: 9\n" +
				"Covered mutation sites: 8\nUncovered mutation sites: 1\nSelected mutation sites: 8\n" +
				"Mutation Report\nKilled: 7\nSurvived: 1\nUncovered: 1\n",
			executed: true, killed: 7, survived: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := parseMutationReport([]byte(test.payload))
			if err != nil {
				t.Fatalf("parse report: %v", err)
			}
			if result.executed != test.executed || result.killed != test.killed || result.survived != test.survived {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}
