package architecture

import "github.com/cgardev/goconduct/internal/library/foundationdomain"

func newValidationError(message string, cause error) error {
	return foundationdomain.NewError(foundationdomain.ErrValidation, message, cause)
}

func newUnavailableError(message string, cause error) error {
	return foundationdomain.NewError(foundationdomain.ErrUnavailable, message, cause)
}

func newDataIntegrityError(message string, cause error) error {
	return foundationdomain.NewError(foundationdomain.ErrDataIntegrity, message, cause)
}

func newInternalError(message string, cause error) error {
	return foundationdomain.NewError(foundationdomain.ErrInternal, message, cause)
}

func newBusinessRuleError(message string, cause error) error {
	return foundationdomain.NewError(foundationdomain.ErrBusinessRule, message, cause)
}
