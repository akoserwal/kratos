package middleware

import (
	"context"
)

// LegacyHandler defines the handler invoked by Middleware.
type LegacyHandler func(ctx context.Context, req interface{}) (interface{}, error)
type Handler func(ctx context.Context, receiveMsg func(any) error, sendMsg func(msg any) error) error

// Middleware is HTTP/gRPC transport middleware.
type Middleware func(Handler) Handler

// Chain returns a Middleware that specifies the chained handler for endpoint.
func Chain(m ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(m) - 1; i >= 0; i-- {
			next = m[i](next)
		}
		return next
	}
}
