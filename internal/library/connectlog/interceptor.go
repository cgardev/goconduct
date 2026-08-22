// Package connectlog provides structured request logging for Connect RPC.
package connectlog

import (
	"cmp"
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
)

// NewInterceptor logs the procedure, status code, and duration of unary calls.
func NewInterceptor(logger *slog.Logger) connect.Interceptor {
	logger = cmp.Or(logger, slog.Default())
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			started := time.Now()
			response, err := next(ctx, request)
			attributes := []slog.Attr{
				slog.String("rpc.procedure", request.Spec().Procedure),
				slog.Duration("rpc.duration", time.Since(started)),
			}
			if err != nil {
				code := connect.CodeOf(err)
				attributes = append(attributes, slog.Any("error", err))
				attributes = append(attributes, slog.String("rpc.code", code.String()))
				logger.LogAttrs(ctx, levelForCode(code), "RPC failed", attributes...)
				return response, err
			}
			attributes = append(attributes, slog.String("rpc.code", "ok"))
			logger.LogAttrs(ctx, slog.LevelInfo, "RPC handled", attributes...)
			return response, nil
		}
	})
}

func levelForCode(code connect.Code) slog.Level {
	switch code {
	case connect.CodeInternal, connect.CodeUnknown, connect.CodeDataLoss, connect.CodeUnavailable:
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}
