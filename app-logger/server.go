package appLogger

import (
	"context"
	"fmt"
	"time"

	"github.com/tuan-dd/go-common/request"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type (
	ReqServerLogger struct {
		CID        string
		Method     string
		Url        string
		TimeStamp  int64
		ServerName string
	}

	ResServerLogger struct {
		CID        string
		Duration   string
		Error      string
		ServerName string
		StatusCode uint
		Code       uint
	}
)

func (u *ReqServerLogger) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("CID", u.CID)
	enc.AddString("serverName", u.ServerName)
	enc.AddString("method", u.Method)
	enc.AddString("url", u.Url)
	enc.AddInt64("timeStamp", u.TimeStamp)
	return nil
}

func (u *ResServerLogger) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("CID", u.CID)
	enc.AddString("duration", u.Duration)
	enc.AddString("error", u.Error)
	enc.AddUint("statusCode", u.StatusCode)
	return nil
}

func (u *Logger) ReqServerLogger(reqCtx *request.ReqContext, method, path string) {
	u.logger.Info(fmt.Sprintf(`Accepted Request [%s/%s] - %s]`, method, path, reqCtx.CID), zap.Inline(&ReqServerLogger{
		ServerName: u.serviceName,
		Method:     method,
		Url:        path,
		CID:        reqCtx.CID,
		TimeStamp:  reqCtx.RequestTimestamp,
	}))
}

// ReqServerLoggerWithContext logs request with context for OpenTelemetry tracing
func (u *Logger) ReqServerLoggerWithContext(ctx context.Context, reqCtx *request.ReqContext, method, path string) {
	fields := []zap.Field{
		zap.Inline(&ReqServerLogger{
			ServerName: u.serviceName,
			Method:     method,
			Url:        path,
			CID:        reqCtx.CID,
			TimeStamp:  reqCtx.RequestTimestamp,
		}),
	}

	// Add context for OTel tracing
	if ctx != nil {
		fields = append(fields, zap.Any("context", ctx))
	}

	u.logger.Info(fmt.Sprintf(`Accepted Request [%s/%s] - %s]`, method, path, reqCtx.CID), fields...)
}

func (rsl *ResServerLogger) formatAlert(requestTimestamp int64) string {
	return fmt.Sprintf("🚨 *ALERT* at %s 🚨\n\n"+
		"*Service:* %s\n"+
		"*Request ID:* %s\n"+
		"*Status Code:* %d\n"+
		"*Error:*\n%s",
		time.UnixMilli(requestTimestamp).Format(time.RFC3339), rsl.ServerName, rsl.CID, rsl.StatusCode, rsl.Error)
}

func (u *Logger) ResServerLogger(reqCtx *request.ReqContext, statusCode uint, err error) {
	duration := request.FormatMilliseconds(reqCtx.RequestTimestamp)

	data := &ResServerLogger{
		CID:        reqCtx.CID,
		Duration:   duration,
		ServerName: u.serviceName,
		StatusCode: statusCode,
	}
	if err != nil {
		data.Error = err.Error()
		if u.alertSer != nil {
			alertMsg := data.formatAlert(reqCtx.RequestTimestamp)
			if appErr := u.alertSer.SendMsgStr(context.Background(), alertMsg); appErr != nil {
				u.logger.Error("Failed to send alert message", zap.Error(appErr))
			}
		}
		u.logger.Error(fmt.Sprintf(`Rejected Request [%s] - %d - %s]`, reqCtx.CID, statusCode, duration), zap.Inline(data))
	} else {
		u.logger.Info(fmt.Sprintf(`Response [%s] - %d - %s]`, reqCtx.CID, statusCode, duration), zap.Inline(data))
	}
}

// ResServerLoggerWithContext logs response with context for OpenTelemetry tracing
func (u *Logger) ResServerLoggerWithContext(ctx context.Context, reqCtx *request.ReqContext, statusCode uint, err error) {
	duration := request.FormatMilliseconds(reqCtx.RequestTimestamp)

	data := &ResServerLogger{
		CID:        reqCtx.CID,
		Duration:   duration,
		ServerName: u.serviceName,
		StatusCode: statusCode,
	}

	fields := []zap.Field{zap.Inline(data)}
	if ctx != nil {
		fields = append(fields, zap.Any("context", ctx))
	}

	if err != nil {
		data.Error = err.Error()
		if u.alertSer != nil {
			alertMsg := data.formatAlert(reqCtx.RequestTimestamp)
			if appErr := u.alertSer.SendMsgStr(context.Background(), alertMsg); appErr != nil {
				u.logger.Error("Failed to send alert message", zap.Error(appErr))
			}
		}
		u.logger.Error(fmt.Sprintf(`Rejected Request [%s] - %d - %s]`, reqCtx.CID, statusCode, duration), fields...)
	} else {
		u.logger.Info(fmt.Sprintf(`Response [%s] - %d - %s]`, reqCtx.CID, statusCode, duration), fields...)
	}
}
