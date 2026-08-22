package failure

import (
	"errors"
	"testing"
)

func TestError_ClassifyFailure(t *testing.T) {
	cause := errors.New("test dependency failure")
	classifiedError := New(ErrUnavailable, "load graph", cause)

	if !errors.Is(classifiedError, ErrUnavailable) {
		t.Fatalf("error is %v, want ErrUnavailable", classifiedError)
	}
	if !errors.Is(classifiedError, cause) {
		t.Fatalf("error is %v, want cause", classifiedError)
	}
	if classifiedError.Error() != "load graph: test dependency failure" {
		t.Errorf("error message is %q", classifiedError.Error())
	}
}

func TestError_UseFallbackMessages(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "a category supplies its message",
			err:      New(ErrValidation, "", nil),
			expected: ErrValidation.Error(),
		},
		{
			name:     "an empty failure uses the package fallback",
			err:      New(nil, "", nil),
			expected: "unclassified failure",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.err.Error() != testCase.expected {
				t.Errorf("error message is %q, want %q", testCase.err, testCase.expected)
			}
		})
	}
}

func TestError_PreserveEntityIdentity(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		category error
	}{
		{
			name:     "a missing entity",
			err:      NotFound("function", "sample.Run", nil),
			category: ErrNotFound,
		},
		{
			name:     "a duplicated entity",
			err:      Duplicate("function", "sample.Run", nil),
			category: ErrAlreadyExists,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if !errors.Is(testCase.err, testCase.category) {
				t.Fatalf("error is %v, want category %v", testCase.err, testCase.category)
			}
			var classifiedError *Error
			if !errors.As(testCase.err, &classifiedError) {
				t.Fatalf("error type is %T, want *Error", testCase.err)
			}
			if classifiedError.Entity != "function" || classifiedError.ID != "sample.Run" {
				t.Errorf("error identity is entity=%q id=%v", classifiedError.Entity, classifiedError.ID)
			}
		})
	}
}

func TestCategoryConstructors_SelectOneCategory(t *testing.T) {
	cause := errors.New("underlying cause")
	testCases := []struct {
		name     string
		err      error
		category error
	}{
		{name: "validation", err: Validation("reject input", cause), category: ErrValidation},
		{name: "unavailable", err: Unavailable("read file", cause), category: ErrUnavailable},
		{name: "data integrity", err: DataIntegrity("decode report", cause), category: ErrDataIntegrity},
		{name: "business rule", err: BusinessRule("threshold exceeded", cause), category: ErrBusinessRule},
		{name: "internal", err: Internal("encode value", cause), category: ErrInternal},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if !errors.Is(testCase.err, testCase.category) {
				t.Fatalf("error is %v, want category %v", testCase.err, testCase.category)
			}
			if !errors.Is(testCase.err, cause) {
				t.Errorf("error %v does not preserve its cause", testCase.err)
			}
		})
	}
}

func TestError_SeparateDistinctCategories(t *testing.T) {
	classifiedError := Validation("reject input", nil)

	if errors.Is(classifiedError, ErrNotFound) {
		t.Errorf("error %v matches an unrelated category", classifiedError)
	}
}
