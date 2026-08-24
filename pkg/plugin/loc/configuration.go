package loc

import (
	"slices"

	"github.com/cgardev/goconduct/pkg/policy"
)

// SelectionConfiguration defines the roots and path filters of one LOC run.
type SelectionConfiguration struct {
	Paths   []string `json:"paths"`
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

// GeneratedConfiguration defines how the plugin identifies generated files.
type GeneratedConfiguration struct {
	StandardMarker        bool     `json:"standardMarker"`
	PathPatterns          []string `json:"pathPatterns,omitempty"`
	HeaderPatterns        []string `json:"headerPatterns,omitempty"`
	ForceHandwrittenPaths []string `json:"forceHandwrittenPaths,omitempty"`
}

// Configuration defines source selection, generated detection, and limits.
type Configuration struct {
	Selection SelectionConfiguration `json:"selection"`
	Generated GeneratedConfiguration `json:"generated"`
	Policies  []policy.PathPolicy    `json:"pathPolicies,omitempty"`
}

// DefaultConfiguration returns deterministic repository-wide LOC settings.
func DefaultConfiguration() Configuration {
	return Configuration{
		Selection: SelectionConfiguration{
			Paths:   []string{"."},
			Include: []string{"**/*.go"},
			Exclude: []string{
				"**/.*/**",
				"**/_resources/**",
				"**/node_modules/**",
				"**/target/**",
				"**/testdata/**",
				"**/vendor/**",
			},
		},
		Generated: GeneratedConfiguration{
			StandardMarker: true,
			PathPatterns: []string{
				"**/*.connect.go",
				"**/*.pb.*.go",
				"**/*.pb.go",
			},
		},
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
		Selection: SelectionConfiguration{
			Paths:   slices.Clone(configuration.Selection.Paths),
			Include: slices.Clone(configuration.Selection.Include),
			Exclude: slices.Clone(configuration.Selection.Exclude),
		},
		Generated: GeneratedConfiguration{
			StandardMarker:        configuration.Generated.StandardMarker,
			PathPatterns:          slices.Clone(configuration.Generated.PathPatterns),
			HeaderPatterns:        slices.Clone(configuration.Generated.HeaderPatterns),
			ForceHandwrittenPaths: slices.Clone(configuration.Generated.ForceHandwrittenPaths),
		},
		Policies: policies,
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-23T19:50:40Z","module_hash":"29de8ca09edd44a58dcdeb6bd562275f48c268135464a45608ee38cf20c051ae","functions":[{"id":"func/DefaultConfiguration","name":"DefaultConfiguration","line":32,"end_line":55,"hash":"23d5effcb0d99459452205fb654e57eba8eca1e81e0f1770984c08674b5513ce"},{"id":"func/cloneConfiguration","name":"cloneConfiguration","line":57,"end_line":81,"hash":"08420e99b0170c696fcdb370386af773fec6c1ee244c38bbf36876fb19980674"}]}
// mutate4go-manifest-end
