package loc

import (
	"errors"
	"testing"

	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/policy"
)

func TestDefaultConfigurationSelectsGoSourcesAndGeneratedConventions(t *testing.T) {
	configuration := DefaultConfiguration()

	if len(configuration.Selection.Paths) != 1 || configuration.Selection.Paths[0] != "." {
		t.Errorf("the default paths are %v", configuration.Selection.Paths)
	}
	if len(configuration.Selection.Exclude) == 0 || len(configuration.Generated.PathPatterns) == 0 {
		t.Errorf("the default configuration is incomplete: %+v", configuration)
	}
	if !configuration.Generated.StandardMarker {
		t.Error("the default configuration ignores the standard generated marker")
	}
}

func TestNewEvaluatorRejectsInvalidSelectionAndGeneratedPatterns(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Configuration)
	}{
		{
			name: "an empty root list",
			change: func(configuration *Configuration) {
				configuration.Selection.Paths = nil
			},
		},
		{
			name: "an empty root",
			change: func(configuration *Configuration) {
				configuration.Selection.Paths = []string{" "}
			},
		},
		{
			name: "an invalid include pattern",
			change: func(configuration *Configuration) {
				configuration.Selection.Include = []string{"../internal/**"}
			},
		},
		{
			name: "an invalid generated path pattern",
			change: func(configuration *Configuration) {
				configuration.Generated.PathPatterns = []string{`internal\protogen`}
			},
		},
		{
			name: "an invalid handwritten path pattern",
			change: func(configuration *Configuration) {
				configuration.Generated.ForceHandwrittenPaths = []string{`internal\manual`}
			},
		},
		{
			name: "an empty generated header expression",
			change: func(configuration *Configuration) {
				configuration.Generated.HeaderPatterns = []string{" "}
			},
		},
		{
			name: "an invalid generated header expression",
			change: func(configuration *Configuration) {
				configuration.Generated.HeaderPatterns = []string{"["}
			},
		},
		{
			name: "an invalid path policy",
			change: func(configuration *Configuration) {
				configuration.Policies = []policy.PathPolicy{{ID: " "}}
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			configuration := DefaultConfiguration()
			testCase.change(&configuration)

			_, err := NewEvaluator(configuration)

			if !errors.Is(err, failure.ErrValidation) {
				t.Errorf("the constructor reports %v, want a validation failure", err)
			}
		})
	}
}

func TestNewEvaluatorDefensivelyCopiesConfiguration(t *testing.T) {
	configuration := DefaultConfiguration()
	evaluator, err := NewEvaluator(configuration)
	if err != nil {
		t.Fatalf("create LOC evaluator: %v", err)
	}

	configuration.Selection.Paths[0] = "changed"
	configuration.Selection.Include[0] = "changed/**"
	configuration.Generated.PathPatterns[0] = "changed/**"

	if evaluator.configuration.Selection.Paths[0] != "." ||
		evaluator.configuration.Selection.Include[0] != "**/*.go" ||
		evaluator.configuration.Generated.PathPatterns[0] == "changed/**" {
		t.Errorf("caller mutation changed the evaluator: %+v", evaluator.configuration)
	}
}
