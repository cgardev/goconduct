package duplication

// Configuration defines dry4go execution and duplication limits.
type Configuration struct {
	Command           string  `json:"command"`
	Similarity        float64 `json:"similarity"`
	MinimumLines      int     `json:"minimumLines"`
	MinimumNodes      int     `json:"minimumNodes"`
	MaximumCandidates int     `json:"maximumCandidates"`
}

// DefaultConfiguration rejects every structural duplicate reported by dry4go.
func DefaultConfiguration() Configuration {
	return Configuration{
		Command: "dry4go", Similarity: 0.82, MinimumLines: 4,
		MinimumNodes: 20, MaximumCandidates: 0,
	}
}
