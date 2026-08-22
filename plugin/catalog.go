package plugin

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// Catalog composes evaluators from independently linked plugins.
type Catalog struct {
	mutex      sync.RWMutex
	evaluators map[string]Evaluator
}

// NewCatalog creates an empty evaluator catalog.
func NewCatalog() *Catalog {
	return &Catalog{evaluators: make(map[string]Evaluator)}
}

// Register adds one evaluator and rejects ambiguous names.
func (catalog *Catalog) Register(evaluator Evaluator) error {
	if evaluator == nil {
		return fmt.Errorf("evaluator is nil")
	}
	name := evaluator.Name()
	if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name {
		return fmt.Errorf("evaluator name %q is invalid", name)
	}
	catalog.mutex.Lock()
	defer catalog.mutex.Unlock()
	if _, duplicate := catalog.evaluators[name]; duplicate {
		return fmt.Errorf("evaluator name %q is duplicated", name)
	}
	catalog.evaluators[name] = evaluator
	return nil
}

// Names returns the evaluator names in deterministic order.
func (catalog *Catalog) Names() []string {
	catalog.mutex.RLock()
	defer catalog.mutex.RUnlock()
	names := make([]string, 0, len(catalog.evaluators))
	for name := range catalog.evaluators {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Evaluate runs the selected evaluators in deterministic order.
func (catalog *Catalog) Evaluate(
	ctx context.Context,
	names []string,
	request Request,
) ([]Report, error) {
	selected, err := catalog.selectEvaluators(names)
	if err != nil {
		return nil, err
	}
	reports := make([]Report, 0, len(selected))
	for _, evaluator := range selected {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		report, err := evaluator.Evaluate(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("evaluate plugin %q: %w", evaluator.Name(), err)
		}
		if report.Plugin != evaluator.Name() {
			return nil, fmt.Errorf(
				"evaluator %q returned report for %q",
				evaluator.Name(),
				report.Plugin,
			)
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func (catalog *Catalog) selectEvaluators(names []string) ([]Evaluator, error) {
	catalog.mutex.RLock()
	defer catalog.mutex.RUnlock()
	selectedNames := slices.Clone(names)
	if len(selectedNames) == 0 {
		selectedNames = make([]string, 0, len(catalog.evaluators))
		for name := range catalog.evaluators {
			selectedNames = append(selectedNames, name)
		}
	}
	slices.Sort(selectedNames)
	selectedNames = slices.Compact(selectedNames)
	selected := make([]Evaluator, 0, len(selectedNames))
	for _, name := range selectedNames {
		evaluator, available := catalog.evaluators[name]
		if !available {
			return nil, fmt.Errorf("evaluator %q is not registered", name)
		}
		selected = append(selected, evaluator)
	}
	return selected, nil
}
