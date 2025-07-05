package gRPC

import (
	"context"
	"runtime/debug"

	appLogger "github.com/tuan-dd/go-pkg/app-logger"
	"github.com/tuan-dd/go-pkg/common"
	"github.com/tuan-dd/go-pkg/common/constants"
	"github.com/tuan-dd/go-pkg/common/response"
	extractorPkg "github.com/tuan-dd/go-pkg/extractor"
	"google.golang.org/grpc"
)

type (
	UnaryHandler func(context.Context, any) (any, *response.AppError)
	Middleware   func(*appLogger.Logger, *grpc.UnaryServerInfo, grpc.UnaryHandler) grpc.UnaryHandler
)

var extractor = extractorPkg.New()

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
func ChainUnary(logger *appLogger.Logger, middlewares ...Middleware) grpc.UnaryServerInterceptor {
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

func RequestContext(_ *appLogger.Logger, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) grpc.UnaryHandler {
	return func(ctx context.Context, req any) (any, error) {
		userID, _ := extractor.GetUserID(ctx)
		reqCtx := &common.ReqContext{
			CID:              extractor.GetRequestID(ctx),
			RequestTimestamp: extractor.GetRequestTimestamp(ctx),
			IP:               extractor.GetXForwardedFor(ctx),
			UserInfo: &common.UserInfo[any]{
				ID: userID,
			},
		}

		ctx = context.WithValue(ctx, constants.REQUEST_CONTEXT_KEY, reqCtx)

		return handler(ctx, req)
	}
}

func GrpcLogInterceptor(log *appLogger.Logger, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) grpc.UnaryHandler {
	return func(ctx context.Context, req any) (any, error) {
		reqCtx := common.GetReqCtx(ctx)

		log.ReqServerLogger(reqCtx, info.FullMethod, info.FullMethod)

		resp, err := handler(ctx, req)
		errApp := response.ConvertError(err)
		statusCode := constants.Success
		if errApp != nil && errApp.Code >= 9000 {
			statusCode = errApp.Code
			log.ResServerLogger(reqCtx, uint(statusCode), errApp)
		} else {
			log.ResServerLogger(reqCtx, uint(statusCode), nil)
		}

		return resp, errApp.Wrap()
	}
}

func RecoverMiddleware(log *appLogger.Logger, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) grpc.UnaryHandler {
	return func(ctx context.Context, req any) (any, error) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorPanic(common.GetReqCtx(ctx), info.FullMethod, r, string(debug.Stack()))
			}
		}()
		return handler(ctx, req)
	}
}
