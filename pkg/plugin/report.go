package plugin

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/cgardev/goconduct/pkg/failure"
)

const (
	// ReportSchemaVersion is the current normalized evidence contract.
	ReportSchemaVersion = 1
)

// Severity classifies the effect of one finding.
type Severity string

const (
	// SeverityNotice records evidence without failing a policy.
	SeverityNotice Severity = "notice"
	// SeverityWarning records a policy warning.
	SeverityWarning Severity = "warning"
	// SeverityError records a policy failure.
	SeverityError Severity = "error"
)

// Request selects repository content for one evaluation.
type Request struct {
	RepositoryRoot string   `json:"repositoryRoot"`
	Paths          []string `json:"paths,omitempty"`
}

// Metric is one normalized numeric measurement.
type Metric struct {
	ID    string  `json:"id"`
	Path  string  `json:"path,omitempty"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

// Finding explains one deterministic policy result.
type Finding struct {
	ID       string   `json:"id"`
	Rule     string   `json:"rule"`
	Path     string   `json:"path,omitempty"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Actual   *float64 `json:"actual,omitempty"`
	Limit    *float64 `json:"limit,omitempty"`
}

// Report contains the normalized output from one evaluator.
type Report struct {
	SchemaVersion int       `json:"schemaVersion"`
	Plugin        string    `json:"plugin"`
	Metrics       []Metric  `json:"metrics"`
	Findings      []Finding `json:"findings"`
}

// NewReport validates and orders one evaluator report.
func NewReport(name string, metrics []Metric, findings []Finding) (Report, error) {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name {
		return Report{}, failure.Validation(fmt.Sprintf("plugin name %q is invalid", name), nil)
	}
	orderedMetrics := slices.Clone(metrics)
	orderedFindings := slices.Clone(findings)
	if err := validateMetrics(orderedMetrics); err != nil {
		return Report{}, err
	}
	if err := validateFindings(orderedFindings); err != nil {
		return Report{}, err
	}
	slices.SortFunc(orderedMetrics, compareMetric)
	slices.SortFunc(orderedFindings, compareFinding)
	return Report{
		SchemaVersion: ReportSchemaVersion,
		Plugin:        name,
		Metrics:       orderedMetrics,
		Findings:      orderedFindings,
	}, nil
}

func validateMetrics(metrics []Metric) error {
	identifiers := make(map[string]struct{}, len(metrics))
	for _, metric := range metrics {
		if strings.TrimSpace(metric.ID) == "" {
			return failure.Validation("metric identifier is empty", nil)
		}
		if strings.TrimSpace(metric.Name) == "" {
			return failure.Validation(fmt.Sprintf("metric %q name is empty", metric.ID), nil)
		}
		if math.IsNaN(metric.Value) || math.IsInf(metric.Value, 0) {
			return failure.Validation(fmt.Sprintf("metric %q value is not finite", metric.ID), nil)
		}
		if _, duplicate := identifiers[metric.ID]; duplicate {
			return failure.Validation(fmt.Sprintf("metric identifier %q is duplicated", metric.ID), nil)
		}
		identifiers[metric.ID] = struct{}{}
	}
	return nil
}

func validateFindings(findings []Finding) error {
	identifiers := make(map[string]struct{}, len(findings))
	for _, finding := range findings {
		if strings.TrimSpace(finding.ID) == "" {
			return failure.Validation("finding identifier is empty", nil)
		}
		if strings.TrimSpace(finding.Rule) == "" {
			return failure.Validation(fmt.Sprintf("finding %q rule is empty", finding.ID), nil)
		}
		if strings.TrimSpace(finding.Message) == "" {
			return failure.Validation(fmt.Sprintf("finding %q message is empty", finding.ID), nil)
		}
		if err := validateSeverity(finding.Severity); err != nil {
			return fmt.Errorf("finding %q: %w", finding.ID, err)
		}
		if _, duplicate := identifiers[finding.ID]; duplicate {
			return failure.Validation(
				fmt.Sprintf("finding identifier %q is duplicated", finding.ID),
				nil,
			)
		}
		identifiers[finding.ID] = struct{}{}
	}
	return nil
}

func validateSeverity(severity Severity) error {
	switch severity {
	case SeverityNotice, SeverityWarning, SeverityError:
		return nil
	default:
		return failure.Validation(fmt.Sprintf("severity %q is invalid", severity), nil)
	}
}

func compareMetric(left, right Metric) int {
	if comparison := cmp.Compare(left.Path, right.Path); comparison != 0 {
		return comparison
	}
	if comparison := cmp.Compare(left.Name, right.Name); comparison != 0 {
		return comparison
	}
	return cmp.Compare(left.ID, right.ID)
}

func compareFinding(left, right Finding) int {
	if comparison := cmp.Compare(left.Path, right.Path); comparison != 0 {
		return comparison
	}
	if comparison := cmp.Compare(left.Rule, right.Rule); comparison != 0 {
		return comparison
	}
	return cmp.Compare(left.ID, right.ID)
}

// FailingFindings counts the findings that stop a check.
// A notice records evidence and a warning reports a risk, so neither closes a
// gate. Only an error does.
func FailingFindings(findings []Finding) int {
	failing := 0
	for _, finding := range findings {
		if finding.Severity == SeverityError {
			failing++
		}
	}
	return failing
}
