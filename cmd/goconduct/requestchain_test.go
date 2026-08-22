package main

import (
	"io"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
)

func TestRequestChainContainsSharedInterceptors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	chain := buildRequestChain(logger)
	if len(chain) != 2 {
		t.Fatalf("request interceptor count is %d", len(chain))
	}
	for index, interceptor := range chain {
		if interceptor == nil {
			t.Fatalf("request interceptor %d is nil", index)
		}
		_ = connect.WithInterceptors(interceptor)
	}
}
