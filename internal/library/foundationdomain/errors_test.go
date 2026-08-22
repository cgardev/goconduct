package foundationdomain

import (
	"errors"
	"testing"
)

func TestError_ClassifyFailure(t *testing.T) {
	cause := errors.New("test dependency failure")
	classifiedError := NewError(ErrUnavailable, "load graph", cause)

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
			err:      NewError(ErrValidation, "", nil),
			expected: ErrValidation.Error(),
		},
		{
			name:     "an empty error uses the domain fallback",
			err:      NewError(nil, "", nil),
			expected: "domain error",
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
			err:      NewEntityNotFoundError("function", "sample.Run", nil),
			category: ErrNotFound,
		},
		{
			name:     "a duplicated entity",
			err:      NewDuplicateEntityError("function", "sample.Run", nil),
			category: ErrAlreadyExists,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if !errors.Is(testCase.err, testCase.category) {
				t.Fatalf("error is %v, want category %v", testCase.err, testCase.category)
			}
			var domainError *Error
			if !errors.As(testCase.err, &domainError) {
				t.Fatalf("error type is %T, want *Error", testCase.err)
			}
			if domainError.Entity != "function" || domainError.ID != "sample.Run" {
				t.Errorf("error identity is entity=%q id=%v", domainError.Entity, domainError.ID)
			}
		})
	}
}
