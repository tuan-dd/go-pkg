package asynQueue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appLogger "github.com/tuan-dd/go-pkg/app-logger"
	"github.com/tuan-dd/go-pkg/settings"
)

func TestNewConnectionAndShutdown(t *testing.T) {
	cfg := QueueConfig{
		Host:     "18.139.147.231",
		Port:     6379,
		Username: "myredis",
		Password: "redisabc123",
	}

	log, _ := appLogger.NewLogger(&appLogger.LoggerConfig{Level: "debug"},
		&settings.ServerSetting{
			Environment: "dev",
			ServiceName: "test",
		})
	conn, err := NewConnect(&cfg, log)

	assert.Nil(t, err, "should not return error on connection")
	assert.NotNil(t, conn, "connection should not be nil")

	err = conn.NewScheduler(nil)
	assert.Nil(t, err, "should not return error on scheduler creation")

	err = conn.Run()

	assert.Nil(t, err, "should not return error on server run")

	shutdownErr := conn.Shutdown()
	assert.Nil(t, shutdownErr, "should not return error on shutdown")
}
