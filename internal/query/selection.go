package query

import (
	"slices"

	"github.com/cgardev/goconduct/pkg/failure"
)

// Selection contains the number of matches and the selected values.
type Selection[Value any] struct {
	Matched int
	Values  []Value
}

// Select filters, orders, and limits one value collection.
func Select[Value any](
	values []Value,
	matches func(Value) bool,
	compare func(Value, Value) int,
	limit int,
) Selection[Value] {
	selected := make([]Value, 0, len(values))
	for _, value := range values {
		if matches != nil && !matches(value) {
			continue
		}
		selected = append(selected, value)
	}
	if compare != nil {
		slices.SortStableFunc(selected, compare)
	}
	matched := len(selected)
	if limit > 0 {
		selected = selected[:min(limit, len(selected))]
	}
	return Selection[Value]{Matched: matched, Values: selected}
}

// ValidateLimit accepts zero as unlimited and rejects negative query limits.
func ValidateLimit(limit int) error {
	if limit < 0 {
		return failure.Validation("query limit must not be negative", nil)
	}
	return nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-23T21:34:41Z","module_hash":"ef9c9952f2db7739c9518784cb2d917408db2ed7490f13bad70f90001d1e3e1b","functions":[{"id":"func/Select","name":"Select","line":16,"end_line":37,"hash":"9544626951ac9af861bd3e9221de1b571b9fdf9926e5da9fd90a40a7245ef856"},{"id":"func/ValidateLimit","name":"ValidateLimit","line":40,"end_line":45,"hash":"968a50edc0afff2aebdfede4305a5222d24cc78087ad48c0db8d60894d4adba5"}]}
// mutate4go-manifest-end
