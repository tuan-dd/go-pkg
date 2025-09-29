package gRPC

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/tuan-dd/go-common/response"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

type GRPCServer struct {
	App  *grpc.Server
	Port int
	Name string
}
type ServerSetting struct {
	Port        int    `mapstructure:"PORT"`
	Mode        string `mapstructure:"MODE"`
	Level       string `mapstructure:"LEVEL"`
	ServiceName string `mapstructure:"SERVICE_NAME"`
	Version     string `mapstructure:"VERSION"`
	Environment string `mapstructure:"ENVIRONMENT"`
}

func NewGrpcServer(cfg *ServerSetting, log customLogger) (*GRPCServer, *response.AppError) {
	app := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(ChainUnary(log, RecoverMiddleware, GrpcLogInterceptor, ValidateMiddleware)),
	)

	return &GRPCServer{
		App:  app,
		Port: cfg.Port,
		Name: cfg.ServiceName,
	}, nil
}

func (s *GRPCServer) Start(log customLogger, shutDown func()) *response.AppError {
	// Graceful shutdown setup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, unix.SIGTERM, unix.SIGINT, unix.SIGTSTP)

	// Start server in goroutine
	addr := fmt.Sprintf(":%d", s.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("failed to listen", err)
		panic(err)
	}

	serverErr := make(chan error, 1)
	go func() {
		if err := s.App.Serve(lis); err != nil {
			serverErr <- err
		}
	}()

	// Wait for interrupt signal or server error
	log.Info(fmt.Sprintf("gRPC server is running on %s", addr))
	select {
	case err := <-serverErr:
		return response.ServerError(fmt.Sprintf("server error: %v", err.Error()))
	case sig := <-sigChan:
		log.Info("received signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		shutDown()
		s.App.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		log.Info("graceful shutdown completed", nil)
	case <-ctx.Done():
		log.Info("graceful shutdown timeout", nil)
	}

	return nil
}

// NOTE: span attributes for request id should be added in middleware.go RequestContext instead.
// We keep imports attribute & trace if future use is needed; remove if unused to silence lints.
