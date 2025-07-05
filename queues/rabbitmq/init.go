package rabbitmq

import (
	"context"
	"fmt"
	"strings"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/tuan-dd/go-pkg/appLogger"
	"github.com/tuan-dd/go-pkg/common"
	"github.com/tuan-dd/go-pkg/common/constants"
	"github.com/tuan-dd/go-pkg/common/queue"
	"github.com/tuan-dd/go-pkg/common/response"
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
	Log         *appLogger.Logger
	mu          sync.RWMutex
}

type SubBase struct {
	NoDeclareQueue    bool // NoDeclareQueue is used to skip queue declaration
	Durable           bool
	AutoDelete        bool
	QueueExclusive    bool
	ConsumerExclusive bool
	IsNoRecovery      bool // IsNoRecovery is used to skip recovery of the subscription
}
type SubChan struct {
	SubBase
}

func NewConnection(cfg *QueueConfig, log *appLogger.Logger) (*Connection, *response.AppError) {
	return connect(fmt.Sprintf("amqp://%s:%s@%s:%d", cfg.Username, cfg.Password, cfg.Host, cfg.Port), cfg, log)
}

func connect(dns string, cfg *QueueConfig, log *appLogger.Logger) (*Connection, *response.AppError) {
	conn, err := amqp.Dial(dns)
	if err != nil {
		return nil, response.ServerError("failed to connect rabbitmq " + err.Error())
	}

	// Log successful connection
	log.Info("Successfully connected to RabbitMQ server")

	return &Connection{cfg: cfg, conn: conn, Log: log, mapChannels: map[string][]*amqp.Channel{}}, nil
}

//
//func (c *Connection) Shutdown() *response.AppError {
//	c.Log.Info("Starting RabbitMQ shutdown...")
//
//	// First, stop accepting new messages by closing channels
//	for key, channels := range c.mapChannels {
//		if len(channels) == 0 {
//			continue
//		}
//
//		for idx, channel := range channels {
//
//			tag := key + fmt.Sprintf("_%d", idx+1)
//			err := channel.Cancel(tag, false)
//			if err != nil {
//				c.Log.Warn("failed to cancel rabbitmq channel", err, key)
//			}
//			err = channel.Close()
//			if err != nil {
//				if !isChannelClosedError(err) {
//					c.Log.Error("failed to close rabbitmq channel", err, key)
//					return response.ServerError(fmt.Sprintf("failed to close rabbitmq channel %s: %s", key, err.Error()))
//				}
//			}
//			c.Log.Info("Closed RabbitMQ channel", key)
//		}
//		delete(c.mapChannels, key)
//	}
//
//	// Then close the connection
//	if c.conn != nil && !c.conn.IsClosed() {
//		err := c.conn.Close()
//		if err != nil {
//			c.Log.Error("failed to close rabbitmq connection", err)
//			return response.ServerError(fmt.Sprintf("failed to close rabbitmq connection: %s", err.Error()))
//		}
//	}
//
//	c.Log.Info("RabbitMQ shutdown completed")
//	return nil
//}

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

	if options.Config.NoDeclareQueue {
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

	c.mu.Lock()
	defer c.mu.Unlock()
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
		c.Log.ErrorPanic(common.GetReqCtx(ctx), fmt.Sprintf("Recovered from panic in RabbitMQ handler: %s", msg.RoutingKey), r, string(stack))
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
			Headers: headers,
			Topic:   d.RoutingKey,
			Recover: c.recover(ctx, d),
		}

		// process business
		errApp := handler(ctx, &message)

		if errApp != nil {
			err = d.Nack(false, true)
		}
		err = d.Ack(false)
	}
}

func buildCtx(header amqp.Table) context.Context {
	ctx := context.Background()
	if header[string(constants.CORRELATION_ID_KEY)] != nil {
		ctx = context.WithValue(ctx, constants.CORRELATION_ID_KEY, header[string(constants.CORRELATION_ID_KEY)].(string))
	}

	if header[string(constants.REQUEST_CONTEXT_KEY)] != nil {
		ctx = context.WithValue(ctx, constants.REQUEST_CONTEXT_KEY, header[string(constants.REQUEST_CONTEXT_KEY)].(string))
	}

	return ctx
}

// applyMiddlewares applies middleware chain to the handler
func (c *Connection) applyMiddlewares(handler queue.HandlerFunc) queue.HandlerFunc {
	middlewares := c.Middlewares()
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// TODO
func (c *Connection) Publish(ctx context.Context, topic string, msg *queue.Message) *response.AppError {
	// if msg == nil {
	// 	return response.ServerError("message cannot be nil")
	// }
	// header := make(amqp.Table)
	// if msg.Headers == nil {
	// 	maps.Copy(header, *msg.Headers)
	// }

	// err := c.channel.Publish(topic, "", false, false, amqp.Publishing{
	// 	ContentType: "text/plain",
	// 	Body:        msg.Body,
	// 	Headers:     header,
	// })
	// if err != nil {
	// 	return response.ServerError(fmt.Sprintf("failed to publish message to topic %s: %s", topic, err.Error()))
	// }
	return nil
}
