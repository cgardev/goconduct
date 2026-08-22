package configuration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cgardev/gokeel/conf"
)

func TestLoadCombinesArchitectureAndQualityConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".goconduct.json")
	payload := []byte(`{
  "analysis": {"repositoryRoot": "fixture"},
  "architecture": {
    "dependencies": {
      "productionDefault": "deny",
      "allow": [{
        "id": "applications-use-libraries",
        "from": {"roles": ["application"]},
        "to": {"roles": ["library"]},
        "reason": "Applications compose libraries."
      }]
    }
  },
  "quality": {
    "check": {"plugins": ["architecture", "crap"], "failOn": "warning"},
    "coverage": {"command": "custom-go", "packages": ["./internal/..."]},
    "crap": {"maximumScore": 6}
  }
}`)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write configuration: %v", err)
	}
	configuration, err := Load(path)
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}
	if configuration.Analysis.RepositoryRoot != "fixture" {
		t.Fatalf("repository root is %q", configuration.Analysis.RepositoryRoot)
	}
	if configuration.Quality.Coverage.Command != "custom-go" || configuration.Quality.CRAP.MaximumScore != 6 {
		t.Fatalf("unexpected quality configuration: %+v", configuration.Quality)
	}
	if configuration.Quality.Check.FailOn != FailureThresholdWarning {
		t.Fatalf("failure threshold is %q", configuration.Quality.Check.FailOn)
	}
	if configuration.Architecture.Dependencies.ProductionDefault != "deny" ||
		len(configuration.Architecture.Dependencies.Allow) != 1 {
		t.Fatalf("unexpected architecture configuration: %+v", configuration.Architecture)
	}
}

func TestSchemaDescribesCompleteConfiguration(t *testing.T) {
	payload, err := conf.GenerateSchema(ApplicationConfiguration{}, SchemaDefinition())
	if err != nil {
		t.Fatalf("generate schema: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	properties, ok := document["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties are %T", document["properties"])
	}
	for _, name := range []string{"analysis", "architecture", "cache", "quality", "server"} {
		if _, available := properties[name]; !available {
			t.Errorf("schema does not define %q", name)
		}
	}
}

func TestValidateRejectsUnknownAndDuplicatePlugins(t *testing.T) {
	configuration := Default()
	configuration.Quality.Check.Plugins = []string{"missing"}
	if err := Validate(configuration); err == nil {
		t.Fatal("expected unknown plugin error")
	}
	configuration.Quality.Check.Plugins = []string{"coverage", "coverage"}
	if err := Validate(configuration); err == nil {
		t.Fatal("expected duplicate plugin error")
	}
}
