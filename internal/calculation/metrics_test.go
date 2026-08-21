package calculation

import (
	"math"
	"testing"
)

func TestMetrics_CalculateStrategicValues(t *testing.T) {
	t.Run("Scenario: Coupling and type counts define strategic metrics", func(t *testing.T) {
		var instability float64
		var abstractness float64
		var distance float64
		var stableWithLowAbstraction bool

		t.Run("Given two incoming dependencies, one outgoing dependency, and four types", func(*testing.T) {})

		t.Run("When the calculation package calculates each strategic metric", func(*testing.T) {
			instability = Instability(2, 1)
			abstractness = Abstractness(1, 3)
			distance = MainSequenceDistance(abstractness, instability)
			stableWithLowAbstraction = StableWithLowAbstraction(1, 0.2, 0.2, 0.2, 0.2)
		})

		t.Run("Then the calculation package returns the expected ratios", func(t *testing.T) {
			if math.Abs(instability-1.0/3.0) > 1e-12 {
				t.Errorf("instability is %v", instability)
			}
			if abstractness != 0.25 || math.Abs(distance-5.0/12.0) > 1e-12 {
				t.Errorf("abstractness is %v and distance is %v", abstractness, distance)
			}
		})

		t.Run("And the inclusive limits identify a stable component with low abstraction", func(t *testing.T) {
			if !stableWithLowAbstraction {
				t.Error("the limits do not identify a stable component with low abstraction")
			}
		})
	})
}

func TestMetrics_HandleUnclassifiedValues(t *testing.T) {
	t.Run("Scenario: A component has no dependency or type data", func(t *testing.T) {
		var instability float64
		var abstractness float64
		var stableWithLowAbstraction bool

		t.Run("Given zero coupling and zero classified types", func(*testing.T) {})

		t.Run("When the calculation package calculates the metrics", func(*testing.T) {
			instability = Instability(0, 0)
			abstractness = Abstractness(0, 0)
			stableWithLowAbstraction = StableWithLowAbstraction(0, 0, 0, 0.2, 0.2)
		})

		t.Run("Then the ratios use their deterministic zero values", func(t *testing.T) {
			if instability != 0 || abstractness != 0 {
				t.Errorf("unexpected zero-data metrics: instability=%v abstractness=%v", instability, abstractness)
			}
		})

		t.Run("And an isolated component does not meet the classification", func(t *testing.T) {
			if stableWithLowAbstraction {
				t.Error("an isolated component meets the stable low-abstraction classification")
			}
		})
	})
}
