package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"runtime"
)

type graphCacheIdentity struct {
	ProtocolVersion   int           `json:"protocolVersion"`
	SchemaVersion     int           `json:"schemaVersion"`
	RepositoryRoot    string        `json:"repositoryRoot"`
	Scope             AnalysisScope `json:"scope"`
	GoVersion         string        `json:"goVersion"`
	GoOperatingSystem string        `json:"goOperatingSystem"`
	GoArchitecture    string        `json:"goArchitecture"`
	GoFlags           string        `json:"goFlags"`
}

func (analyzer *analyzer) graphCacheKey() (string, error) {
	payload, err := json.Marshal(graphCacheIdentity{
		ProtocolVersion:   graphCacheProtocolVersion,
		SchemaVersion:     graphSchemaVersion,
		RepositoryRoot:    analyzer.repositoryRoot,
		Scope:             analyzer.scope,
		GoVersion:         runtime.Version(),
		GoOperatingSystem: runtime.GOOS,
		GoArchitecture:    runtime.GOARCH,
		GoFlags:           os.Getenv("GOFLAGS"),
	})
	if err != nil {
		return "", newInternalError("encode graph cache identity", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"5dc700b9e54a1cd8183ded794919d9e6c4e6f6e6eebb9649d2608256e5ed5daa","functions":[{"id":"func/analyzer.graphCacheKey","name":"analyzer.graphCacheKey","line":22,"end_line":38,"hash":"3697683e282ae2b94b87690a8093d45d0973a3765cd824fa5bd84725b28bd9a6"}]}
// mutate4go-manifest-end
