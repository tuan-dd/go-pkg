package natsQueue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tuan-dd/go-pkg/common"
	"github.com/tuan-dd/go-pkg/common/constants"
	"github.com/tuan-dd/go-pkg/common/queue"
	"github.com/tuan-dd/go-pkg/common/response"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"golang.org/x/sync/errgroup"
)

// JetStream subscription
func (c *Connection) Subscribe(topic string, options queue.Options[SubJSOption], handler queue.HandlerFunc) *response.AppError {
	ctx := context.Background()

	stream, ok := c.mapTopic[topic]
	if !ok {
		return response.ServerError(fmt.Sprintf("stream %s not found", topic))
	}

	s, _ := stream.CreateOrUpdateConsumer(ctx, buildConsumerConfig(options.Config.BasicJSOption))

	handler = c.applyMiddlewares(handler)

	if options.Config.PullMaxMessages <= 0 {
		options.Config.PullMaxMessages = 100
	}
	conn, err := s.Consume(c.jsHandleMsg(handler, options.Config.BasicJSOption), jetstream.PullMaxMessages(options.Config.PullMaxMessages))
	if err != nil {
		c.Log.Error("Failed to subscribe to topic", err, topic)
		return response.ServerError(fmt.Sprintf("failed to subscribe to topic %s: %s", topic, err.Error()))
	}

	c.subscriptionJS = append(c.subscriptionJS, conn)

	return nil
}

func (c *Connection) SubscribeChan(topic string, options queue.Options[SubJSOption], handler queue.HandlerFunc) *response.AppError {
	for range options.Concurrent {
		err := c.Subscribe(topic, options, handler)
		if err != nil {
			c.Log.Error("Failed to subscribe to topic", err, topic)
			return response.ServerError(fmt.Sprintf("failed to subscribe to topic %s: %s", topic, err.Error()))
		}
	}

	return nil
}

func (c *Connection) SubscribeMessage(topic string, options queue.Options[SubJSOption], handler queue.HandlerFunc) *response.AppError {
	ctx := context.Background()
	s, err := c.js.CreateOrUpdateConsumer(ctx, topic, buildConsumerConfig(options.Config.BasicJSOption))
	if err != nil {
		c.Log.Error("Failed to create consumer", err, topic)
		return response.ServerError(fmt.Sprintf("failed to create consumer for topic %s: %s", topic, err.Error()))
	}
	handler = c.applyMiddlewares(handler)

	if options.Config.PullMaxMessages <= 0 {
		options.Config.PullMaxMessages = 100
	}

	it, err := s.Messages(jetstream.PullMaxMessages(options.Config.PullMaxMessages))
	if err != nil {
		c.Log.Error("Failed to subscribe to topic", err, topic)
		return response.ServerError(fmt.Sprintf("failed to subscribe to topic %s: %s", topic, err.Error()))
	}

	if options.Concurrent > 1 {
		eg, _ := errgroup.WithContext(ctx)
		eg.SetLimit(int(options.Concurrent))
		go func() {
			for {
				msg, err := it.Next()
				if err != nil {
					err := eg.Wait()
					if err != nil {
						c.Log.Error("Error waiting for goroutines nats", err)
					}
					return
				}
				eg.Go(func() error {
					c.jsHandleMsg(handler, options.Config.BasicJSOption)(msg)

					return nil
				})
			}
		}()
	} else {
		go func() {
			for {
				msg, err := it.Next()
				if err != nil {
					return
				}
				c.jsHandleMsg(handler, options.Config.BasicJSOption)(msg)
			}
		}()
	}

	c.subscriptionJSMsg = append(c.subscriptionJSMsg, it)

	return nil
}

func (c *Connection) SubscribeManage(topic string, options queue.Options[SubJSOption], handler queue.HandlerFunc) *response.AppError {
	ctx := context.Background()

	s, err := c.js.CreateOrUpdateConsumer(ctx, topic, buildConsumerConfig(options.Config.BasicJSOption))
	if err != nil {
		c.Log.Error("Failed to create consumer", err, topic)
		return nil
	}

	handler = c.applyMiddlewares(handler)
	conn, err := s.Consume(c.jsHandleMsg(handler, options.Config.BasicJSOption))
	if err != nil {
		c.Log.Error("Failed to subscribe to topic", err, topic)
		return response.ServerError(fmt.Sprintf("failed to subscribe to topic %s: %s", topic, err.Error()))
	}

	c.subscriptionJS = append(c.subscriptionJS, conn)

	return nil
}

