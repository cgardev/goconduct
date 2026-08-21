package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
		return "", fmt.Errorf("encode graph cache identity: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T15:48:49Z","module_hash":"edb9481daea2da92ab9223002d9f950405b1a8811714db148ef0d2aa34d486c0","functions":[{"id":"func/analyzer.graphCacheKey","name":"analyzer.graphCacheKey","line":23,"end_line":39,"hash":"cfaba175605e61592d366b05d1a98e4dd22cc6b1f414eef35b689e83ff5e2c40"}]}
// mutate4go-manifest-end
