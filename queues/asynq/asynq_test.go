package asynQueue

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tuan-dd/go-common/response"
)

func TestNewConnectionAndShutdown(t *testing.T) {
	cfg := QueueConfig{
		Host:     "18.136.192.113",
		Port:     6379,
		Username: "myredis",
		Password: "redisabc123",
	}

	log := slog.Default()
	conn, err := NewConnect(&cfg, log)

	assert.Nil(t, err, "should not return error on connection")
	assert.NotNil(t, conn, "connection should not be nil")

	err = conn.NewScheduler(nil)
	assert.Nil(t, err, "should not return error on scheduler creation")

	var shutdownErr *response.AppError
	go func(tt assert.TestingT) bool {
		time.Sleep(3 * time.Second)
		shutdownErr = conn.Shutdown()
		return true
	}(t)
	slog.Info("Server is Starting")
	err = conn.Run()
	slog.Info("Server is Shutdown")
	assert.Nil(t, err, "should not return error on server run")

	assert.Nil(t, shutdownErr.Wrap(), "should not return error on shutdown")
}
