package rabbitmq

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tuan-dd/go-pkg/appLogger"
	"github.com/tuan-dd/go-pkg/settings"
)

func TestNewConnectionAndShutdown(t *testing.T) {
	cfg := QueueConfig{
		Host:     "localhost",
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

	shutdownErr := conn.Shutdown()
	assert.Nil(t, shutdownErr, "should not return error on shutdown")
}
