package mutation

import "slices"

// Configuration defines the coverage run and the mutation limits.
type Configuration struct {
	Command          string   `json:"command"`
	Packages         []string `json:"packages"`
	Paths            []string `json:"paths,omitempty"`
	Execute          bool     `json:"execute"`
	TimeoutFactor    int      `json:"timeoutFactor,omitempty"`
	MaximumSurvivors int      `json:"maximumSurvivors"`
	MaximumUncovered int      `json:"maximumUncovered"`
}

// DefaultConfiguration reports mutation sites without changing any source file.
// An execution applies every covered mutation and runs the test suite for each.
func DefaultConfiguration() Configuration {
	return Configuration{
		Command: "go", Packages: []string{"./..."}, TimeoutFactor: 10,
		MaximumSurvivors: 0, MaximumUncovered: 0,
	}
}

func cloneConfiguration(configuration Configuration) Configuration {
	configuration.Paths = slices.Clone(configuration.Paths)
	configuration.Packages = slices.Clone(configuration.Packages)
	return configuration
}
