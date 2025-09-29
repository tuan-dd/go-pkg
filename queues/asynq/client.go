package asynQueue

import (
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/tuan-dd/go-common/response"
	typeCustom "github.com/tuan-dd/go-common/type-custom"
)

type Client struct {
	*asynq.Client
	Log typeCustom.Logger
}

func (c *Client) NewClient(cfg *QueueConfig) (*Client, *response.AppError) {
	dns := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	client := &Client{
		Client: asynq.NewClient(asynq.RedisClientOpt{Addr: dns, Username: cfg.Username, Password: cfg.Password}),
		Log:    c.Log,
	}
	if err := client.Ping(); err != nil {
		c.Log.Error("Failed to connect Asynq client", err, dns)
		return nil, response.ServerError(fmt.Sprintf("failed to connect Asynq client: %s", err.Error()))
	}
	return client, nil
}

func BuildTask(typename string, payload []byte, opts ...asynq.Option) *asynq.Task {
	otpions := make([]asynq.Option, 0, len(opts)+1)
	otpions = append(otpions, asynq.Timeout(60*time.Second)) // Default deadline of 60 seconds
	if len(opts) == 0 {
		otpions = append(otpions, asynq.MaxRetry(3))
	}
	task := asynq.NewTask(typename, payload, otpions...)
	return task
}

func (c *Client) EnqueueTask(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, *response.AppError) {
	info, err := c.Enqueue(task, opts...)
	if err != nil {
		c.Log.Error("Failed to enqueue task", err, task.Type)
		return nil, response.ServerError(fmt.Sprintf("failed to enqueue task %s: %s", task.Type(), err.Error()))
	}
	return info, nil
}

func (c *Client) EnqueuePayLoadTask(taskName string, payload []byte, opts ...asynq.Option) (*asynq.TaskInfo, *response.AppError) {
	task := BuildTask(taskName, payload, opts...)
	info, err := c.Enqueue(task)
	if err != nil {
		c.Log.Error("Failed to enqueue payload task", err, task.Type())
		return nil, response.ServerError(fmt.Sprintf("failed to enqueue payload task %s: %s", task.Type(), err.Error()))
	}
	return info, nil
}

func (c *Client) shutdown() *response.AppError {
	if c.Client != nil {
		if err := c.Close(); err != nil {
			c.Log.Error("Failed to shutdown Asynq client", err)
			return response.ServerError(fmt.Sprintf("failed to shutdown Asynq client: %s", err.Error()))
		}
	}
	return nil
}
