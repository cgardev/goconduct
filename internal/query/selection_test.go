package query

import (
	"cmp"
	"errors"
	"slices"
	"testing"

	"github.com/cgardev/goconduct/pkg/failure"
)

func TestSelectFiltersOrdersAndLimitsValues(t *testing.T) {
	values := []int{4, 1, 3, 2}

	selection := Select(
		values,
		func(value int) bool { return value%2 == 0 },
		func(left, right int) int { return cmp.Compare(right, left) },
		1,
	)

	if selection.Matched != 2 {
		t.Errorf("the selection matches %d values, want 2", selection.Matched)
	}
	if !slices.Equal(selection.Values, []int{4}) {
		t.Errorf("the selection returns %v, want [4]", selection.Values)
	}
	if !slices.Equal(values, []int{4, 1, 3, 2}) {
		t.Errorf("the selection mutates its input: %v", values)
	}
}

func TestSelectSupportsOptionalOperationsAndUnlimitedResults(t *testing.T) {
	values := []string{"first", "second"}

	selection := Select[string](values, nil, nil, 0)

	if selection.Matched != 2 || !slices.Equal(selection.Values, values) {
		t.Errorf("the unlimited selection is %+v", selection)
	}
}

func TestValidateLimitRejectsNegativeValues(t *testing.T) {
	if err := ValidateLimit(0); err != nil {
		t.Errorf("the unlimited query fails validation: %v", err)
	}
	if err := ValidateLimit(1); err != nil {
		t.Errorf("the limited query fails validation: %v", err)
	}
	if err := ValidateLimit(-1); !errors.Is(err, failure.ErrValidation) {
		t.Errorf("the negative query reports %v, want a validation failure", err)
	}
}
