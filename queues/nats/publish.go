package natsQueue

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/tuan-dd/go-common/constants"
	"github.com/tuan-dd/go-common/queue"
	"github.com/tuan-dd/go-common/request"
	"github.com/tuan-dd/go-common/response"
)

func (c *Connection) Publish(ctx context.Context, topic string, msg *queue.Message) (*jetstream.PubAck, *response.AppError) {
	if msg == nil {
		return nil, response.ServerError("message is nil")
	}
	errApp := EnBodyCompression(msg)

	if errApp != nil {
		return nil, errApp
	}

	jsCtx, err := c.js.PublishMsg(ctx, c.BuildMessagePub(ctx, topic, msg))
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

	jsCtx, err := c.js.PublishMsgAsync(c.BuildMessagePub(ctx, topic, msg))
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

	err := c.conn.PublishMsg(c.BuildMessagePub(ctx, topic, msg))
	if err != nil {
		c.Log.Error(fmt.Sprintf("failed to publish message: %s", topic), err)
		return response.ServerError(fmt.Sprintf("failed to publish message: %s", err.Error()))
	}

	return nil
}

// TODO reply
func (c *Connection) BuildMessagePub(ctx context.Context, topic string, msg *queue.Message) *nats.Msg {
	natMsg := nats.NewMsg(topic)
	natMsg.Data = msg.Body
	if msg.NoHeader {
		return natMsg
	}
	c.buildHeader(ctx, natMsg)
	if msg.Header != nil {
		for k, v := range *msg.Header {
			natMsg.Header.Add(k, fmt.Sprintf("%v", v))
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

func (c *Connection) buildHeader(ctx context.Context, msg *nats.Msg) {
	reqCtx := request.GetReqCtx(ctx)
	reqCtxBytes, err := sonic.Marshal(reqCtx)
	if err == nil {
		reqCtxBase64 := base64.StdEncoding.EncodeToString(reqCtxBytes)
		msg.Header.Set(string(constants.REQUEST_CONTEXT_KEY), reqCtxBase64)
	}
	msg.Header.Set(string(constants.REQUEST_ID_KEY), reqCtx.CID)
}
