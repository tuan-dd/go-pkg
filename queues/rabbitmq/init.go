package rabbitmq

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/tuan-dd/go-common/constants"
	"github.com/tuan-dd/go-common/queue"
	"github.com/tuan-dd/go-common/request"
	"github.com/tuan-dd/go-common/response"
	typeCustom "github.com/tuan-dd/go-common/type-custom"
)

type QueueConfig struct {
	Host     string `mapstructure:"HOST"`
	Port     int    `mapstructure:"PORT"`
	Username string `mapstructure:"USERNAME"`
	Password string `mapstructure:"PASSWORD"`
}

type Connection struct {
	queue.QueueServer
	mapChannels map[string][]*amqp.Channel
	conn        *amqp.Connection
	cfg         *QueueConfig
	Log         typeCustom.LoggerWithAlert
}

type SubBase struct {
	DeclareQueue      bool // NoDeclareQueue is used to skip queue declaration
	Durable           bool
	AutoDelete        bool
	QueueExclusive    bool
	ConsumerExclusive bool
	IsNoRecovery      bool // IsNoRecovery is used to skip recovery of the subscription
}
type SubChan struct {
	SubBase
}

func NewConnection(cfg *QueueConfig, log typeCustom.LoggerWithAlert) (*Connection, *response.AppError) {
	return connect(fmt.Sprintf("amqp://%s:%s@%s:%d", cfg.Username, cfg.Password, cfg.Host, cfg.Port), cfg, log)
}

func connect(dns string, cfg *QueueConfig, log typeCustom.LoggerWithAlert) (*Connection, *response.AppError) {
	conn, err := amqp.Dial(dns)
	if err != nil {
		return nil, response.ServerError("failed to connect rabbitmq " + err.Error())
	}

	log.Info("Successfully connected to RabbitMQ server")

	return &Connection{cfg: cfg, conn: conn, Log: log, mapChannels: map[string][]*amqp.Channel{}}, nil
}

func isChannelClosedError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "channel/connection is not open") ||
		strings.Contains(errStr, "channel is closed") ||
		strings.Contains(errStr, "connection is closed") ||
		strings.Contains(errStr, "Exception (504)")
}

func (c *Connection) Shutdown() *response.AppError {
	c.Log.Info("Starting RabbitMQ shutdown...")

	for key, channels := range c.mapChannels {
		for idx, channel := range channels {
			tag := fmt.Sprintf("%s_%d", key, idx+1)

			if err := channel.Cancel(tag, false); err != nil {
				if isChannelClosedError(err) {
					c.Log.Warn("channel already closed when canceling", err, key)
				} else {
					c.Log.Warn("failed to cancel channel", err, key)
				}
			}

			if err := channel.Close(); err != nil {
				if isChannelClosedError(err) {
					c.Log.Warn("channel already closed", err, key)
				} else {
					c.Log.Error("failed to close channel", err, key)
				}
			} else {
				c.Log.Info("Closed channel", key)
			}
		}
		delete(c.mapChannels, key)
	}

	if c.conn != nil && !c.conn.IsClosed() {
		if err := c.conn.Close(); err != nil {
			if isChannelClosedError(err) {
				c.Log.Warn("connection already closed", err)
			} else {
				c.Log.Error("failed to close connection", err)
				return response.ServerError(fmt.Sprintf("failed to close connection: %s", err.Error()))
			}
		}
	}

	c.Log.Info("RabbitMQ shutdown completed")
	return nil
}

func (c *Connection) Subscribe(topic string, options queue.Options[SubBase], handler queue.HandlerFunc) *response.AppError {
	autoAck := options.AutoAck

	channel, err := c.conn.Channel()
	if err != nil {
		c.Log.Error("failed to create channel", err, topic)
		return response.ServerError(fmt.Sprintf("failed to create channel for topic %s: %s", topic, err.Error()))
	}

	// Declare the queue if needed
	if options.Config.DeclareQueue {
		q, err := channel.QueueDeclare(
			topic,
			options.Config.Durable,        // durable
			options.Config.AutoDelete,     // auto-delete
			options.Config.QueueExclusive, // exclusive
			false,                         // no-wait
			nil,                           // arguments
		)
		if err != nil {
			c.Log.Error("failed to declare queue", err, topic)
			return response.ServerError(fmt.Sprintf("failed to declare queue for topic %s: %s", topic, err.Error()))
		}
		topic = q.Name
	}

	tag := topic + fmt.Sprintf("_%d", len(c.mapChannels[topic])+1)
	msgs, err := channel.Consume(
		topic,                            // queue
		tag,                              // consumer
		autoAck,                          // auto-ack
		options.Config.ConsumerExclusive, // exclusive
		false,                            // no-local
		false,                            // no-wait
		nil,                              // args
	)
	if err != nil {
		c.Log.Error("failed to consume messages from queue", err, topic)
		return response.ServerError(fmt.Sprintf("failed to consume messages from queue %s: %s", topic, err.Error()))
	}

	handler = c.applyMiddlewares(handler)
	process := c.Process(handler)
	go func() {
		for d := range msgs {
			process(&d)
		}
	}()

	if _, exists := c.mapChannels[topic]; !exists {
		c.mapChannels[topic] = []*amqp.Channel{}
	}
	c.mapChannels[topic] = append(c.mapChannels[topic], channel)

	return nil
}

