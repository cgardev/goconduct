package crap

import (
	"slices"

	"github.com/cgardev/goconduct/policy"
)

const metricCRAPScore = "crap.score"

// Configuration defines crap4go execution and score limits.
type Configuration struct {
	Command      string              `json:"command"`
	TestCommand  string              `json:"testCommand,omitempty"`
	MaximumScore float64             `json:"maximumScore"`
	MaxWorkers   int                 `json:"maxWorkers,omitempty"`
	Policies     []policy.PathPolicy `json:"pathPolicies,omitempty"`
}

// DefaultConfiguration returns the experimental agent-oriented score limit.
func DefaultConfiguration() Configuration {
	return Configuration{Command: "crap4go", MaximumScore: 8}
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
	return configuration
}
