package asynQueue

import (
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/tuan-dd/go-common/response"
)

func (c *Connection) ScheduleTask(cronspec string, task *asynq.Task, opts ...asynq.Option) (string, *response.AppError) {
	entryID, err := c.Scheduler.Register(cronspec, task, opts...)
	if err != nil {
		return "", response.ServerError(fmt.Sprintf("failed to schedule task: %s", err.Error()))
	}
	return entryID, nil
}
