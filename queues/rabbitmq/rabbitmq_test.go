package rabbitmq

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tuan-dd/go-common/queue"
	"github.com/tuan-dd/go-common/request"
	"github.com/tuan-dd/go-common/response"
)

type CustomerLogger struct {
	*slog.Logger
}

func (l *CustomerLogger) AppErrorWithAlert(reqCtx *request.ReqContext, err *response.AppError, fields ...any) {
	l.Error("Error with alert")
}

func (l *CustomerLogger) ErrorPanic(reqCtx *request.ReqContext, msg string, err any, stack string) {
	l.Error("Error panic", "msg", msg, "err", err, "stack", stack)
}

var logger = &CustomerLogger{
	Logger: slog.Default(),
}

func TestNewConnectionAndShutdown(t *testing.T) {
	cfg := QueueConfig{
		Host:     "translations-rmq.feedstream.org",
		Port:     5674,
		Username: "cebettrs",
		Password: "5cWsjzNV",
	}

	conn, err := NewConnection(&cfg, logger)
	assert.Nil(t, err, "should not return error on connection")
	assert.NotNil(t, conn, "connection should not be nil")

	err = conn.Subscribe("T18767906_translation", queue.Options[SubBase]{
		AutoAck:    true,
		Concurrent: 2,
		Config: SubBase{
			DeclareQueue: true,
		},
	}, func(ctx context.Context, msg *queue.Message) *response.AppError {
		logger.Info("Received message", "msg", msg)
		return nil
	})

	assert.Nil(t, err, "should not return error on subscribe")

	time.Sleep(5 * time.Second) // wait for messages to be processed
	shutdownErr := conn.Shutdown()
	assert.Nil(t, shutdownErr, "should not return error on shutdown")
}
