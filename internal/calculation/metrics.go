// Package calculation provides deterministic strategic design metrics.
package calculation

import "math"

// Instability returns the ratio of efferent coupling to total coupling.
func Instability(afferentCoupling, efferentCoupling int) float64 {
	totalCoupling := afferentCoupling + efferentCoupling
	if totalCoupling == 0 {
		return 0
	}

	return float64(efferentCoupling) / float64(totalCoupling)
}

// Abstractness returns the ratio of abstract types to classified types.
func Abstractness(abstractTypes, concreteTypes int) float64 {
	classifiedTypes := abstractTypes + concreteTypes
	if classifiedTypes == 0 {
		return 0
	}

	return float64(abstractTypes) / float64(classifiedTypes)
}

// MainSequenceDistance returns the normalized distance from the main sequence.
func MainSequenceDistance(abstractness, instability float64) float64 {
	return math.Abs(abstractness + instability - 1)
}

// StableWithLowAbstraction reports whether a stable component has low abstraction.
func StableWithLowAbstraction(
	afferentCoupling int,
	instability float64,
	abstractness float64,
	maximumInstability float64,
	maximumAbstractness float64,
) bool {
	return afferentCoupling > 0 &&
		instability <= maximumInstability &&
		abstractness <= maximumAbstractness
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T15:56:45Z","module_hash":"929d526e01699dd32252f18303559b7afa235679a751e6c4480c90fcc6fe83f8","functions":[{"id":"func/Instability","name":"Instability","line":7,"end_line":14,"hash":"c1e8af3a7f2e0548c70e1accc36a764b66cc8b4a9726dc814414e8c5a7dcfde8"},{"id":"func/Abstractness","name":"Abstractness","line":17,"end_line":24,"hash":"755655807cfbac8b2dd7326091cec067377526d340b6f21ba2e608dd5e608637"},{"id":"func/MainSequenceDistance","name":"MainSequenceDistance","line":27,"end_line":29,"hash":"089346a5b0552ec432c02ee7704b6a892fb564b7d6382be1db592052533c434f"},{"id":"func/StableWithLowAbstraction","name":"StableWithLowAbstraction","line":32,"end_line":42,"hash":"c8e5a03042a6fb5b72b1d745fc311ed9ea978422e6e8bcfca7c191e2a341272b"}]}
// mutate4go-manifest-end
