package natsQueue

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/tuan-dd/go-pkg/common/queue"
	"github.com/tuan-dd/go-pkg/common/response"
)

func (c *Connection) Publish(ctx context.Context, topic string, msg *queue.Message) (*jetstream.PubAck, *response.AppError) {
	if msg == nil {
		return nil, response.ServerError("message is nil")
	}
	errApp := EnBodyCompression(msg)

	if errApp != nil {
		return nil, errApp
	}

	jsCtx, err := c.js.PublishMsg(ctx, c.BuildMessagePub(topic, msg))
	if err != nil {
		c.Log.Error(fmt.Sprintf("failed to publish message: %s", topic), err)
		return nil, response.ServerError(fmt.Sprintf("failed to publish message: %s", err.Error()))
	}

	return jsCtx, nil
}

func (c *Connection) PublishAsync(ctx context.Context, topic string, msg *queue.Message) (jetstream.PubAckFuture, *response.AppError) {
	if msg == nil {
		return nil, response.ServerError("message is nil")
	}

	errApp := EnBodyCompression(msg)

	if errApp != nil {
		return nil, errApp
	}

	jsCtx, err := c.js.PublishMsgAsync(c.BuildMessagePub(topic, msg))
	if err != nil {
		c.Log.Error(fmt.Sprintf("failed to publish message: %s", topic), err)
		return nil, response.ServerError(fmt.Sprintf("failed to publish message: %s", err.Error()))
	}

	return jsCtx, nil
}

func (c *Connection) PublishNor(ctx context.Context, topic string, msg *queue.Message) *response.AppError {
	if msg == nil {
		return response.ServerError("message is nil")
	}

	errApp := EnBodyCompression(msg)

	if errApp != nil {
		return errApp
	}

	err := c.conn.PublishMsg(c.BuildMessagePub(topic, msg))
	if err != nil {
		c.Log.Error(fmt.Sprintf("failed to publish message: %s", topic), err)
		return response.ServerError(fmt.Sprintf("failed to publish message: %s", err.Error()))
	}

	return nil
}

// TODO reply
func (c *Connection) BuildMessagePub(topic string, msg *queue.Message) *nats.Msg {
	natMsg := nats.NewMsg(topic)
	natMsg.Data = msg.Body
	if msg.Headers != nil {
		for k, v := range *msg.Headers {
			natMsg.Header.Add(k, v.(string))
		}
	}
	if msg.ID != "" {
		natMsg.Header.Set(string(NatsMSGID), msg.ID)
	}

	return natMsg
}

// TODO: Implement PublishWithReply
func (c *Connection) PublishWithReply(topic string, msg *queue.Message, options PubJsOption) *response.AppError {
	return nil
}
