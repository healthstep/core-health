package middleware

import (
	"context"

	"github.com/helthtech/core-health/internal/obs"
	"github.com/porebric/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func GRPCUnaryAccessLog() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		ctx = obs.WithTrace(ctx)
		var remote string
		if pr, ok := peer.FromContext(ctx); ok && pr != nil {
			remote = pr.Addr.String()
		}
		reqS := protoToJSON(req)
		logger.Info(ctx, "grpc request", "method", info.FullMethod, "client", remote, "request", reqS)
		resp, err := h(ctx, req)
		respS := protoToJSON(resp)
		if err != nil {
			logger.Error(ctx, err, "grpc error", "method", info.FullMethod, "response", respS)
			return resp, err
		}
		logger.Info(ctx, "grpc response", "method", info.FullMethod, "response", respS)
		return resp, err
	}
}

func GRPCStreamAccessLog() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, h grpc.StreamHandler) error {
		ctx := obs.WithTrace(ss.Context())
		var remote string
		if pr, ok := peer.FromContext(ctx); ok && pr != nil {
			remote = pr.Addr.String()
		}
		logger.Info(ctx, "grpc stream start", "method", info.FullMethod, "client", remote)
		ws := &wrappedStream{ServerStream: ss, ctx: ctx}
		err := h(srv, ws)
		if err != nil {
			logger.Error(ctx, err, "grpc stream error", "method", info.FullMethod)
		} else {
			logger.Info(ctx, "grpc stream end", "method", info.FullMethod)
		}
		return err
	}
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

func protoToJSON(msg any) string {
	if msg == nil {
		return "null"
	}
	p, ok := msg.(proto.Message)
	if !ok {
		return "{}"
	}
	b, e := protojson.Marshal(p)
	if e != nil {
		return `{"_marshal_error":true}`
	}
	s := string(b)
	const max = 8000
	if len(s) > max {
		return s[:max] + `...truncated`
	}
	return s
}
