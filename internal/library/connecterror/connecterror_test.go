package connecterror

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	"github.com/cgardev/goconduct/internal/library/foundationdomain"
)

func TestFromMapsClassifiedErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode connect.Code
	}{
		{name: "validation", err: foundationdomain.ErrValidation, wantCode: connect.CodeInvalidArgument},
		{name: "missing entity", err: foundationdomain.ErrNotFound, wantCode: connect.CodeNotFound},
		{name: "unavailable dependency", err: foundationdomain.ErrUnavailable, wantCode: connect.CodeUnavailable},
		{name: "canceled context", err: context.Canceled, wantCode: connect.CodeCanceled},
		{name: "expired context", err: context.DeadlineExceeded, wantCode: connect.CodeDeadlineExceeded},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if code := connect.CodeOf(From(t.Context(), test.err)); code != test.wantCode {
				t.Fatalf("Connect code is %s, want %s", code, test.wantCode)
			}
		})
	}
}

func TestFromUsesSafeDomainMessageAndSanitizesUnknownErrors(t *testing.T) {
	domainError := foundationdomain.NewError(
		foundationdomain.ErrBusinessRule,
		"configured policy rejects this dependency",
		errors.New("private implementation detail"),
	)
	converted := From(t.Context(), domainError)
	if converted.Error() != "failed_precondition: configured policy rejects this dependency" {
		t.Fatalf("classified error is %q", converted.Error())
	}

	unknown := From(t.Context(), errors.New("secret path"))
	if connect.CodeOf(unknown) != connect.CodeInternal || unknown.Error() != "internal: internal error" {
		t.Fatalf("unknown error is %q", unknown.Error())
	}
}

func TestFromPreservesExistingConnectErrors(t *testing.T) {
	want := connect.NewError(connect.CodeResourceExhausted, errors.New("limit reached"))
	if got := From(t.Context(), want); !errors.Is(got, want) {
		t.Fatalf("Connect error is %v", got)
	}
}
