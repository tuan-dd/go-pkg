package rabbitmq

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appLogger "github.com/tuan-dd/go-pkg/app-logger"
	"github.com/tuan-dd/go-pkg/common/queue"
	"github.com/tuan-dd/go-pkg/common/response"
	"github.com/tuan-dd/go-pkg/settings"
)

func TestNewConnectionAndShutdown(t *testing.T) {
	cfg := QueueConfig{
		Host:     "rabbitmq.com",
		Port:     5673,
		Username: "example",
		Password: "example",
	}

	log, _ := appLogger.NewLogger(&appLogger.LoggerConfig{Level: "debug"},
		&settings.ServerSetting{
			Environment: "dev",
			ServiceName: "test",
		})
	conn, err := NewConnection(&cfg, log)
	assert.Nil(t, err, "should not return error on connection")
	assert.NotNil(t, conn, "connection should not be nil")

	err = conn.SubscribeChan("example", queue.Options[SubChan]{
		AutoAck:    true,
		Concurrent: 2,
	}, func(ctx context.Context, msg *queue.Message) *response.AppError {
		log.Info("Received message", msg)
		return nil
	})

	assert.Nil(t, err, "should not return error on subscribe")

	time.Sleep(3 * time.Second) // wait for messages to be processed
	shutdownErr := conn.Shutdown()
	assert.Nil(t, shutdownErr, "should not return error on shutdown")
}
