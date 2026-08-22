package policy

import (
	"strings"
	"testing"

	"github.com/cgardev/goconduct/plugin"
)

func TestResolverSelectsPathSpecificThreshold(t *testing.T) {
	resolver, err := NewResolver([]PathPolicy{
		{
			ID:      "domain",
			Include: []string{"internal/domain/**"},
			Exclude: []string{"internal/domain/generated/**"},
			Thresholds: []Threshold{{
				Metric: "coverage.percent", Comparison: ComparisonMinimum,
				Value: 100, Severity: plugin.SeverityError,
			}},
		},
	})
	if err != nil {
		t.Fatalf("create resolver: %v", err)
	}
	threshold, found, err := resolver.Resolve("internal/domain/order/service.go", "coverage.percent")
	if err != nil {
		t.Fatalf("resolve threshold: %v", err)
	}
	if !found || threshold.PolicyID != "domain" || threshold.Value != 100 {
		t.Fatalf("unexpected threshold: %+v, found=%t", threshold, found)
	}
	if threshold.Passes(99.9) || !threshold.Passes(100) {
		t.Fatal("minimum comparison returns an invalid result")
	}
	if _, found, err := resolver.Resolve("internal/domain/generated/model.go", "coverage.percent"); err != nil || found {
		t.Fatalf("excluded path result is found=%t, error=%v", found, err)
	}
}

func TestResolverRejectsAmbiguousPoliciesAtResolution(t *testing.T) {
	resolver, err := NewResolver([]PathPolicy{
		{
			ID: "all", Include: []string{"internal/**"},
			Thresholds: []Threshold{{Metric: "coverage.percent", Comparison: ComparisonMinimum, Value: 80, Severity: plugin.SeverityWarning}},
		},
		{
			ID: "domain", Include: []string{"internal/domain/**"},
			Thresholds: []Threshold{{Metric: "coverage.percent", Comparison: ComparisonMinimum, Value: 100, Severity: plugin.SeverityError}},
		},
	})
	if err != nil {
		t.Fatalf("create resolver: %v", err)
	}
	_, _, err = resolver.Resolve("internal/domain/order.go", "coverage.percent")
	if err == nil || !strings.Contains(err.Error(), "ambiguous policies") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
}

func TestResolverRejectsInvalidConfiguration(t *testing.T) {
	tests := []PathPolicy{
		{ID: "", Include: []string{"**"}, Thresholds: validThresholds()},
		{ID: "empty", Thresholds: validThresholds()},
		{ID: "pattern", Include: []string{"../internal/**"}, Thresholds: validThresholds()},
		{ID: "threshold", Include: []string{"**"}},
		{ID: "comparison", Include: []string{"**"}, Thresholds: []Threshold{{Metric: "coverage.percent", Comparison: "approximately", Severity: plugin.SeverityError}}},
	}
	for _, candidate := range tests {
		t.Run(candidate.ID, func(t *testing.T) {
			if _, err := NewResolver([]PathPolicy{candidate}); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func validThresholds() []Threshold {
	return []Threshold{{
		Metric: "coverage.percent", Comparison: ComparisonMinimum,
		Value: 80, Severity: plugin.SeverityError,
	}}
}
