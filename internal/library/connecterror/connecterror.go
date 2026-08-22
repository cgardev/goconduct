// Package connecterror translates classified domain errors into Connect errors.
package connecterror

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/cgardev/goconduct/internal/library/foundationdomain"
)

var categoryMappings = []struct {
	category error
	code     connect.Code
}{
	{foundationdomain.ErrValidation, connect.CodeInvalidArgument},
	{foundationdomain.ErrOutOfRange, connect.CodeOutOfRange},
	{foundationdomain.ErrNotFound, connect.CodeNotFound},
	{foundationdomain.ErrAlreadyExists, connect.CodeAlreadyExists},
	{foundationdomain.ErrUnauthenticated, connect.CodeUnauthenticated},
	{foundationdomain.ErrPermissionDenied, connect.CodePermissionDenied},
	{foundationdomain.ErrResourceConstraint, connect.CodeResourceExhausted},
	{foundationdomain.ErrBusinessRule, connect.CodeFailedPrecondition},
	{foundationdomain.ErrAborted, connect.CodeAborted},
	{foundationdomain.ErrUnavailable, connect.CodeUnavailable},
	{foundationdomain.ErrTimeout, connect.CodeDeadlineExceeded},
	{foundationdomain.ErrUnimplemented, connect.CodeUnimplemented},
}

// From converts one domain error into a safe Connect error.
func From(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	var connectError *connect.Error
	if errors.As(err, &connectError) {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return connect.NewError(connect.CodeCanceled, errors.New("request canceled"))
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return connect.NewError(connect.CodeDeadlineExceeded, errors.New("deadline exceeded"))
	}
	if code, category, classified := classify(err); classified {
		return connect.NewError(code, errors.New(clientMessage(err, category)))
	}
	slog.ErrorContext(ctx, "unclassified error at API boundary", slog.Any("error", err))
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

func classify(err error) (connect.Code, error, bool) {
	var domainError *foundationdomain.Error
	if errors.As(err, &domainError) {
		for _, mapping := range categoryMappings {
			if domainError.Category != nil && errors.Is(domainError.Category, mapping.category) {
				return mapping.code, mapping.category, true
			}
		}
		return connect.CodeInternal, nil, false
	}
	for _, mapping := range categoryMappings {
		if errors.Is(err, mapping.category) {
			return mapping.code, mapping.category, true
		}
	}
	return connect.CodeInternal, nil, false
}

func clientMessage(err error, category error) string {
	var domainError *foundationdomain.Error
	if errors.As(err, &domainError) && domainError.Message != "" {
		return domainError.Message
	}
	return category.Error()
}
