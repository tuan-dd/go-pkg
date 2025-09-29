package obs

import (
	"context"
	"fmt"
	"time"

	"github.com/tuan-dd/go-common/response"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	// Traces
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	// Metrics
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	// Logs
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

type Providers struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider
	Conn           *grpc.ClientConn
	Shutdown       func(context.Context) error
}

type OtelConfig struct {
	Endpoint   string `mapstructure:"ENDPOINT"`
	Insecure   bool   `mapstructure:"INSECURE"`
	SvcName    string `mapstructure:"SERVICE_NAME"`
	SvcVersion string `mapstructure:"SERVICE_VERSION"`
	Env        string `mapstructure:"ENVIRONMENT"`
}

func InitOTel(ctx context.Context, cfg *OtelConfig) (*Providers, *response.AppError) {
	// 0) Shared gRPC conn
	var credsOpt []grpc.DialOption
	if cfg.Insecure {
		credsOpt = append(credsOpt, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		credsOpt = append(credsOpt, grpc.WithTransportCredentials(credentials.NewTLS(nil)))
	}
	conn, err := grpc.NewClient(cfg.Endpoint,
		credsOpt...,
	)
	if err != nil {
		return nil, response.ServerError(fmt.Sprintf("failed to create gRPC connection to collector: %v", err))
	}

	// Resource (gắn nhãn chung)
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.SvcName),
			semconv.ServiceVersion(cfg.SvcVersion),
			semconv.DeploymentEnvironmentName(cfg.Env),
		),
	)
	if err != nil {
		return nil, response.ServerError(fmt.Sprintf("failed to create resource: %v", err))
	}

	// 1) Trace exporter + provider
	traceExp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, response.ServerError(fmt.Sprintf("failed to create trace exporter: %v", err))
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp,
			sdktrace.WithMaxQueueSize(4096),
			sdktrace.WithExportTimeout(10*time.Second),
			sdktrace.WithBatchTimeout(3*time.Second),
		),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// 2) Metric exporter + provider (PeriodicReader)
	metricExp, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, response.ServerError(fmt.Sprintf("failed to create metric exporter: %v", err))
	}

	reader := sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(30*time.Second))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithResource(res))
	otel.SetMeterProvider(mp)

	// 3) Log exporter + provider
	logExp, err := otlploggrpc.New(ctx, otlploggrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, response.ServerError(fmt.Sprintf("failed to create log exporter: %v", err))
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(
			logExp,
			sdklog.WithMaxQueueSize(4096),
			sdklog.WithExportMaxBatchSize(512),
			sdklog.WithExportTimeout(10*time.Second),
		)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(lp)

	// Propagator W3C (traceparent + baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	// 4) Bridge zap → OTel Logs (không ghi file/stdout)

	shutdown := func(ctx context.Context) error {
		// Flush theo thứ tự: logs → metrics → traces
		_ = lp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		_ = tp.Shutdown(ctx)
		return conn.Close()
	}

	return &Providers{
		TracerProvider: tp,
		MeterProvider:  mp,
		LoggerProvider: lp,
		Conn:           conn,
		Shutdown:       shutdown,
	}, nil
}
