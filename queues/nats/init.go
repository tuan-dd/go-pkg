package natsQueue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/samber/lo"

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

type Options struct {
	group   []string
	ChanSub *int
}

type Connection struct {
	queue.QueueServer
	conn         *nats.Conn
	subscription []*nats.Subscription
	// subscriptionCh chan *nats.Msg
	js  jetstream.JetStream
	Log *appLogger.Logger
}

func NewConnection(cfg *QueueConfig, log *appLogger.Logger) (*Connection, *response.AppError) {
	return connect(fmt.Sprintf("amqp://%s:%s@%s:%d", cfg.Username, cfg.Password, cfg.Host, cfg.Port), log)
}

func connect(dns string, log *appLogger.Logger) (*Connection, *response.AppError) {
	conn, err := nats.Connect(dns)

	js, _ := jetstream.New(conn)
	if err != nil {
		return nil, response.ServerError("failed to connect nats " + err.Error())
	}

	return &Connection{conn: conn, js: js, Log: log}, nil
}

func (c *Connection) Shutdown() *response.AppError {
	defer func() {
		if err := c.conn.Drain(); err != nil {
			c.Log.Error("Nats error", err)
		}
	}()
	for _, sub := range c.subscription {
		if err := sub.Unsubscribe(); err != nil {
			return response.ServerError(err.Error())
		}
	}
	c.conn.Close()
	return nil
}

func (c *Connection) Subscribe(topic string, options queue.Options[Options], handler queue.HandlerFunc) *response.AppError {
	ctx := context.Background()
	stream, err := c.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     topic,
		Subjects: options.Config.group,
	})
	if err != nil {
		return response.ServerError(err.Error())
	}
	cons, _ := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{})
	_, _ = cons.Consume(func(msg jetstream.Msg) {
		c.Process(msg, handler)
	})

	return nil
}

func (c *Connection) Process(d jetstream.Msg, handler queue.HandlerFunc) {
	var err error
	defer func() {
		if err != nil {
			c.Log.Error("Nats error", err, d)
		}
	}()

	headers := map[string]any{}

	for k, v := range d.Headers() {
		data, err := json.Marshal(v[0])
		if err != nil {
			return
		}
		headers[k] = data
	}
	ctx := buildCtx(d.Headers())
	message := queue.Message{
		ID:      d.InProgress().Error(),
		Body:    []byte(d.Reply()),
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
		c.Log.Error("Nats error", errApp, d)
		err = d.Nak()
	}

	err = d.Ack()
}

func buildCtx(header nats.Header) context.Context {
	ctx := context.Background()

	if header.Get(string(constants.CORRELATION_ID_KEY)) != "" {
		ctx = context.WithValue(ctx, constants.CORRELATION_ID_KEY, header.Get(string(constants.CORRELATION_ID_KEY)))
	}

	if header.Get(string(constants.REQUEST_CONTEXT_KEY)) != "" {

		reqCtx := json.Unmarshal([]byte(header.Get(string(constants.REQUEST_CONTEXT_KEY))), &common.ReqContext{})
		ctx = context.WithValue(ctx, constants.REQUEST_CONTEXT_KEY, reqCtx)
	}

	return ctx
}

func (c *Connection) Publish(ctx context.Context, topic string, msg *queue.Message) *response.AppError {
	headers := make(map[string][]string)
	for k, v := range lo.FromPtr(msg.Headers) {
		data, err := json.Marshal(v)
		if err != nil {
			return response.ServerError(err.Error())
		}
		headers[k] = []string{string(data)}
	}
	err := c.conn.PublishMsg(&nats.Msg{Subject: topic, Data: msg.Body, Header: headers})
	if err != nil {
		return response.ServerError(err.Error())
	}
	return nil
}
