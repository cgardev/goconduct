package mutation

import "slices"

// Configuration defines mutate4go execution and mutation limits.
type Configuration struct {
	Command          string   `json:"command"`
	Paths            []string `json:"paths,omitempty"`
	Execute          bool     `json:"execute"`
	ReuseCoverage    bool     `json:"reuseCoverage,omitempty"`
	SinceLastRun     bool     `json:"sinceLastRun,omitempty"`
	MutateAll        bool     `json:"mutateAll,omitempty"`
	TestCommand      string   `json:"testCommand,omitempty"`
	MaxWorkers       int      `json:"maxWorkers,omitempty"`
	TimeoutFactor    int      `json:"timeoutFactor,omitempty"`
	MaximumSurvivors int      `json:"maximumSurvivors"`
	MaximumUncovered int      `json:"maximumUncovered"`
}

// DefaultConfiguration scans explicit paths without changing source files.
func DefaultConfiguration() Configuration {
	return Configuration{
		Command: "mutate4go", TimeoutFactor: 10,
		MaximumSurvivors: 0, MaximumUncovered: 0,
	}
}

func cloneConfiguration(configuration Configuration) Configuration {
	configuration.Paths = slices.Clone(configuration.Paths)
	return configuration
}
