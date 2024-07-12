package validate

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
)

type validator interface {
	Validate() error
}

// Validator is a validator middleware.
func Validator() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, receiveMsg, sendMsg func(msg any) error, info CallInfo) error {
			recv := func(req any) error {
				if v, ok := req.(validator); ok {
					if err := v.Validate(); err != nil {
						return errors.BadRequest("VALIDATOR", err.Error()).WithCause(err)
					}
				}
				return nil
			}

			return handler(ctx, recv, func(any) error { return nil }, info)
		}
	}
}
