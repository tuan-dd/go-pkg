package asynQueue

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/tuan-dd/go-common/response"
	typeCustom "github.com/tuan-dd/go-common/type-custom"
)

type QueueConfig struct {
	Host     string `mapstructure:"HOST"`
	Port     int    `mapstructure:"PORT"`
	Username string `mapstructure:"USERNAME"`
	Password string `mapstructure:"PASSWORD"`
	Config   *asynq.Config
}

type Connection struct {
	dns       string
	server    *asynq.Server
	Queue     *asynq.ServeMux
	Client    *Client
	Scheduler *asynq.Scheduler
	cfg       *QueueConfig
	Log       typeCustom.Logger
}

type Options struct {
	Concurrency int
}

type Level int

const (
	Critical Level = 6
	Default  Level = 3
	Low      Level = 1
)

func DefaultConfig() *asynq.Config {
	return &asynq.Config{
		Concurrency: 20,
		Queues:      map[string]int{"critical": 6, "default": 3, "low": 1},
	}
}

func DefaultSchedulerCfg(logger typeCustom.Logger) *asynq.SchedulerOpts {
	return &asynq.SchedulerOpts{
		Location: time.UTC,
	}
}

// TODO
func (c *Connection) ErrorHandler(ctx context.Context, task *asynq.Task, err error) {
	// c.Log.ResServerLogger(common.GetReqCtx(ctx), constants.InternalServerErr, err)
}

func NewConnect(cfg *QueueConfig, logger typeCustom.Logger) (*Connection, *response.AppError) {
	dns := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	if cfg.Config == nil {
		cfg.Config = DefaultConfig()
	}

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: dns, Username: cfg.Username, Password: cfg.Password},
		*cfg.Config,
	)

	if err := srv.Ping(); err != nil {
		logger.Error("Failed to connect to Asynq server", err)
		return nil, response.ServerError(fmt.Sprintf("failed to connect to Asynq server: %s", err.Error()))
	}

	return &Connection{
		server: srv,
		Log:    logger,
		dns:    dns,
		Queue:  asynq.NewServeMux(),
		cfg:    cfg,
		Client: &Client{
			Log:    logger,
			Client: asynq.NewClient(asynq.RedisClientOpt{Addr: dns}),
		},
	}, nil
}

func (c *Connection) NewScheduler(cfg *asynq.SchedulerOpts) *response.AppError {
	if c.Scheduler != nil {
		return response.ServerError("Scheduler already initialized")
	}

	if cfg == nil {
		cfg = DefaultSchedulerCfg(c.Log)
	}

	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{Addr: c.dns, Username: c.cfg.Username, Password: c.cfg.Password},
		cfg,
	)

	c.Scheduler = scheduler
	return nil
}

func (c *Connection) Shutdown() *response.AppError {
	if c.Scheduler != nil {
		c.Scheduler.Shutdown()
	}
	if c.Client != nil {
		err := c.Client.shutdown()
		if err != nil {
			return response.ServerError(fmt.Sprintf("failed to shutdown Asynq client: %s", err.Error()))
		}
	}
	if c.server != nil {
		c.server.Shutdown()
	}

	return nil
}

func (c *Connection) Run() *response.AppError {
	if c.server == nil {
		return response.ServerError("Server not initialized")
	}

	if c.Scheduler != nil {
		go func() {
			if err := c.Scheduler.Start(); err != nil {
				c.Log.Error("Failed to start scheduler", err)
				panic(fmt.Sprintf("failed to start scheduler: %s", err.Error()))
			}
		}()
	}

	if err := c.server.Start(c.Queue); err != nil {
		c.Log.Error("Failed to start server", err)
		return response.ServerError(fmt.Sprintf("failed to start server: %s", err.Error()))
	}

	return nil
}
