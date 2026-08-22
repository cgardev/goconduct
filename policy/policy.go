// Package policy validates and resolves deterministic path policies.
package policy

import (
	"fmt"
	"math"
	"path"
	"slices"
	"strings"

	"github.com/cgardev/goconduct/plugin"
)

// Comparison defines how a metric is compared with its limit.
type Comparison string

const (
	// ComparisonMinimum requires the metric to meet or exceed the limit.
	ComparisonMinimum Comparison = "minimum"
	// ComparisonMaximum requires the metric to remain at or below the limit.
	ComparisonMaximum Comparison = "maximum"
)

// Threshold applies one metric limit to selected paths.
type Threshold struct {
	Metric     string          `json:"metric"`
	Comparison Comparison      `json:"comparison"`
	Value      float64         `json:"value"`
	Severity   plugin.Severity `json:"severity"`
}

// PathPolicy associates metric limits with repository path patterns.
type PathPolicy struct {
	ID         string      `json:"id"`
	Include    []string    `json:"include"`
	Exclude    []string    `json:"exclude,omitempty"`
	Thresholds []Threshold `json:"thresholds"`
}

// ResolvedThreshold identifies the policy that selected one threshold.
type ResolvedThreshold struct {
	PolicyID string
	Threshold
}

// Resolver selects exactly one applicable threshold for a path and metric.
type Resolver struct {
	policies []PathPolicy
}

// NewResolver validates and defensively copies path policies.
func NewResolver(policies []PathPolicy) (*Resolver, error) {
	cloned := make([]PathPolicy, 0, len(policies))
	identifiers := make(map[string]struct{}, len(policies))
	for _, candidate := range policies {
		if strings.TrimSpace(candidate.ID) == "" || strings.TrimSpace(candidate.ID) != candidate.ID {
			return nil, fmt.Errorf("path policy identifier %q is invalid", candidate.ID)
		}
		if _, duplicate := identifiers[candidate.ID]; duplicate {
			return nil, fmt.Errorf("path policy identifier %q is duplicated", candidate.ID)
		}
		identifiers[candidate.ID] = struct{}{}
		if len(candidate.Include) == 0 {
			return nil, fmt.Errorf("path policy %q has no include patterns", candidate.ID)
		}
		for _, pattern := range append(slices.Clone(candidate.Include), candidate.Exclude...) {
			if err := validatePattern(pattern); err != nil {
				return nil, fmt.Errorf("path policy %q: %w", candidate.ID, err)
			}
		}
		if len(candidate.Thresholds) == 0 {
			return nil, fmt.Errorf("path policy %q has no thresholds", candidate.ID)
		}
		metrics := make(map[string]struct{}, len(candidate.Thresholds))
		for _, threshold := range candidate.Thresholds {
			if err := validateThreshold(threshold); err != nil {
				return nil, fmt.Errorf("path policy %q: %w", candidate.ID, err)
			}
			if _, duplicate := metrics[threshold.Metric]; duplicate {
				return nil, fmt.Errorf(
					"path policy %q metric %q is duplicated",
					candidate.ID,
					threshold.Metric,
				)
			}
			metrics[threshold.Metric] = struct{}{}
		}
		cloned = append(cloned, PathPolicy{
			ID:         candidate.ID,
			Include:    slices.Clone(candidate.Include),
			Exclude:    slices.Clone(candidate.Exclude),
			Thresholds: slices.Clone(candidate.Thresholds),
		})
	}
	slices.SortFunc(cloned, func(left, right PathPolicy) int {
		return strings.Compare(left.ID, right.ID)
	})
	return &Resolver{policies: cloned}, nil
}