func (c *Connection) SubscribeChan(topic string, options queue.Options[SubChan], handler queue.HandlerFunc) *response.AppError {
	if options.Concurrent <= 0 {
		options.Concurrent = 10
	}

	for range options.Concurrent {
		err := c.Subscribe(topic, queue.Options[SubBase]{
			AutoAck: options.AutoAck,
			Config:  options.Config.SubBase,
		}, handler)
		if err != nil {
			c.Log.Error("failed to subscribe to topic", err, topic)
			return err
		}
	}

	return nil
}

func (c *Connection) recover(ctx context.Context, msg *amqp.Delivery) func(r any, stack []byte) *response.AppError {
	return func(r any, stack []byte) *response.AppError {
		c.Log.ErrorPanic(request.GetReqCtx(ctx), fmt.Sprintf("Recovered from panic in RabbitMQ handler: %s", msg.RoutingKey), r, string(stack))
		return response.ServerError(fmt.Sprintf("panic in RabbitMQ handler: %s", msg.RoutingKey))
	}
}

func (c *Connection) Process(handler queue.HandlerFunc) func(d *amqp.Delivery) {
	return func(d *amqp.Delivery) {
		var err error
		defer func() {
			if err != nil {
				c.Log.Error("RabbitMQ error", err, d)
			}
		}()

		headers := &queue.Header{}
		ctx := buildCtx(d.Headers)
		message := queue.Message{
			ID:      d.MessageId,
			Body:    d.Body,
			Header:  headers,
			Topic:   d.RoutingKey,
			Recover: c.recover(ctx, d),
		}

		// process business
		errApp := handler(ctx, &message)

		if errApp != nil {
			err = d.Nack(false, true)
		} else {
			err = d.Ack(false)
		}
	}
}

func buildCtx(header amqp.Table) context.Context {
	ctx := context.Background()

	// Set request ID if present
	if requestID, exists := header[string(constants.REQUEST_ID_KEY)]; exists {
		if id, ok := requestID.(string); ok {
			ctx = context.WithValue(ctx, constants.REQUEST_ID_KEY, id)
		}
	}

	if reqCtxAny, exists := header[string(constants.REQUEST_CONTEXT_KEY)]; exists {
		if reqCtxStr, ok := reqCtxAny.(string); ok {
			if reqCtx := decodeRequestContext(reqCtxStr); reqCtx != nil {
				ctx = context.WithValue(ctx, constants.REQUEST_CONTEXT_KEY, reqCtx)
				return ctx
			}
		}
	}

	// Set default request context
	defaultReqCtx := request.BuildRequestContext(nil, &request.UserInfo[any]{})
	return context.WithValue(ctx, constants.REQUEST_CONTEXT_KEY, defaultReqCtx)
}

func decodeRequestContext(encoded string) *request.ReqContext {
	reqCtxBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}

	reqCtx := &request.ReqContext{}
	if err := sonic.Unmarshal(reqCtxBytes, reqCtx); err != nil {
		return nil
	}

	return reqCtx
}

// applyMiddlewares applies middleware chain to the handler
func (c *Connection) applyMiddlewares(handler queue.HandlerFunc) queue.HandlerFunc {
	middlewares := c.Middlewares()
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

func (c *Connection) Publish(ctx context.Context, topic string, msg *queue.Message) *response.AppError {
	return nil
}

func (c *Connection) HealthCheck() *response.AppError {
	if c.conn == nil {
		return response.ServerError("RabbitMQ connection is nil")
	}

	if c.conn.IsClosed() {
		return response.ServerError("RabbitMQ connection is closed")
	}

	unhealthyChannels := []string{}
	for topic, channels := range c.mapChannels {
		for idx, channel := range channels {
			if channel.IsClosed() {
				channelName := fmt.Sprintf("%s_%d", topic, idx+1)
				unhealthyChannels = append(unhealthyChannels, channelName)
			}
		}
	}

	if len(unhealthyChannels) > 0 {
		return response.ServerError(fmt.Sprintf("unhealthy channels found: %s", strings.Join(unhealthyChannels, ", ")))
	}

	c.Log.Info("RabbitMQ health check passed")
	return nil
}

func (c *Connection) RemoveChannel(topic string, numberChannelRemove int) {
	if channels, ok := c.mapChannels[topic]; ok {
		if numberChannelRemove >= len(channels) {
			channels, ok := c.mapChannels[topic]
			if !ok {
				return
			}
			for _, channel := range channels {
				if err := channel.Close(); err != nil {
					c.Log.Error("failed to close channel", err, topic)
				} else {
					c.Log.Info("Closed channel", topic)
				}
			}
			return
		}

		for i := range numberChannelRemove {
			channel := channels[i]
			if err := channel.Close(); err != nil {
				c.Log.Error("failed to close channel", err, topic)
			} else {
				c.Log.Info("Closed channel", topic)
			}
		}
		c.mapChannels[topic] = make([]*amqp.Channel, len(channels)-numberChannelRemove)
		copy(c.mapChannels[topic], channels[numberChannelRemove:])
	}
}
