package main

import (
	"log/slog"

	"connectrpc.com/connect"
	"connectrpc.com/validate"

	"github.com/cgardev/goconduct/internal/library/connectlog"
)

func buildRequestChain(logger *slog.Logger) []connect.Interceptor {
	return []connect.Interceptor{
		connectlog.NewInterceptor(logger),
		validate.NewInterceptor(),
	}
}
