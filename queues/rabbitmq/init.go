package rabbitmq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/samber/lo"
	"github.com/tuan-dd/go-pkg/appLogger"
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
	client *amqp.Channel
	conn   *amqp.Connection
	cfg    *QueueConfig
	Log    *appLogger.Logger
}

type Options struct {
	Concurrency int
}

func NewConnection(cfg *QueueConfig, log *appLogger.Logger) (*Connection, *response.AppError) {
	return connect(fmt.Sprintf("amqp://%s:%s@%s:%d", cfg.Username, cfg.Password, cfg.Host, cfg.Port), cfg, log)
}

func connect(dns string, cfg *QueueConfig, log *appLogger.Logger) (*Connection, *response.AppError) {
	conn, err := amqp.Dial(dns)
	if err != nil {
		return nil, response.ServerError("failed to connect rabbitmq " + err.Error())
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, response.ServerError("failed to open a channel " + err.Error())
	}

	return &Connection{client: ch, cfg: cfg, conn: conn, Log: log}, nil
}

func (c *Connection) Shutdown() *response.AppError {
	err := c.client.Close()
	if err != nil {
		return response.ServerError(err.Error())
	}

	err = c.conn.Close()
	if err != nil {
		return response.ServerError(err.Error())
	}

	return nil
}

func (c *Connection) Subscribe(topic string, options queue.Options[Options], handler queue.HandlerFunc) *response.AppError {
	autoAck := options.AutoAck

	q, err := c.client.QueueDeclare(
		topic,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return response.ServerError(err.Error())
	}

	msgs, err := c.client.Consume(
		q.Name,  // queue
		"",      // consumer
		autoAck, // auto-ack
		false,   // exclusive
		false,   // no-local
		false,   // no-wait
		nil,     // args
	)
	if err != nil {
		return response.ServerError(err.Error())
	}

	go func() {
		for d := range msgs {
			c.Process(&d, handler)
		}
	}()

	return nil
}

func (c *Connection) Process(d *amqp.Delivery, handler queue.HandlerFunc) {
	var err error
	defer func() {
		if err != nil {
			c.Log.Error("RabbitMQ error", err, d)
		}
	}()

	headers := map[string]any(d.Headers)
	ctx := buildCtx(d.Headers)
	message := queue.Message{
		ID:      d.MessageId,
		Body:    d.Body,
		Headers: &headers,
	}

	middlewares := c.Middlewares()
	if len(middlewares) > 0 {
		for i := len(middlewares) - 1; i >= 0; i-- {
			handler = middlewares[i](handler)
		}
	}

	// process business
	errApp := handler(ctx, &message)
	if err != nil {
		return
	}

	if errApp != nil {
		c.Log.Error("RabbitMQ error", errApp, d)
		err = d.Nack(false, true)
	}
	err = d.Ack(false)
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

// TODO
func (c *Connection) Publish(ctx context.Context, topic string, msg *queue.Message) *response.AppError {
	err := c.client.Publish(topic, "", false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        msg.Body,
		Headers:     lo.FromPtr(msg.Headers),
	})
	if err != nil {
		return response.ServerError(err.Error())
	}
	return nil
}
