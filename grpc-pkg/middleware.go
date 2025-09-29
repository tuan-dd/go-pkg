package gRPC

import (
	"context"
	"runtime/debug"

	"github.com/tuan-dd/go-common/constants"
	"github.com/tuan-dd/go-common/request"
	"github.com/tuan-dd/go-common/response"
	typeCustom "github.com/tuan-dd/go-common/type-custom"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	// OpenTelemetry attributes for request id tagging
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type (
	UnaryHandler func(context.Context, any) (any, *response.AppError)
	Middleware   func(customLogger, *grpc.UnaryServerInfo, grpc.UnaryHandler) grpc.UnaryHandler

	customLogger interface {
		typeCustom.LoggerWithAlert
		typeCustom.LoggerInterceptor
	}
)

var extractorPkg = NewExtractor()

// Middleware is a function that wraps a gRPC UnaryHandler and returns a new UnaryHandler.
// It can be used to implement various middleware functionalities such as logging, authentication,
// error handling, etc.

// ChainUnary creates a gRPC UnaryServerInterceptor that chains multiple middleware functions.
// The middlewares are executed in reverse order (last middleware specified is executed first).
// This allows you to compose multiple middleware functions into a single interceptor.
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(ChainUnary(
//	        LoggingMiddleware,
//	        AuthMiddleware,
//	        // other middlewares...
//	    )),
//	)
func ChainUnary(logger customLogger, middlewares ...Middleware) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		chain := handler
		if len(middlewares) > 0 {
			for i := len(middlewares) - 1; i >= 0; i-- {
				chain = middlewares[i](logger, info, chain)
			}
		}
		return chain(ctx, req)
	}
}

func RequestContext(_ customLogger, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) grpc.UnaryHandler {
	return func(ctx context.Context, req any) (any, error) {
		userID, _ := extractorPkg.GetUserID(ctx)
		reqCtx := &request.ReqContext{
			CID:              extractorPkg.GetRequestID(ctx),
			RequestTimestamp: extractorPkg.GetRequestTimestamp(ctx),
			IP:               extractorPkg.GetXForwardedFor(ctx),
			UserInfo: &request.UserInfo[any]{
				ID: userID,
			},
		}

		ctx = context.WithValue(ctx, constants.REQUEST_CONTEXT_KEY, reqCtx)
		ctx = context.WithValue(ctx, constants.REQUEST_ID_KEY, reqCtx.CID)

		// Add request id as span attributes for faster querying
		if span := trace.SpanFromContext(ctx); span.IsRecording() && reqCtx.CID != "" {
			span.SetAttributes(
				attribute.String("cid", reqCtx.CID),
			)
		}

		return handler(ctx, req)
	}
}

func GrpcLogInterceptor(log customLogger, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) grpc.UnaryHandler {
	return func(ctx context.Context, req any) (any, error) {
		reqCtx := request.GetReqCtx(ctx)

		log.ReqServerLogger(reqCtx, info.FullMethod, info.FullMethod)

		resp, err := handler(ctx, req)
		errApp := response.ConvertError(err)
		statusCode := response.Success

		if errApp != nil && errApp.Code >= 9000 {
			if status.Code(err) == codes.Unimplemented {
				log.ResServerLogger(reqCtx, uint(statusCode), nil)
				return resp, nil
			}
			statusCode = errApp.Code
			log.ResServerLogger(reqCtx, uint(statusCode), errApp)
		} else {
			log.ResServerLogger(reqCtx, uint(statusCode), nil)
		}

		return resp, errApp.Wrap()
	}
}

func RecoverMiddleware(log customLogger, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) grpc.UnaryHandler {
	return func(ctx context.Context, req any) (any, error) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorPanic(request.GetReqCtx(ctx), info.FullMethod, r, string(debug.Stack()))
			}
		}()
		return handler(ctx, req)
	}
}

type ValidateError interface {
	Validate() error
}

func ValidateMiddleware(log customLogger, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) grpc.UnaryHandler {
	return func(ctx context.Context, req any) (any, error) {
		if v, ok := req.(ValidateError); ok {
			if err := v.Validate(); err != nil {
				return nil, response.QueryInvalidErr(err.Error())
			}
		}
		return handler(ctx, req)
	}
}
