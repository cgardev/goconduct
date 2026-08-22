package quality

import "slices"

// Configuration supplies application defaults to the quality module.
type Configuration struct {
	RepositoryRoot string
	Plugins        []string
	Paths          []string
}

func cloneConfiguration(configuration Configuration) Configuration {
	return Configuration{
		RepositoryRoot: configuration.RepositoryRoot,
		Plugins:        slices.Clone(configuration.Plugins),
		Paths:          slices.Clone(configuration.Paths),
	}
}
