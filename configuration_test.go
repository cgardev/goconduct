package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/cgardev/gokeel/conf"
)

func TestApplicationConfiguration_LoadDefaults(t *testing.T) {
	t.Run("Scenario: The optional configuration document is absent", func(t *testing.T) {
		var configuration ApplicationConfiguration
		var loadError error

		t.Run("Given a path that does not identify a document", func(t *testing.T) {
			configuration = ApplicationConfiguration{}
		})

		t.Run("When the loader reads the application configuration", func(t *testing.T) {
			configuration, loadError = loadApplicationConfiguration(
				filepath.Join(t.TempDir(), "absent.json"),
			)
		})

		if !t.Run("Then the loader returns the server defaults", func(t *testing.T) {
			if loadError != nil {
				t.Fatalf("loadApplicationConfiguration fails: %v", loadError)
			}
			if configuration.Server.Address != defaultAddress ||
				configuration.Server.RefreshInterval != defaultRefreshInterval() {
				t.Fatalf("unexpected server defaults: %+v", configuration.Server)
			}
		}) {
			return
		}

		t.Run("And the defaults contain the complete repository scope", func(t *testing.T) {
			if !slices.Equal(configuration.Analysis.Paths, []string{"."}) ||
				configuration.Analysis.RepositoryRoot != "." ||
				len(configuration.Analysis.IgnoredPaths) == 0 ||
				len(configuration.Analysis.Components.Libraries) == 0 {
				t.Errorf("unexpected analysis defaults: %+v", configuration.Analysis)
			}
		})

		t.Run("And CLI queries use the active cache with a bounded request", func(t *testing.T) {
			if configuration.Cache.Mode != CacheModeAuto ||
				configuration.Cache.RequestTimeout != defaultCacheTimeout {
				t.Errorf("unexpected cache defaults: %+v", configuration.Cache)
			}
		})
	})
}

func TestApplicationConfiguration_ApplyDocument(t *testing.T) {
	t.Run("Scenario: A document selects a custom generic Go layout", func(t *testing.T) {
		var configurationPath string
		var configuration ApplicationConfiguration
		var loadError error

		t.Run(
			"Given a JSON document with placeholders, paths, exclusions, and component templates",
			func(step *testing.T) {
				repositoryRoot := t.TempDir()
				t.Setenv("DEPENDENCY_GRAPH_ROOT", repositoryRoot)
				configurationPath = filepath.Join(t.TempDir(), "configuration.json")
				writeFixtureFile(step, filepath.Dir(configurationPath), filepath.Base(configurationPath), `{
  "server": {
    "address": "127.0.0.1:7000",
    "refreshInterval": "2s"
  },
  "cache": {
    "mode": "server",
    "requestTimeout": "3s"
  },
  "analysis": {
    "repositoryRoot": "${DEPENDENCY_GRAPH_ROOT}",
    "paths": ["services", "packages"],
    "ignoredPaths": ["generated", "packages/legacy"],
    "components": {
      "applications": ["services/{application}"],
      "applicationModules": ["services/{application}/features/{component}"],
      "sharedModules": [],
      "libraries": ["packages/{component}"],
      "infrastructure": [],
      "developmentTools": []
    }
  }
}`)
			},
		)

		t.Run("When the loader applies the document to the code defaults", func(t *testing.T) {
			configuration, loadError = loadApplicationConfiguration(configurationPath)
		})

		if !t.Run("Then the configuration contains the server and repository values", func(t *testing.T) {
			if loadError != nil {
				t.Fatalf("loadApplicationConfiguration fails: %v", loadError)
			}
			if configuration.Server.Address != "127.0.0.1:7000" ||
				configuration.Server.RefreshInterval != 2*time.Second ||
				configuration.Analysis.RepositoryRoot == "${DEPENDENCY_GRAPH_ROOT}" {
				t.Fatalf("unexpected bound configuration: %+v", configuration)
			}
		}) {
			return
		}

		t.Run("And arrays replace defaults and do not retain project-specific paths", func(t *testing.T) {
			if !slices.Equal(configuration.Analysis.Paths, []string{"services", "packages"}) ||
				!slices.Equal(
					configuration.Analysis.IgnoredPaths,
					[]string{"generated", "packages/legacy"},
				) ||
				!slices.Equal(
					configuration.Analysis.Components.Libraries,
					[]string{"packages/{component}"},
				) ||
				len(configuration.Analysis.Components.SharedModules) != 0 {
				t.Errorf("the loader does not apply document arrays exactly: %+v", configuration.Analysis)
			}
		})

		t.Run("And the document selects the required server cache", func(t *testing.T) {
			if configuration.Cache.Mode != CacheModeServer ||
				configuration.Cache.RequestTimeout != 3*time.Second {
				t.Errorf("unexpected bound cache configuration: %+v", configuration.Cache)
			}
		})
	})
}

