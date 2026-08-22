package plugin

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/cgardev/goconduct/failure"
)

type catalogEvaluator struct {
	name  string
	err   error
	calls *[]string
}

func (evaluator catalogEvaluator) Name() string {
	return evaluator.name
}

func (evaluator catalogEvaluator) Evaluate(context.Context, Request) (Report, error) {
	*evaluator.calls = append(*evaluator.calls, evaluator.name)
	if evaluator.err != nil {
		return Report{}, evaluator.err
	}
	return NewReport(evaluator.name, nil, nil)
}

func TestCatalogComposesEvaluatorsDeterministically(t *testing.T) {
	catalog := NewCatalog()
	calls := make([]string, 0, 2)
	for _, evaluator := range []Evaluator{
		catalogEvaluator{name: "mutation", calls: &calls},
		catalogEvaluator{name: "coverage", calls: &calls},
	} {
		if err := catalog.Register(evaluator); err != nil {
			t.Fatalf("register evaluator: %v", err)
		}
	}
	reports, err := catalog.Evaluate(t.Context(), nil, Request{RepositoryRoot: "."})
	if err != nil {
		t.Fatalf("evaluate catalog: %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"coverage", "mutation"}) {
		t.Fatalf("evaluation order is %v", calls)
	}
	if got := []string{reports[0].Plugin, reports[1].Plugin}; !reflect.DeepEqual(got, calls) {
		t.Fatalf("report order is %v", got)
	}
}

func TestCatalogRejectsAmbiguousAndUnknownEvaluators(t *testing.T) {
	catalog := NewCatalog()
	calls := make([]string, 0)
	evaluator := catalogEvaluator{name: "coverage", calls: &calls}
	if err := catalog.Register(evaluator); err != nil {
		t.Fatalf("register evaluator: %v", err)
	}
	if err := catalog.Register(evaluator); err == nil {
		t.Fatal("expected duplicate evaluator error")
	}
	_, err := catalog.Evaluate(t.Context(), []string{"missing"}, Request{})
	if !errors.Is(err, failure.ErrValidation) {
		t.Fatalf("unknown evaluator error is %v", err)
	}
	var classifiedError *failure.Error
	if !errors.As(err, &classifiedError) || classifiedError.ID != "missing" {
		t.Fatalf("unknown evaluator error keeps identity %v", err)
	}
}

func TestCatalogNamesAndErrorsRemainStable(t *testing.T) {
	catalog := NewCatalog()
	calls := make([]string, 0)
	sentinel := errors.New("broken tool")
	if err := catalog.Register(catalogEvaluator{name: "coverage", err: sentinel, calls: &calls}); err != nil {
		t.Fatalf("register evaluator: %v", err)
	}
	if got := catalog.Names(); !reflect.DeepEqual(got, []string{"coverage"}) {
		t.Fatalf("catalog names are %v", got)
	}
	_, err := catalog.Evaluate(t.Context(), nil, Request{})
	if !errors.Is(err, sentinel) {
		t.Fatalf("catalog error is %v", err)
	}
}
