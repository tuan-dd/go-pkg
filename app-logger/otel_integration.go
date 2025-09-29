package appLogger

import (
	"context"
	"math"

	"github.com/tuan-dd/go-common/response"
	"go.opentelemetry.io/otel/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// OTelCore wraps an OpenTelemetry LoggerProvider to act as a zap Core
type OTelCore struct {
	loggerProvider log.LoggerProvider
	logger         log.Logger
	level          zapcore.Level
	serviceName    string
}

// NewOTelCore creates a new OpenTelemetry core for zap
func NewOTelCore(provider log.LoggerProvider, serviceName string, level zapcore.Level) *OTelCore {
	logger := provider.Logger(serviceName)
	return &OTelCore{
		loggerProvider: provider,
		logger:         logger,
		level:          level,
		serviceName:    serviceName,
	}
}

// Enabled returns whether the given level is enabled
func (c *OTelCore) Enabled(level zapcore.Level) bool {
	return level >= c.level
}

// With adds structured context to the Core
func (c *OTelCore) With(fields []zapcore.Field) zapcore.Core {
	// Create a new core with the same config but could add fields to context
	return &OTelCore{
		loggerProvider: c.loggerProvider,
		logger:         c.logger,
		level:          c.level,
		serviceName:    c.serviceName,
	}
}

// Check determines whether the supplied Entry should be logged
func (c *OTelCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

// Write serializes the Entry and any Fields supplied at the log site and writes them to OpenTelemetry
func (c *OTelCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	// Convert zap entry to OpenTelemetry log record
	record := log.Record{}
	record.SetTimestamp(entry.Time)
	record.SetBody(log.StringValue(entry.Message))
	record.SetSeverity(convertZapLevelToOTel(entry.Level))
	record.SetSeverityText(entry.Level.String())

	// Convert zap fields to OpenTelemetry attributes
	for _, field := range fields {
		switch field.Type {
		case zapcore.StringType:
			record.AddAttributes(log.String(field.Key, field.String))
		case zapcore.Int64Type:
			record.AddAttributes(log.Int64(field.Key, field.Integer))
		case zapcore.Float64Type:
			record.AddAttributes(log.Float64(field.Key, math.Float64frombits(uint64(field.Integer))))
		case zapcore.BoolType:
			record.AddAttributes(log.Bool(field.Key, field.Integer == 1))
		default:
			record.AddAttributes(log.String(field.Key, field.String))
		}
	}

	// Emit the log record
	c.logger.Emit(context.Background(), record)
	return nil
}

// Sync flushes buffered logs (no-op for OpenTelemetry)
func (c *OTelCore) Sync() error {
	// OpenTelemetry handles flushing through the SDK
	return nil
}

// convertZapLevelToOTel converts zap log levels to OpenTelemetry severity
func convertZapLevelToOTel(level zapcore.Level) log.Severity {
	switch level {
	case zapcore.DebugLevel:
		return log.SeverityDebug
	case zapcore.InfoLevel:
		return log.SeverityInfo
	case zapcore.WarnLevel:
		return log.SeverityWarn
	case zapcore.ErrorLevel:
		return log.SeverityError
	case zapcore.DPanicLevel, zapcore.PanicLevel:
		return log.SeverityFatal2
	case zapcore.FatalLevel:
		return log.SeverityFatal3
	default:
		return log.SeverityInfo
	}
}

// WithOTelProvider is an option to add OpenTelemetry integration
func WithOTelProvider(provider log.LoggerProvider) option {
	return func(logger *Logger) {
		// This will be used when creating the logger cores
		// You can store it in the Logger struct if needed
	}
}

// CreateLoggerWithOTel creates a logger with OpenTelemetry integration
func NewLoggerWithOTel(cfg *LoggerConfig, serverConfig *ServerSetting, otelProvider log.LoggerProvider, opts ...option) (*Logger, *response.AppError) {
	level := getLogLevel(cfg.Level)

	cores := buildCore(cfg, serverConfig, opts...)
	// OpenTelemetry core (new)
	if otelProvider != nil {
		cores = append(cores, NewOTelCore(otelProvider, serverConfig.ServiceName, level))
	}

	// Combine cores
	core := zapcore.NewTee(cores...)
	zapLogger := zap.New(core, zap.AddStacktrace(zapcore.ErrorLevel))

	logger := &Logger{
		logger:      zapLogger,
		serviceName: serverConfig.ServiceName,
	}

	for _, opt := range opts {
		opt(logger)
	}

	return logger, nil
}
