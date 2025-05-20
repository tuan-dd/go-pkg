package asynq

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/tuan-dd/go-pkg/appLogger"
	"github.com/tuan-dd/go-pkg/common/queue"
	"github.com/tuan-dd/go-pkg/common/response"
)

type QueueConfig struct {
	Host     string `mapstructure:"DB_HOST"`
	Port     int    `mapstructure:"DB_PORT"`
	Username string `mapstructure:"DB_USERNAME"`
	Password string `mapstructure:"DB_PASSWORD"`
}

type Connection struct {
	queue.QueueServer
	client *asynq.Server
	cfg    *QueueConfig
	Log    *appLogger.Logger
}

type Options struct {
	Concurrency int
}

func NewConnection(cfg *QueueConfig) (*Connection, *response.AppError) {
	return connect(fmt.Sprintf("amqp://%s:%s@%s:%d", cfg.Username, cfg.Password, cfg.Host, cfg.Port), cfg)
}

func middlewareA(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		return next.ProcessTask(ctx, t)
	})
}

func connect(dns string, cfg *QueueConfig) (*Connection, *response.AppError) {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: dns},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)

	mux := asynq.NewServeMux()

	mux.Use(middlewareA)
	err := srv.Run(mux)
	if err != nil {
		return nil, response.ServerError("failed to connect asynq " + err.Error())
	}
	// if err != nil {
	// 	return nil, response.ServerError("failed to connect rabbitmq " + err.Error())
	// }
	// ch, err := conn.Channel()
	// if err != nil {
	// 	return nil, response.ServerError("failed to open a channel " + err.Error())
	// }

	return &Connection{client: srv, cfg: cfg}, nil
}
