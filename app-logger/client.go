package appLogger

import (
	"fmt"

	"github.com/tuan-dd/go-common/request"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type (
	ReqClientLogger struct {
		CID       string
		Method    string
		Url       string
		IP        string
		UserId    string
		TimeStamp int64
	}

	ResClientLogger struct {
		CID        string
		Duration   string
		Error      string
		UserId     string
		StatusCode uint
		Code       uint
	}
)

func (u ReqClientLogger) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("CID", u.CID)
	enc.AddString("method", u.Method)
	enc.AddString("url", u.Url)
	enc.AddString("ip", u.IP)
	enc.AddString("userId", u.UserId)
	enc.AddInt64("timeStamp", u.TimeStamp)
	return nil
}

func (u ResClientLogger) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("CID", u.CID)
	enc.AddString("duration", u.Duration)
	enc.AddString("error", u.Error)
	enc.AddString("userId", u.UserId)
	enc.AddUint("statusCode", u.StatusCode)
	return nil
}

func (logger *Logger) ReqClientLog(reqCtx *request.ReqContext, method, path string) {
	logger.logger.Info(fmt.Sprintf(`Accepted Request [%s/%s] - %s]`, method, path, reqCtx.CID), zap.Inline(ReqClientLogger{
		CID:       reqCtx.CID,
		Method:    method,
		Url:       path,
		IP:        reqCtx.IP,
		UserId:    reqCtx.UserInfo.ID,
		TimeStamp: reqCtx.RequestTimestamp,
	}))
}

func (logger *Logger) ResClientLog(reqCtx *request.ReqContext, statusCode uint, err error) {
	duration := request.FormatMilliseconds(reqCtx.RequestTimestamp)

	data := ResClientLogger{
		CID:        reqCtx.CID,
		Duration:   duration,
		UserId:     reqCtx.UserInfo.ID,
		StatusCode: statusCode,
		Code:       statusCode,
	}

	if err != nil {
		data.Error = err.Error()
		logger.logger.Error(fmt.Sprintf(`Rejected Request [%s] - %d - %s]`, reqCtx.CID, statusCode, duration), zap.Inline(data))
	} else {
		logger.logger.Info(fmt.Sprintf(`Response [%s] - %d - %s]`, reqCtx.CID, statusCode, duration), zap.Inline(data))
	}
}
