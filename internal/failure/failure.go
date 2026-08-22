// Package failure defines the dependency graph error categories.
// It keeps the command independent from shared project libraries.
package failure

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound indicates that a requested graph resource does not exist.
	ErrNotFound = errors.New("dependency graph resource not found")
	// ErrValidation indicates that command input or configuration is invalid.
	ErrValidation = errors.New("dependency graph validation failed")
	// ErrBusinessRule indicates that an architecture policy rejects a graph.
	ErrBusinessRule = errors.New("dependency graph business rule violated")
	// ErrInternal indicates an unexpected calculation or encoding failure.
	ErrInternal = errors.New("dependency graph internal error")
	// ErrUnavailable indicates that a required file, cache, or transport is unavailable.
	ErrUnavailable = errors.New("dependency graph dependency unavailable")
	// ErrDataIntegrity indicates incompatible or incomplete graph data.
	ErrDataIntegrity = errors.New("dependency graph data integrity violation")
)

// Error classifies one failure and retains its optional cause and resource identity.
type Error struct {
	Category error
	Message  string
	Entity   string
	ID       any
	Err      error
}

// Error returns the failure message and its cause.
func (failure *Error) Error() string {
	message := failure.Message
	if message == "" && failure.Category != nil {
		message = failure.Category.Error()
	}
	if message == "" {
		message = "dependency graph error"
	}
	if failure.Err != nil {
		return message + ": " + failure.Err.Error()
	}
	return message
}

// Unwrap returns the category and cause for errors.Is and errors.As.
func (failure *Error) Unwrap() []error {
	var chain []error
	if failure.Category != nil {
		chain = append(chain, failure.Category)
	}
	if failure.Err != nil {
		chain = append(chain, failure.Err)
	}
	return chain
}

// NewError creates a classified dependency graph error.
func NewError(category error, message string, cause error) error {
	return &Error{Category: category, Message: message, Err: cause}
}

// NewEntityNotFoundError reports one missing graph resource.
func NewEntityNotFoundError(entity string, identifier any, cause error) error {
	return &Error{
		Category: ErrNotFound,
		Message:  fmt.Sprintf("%s with identifier %v not found", entity, identifier),
		Entity:   entity,
		ID:       identifier,
		Err:      cause,
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-22T07:15:59Z","module_hash":"d3a6803cf3d7c62b9a04eddda85b120ff768aff4de0d95e4e0ae80107e5796c9","functions":[{"id":"func/Error.Error","name":"Error.Error","line":35,"end_line":47,"hash":"e7d292fa7be5a1206340047734e4d46c3f30c4aff54d366d97e36ea0541b1081"},{"id":"func/Error.Unwrap","name":"Error.Unwrap","line":50,"end_line":59,"hash":"d63bc012d3832343bb88baa72c213ef190f0601deaa6075c4ea0c38b09bef9f5"},{"id":"func/NewError","name":"NewError","line":62,"end_line":64,"hash":"d1c078264534ec5bb7f7ad129703ce150b355bd3472aac9a133bf7d4da2a58e8"},{"id":"func/NewEntityNotFoundError","name":"NewEntityNotFoundError","line":67,"end_line":75,"hash":"539a03c78bcad41b8c4d241a1ae7b05d081f5c2e382da53f7727f2fdc9a3a4c0"}]}
// mutate4go-manifest-end