func (c *Connection) jsHandleMsg(handler queue.HandlerFunc, options BasicJSOption) jetstream.MessageHandler {
	return func(msg jetstream.Msg) {
		var err error

		header := &queue.Header{}

		natHeader := msg.Headers()
		for k, v := range natHeader {
			header.Add(k, v[0])
		}
		ctx := buildCtx(natHeader)
		message := queue.Message{
			ID:      natHeader.Get(string(NatsMSGID)),
			Body:    msg.Data(),
			Headers: header,
		}

		errApp := handler(ctx, &message)
		if errApp == nil {
			err = msg.Ack()
			if err != nil {
				c.Log.Error("Failed to ack message", err, msg.Subject)
			}
			return
		}

		// Error handling based on delivery attempts
		meta, err := msg.Metadata()
		if err != nil || meta == nil {
			c.Log.Error("Failed to get message metadata", err, msg.Subject)
			err = msg.Nak()

		} else if meta.NumDelivered > 1 {
			err = msg.NakWithDelay(options.Delay)
		} else {
			err = msg.Nak() // First failure, reject without delay
		}

		if err != nil {
			c.Log.Error("Failed to nack message", err, msg.Subject)
			return
		}
	}
}

// NATS subscription
func (c *Connection) SubscribeChanNor(topic string, options queue.Options[SubChanOption], handler queue.HandlerFunc) *response.AppError {
	var err error
	var sub *nats.Subscription

	if options.Config.ChanNumber <= 0 {
		options.Config.ChanNumber = 10
	}

	chanMsg := make(chan *nats.Msg, options.Config.ChanNumber)
	handler = c.applyMiddlewares(handler)
	if options.Config.Group != "" {
		sub, err = c.conn.ChanQueueSubscribe(topic, options.Config.Group, chanMsg)
	} else {
		sub, err = c.conn.ChanSubscribe(topic, chanMsg)
	}

	if err != nil {
		c.Log.Error("Failed to subscribe to topic", err, topic)
		return response.ServerError(fmt.Sprintf("failed to subscribe to topic %s: %s", topic, err.Error()))
	}

	if options.Concurrent > 1 {
		eg, _ := errgroup.WithContext(context.Background())
		eg.SetLimit(int(options.Concurrent))
		go func() {
			for msg := range chanMsg {
				eg.Go(func() error {
					c.natHandlerMsg(handler)(msg)
					return nil
				})
			}
			err := eg.Wait()
			if err != nil {
				c.Log.Error("Error waiting for goroutines nats", err)
			}
		}()
	} else {
		go func() {
			for msg := range chanMsg {
				c.natHandlerMsg(handler)(msg)
			}
		}()
	}

	c.subscription = append(c.subscription, sub)
	return nil
}

func (c *Connection) SubscribeNor(topic string, options queue.Options[SubOption], handler queue.HandlerFunc) *response.AppError {
	var err error
	var sub *nats.Subscription

	handler = c.applyMiddlewares(handler)
	if options.Config.Group != "" {
		sub, err = c.conn.QueueSubscribe(topic, options.Config.Group, c.natHandlerMsg(handler))
	} else {
		sub, err = c.conn.Subscribe(topic, c.natHandlerMsg(handler))
	}

	if err != nil {
		c.Log.Error("Failed to subscribe to topic", err, topic)
		return response.ServerError(fmt.Sprintf("failed to subscribe to topic %s: %s", topic, err.Error()))
	}
	c.subscription = append(c.subscription, sub)

	return nil
}

func (c *Connection) natHandlerMsg(handler queue.HandlerFunc) nats.MsgHandler {
	return func(msg *nats.Msg) {
		header := &queue.Header{}

		natHeader := msg.Header
		for k, v := range natHeader {
			header.Add(k, v[0])
		}

		ctx := buildCtx(natHeader)
		message := queue.Message{
			ID:      natHeader.Get(string(NatsMSGID)),
			Body:    msg.Data,
			Headers: header,
		}
		errApp := handler(ctx, &message)
		var err error
		if errApp != nil {
			if msg.Reply != "" {
				err = msg.Nak()
			} else {
				err = msg.Ack()
			}
		}

		if err != nil {
			c.Log.Error("Error processing message", err, msg)
		}
	}
}

// applyMiddlewares applies middleware chain to the handler
func (c *Connection) applyMiddlewares(handler queue.HandlerFunc) queue.HandlerFunc {
	middlewares := c.Middlewares()
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

func buildCtx(header nats.Header) context.Context {
	ctx := context.Background()

	if correlationID := header.Get(string(constants.CORRELATION_ID_KEY)); correlationID != "" {
		ctx = context.WithValue(ctx, constants.CORRELATION_ID_KEY, correlationID)
	}

	if reqCtxStr := header.Get(string(constants.REQUEST_CONTEXT_KEY)); reqCtxStr != "" {
		var reqCtx common.ReqContext
		if err := json.Unmarshal([]byte(reqCtxStr), &reqCtx); err == nil {
			ctx = context.WithValue(ctx, constants.REQUEST_CONTEXT_KEY, &reqCtx)
		}
	}

	return ctx
}
