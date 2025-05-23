package gRPC

import (
	"context"
	"fmt"
	"net/url"
	"path"

	"github.com/tuan-dd/go-pkg/common/constants"
	"github.com/tuan-dd/go-pkg/common/response"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type GrpcClientConn struct {
	conn         *grpc.ClientConn
	methodPrefix string
}

// type check
var _ grpc.ClientConnInterface = (*GrpcClientConn)(nil)

// Invoke implements grpc.ClientConnInterface.
func (c *GrpcClientConn) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	newMethod := path.Join(c.methodPrefix, method)
	err := c.conn.Invoke(ctx, newMethod, args, reply, opts...)
	if err != nil {
		return response.ServerError(fmt.Sprintf("failed to invoke grpc method: %s", err.Error()))
	}
	return nil
}

// NewStream implements grpc.ClientConnInterface.
func (c *GrpcClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	newMethod := path.Join(c.methodPrefix, method)
	stream, err := c.conn.NewStream(ctx, desc, newMethod, opts...)
	if err != nil {
		return nil, response.ServerError(fmt.Sprintf("failed to create grpc client stream: %s", err.Error()))
	}
	return stream, nil
}

func NewClientConnectionV2(target string, opts ...grpc.DialOption) (*GrpcClientConn, *response.AppError) {
	parsedUrl, err := url.Parse(target)
	if err != nil {
		return nil, response.ServerError(fmt.Sprintf("failed to parse target url: %s", err.Error()))
	}
	scheme := parsedUrl.Scheme
	path := parsedUrl.Path
	port := parsedUrl.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	address := fmt.Sprintf("%s:%s", parsedUrl.Hostname(), port)

	options := []grpc.DialOption{
		grpc.WithUnaryInterceptor(InjectRequestMetadata),
	}

	if scheme == "https" {
		options = append(options, grpc.WithTransportCredentials(credentials.NewTLS(nil)))
	} else {
		options = append(options, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	options = append(options, opts...)

	conn, err := grpc.NewClient(address, options...)
	if err != nil {
		return nil, response.ServerError(fmt.Sprintf("failed to create grpc client connection: %s", err.Error()))
	}

	return &GrpcClientConn{
		conn:         conn,
		methodPrefix: path,
	}, nil
}

type Config struct {
	IsSecure    bool
	Url         string
	NameService string
}

func NewClientConnection(config *Config, opts ...grpc.DialOption) (*grpc.ClientConn, *response.AppError) {
	options := []grpc.DialOption{
		grpc.WithUnaryInterceptor(InjectRequestMetadata),
	}

	if config.IsSecure {
		options = append(options, grpc.WithTransportCredentials(credentials.NewTLS(nil)))
	} else {
		options = append(options, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	options = append(options, opts...)
	conn, err := grpc.NewClient(config.Url, options...)
	if err != nil {
		return nil, response.ServerError(fmt.Sprintf("failed to create grpc client connection: %s", err.Error()))
	}
	return conn, nil
}

func InjectRequestMetadata(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}

	inMd, ok := metadata.FromIncomingContext(ctx)
	if ok {
		md.Set(string(constants.REQUEST_ID_KEY), inMd.Get(string(constants.REQUEST_ID_KEY))...)
		md.Set(string(constants.UserID), inMd.Get(string(constants.UserID))...)
		md.Set(string(constants.XForwardedFor), inMd.Get(string(constants.XForwardedFor))...)
		md.Set(string(constants.StartTime), inMd.Get(string(constants.StartTime))...)
	}

	newCtx := metadata.NewOutgoingContext(ctx, md)
	return invoker(newCtx, method, req, reply, cc, opts...)
}
