package appLogger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tuan-dd/go-common/constants"
	"github.com/tuan-dd/go-common/request"
	"github.com/tuan-dd/go-common/response"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	// OpenTelemetry imports
	"go.opentelemetry.io/otel/log"
)

// Logger is the interface for logging
type (
	Logger struct {
		logger      *zap.Logger
		serviceName string
		alertSer    AlertSerIf
	}

	NotifyConfig struct {
		Token       string `mapstructure:"NOTIFY_TOKEN"`
		ChannelType string `mapstructure:"NOTIFY_CHANNEL_TYPE"` // "slack", "telegram", "email"
	}

	AlertSerIf interface {
		SendMsgStr(ctx context.Context, message string) *response.AppError
	}

	LoggerConfig struct {
		Level       string `mapstructure:"LOG_LEVEL"`
		MaxSize     int    `mapstructure:"LOG_MAX_SIZE"`
		MaxAge      int    `mapstructure:"LOG_MAX_AGE"`
		MaxBackups  int    `mapstructure:"LOG_MAX_BACKUPS"`
		Compress    bool   `mapstructure:"LOG_COMPRESS"`
		LogToFile   bool   `mapstructure:"LOG_TO_FILE"`
		LogFilePath string `mapstructure:"LOG_FILE_PATH"`
		// OpenTelemetry config
		EnableOTel   bool               `mapstructure:"ENABLE_OTEL"`
		OTelProvider log.LoggerProvider `json:"-"` // Don't serialize this
	}

	ServerSetting struct {
		Port        int    `mapstructure:"PORT"`
		Mode        string `mapstructure:"MODE"`
		Level       string `mapstructure:"LEVEL"`
		ServiceName string `mapstructure:"SERVICE_NAME"`
		Version     string `mapstructure:"VERSION"`
		Environment string `mapstructure:"ENVIRONMENT"`
	}
)

func getPath(path string) string {
	var fullPath string

	// Check if path is absolute
	if filepath.IsAbs(path) {
		fullPath = path
	} else {
		// Make it relative to current working directory
		dir, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		fullPath = filepath.Join(dir, path)
	}

	// Extract directory from file path
	logDir := filepath.Dir(fullPath)
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		err := os.MkdirAll(logDir, 0o755)
		if err != nil {
			panic(err)
		}
	}

	return fullPath
}

type option func(*Logger)

func WithAlertService(alertSer AlertSerIf) option {
	return func(cfg *Logger) {
		cfg.alertSer = alertSer
	}
}

func buildCore(cfg *LoggerConfig, serverConfig *ServerSetting, opts ...option) []zapcore.Core {
	level := getLogLevel(cfg.Level)

	var loggerConfig zapcore.EncoderConfig
	if serverConfig.Environment != "prod" {
		loggerConfig = zap.NewDevelopmentEncoderConfig()
		loggerConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		loggerConfig = zap.NewProductionEncoderConfig()
		loggerConfig.EncodeTime = nil
	}

	loggerConfig.CallerKey = ""
	var cores []zapcore.Core

	// Console output core
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(loggerConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)
	cores = append(cores, consoleCore)

	if cfg.LogToFile {
		logFile := cfg.LogFilePath
		if logFile == "" {
			logFile = "app.log" // default file name
		}

		path := getPath(logFile)

		// Use separate production config for file logging
		fileEncoderConfig := zap.NewProductionEncoderConfig()
		fileEncoderConfig.CallerKey = ""
		fileEncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)

		fileCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(fileEncoderConfig),
			zapcore.AddSync(&lumberjack.Logger{
				Filename:   path,
				MaxSize:    cfg.MaxSize,
				MaxAge:     cfg.MaxAge,
				MaxBackups: cfg.MaxBackups,
				Compress:   cfg.Compress,
			}),
			zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= zapcore.ErrorLevel // chỉ error và panic
			}),
		)
		cores = append(cores, fileCore)
	}

	// OpenTelemetry core (if enabled)
	return cores
}

