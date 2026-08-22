package duplication

// Configuration defines the duplication analysis limits.
type Configuration struct {
	Similarity        float64 `json:"similarity"`
	MinimumLines      int     `json:"minimumLines"`
	MinimumNodes      int     `json:"minimumNodes"`
	MaximumCandidates int     `json:"maximumCandidates"`
}

// DefaultConfiguration rejects every structural duplicate the analysis reports.
func DefaultConfiguration() Configuration {
	return Configuration{
		Similarity: 0.82, MinimumLines: 4,
		MinimumNodes: 20, MaximumCandidates: 0,
	}
}