func TestApplicationConfiguration_RejectInvalidDocument(t *testing.T) {
	testCases := []struct {
		name           string
		document       string
		wantUnknownKey bool
	}{
		{name: "the JSON syntax is invalid", document: `{"analysis":`},
		{
			name:           "an unknown analysis key is present",
			document:       `{"analysis":{"ignoredPath":["vendor"]}}`,
			wantUnknownKey: true,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var configurationPath string
			var loadError error

			t.Run("Given an invalid external configuration document", func(step *testing.T) {
				configurationPath = filepath.Join(t.TempDir(), "configuration.json")
				writeFixtureFile(
					step,
					filepath.Dir(configurationPath),
					filepath.Base(configurationPath),
					testCase.document,
				)
			})

			t.Run("When the loader reads the application configuration", func(t *testing.T) {
				_, loadError = loadApplicationConfiguration(configurationPath)
			})

			t.Run("Then startup rejects the document with the expected category", func(t *testing.T) {
				if loadError == nil {
					t.Fatal("the loader accepts the invalid configuration document")
				}
				if testCase.wantUnknownKey && !errors.Is(loadError, conf.ErrUnknownKey) {
					t.Fatalf("configuration error is %v, want ErrUnknownKey", loadError)
				}
			})
		})
	}
}

func TestConfigurationSchema_DescribeExternalContract(t *testing.T) {
	t.Run("Scenario: An editor requests the configuration schema for the dependency graph", func(t *testing.T) {
		var output bytes.Buffer
		var commandError error
		var schema map[string]any

		t.Run("Given the standard configuration-schema command", func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			command := newRootCommand(logger)
			command.SetOut(&output)
			command.SetArgs([]string{"configuration-schema"})
			commandError = command.ExecuteContext(t.Context())
		})

		t.Run("When the decoder reads the generated document", func(t *testing.T) {
			if commandError != nil {
				return
			}
			commandError = json.Unmarshal(output.Bytes(), &schema)
		})

		if !t.Run("Then the command generates the schema", func(t *testing.T) {
			if commandError != nil {
				t.Fatalf("configuration-schema fails: %v", commandError)
			}
		}) {
			return
		}

		t.Run("And the schema exposes cache, server, and analysis properties", func(t *testing.T) {
			properties, ok := schema["properties"].(map[string]any)
			if !ok || properties["cache"] == nil ||
				properties["server"] == nil || properties["analysis"] == nil {
				t.Errorf("configuration schema has unexpected properties: %v", schema)
			}
		})
	})
}

func TestCacheConfiguration_ValidateModeAndTimeout(t *testing.T) {
	testCases := []struct {
		name          string
		configuration CacheConfiguration
		wantError     bool
	}{
		{
			name: "the automatic cache has a positive timeout",
			configuration: CacheConfiguration{
				Mode:           CacheModeAuto,
				RequestTimeout: time.Second,
			},
		},
		{
			name: "the server cache has a positive timeout",
			configuration: CacheConfiguration{
				Mode:           CacheModeServer,
				RequestTimeout: time.Second,
			},
		},
		{
			name: "the local source has a positive timeout",
			configuration: CacheConfiguration{
				Mode:           CacheModeLocal,
				RequestTimeout: time.Second,
			},
		},
		{
			name: "the minimum positive timeout is accepted",
			configuration: CacheConfiguration{
				Mode:           CacheModeAuto,
				RequestTimeout: time.Nanosecond,
			},
		},
		{
			name: "the cache mode is not in the closed set",
			configuration: CacheConfiguration{
				Mode:           "unknown",
				RequestTimeout: time.Second,
			},
			wantError: true,
		},
		{
			name: "the request timeout is zero",
			configuration: CacheConfiguration{
				Mode: CacheModeAuto,
			},
			wantError: true,
		},
	}
	for _, testCase := range testCases {
		t.Run("Scenario: "+testCase.name, func(t *testing.T) {
			var validationError error

			t.Run("Given a graph cache configuration", func(*testing.T) {})

			t.Run("When startup validates the configuration", func(*testing.T) {
				validationError = validateCacheConfiguration(testCase.configuration)
			})

			t.Run("Then validation returns the expected result", func(t *testing.T) {
				if (validationError != nil) != testCase.wantError {
					t.Errorf("validation error is %v, want error %t", validationError, testCase.wantError)
				}
			})
		})
	}
}
