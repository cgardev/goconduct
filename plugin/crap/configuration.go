package crap

import (
	"slices"

	"github.com/cgardev/goconduct/policy"
)

const metricCRAPScore = "crap.score"

// Configuration defines the coverage run and the CRAP score limits.
type Configuration struct {
	Command      string              `json:"command"`
	Packages     []string            `json:"packages"`
	MaximumScore float64             `json:"maximumScore"`
	Policies     []policy.PathPolicy `json:"pathPolicies,omitempty"`
}

// DefaultConfiguration returns the experimental agent-oriented score limit.
func DefaultConfiguration() Configuration {
	return Configuration{Command: "go", Packages: []string{"./..."}, MaximumScore: 8}
}

func cloneConfiguration(configuration Configuration) Configuration {
	policies := make([]policy.PathPolicy, 0, len(configuration.Policies))
	for _, candidate := range configuration.Policies {
		policies = append(policies, policy.PathPolicy{
			ID:         candidate.ID,
			Include:    slices.Clone(candidate.Include),
			Exclude:    slices.Clone(candidate.Exclude),
			Thresholds: slices.Clone(candidate.Thresholds),
		})
	}
	configuration.Policies = policies
	configuration.Packages = slices.Clone(configuration.Packages)
	return configuration
}
