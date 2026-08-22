// Package foundationdomain defines stable error categories shared across application boundaries.
package foundationdomain

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
	// ErrBusinessRule indicates that an operation violates a business rule.
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
func (domainError *Error) Error() string {
	message := domainError.Message
	if message == "" && domainError.Category != nil {
		message = domainError.Category.Error()
	}
	if message == "" {
		message = "domain error"
	}
	if domainError.Err != nil {
		return message + ": " + domainError.Err.Error()
	}
	return message
}

// Unwrap exposes the category and cause for errors.Is and errors.As.
func (domainError *Error) Unwrap() []error {
	chain := make([]error, 0, 2)
	if domainError.Category != nil {
		chain = append(chain, domainError.Category)
	}
	if domainError.Err != nil {
		chain = append(chain, domainError.Err)
	}
	return chain
}

// NewError creates a classified domain error.
func NewError(category error, message string, cause error) error {
	return &Error{Category: category, Message: message, Err: cause}
}

// NewEntityNotFoundError reports one missing entity.
func NewEntityNotFoundError(entity string, identifier any, cause error) error {
	return &Error{
		Category: ErrNotFound,
		Message:  fmt.Sprintf("%s with identifier %v not found", entity, identifier),
		Entity:   entity,
		ID:       identifier,
		Err:      cause,
	}
}

// NewDuplicateEntityError reports one duplicated entity identity.
func NewDuplicateEntityError(entity string, identifier any, cause error) error {
	return &Error{
		Category: ErrAlreadyExists,
		Message:  fmt.Sprintf("%s with identifier %v already exists", entity, identifier),
		Entity:   entity,
		ID:       identifier,
		Err:      cause,
	}
}
