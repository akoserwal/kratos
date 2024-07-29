package grpc

import (
	"context"

	"google.golang.org/grpc"
	grpcmd "google.golang.org/grpc/metadata"

	ic "github.com/go-kratos/kratos/v2/internal/context"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// unaryServerInterceptor is a gRPC unary server interceptor
func (s *Server) unaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, cancel := ic.Merge(ctx, s.baseCtx)
		defer cancel()
		md, _ := grpcmd.FromIncomingContext(ctx)
		replyHeader := grpcmd.MD{}
		tr := &Transport{
			operation:   info.FullMethod,
			reqHeader:   headerCarrier(md),
			replyHeader: headerCarrier(replyHeader),
		}
		if s.endpoint != nil {
			tr.endpoint = s.endpoint.String()
		}
		ctx = transport.NewServerContext(ctx, tr)
		if s.timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, s.timeout)
			defer cancel()
		}

		h := func(ctx context.Context, recv func(any) error, send func(msg any) error) error {
			err := recv(req)
			if err != nil {
				return err
			}

			reply, err := handler(ctx, req)

			if err != nil {
				return err
			}

			return send(reply)
		}

		if next := s.middleware.Match(tr.Operation()); len(next) > 0 {
			h = middleware.Chain(next...)(h)
		}

		var reply any
		err := h(ctx, func(a any) error { return nil }, func(msg any) error { reply = msg; return nil })

		if len(replyHeader) > 0 {
			_ = grpc.SetHeader(ctx, replyHeader)
		}
		return reply, err
	}
}

// wrappedStream is rewrite grpc stream's context
type wrappedStream struct {
	grpc.ServerStream
	ctx         context.Context
	postSendMsg func(any) error
	postRecvMsg func(any) error
}

func NewWrappedStream(ctx context.Context, stream grpc.ServerStream, postSendMsg func(any) error, postRecvMsg func(any) error) grpc.ServerStream {
	return &wrappedStream{
		ServerStream: stream,
		ctx:          ctx,
		postSendMsg:  postSendMsg,
		postRecvMsg:  postRecvMsg,
	}
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

func (w *wrappedStream) SendMsg(m any) error {
	if err := w.ServerStream.SendMsg(m); err != nil {
		return err
	}

	return w.postSendMsg(m)
}

func (w *wrappedStream) RecvMsg(m any) error {
	if err := w.ServerStream.RecvMsg(m); err != nil {
		return err
	}

	return w.postRecvMsg(m)
}

// streamServerInterceptor is a gRPC stream server interceptor
func (s *Server) streamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, cancel := ic.Merge(ss.Context(), s.baseCtx)
		defer cancel()
		md, _ := grpcmd.FromIncomingContext(ctx)
		replyHeader := grpcmd.MD{}
		ctx = transport.NewServerContext(ctx, &Transport{
			endpoint:    s.endpoint.String(),
			operation:   info.FullMethod,
			reqHeader:   headerCarrier(md),
			replyHeader: headerCarrier(replyHeader),
		})

		h := func(ctx context.Context, recv func(any) error, send func(msg any) error) error {
			ws := NewWrappedStream(ctx, ss, send, recv)
			return handler(srv, ws)
		}

		if next := s.middleware.Match(info.FullMethod); len(next) > 0 {
			h = middleware.Chain(next...)(h)
		}

		err := h(ctx, func(a any) error { return nil }, func(msg any) error { return nil })
		if len(replyHeader) > 0 {
			_ = grpc.SetHeader(ctx, replyHeader)
		}
		return err
	}
}