func NewLogger(cfg *LoggerConfig, serverConfig *ServerSetting, opts ...option) (*Logger, *response.AppError) {
	cores := buildCore(cfg, serverConfig, opts...)
	// Combine cores
	core := zapcore.NewTee(cores...)
	logger := zap.New(core, zap.AddStacktrace(zapcore.ErrorLevel))

	loggerConn := &Logger{
		logger:      logger,
		serviceName: serverConfig.ServiceName,
	}

	for _, opt := range opts {
		opt(loggerConn)
	}
	return loggerConn, nil
}

func NewTestLogger() *Logger {
	loggerConfig := zap.NewDevelopmentEncoderConfig()
	loggerConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	loggerConfig.CallerKey = ""
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(loggerConfig),
		zapcore.AddSync(os.Stdout),
		zap.DebugLevel,
	)

	logger := zap.New(consoleCore, zap.AddStacktrace(zapcore.ErrorLevel))

	return &Logger{
		logger:      logger,
		serviceName: "test-service",
	}
}

func getLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// with context
func (l *Logger) WithContext(ctx context.Context) *Logger {
	var fields []zap.Field

	if cid, ok := ctx.Value(constants.REQUEST_ID_KEY).(string); ok {
		fields = append(fields, zap.String("cid", cid))
	}

	return &Logger{
		logger:      l.logger.With(fields...),
		serviceName: l.serviceName,
		alertSer:    l.alertSer,
	}
}

// WithField adds a field to the logger
func (l *Logger) WithField(key string, value any) *Logger {
	return &Logger{
		logger:      l.logger.With(zap.Any(key, value)),
		serviceName: l.serviceName,
		alertSer:    l.alertSer,
	}
}

// WithError adds an error to the logger
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		logger:      l.logger.With(zap.Error(err)),
		serviceName: l.serviceName,
		alertSer:    l.alertSer,
	}
}

func (l *Logger) ErrCtx(ctx context.Context, msg string, err error, fields ...zap.Field) {
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	l.WithContext(ctx).logger.Error(msg, fields...)
}

func (l *Logger) InfoCtx(ctx context.Context, msg string, err error, fields ...zap.Field) {
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	l.WithContext(ctx).logger.Info(msg, fields...)
}

func (l *Logger) Info(msg string, fields ...any) {
	if len(fields) == 0 {
		l.logger.Info(msg)
		return
	}

	if len(fields) == 1 {
		l.logger.Info(msg, zap.Any("data", fields[0]))
		return
	}
	l.logger.Info(msg, zap.Any("data", fields))
}

func (l *Logger) Warn(msg string, fields ...any) {
	if len(fields) == 0 {
		l.logger.Warn(msg)
		return
	}

	if len(fields) == 1 {
		l.logger.Warn(msg, zap.Any("data", fields[0]))
		return
	}
	l.logger.Warn(msg, zap.Any("data", fields))
}

func (l *Logger) Error(msg string, fields ...any) {
	orderedFields := []zap.Field{}

	err, ok := fields[0].(error)
	if ok {
		orderedFields = append(orderedFields, zap.Error(err))
		if len(fields) > 1 {
			orderedFields = append(orderedFields, zap.Any("data", fields[1:]))
		}
	} else {
		if len(fields) == 1 {
			orderedFields = append(orderedFields, zap.Any("data", fields[0]))
		} else if len(fields) > 1 {
			orderedFields = append(orderedFields, zap.Any("data", fields))
		}
	}

	l.logger.Error(l.buildMessage(msg), orderedFields...)
}

func (l *Logger) Debug(msg string, fields ...any) {
	if len(fields) == 1 {
		l.logger.Debug(l.buildMessage(msg), zap.Any("data", fields[0]))
		return
	}
	l.logger.Debug(msg, zap.Any("data", fields))
}

func (l *Logger) Logger() *zap.Logger {
	return l.logger
}

func (l *Logger) buildMessage(msg string) string {
	return l.serviceName + ": " + msg
}

func (l *Logger) DBLog(ctx context.Context, data ...any) {
	reqCtx := request.GetReqCtx(ctx)

	duration := request.CalculateDuration(reqCtx.RequestTimestamp)

	l.logger.Info(reqCtx.CID, zap.String("sql", fmt.Sprintf("SQL:%s:%dms", data[0], duration)))
}
