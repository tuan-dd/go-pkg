package appLogger

import (
	"context"
	"fmt"
	"time"

	"github.com/tuan-dd/go-common/request"
	"github.com/tuan-dd/go-common/response"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type (
	PanicLogger struct {
		CID        string
		ServerName string
		TimeStamp  int64
		Stack      string
	}
	AppErrorLogger struct {
		CID        string
		ServerName string
		message    string
		code       int
		Stack      string
	}
)

func (u PanicLogger) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("CID", u.CID)
	enc.AddInt64("timeStamp", u.TimeStamp)
	enc.AddString("stack", u.Stack)
	return nil
}

func (l *PanicLogger) formatAlert(message any) string {
	return fmt.Sprintf("🚨 *PANIC ALERT* at %s 🚨\n\n"+
		"*Service:* %s\n"+
		"*Request ID:* %s\n"+
		"*Message:* %v\n"+
		"*Stack Trace:*\n```\n%s\n```",
		time.UnixMilli(l.TimeStamp).Format(time.RFC3339), l.ServerName, l.CID, message, l.Stack)
}

func (l *Logger) ErrorPanic(reqCtx *request.ReqContext, msg string, err any, stack string) {
	panicLogger := PanicLogger{
		CID:        reqCtx.CID,
		ServerName: l.serviceName,
		TimeStamp:  reqCtx.RequestTimestamp,
		Stack:      stack,
	}

	if l.alertSer != nil {
		alertMsg := panicLogger.formatAlert(err)
		if appErr := l.alertSer.SendMsgStr(context.Background(), alertMsg); appErr != nil {
			l.logger.Error("Failed to send alert message", zap.Error(appErr))
		}
	}

	l.logger.Error(l.buildMessage(msg), zap.Any("error", err), zap.Inline(panicLogger))
}

// ErrorPanicWithContext logs panic with context for OpenTelemetry tracing
func (l *Logger) ErrorPanicWithContext(ctx context.Context, reqCtx *request.ReqContext, msg string, err any, stack string) {
	panicLogger := PanicLogger{
		CID:        reqCtx.CID,
		ServerName: l.serviceName,
		TimeStamp:  reqCtx.RequestTimestamp,
		Stack:      stack,
	}

	if l.alertSer != nil {
		alertMsg := panicLogger.formatAlert(err)
		if appErr := l.alertSer.SendMsgStr(context.Background(), alertMsg); appErr != nil {
			l.logger.Error("Failed to send alert message", zap.Error(appErr))
		}
	}

	fields := []zap.Field{
		zap.Any("error", err),
		zap.Inline(panicLogger),
	}

	// Add context for OTel tracing
	if ctx != nil {
		fields = append(fields, zap.Any("context", ctx))
	}

	l.logger.Error(l.buildMessage(msg), fields...)
}

func (u AppErrorLogger) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("CID", u.CID)
	enc.AddInt("code", u.code)
	enc.AddString("stack", u.Stack)
	return nil
}

func (l *AppErrorLogger) formatAlert(timeStamp int64) string {
	return fmt.Sprintf("🚨 *ERROR ALERT* at %s 🚨\n\n"+
		"*Service:* %s\n"+
		"*Request ID:* %s\n"+
		"*Message:* %v\n"+
		"*Stack Trace:*\n```\n%s\n```",
		time.UnixMilli(timeStamp).Format(time.RFC3339), l.ServerName, l.CID, l.message, l.Stack)
}

func (l *Logger) AppErrorWithAlert(reqCtx *request.ReqContext, err *response.AppError, fields ...any) {
	errorLogger := AppErrorLogger{
		CID:        reqCtx.CID,
		ServerName: l.serviceName,
		message:    err.Error(),
		code:       int(err.Code),
	}
	stack, ok := err.Data.(string)
	if ok {
		errorLogger.Stack = stack
	}

	allFields := []zap.Field{zap.Inline(errorLogger)}
	if len(fields) > 0 {
		allFields = append(allFields, zap.Any("data", fields))
	}
	l.logger.Error(l.buildMessage(fmt.Sprintf("error %v", err.Message)), allFields...)

	if l.alertSer != nil {
		alertMsg := errorLogger.formatAlert(reqCtx.RequestTimestamp)
		if appErr := l.alertSer.SendMsgStr(context.Background(), alertMsg); appErr != nil {
			l.logger.Error("Failed to send alert message", zap.Error(appErr))
		}
	}
}

// AppErrorWithAlertAndContext logs app error with context for OpenTelemetry tracing
func (l *Logger) AppErrorWithAlertAndContext(ctx context.Context, reqCtx *request.ReqContext, err *response.AppError, fields ...any) {
	errorLogger := AppErrorLogger{
		CID:        reqCtx.CID,
		ServerName: l.serviceName,
		message:    err.Error(),
		code:       int(err.Code),
	}
	stack, ok := err.Data.(string)
	if ok {
		errorLogger.Stack = stack
	}

	allFields := []zap.Field{zap.Inline(errorLogger)}
	if len(fields) > 0 {
		allFields = append(allFields, zap.Any("data", fields))
	}

	// Add context for OTel tracing
	if ctx != nil {
		allFields = append(allFields, zap.Any("context", ctx))
	}

	l.logger.Error(l.buildMessage(fmt.Sprintf("error %v", err.Message)), allFields...)

	if l.alertSer != nil {
		alertMsg := errorLogger.formatAlert(reqCtx.RequestTimestamp)
		if appErr := l.alertSer.SendMsgStr(context.Background(), alertMsg); appErr != nil {
			l.logger.Error("Failed to send alert message", zap.Error(appErr))
		}
	}
}
