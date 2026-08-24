package policy

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
)

func TestPathSelectorAppliesSharedIncludeAndExcludePatterns(t *testing.T) {
	selector, err := NewPathSelector(PathSelection{
		Include: []string{"internal/**", "pkg/**", "internal/**"},
		Exclude: []string{"**/testdata/**", "internal/generated/**"},
	})
	if err != nil {
		t.Fatalf("create path selector: %v", err)
	}
	tests := map[string]bool{
		"internal/order/service.go":        true,
		"internal/order/testdata/input.go": false,
		"internal/generated/model.go":      false,
		"pkg/plugin/loc/evaluator.go":      true,
		"cmd/goconduct/main.go":            false,
	}
	for candidate, want := range tests {
		got, err := selector.Select(candidate)
		if err != nil {
			t.Fatalf("select %q: %v", candidate, err)
		}
		if got != want {
			t.Errorf("selection of %q is %t, want %t", candidate, got, want)
		}
	}
}

func TestPathSelectorWithNoIncludesSelectsEveryNonExcludedPath(t *testing.T) {
	selector, err := NewPathSelector(PathSelection{Exclude: []string{"**/vendor/**"}})
	if err != nil {
		t.Fatalf("create path selector: %v", err)
	}
	for candidate, want := range map[string]bool{
		"main.go":             true,
		"vendor/tool/tool.go": false,
	} {
		got, selectErr := selector.Select(candidate)
		if selectErr != nil {
			t.Fatalf("select %q: %v", candidate, selectErr)
		}
		if got != want {
			t.Errorf("selection of %q is %t, want %t", candidate, got, want)
		}
	}
}

func TestPathSelectorRejectsInvalidPatternsAndPaths(t *testing.T) {
	for _, selection := range []PathSelection{
		{Include: []string{"../internal/**"}},
		{Exclude: []string{`internal\generated`}},
	} {
		if _, err := NewPathSelector(selection); !errors.Is(err, failure.ErrValidation) {
			t.Errorf("the selection reports %v, want a validation failure", err)
		}
	}
	selector, err := NewPathSelector(PathSelection{})
	if err != nil {
		t.Fatalf("create empty path selector: %v", err)
	}
	if _, err := selector.Select("../escape.go"); !errors.Is(err, failure.ErrValidation) {
		t.Errorf("the path reports %v, want a validation failure", err)
	}
}

func TestPathSelectorDefensivelyCopiesPatterns(t *testing.T) {
	selection := PathSelection{Include: []string{"internal/**"}}
	selector, err := NewPathSelector(selection)
	if err != nil {
		t.Fatalf("create path selector: %v", err)
	}
	selection.Include[0] = "cmd/**"
	selected := make([]string, 0)
	for _, candidate := range []string{"cmd/main.go", "internal/order.go"} {
		matches, selectErr := selector.Select(candidate)
		if selectErr != nil {
			t.Fatalf("select %q: %v", candidate, selectErr)
		}
		if matches {
			selected = append(selected, candidate)
		}
	}
	if !reflect.DeepEqual(selected, []string{"internal/order.go"}) {
		t.Errorf("the selector keeps %v after caller mutation", selected)
	}
}

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
