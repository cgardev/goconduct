package crap

import (
	"testing"

	"github.com/cgardev/goconduct/pkg/plugin"
	"github.com/cgardev/goconduct/pkg/policy"
)

func samplePolicies() []policy.PathPolicy {
	return []policy.PathPolicy{{
		ID:      "sources",
		Include: []string{"internal/**"},
		Exclude: []string{"internal/protogen/**"},
		Thresholds: []policy.Threshold{{
			Metric:     metricCRAPScore,
			Comparison: policy.ComparisonMaximum,
			Value:      8,
			Severity:   plugin.SeverityError,
		}},
	}}
}

func TestDefaultConfigurationSelectsTheGoToolAndTheAgentLimit(t *testing.T) {
	configuration := DefaultConfiguration()

	if configuration.Command != "go" {
		t.Errorf("the default command is %q, want go", configuration.Command)
	}
	if len(configuration.Packages) != 1 || configuration.Packages[0] != "./..." {
		t.Errorf("the default packages are %v, want the complete module", configuration.Packages)
	}
	if configuration.MaximumScore != 8 {
		t.Errorf("the default maximum score is %v, want 8", configuration.MaximumScore)
	}
	if len(configuration.Policies) != 0 {
		t.Errorf("the default configuration holds the policies %+v", configuration.Policies)
	}
}

func TestCloneConfigurationCopiesEverySliceOfTheCaller(t *testing.T) {
	original := DefaultConfiguration()
	original.Policies = samplePolicies()

	clone := cloneConfiguration(original)
	original.Packages[0] = "./web/..."
	original.Policies[0].ID = "changed"
	original.Policies[0].Include[0] = "web/**"
	original.Policies[0].Exclude[0] = "web/dist/**"
	original.Policies[0].Thresholds[0].Value = 1

	if len(clone.Packages) != 1 || clone.Packages[0] != "./..." {
		t.Errorf("the clone follows the packages of the caller: %v", clone.Packages)
	}
	if len(clone.Policies) != 1 {
		t.Fatalf("the clone holds %d policies, want 1", len(clone.Policies))
	}
	cloned := clone.Policies[0]
	if cloned.ID != "sources" {
		t.Errorf("the cloned policy identifier is %q, want sources", cloned.ID)
	}
	if len(cloned.Include) != 1 || cloned.Include[0] != "internal/**" {
		t.Errorf("the clone follows the include patterns of the caller: %v", cloned.Include)
	}
	if len(cloned.Exclude) != 1 || cloned.Exclude[0] != "internal/protogen/**" {
		t.Errorf("the clone follows the exclude patterns of the caller: %v", cloned.Exclude)
	}
	if len(cloned.Thresholds) != 1 || cloned.Thresholds[0].Value != 8 {
		t.Errorf("the clone follows the thresholds of the caller: %+v", cloned.Thresholds)
	}
}

func TestNewEvaluatorKeepsACopyOfTheConfiguration(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.Policies = samplePolicies()

	evaluator, err := NewEvaluator(&profileRunner{}, configuration)
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	configuration.Packages[0] = "./web/..."
	configuration.Policies[0].ID = "changed"

	if evaluator.configuration.Packages[0] != "./..." {
		t.Errorf("the evaluator follows the packages of the caller: %v", evaluator.configuration.Packages)
	}
	if len(evaluator.configuration.Policies) != 1 {
		t.Fatalf("the evaluator holds %d policies, want 1", len(evaluator.configuration.Policies))
	}
	if evaluator.configuration.Policies[0].ID != "sources" {
		t.Errorf(
			"the evaluator policy identifier is %q, want sources",
			evaluator.configuration.Policies[0].ID,
		)
	}
}