// Resolve returns the unique threshold for one repository path and metric.
func (resolver *Resolver) Resolve(
	repositoryPath string,
	metric string,
) (ResolvedThreshold, bool, error) {
	normalizedPath, err := normalizeRepositoryPath(repositoryPath)
	if err != nil {
		return ResolvedThreshold{}, false, err
	}
	matches := make([]ResolvedThreshold, 0, 1)
	for _, candidate := range resolver.policies {
		if !matchesAny(candidate.Include, normalizedPath) || matchesAny(candidate.Exclude, normalizedPath) {
			continue
		}
		for _, threshold := range candidate.Thresholds {
			if threshold.Metric == metric {
				matches = append(matches, ResolvedThreshold{
					PolicyID:  candidate.ID,
					Threshold: threshold,
				})
			}
		}
	}
	if len(matches) == 0 {
		return ResolvedThreshold{}, false, nil
	}
	if len(matches) > 1 {
		identifiers := make([]string, 0, len(matches))
		for _, match := range matches {
			identifiers = append(identifiers, match.PolicyID)
		}
		return ResolvedThreshold{}, false, fmt.Errorf(
			"path %q metric %q matches ambiguous policies %v",
			normalizedPath,
			metric,
			identifiers,
		)
	}
	return matches[0], true, nil
}

// Passes reports whether an actual value satisfies the threshold.
func (threshold Threshold) Passes(actual float64) bool {
	switch threshold.Comparison {
	case ComparisonMinimum:
		return actual >= threshold.Value
	case ComparisonMaximum:
		return actual <= threshold.Value
	default:
		return false
	}
}

func validateThreshold(threshold Threshold) error {
	if strings.TrimSpace(threshold.Metric) == "" || strings.TrimSpace(threshold.Metric) != threshold.Metric {
		return fmt.Errorf("threshold metric %q is invalid", threshold.Metric)
	}
	if threshold.Comparison != ComparisonMinimum && threshold.Comparison != ComparisonMaximum {
		return fmt.Errorf("threshold comparison %q is invalid", threshold.Comparison)
	}
	if math.IsNaN(threshold.Value) || math.IsInf(threshold.Value, 0) {
		return fmt.Errorf("threshold %q value is not finite", threshold.Metric)
	}
	switch threshold.Severity {
	case plugin.SeverityNotice, plugin.SeverityWarning, plugin.SeverityError:
		return nil
	default:
		return fmt.Errorf("threshold severity %q is invalid", threshold.Severity)
	}
}

func validatePattern(pattern string) error {
	if pattern == "" || strings.TrimSpace(pattern) != pattern {
		return fmt.Errorf("path pattern %q is invalid", pattern)
	}
	if strings.Contains(pattern, "\\") || strings.HasPrefix(pattern, "/") {
		return fmt.Errorf("path pattern %q is not repository-relative", pattern)
	}
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("path pattern %q contains an invalid segment", pattern)
		}
		if segment == "**" {
			continue
		}
		if _, err := path.Match(segment, "candidate"); err != nil {
			return fmt.Errorf("path pattern %q is invalid: %w", pattern, err)
		}
	}
	return nil
}

func normalizeRepositoryPath(repositoryPath string) (string, error) {
	if repositoryPath == "" || strings.Contains(repositoryPath, "\\") || strings.HasPrefix(repositoryPath, "/") {
		return "", fmt.Errorf("repository path %q is invalid", repositoryPath)
	}
	normalized := path.Clean(repositoryPath)
	if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", fmt.Errorf("repository path %q is invalid", repositoryPath)
	}
	return normalized, nil
}

func matchesAny(patterns []string, repositoryPath string) bool {
	pathSegments := strings.Split(repositoryPath, "/")
	for _, pattern := range patterns {
		if matchSegments(strings.Split(pattern, "/"), pathSegments, make(map[[2]int]bool), make(map[[2]int]bool)) {
			return true
		}
	}
	return false
}

func matchSegments(
	pattern []string,
	candidate []string,
	known map[[2]int]bool,
	values map[[2]int]bool,
) bool {
	var visit func(int, int) bool
	visit = func(patternIndex, candidateIndex int) bool {
		key := [2]int{patternIndex, candidateIndex}
		if known[key] {
			return values[key]
		}
		known[key] = true
		matched := false
		switch {
		case patternIndex == len(pattern):
			matched = candidateIndex == len(candidate)
		case pattern[patternIndex] == "**":
			matched = visit(patternIndex+1, candidateIndex)
			if !matched && candidateIndex < len(candidate) {
				matched = visit(patternIndex, candidateIndex+1)
			}
		case candidateIndex < len(candidate):
			segmentMatched, err := path.Match(pattern[patternIndex], candidate[candidateIndex])
			matched = err == nil && segmentMatched && visit(patternIndex+1, candidateIndex+1)
		}
		values[key] = matched
		return matched
	}
	return visit(0, 0)
}
