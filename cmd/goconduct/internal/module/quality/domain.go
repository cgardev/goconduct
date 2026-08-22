package quality

import "github.com/cgardev/goconduct/plugin"

// CheckSummary counts evidence by type and severity.
type CheckSummary struct {
	Plugins  int
	Metrics  int
	Findings int
	Notices  int
	Warnings int
	Errors   int
}

// CheckResult contains reports and their aggregate summary.
type CheckResult struct {
	Summary CheckSummary
	Reports []plugin.Report
}

func newCheckResult(reports []plugin.Report) CheckResult {
	result := CheckResult{Reports: reports}
	result.Summary.Plugins = len(reports)
	for _, report := range reports {
		result.Summary.Metrics += len(report.Metrics)
		result.Summary.Findings += len(report.Findings)
		for _, finding := range report.Findings {
			switch finding.Severity {
			case plugin.SeverityNotice:
				result.Summary.Notices++
			case plugin.SeverityWarning:
				result.Summary.Warnings++
			case plugin.SeverityError:
				result.Summary.Errors++
			}
		}
	}
	return result
}
