package failure

import (
	"errors"
	"testing"
)

func TestError_ClassifyFailure(t *testing.T) {
	t.Run("Scenario: A failure has a category and an underlying cause", func(t *testing.T) {
		var cause error
		var classifiedError error

		t.Run("Given one unavailable dependency cause", func(*testing.T) {
			cause = errors.New("test dependency failure")
		})

		t.Run("When the caller creates a classified error", func(*testing.T) {
			classifiedError = NewError(ErrUnavailable, "load graph", cause)
		})

		t.Run("Then the error matches its category and cause", func(t *testing.T) {
			if !errors.Is(classifiedError, ErrUnavailable) {
				t.Fatalf("error is %v, want ErrUnavailable", classifiedError)
			}
			if !errors.Is(classifiedError, cause) {
				t.Fatalf("error is %v, want cause", classifiedError)
			}
		})

		t.Run("And the error retains its message", func(t *testing.T) {
			if classifiedError.Error() != "load graph: test dependency failure" {
				t.Errorf("error message is %q", classifiedError.Error())
			}
		})
	})
}

func TestError_UseCategoryAsDefaultMessage(t *testing.T) {
	t.Run("Scenario: A classified failure has no custom message", func(t *testing.T) {
		var classifiedError error

		t.Run("Given one validation category", func(*testing.T) {})

		t.Run("When the caller creates an error without a message", func(*testing.T) {
			classifiedError = NewError(ErrValidation, "", nil)
		})

		t.Run("Then the error uses the category message", func(t *testing.T) {
			if classifiedError.Error() != ErrValidation.Error() {
				t.Fatalf("error message is %q, want %q", classifiedError, ErrValidation)
			}
		})
	})
}

func TestError_UseFallbackMessage(t *testing.T) {
	t.Run("Scenario: A failure has no category, message, or cause", func(t *testing.T) {
		var unclassifiedError error

		t.Run("Given an empty error description", func(*testing.T) {})

		t.Run("When the caller creates the error", func(*testing.T) {
			unclassifiedError = NewError(nil, "", nil)
		})

		t.Run("Then the error uses the dependency graph fallback message", func(t *testing.T) {
			if unclassifiedError.Error() != "dependency graph error" {
				t.Fatalf("error message is %q", unclassifiedError)
			}
		})
	})
}

func TestError_IdentifyMissingEntity(t *testing.T) {
	t.Run("Scenario: A query cannot find one graph function", func(t *testing.T) {
		var notFoundError error
		var typedError *Error

		t.Run("Given one missing function identifier", func(*testing.T) {})

		t.Run("When the query creates a not-found error", func(*testing.T) {
			notFoundError = NewEntityNotFoundError(
				"dependency graph function",
				"sample.Run",
				nil,
			)
		})

		if !t.Run("Then the error matches the not-found category", func(t *testing.T) {
			if !errors.Is(notFoundError, ErrNotFound) {
				t.Fatalf("error is %v, want ErrNotFound", notFoundError)
			}
			if !errors.As(notFoundError, &typedError) {
				t.Fatalf("error type is %T, want *Error", notFoundError)
			}
		}) {
			return
		}

		t.Run("And the error retains the entity identity", func(t *testing.T) {
			if typedError.Entity != "dependency graph function" || typedError.ID != "sample.Run" {
				t.Errorf("error identity is entity=%q id=%v", typedError.Entity, typedError.ID)
			}
		})
	})
}
