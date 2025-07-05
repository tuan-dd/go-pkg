package appLogger

import (
	"context"
	"fmt"
	"os"

	"github.com/tuan-dd/go-pkg/common"
	"github.com/tuan-dd/go-pkg/common/constants"
	"github.com/tuan-dd/go-pkg/common/response"
	"github.com/tuan-dd/go-pkg/settings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is the interface for logging
type (
	Logger struct {
		logger      *zap.Logger
		serviceName string
	}

	PanicLogger struct {
		CID        string
		ServerName string
		TimeStamp  int64
		Stack      string
	}

	NotifyConfig struct {
		Token       string `mapstructure:"NOTIFY_TOKEN"`
		ChannelType string `mapstructure:"NOTIFY_CHANNEL_TYPE"` // "slack", "telegram", "email"
	}

	LoggerConfig struct {
		Level      string `mapstructure:"LOG_LEVEL"`
		MaxSize    int    `mapstructure:"LOG_MAX_SIZE"`
		MaxAge     int    `mapstructure:"LOG_MAX_AGE"`
		MaxBackups int    `mapstructure:"LOG_MAX_BACKUPS"`
		Compress   bool   `mapstructure:"LOG_COMPRESS"`
	}
)

func NewLogger(cfg *LoggerConfig, serverConfig *settings.ServerSetting) (*Logger, *response.AppError) {
	level := getLogLevel(cfg.Level)

	var loggerConfig zapcore.EncoderConfig
	if serverConfig.Environment != "prod" {
		loggerConfig = zap.NewDevelopmentEncoderConfig()
		loggerConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		loggerConfig = zap.NewProductionEncoderConfig()
	}

	loggerConfig.CallerKey = ""
	loggerConfig.EncodeTime = nil

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(loggerConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)

	logger := zap.New(core, zap.AddStacktrace(zapcore.ErrorLevel))

	return &Logger{
		logger:      logger,
		serviceName: serverConfig.ServiceName,
	}, nil
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
func (l *Logger) WithContext(ctx context.Context) *zap.Logger {
	var fields []zap.Field

	// Add correlation Id if exists

	if cid, ok := ctx.Value(constants.REQUEST_ID_KEY).(string); ok {
		fields = append(fields, zap.String("cid", cid))
	}

	// Add request Id if exists

	if rid, ok := ctx.Value(constants.REQUEST_ID_KEY).(string); ok {
		fields = append(fields, zap.String("rid", rid))
	}

	return l.logger.With(fields...)
}

// WithField adds a field to the logger
func (l *Logger) WithField(key string, value any) *zap.Logger {
	return l.logger.With(zap.Any(key, value))
}

// WithError adds an error to the logger
func (l *Logger) WithError(err error) *zap.Logger {
	return l.logger.With(zap.Error(err))
}

func (l *Logger) ErrCtx(ctx context.Context, msg string, err error, fields ...zap.Field) {
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	l.WithContext(ctx).Error(msg, fields...)
}

func (l *Logger) InfoCtx(ctx context.Context, msg string, err error, fields ...zap.Field) {
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	l.WithContext(ctx).Info(msg, fields...)
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

func (l *Logger) Error(msg string, err error, fields ...any) {
	if err != nil {
		l.logger.Error(msg, zap.Error(err), zap.Any("data", fields))
		return
	}
	if len(fields) == 1 {
		l.logger.Error(msg, zap.Any("data", fields[0]))
		return
	}

	l.logger.Error(msg, zap.Any("data", fields))
}

func (l *Logger) AppError(msg string, err *response.AppError) {
	if err != nil {
		l.logger.Error(msg, zap.Error(err), zap.Any("data", err.Data))
	}
}

func (l *Logger) Debug(msg string, fields ...any) {
	if len(fields) == 1 {
		l.logger.Debug(msg, zap.Any("data", fields[0]))
		return
	}
	l.logger.Debug(msg, zap.Any("data", fields))
}

func (l *Logger) Logger() *zap.Logger {
	return l.logger
}

func (u PanicLogger) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("CID", u.CID)
	enc.AddString("serverName", u.ServerName)
	enc.AddInt64("timeStamp", u.TimeStamp)
	enc.AddString("stack", u.Stack)
	return nil
}

func (l *Logger) BuildMessage(msg string) string {
	return l.serviceName + ":" + msg
}

func (l *Logger) ErrorPanic(reqCtx *common.ReqContext, msg string, err any, stack string) {
	l.logger.Error(l.BuildMessage(msg), zap.Any("error", err), zap.Inline(PanicLogger{
		CID:        reqCtx.CID,
		ServerName: l.serviceName,
		TimeStamp:  reqCtx.RequestTimestamp,
		Stack:      stack,
	}))
}

func (l *Logger) DBLog(ctx context.Context, data ...any) {
	reqCtx := common.GetReqCtx(ctx)

	duration := common.CalculateDuration(reqCtx.RequestTimestamp)

	l.logger.Info(reqCtx.CID, zap.String("sql", fmt.Sprintf("SQL:%s:%dms", data[0], duration)))
}
