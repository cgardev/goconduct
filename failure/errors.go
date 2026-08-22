// Package failure defines the stable error categories shared by every goconduct package.
// A caller classifies one failure with errors.Is and reads its context with errors.As.
// Public plugins return classified failures so external code can react to the category
// instead of the message text.
package failure

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound indicates that the requested entity does not exist.
	ErrNotFound = errors.New("entity not found")
	// ErrAlreadyExists indicates that an entity with the same identity exists.
	ErrAlreadyExists = errors.New("entity already exists")
	// ErrValidation indicates that input failed validation.
	ErrValidation = errors.New("validation failed")
	// ErrUnauthenticated indicates that caller authentication failed.
	ErrUnauthenticated = errors.New("authentication failed")
	// ErrPermissionDenied indicates that the caller lacks permission.
	ErrPermissionDenied = errors.New("permission denied")
	// ErrResourceConstraint indicates that a resource limit was violated.
	ErrResourceConstraint = errors.New("resource constraint violated")
	// ErrBusinessRule indicates that an operation violates a rule of the product.
	ErrBusinessRule = errors.New("business rule violated")
	// ErrAborted indicates that an operation stopped before completion.
	ErrAborted = errors.New("operation aborted")
	// ErrOutOfRange indicates that a value is outside its allowed range.
	ErrOutOfRange = errors.New("value out of range")
	// ErrInternal indicates an unexpected internal failure.
	ErrInternal = errors.New("internal error")
	// ErrUnavailable indicates that a required dependency is unavailable.
	ErrUnavailable = errors.New("required dependency unavailable")
	// ErrTimeout indicates that an operation exceeded its deadline.
	ErrTimeout = errors.New("deadline exceeded")
	// ErrUnimplemented indicates that a requested capability is unavailable.
	ErrUnimplemented = errors.New("feature not implemented")
	// ErrDataIntegrity indicates inconsistent or corrupted data.
	ErrDataIntegrity = errors.New("data integrity violation")
	// ErrUnknown indicates a failure without a more specific category.
	ErrUnknown = errors.New("unknown error")
)

// Error retains one stable category, optional entity context, and an underlying cause.
type Error struct {
	Category error
	Message  string
	Entity   string
	ID       any
	Err      error
}

// Error returns the configured message and underlying cause.
func (classified *Error) Error() string {
	message := classified.Message
	if message == "" && classified.Category != nil {
		message = classified.Category.Error()
	}
	if message == "" {
		message = "unclassified failure"
	}
	if classified.Err != nil {
		return message + ": " + classified.Err.Error()
	}
	return message
}

// Unwrap exposes the category and cause for errors.Is and errors.As.
func (classified *Error) Unwrap() []error {
	chain := make([]error, 0, 2)
	if classified.Category != nil {
		chain = append(chain, classified.Category)
	}
	if classified.Err != nil {
		chain = append(chain, classified.Err)
	}
	return chain
}

// New creates one classified failure.
func New(category error, message string, cause error) error {
	return &Error{Category: category, Message: message, Err: cause}
}

// NotFound reports one missing entity and keeps its identity.
func NotFound(entity string, identifier any, cause error) error {
	return newEntityError(ErrNotFound, entity, identifier, "not found", cause)
}

// Duplicate reports one duplicated entity identity in a live registry.
func Duplicate(entity string, identifier any, cause error) error {
	return newEntityError(ErrAlreadyExists, entity, identifier, "already exists", cause)
}

// newEntityError keeps the identity and the condition of one entity failure.
func newEntityError(
	category error,
	entity string,
	identifier any,
	condition string,
	cause error,
) error {
	return &Error{
		Category: category,
		Message:  fmt.Sprintf("%s with identifier %v %s", entity, identifier, condition),
		Entity:   entity,
		ID:       identifier,
		Err:      cause,
	}
}

// Validation reports rejected input, configuration, or evidence.
func Validation(message string, cause error) error {
	return New(ErrValidation, message, cause)
}

// Unavailable reports one failed external dependency.
func Unavailable(message string, cause error) error {
	return New(ErrUnavailable, message, cause)
}

// DataIntegrity reports inconsistent or malformed external data.
func DataIntegrity(message string, cause error) error {
	return New(ErrDataIntegrity, message, cause)
}

// BusinessRule reports a state or rule of the product that rejects the operation.
func BusinessRule(message string, cause error) error {
	return New(ErrBusinessRule, message, cause)
}

// Internal reports a failure that the code invariants must prevent.
func Internal(message string, cause error) error {
	return New(ErrInternal, message, cause)
}
