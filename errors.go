package main

import "github.com/cgardev/goconduct/internal/failure"

func newValidationError(message string, cause error) error {
	return failure.NewError(failure.ErrValidation, message, cause)
}

func newUnavailableError(message string, cause error) error {
	return failure.NewError(failure.ErrUnavailable, message, cause)
}

func newDataIntegrityError(message string, cause error) error {
	return failure.NewError(failure.ErrDataIntegrity, message, cause)
}

func newInternalError(message string, cause error) error {
	return failure.NewError(failure.ErrInternal, message, cause)
}

func newBusinessRuleError(message string, cause error) error {
	return failure.NewError(failure.ErrBusinessRule, message, cause)
}
