package coverage

import (
	"slices"

	"github.com/cgardev/goconduct/policy"
)

const metricCoveragePercent = "coverage.percent"

// Configuration defines Go coverage execution and path policies.
type Configuration struct {
	Command  string              `json:"command"`
	Packages []string            `json:"packages"`
	Policies []policy.PathPolicy `json:"pathPolicies,omitempty"`
}

// DefaultConfiguration returns a standalone Go coverage configuration.
func DefaultConfiguration() Configuration {
	return Configuration{
		Command:  "go",
		Packages: []string{"./..."},
	}
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
	return Configuration{
		Command:  configuration.Command,
		Packages: slices.Clone(configuration.Packages),
		Policies: policies,
	}
}
