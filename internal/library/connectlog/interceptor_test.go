package connectlog

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"
)

func TestInterceptorLogsSuccessfulAndFailedUnaryCalls(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	interceptor := NewInterceptor(logger)
	request := connect.NewRequest(&struct{}{})

	success := interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(&struct{}{}), nil
	})
	if _, err := success(t.Context(), request); err != nil {
		t.Fatalf("handle successful RPC: %v", err)
	}

	failure := interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("offline"))
	})
	if _, err := failure(t.Context(), request); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("handle failed RPC: %v", err)
	}

	logs := output.String()
	for _, fragment := range []string{"RPC handled", "RPC failed", "rpc.code=ok", "rpc.code=unavailable"} {
		if !strings.Contains(logs, fragment) {
			t.Fatalf("logs do not contain %q: %s", fragment, logs)
		}
	}
}

func TestLevelForCodeSeparatesServerAndClientFailures(t *testing.T) {
	if levelForCode(connect.CodeInternal) != slog.LevelError {
		t.Fatal("internal errors must use error level")
	}
	if levelForCode(connect.CodeInvalidArgument) != slog.LevelWarn {
		t.Fatal("client errors must use warning level")
	}
}
