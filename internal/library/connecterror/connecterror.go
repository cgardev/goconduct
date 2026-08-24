// Package connecterror translates classified failures into safe Connect errors.
package connecterror

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/cgardev/goconduct/pkg/failure"
)

var categoryMappings = []struct {
	category error
	code     connect.Code
}{
	{failure.ErrValidation, connect.CodeInvalidArgument},
	{failure.ErrOutOfRange, connect.CodeOutOfRange},
	{failure.ErrNotFound, connect.CodeNotFound},
	{failure.ErrAlreadyExists, connect.CodeAlreadyExists},
	{failure.ErrUnauthenticated, connect.CodeUnauthenticated},
	{failure.ErrPermissionDenied, connect.CodePermissionDenied},
	{failure.ErrResourceConstraint, connect.CodeResourceExhausted},
	{failure.ErrBusinessRule, connect.CodeFailedPrecondition},
	{failure.ErrAborted, connect.CodeAborted},
	{failure.ErrUnavailable, connect.CodeUnavailable},
	{failure.ErrTimeout, connect.CodeDeadlineExceeded},
	{failure.ErrUnimplemented, connect.CodeUnimplemented},
}

// From converts one classified failure into a safe Connect error.
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
	var classifiedError *failure.Error
	if errors.As(err, &classifiedError) {
		for _, mapping := range categoryMappings {
			if classifiedError.Category != nil && errors.Is(classifiedError.Category, mapping.category) {
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
	var classifiedError *failure.Error
	if errors.As(err, &classifiedError) && classifiedError.Message != "" {
		return classifiedError.Message
	}
	return category.Error()
}
