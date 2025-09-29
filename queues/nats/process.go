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
	"golang.org/x/sync/errgroup"
)

// JetStream subscription
func (c *Connection) Subscribe(topic string, options queue.Options[SubJSOption], handler queue.HandlerFunc) *response.AppError {
	ctx := context.Background()

	stream, ok := c.mapTopic[topic]
	if !ok {
		return response.ServerError(fmt.Sprintf("stream %s not found", topic))
	}

	s, _ := stream.CreateOrUpdateConsumer(ctx, buildConsumerConfig(options.Config))

	handler = c.applyMiddlewares(handler, options.Config.IsNoRecovery)

	if options.Config.PullMaxMessages <= 0 {
		options.Config.PullMaxMessages = 100
	}

	if options.Concurrent == 0 {
		options.Concurrent = 1
	}

	handlerMsg := c.jsHandleMsg(handler, &options.Config)
	for range options.Concurrent {
		conn, err := s.Consume(handlerMsg, jetstream.PullMaxMessages(options.Config.PullMaxMessages))
		if err != nil {
			c.Log.Error("Failed to subscribe to topic", err, topic)
			return response.ServerError(fmt.Sprintf("failed to subscribe to topic %s: %s", topic, err.Error()))
		}
		c.subscriptionJS = append(c.subscriptionJS, conn)
	}

	return nil
}

func (c *Connection) SubscribeMessage(topic string, options queue.Options[SubJSOption], handler queue.HandlerFunc) *response.AppError {
	ctx := context.Background()

	stream, ok := c.mapTopic[topic]
	if !ok {
		return response.ServerError(fmt.Sprintf("stream %s not found", topic))
	}

	s, err := stream.CreateOrUpdateConsumer(ctx, buildConsumerConfig(options.Config))
	if err != nil {
		c.Log.Error("Failed to create consumer", err, topic)
		return response.ServerError(fmt.Sprintf("failed to create consumer for topic %s: %s", topic, err.Error()))
	}

	handler = c.applyMiddlewares(handler, options.Config.IsNoRecovery)

	if options.Config.PullMaxMessages <= 0 {
		options.Config.PullMaxMessages = 100
	}

	it, err := s.Messages(jetstream.PullMaxMessages(options.Config.PullMaxMessages))
	if err != nil {
		c.Log.Error("Failed to subscribe to topic", err, topic)
		return response.ServerError(fmt.Sprintf("failed to subscribe to topic %s: %s", topic, err.Error()))
	}

	handlerMsg := c.jsHandleMsg(handler, &options.Config)

	if options.Concurrent > 1 {
		eg, _ := errgroup.WithContext(ctx)
		eg.SetLimit(int(options.Concurrent))
		go func() {
			for range options.Concurrent {
				eg.Go(func() error {
					for {
						msg, err := it.Next()
						if err != nil {
							return err
						}
						handlerMsg(msg)
					}
				})
			}
			// Wait for all goroutines to finish
			// This will block until all goroutines complete
			err := eg.Wait()
			if err != nil {
				c.Log.Error("Error waiting for goroutines nats", err)
			}
		}()
	} else {
		go func() {
			for {
				msg, err := it.Next()
				if err != nil {
					return
				}
				handlerMsg(msg)
			}
		}()
	}

	c.subscriptionJSMsg = append(c.subscriptionJSMsg, it)

	return nil
}

func (c *Connection) SubscribeManage(topic string, options queue.Options[SubJSOption], handler queue.HandlerFunc) *response.AppError {
	ctx := context.Background()

	s, err := c.js.CreateOrUpdateConsumer(ctx, topic, buildConsumerConfig(options.Config))
	if err != nil {
		c.Log.Error("Failed to create consumer", err, topic)
		return nil
	}

	handler = c.applyMiddlewares(handler, options.Config.IsNoRecovery)
	conn, err := s.Consume(c.jsHandleMsg(handler, &options.Config))
	if err != nil {
		c.Log.Error("Failed to subscribe to topic", err, topic)
		return response.ServerError(fmt.Sprintf("failed to subscribe to topic %s: %s", topic, err.Error()))
	}

	c.subscriptionJS = append(c.subscriptionJS, conn)

	return nil
}

func (c *Connection) jsHandleMsg(handler queue.HandlerFunc, options *SubJSOption) jetstream.MessageHandler {
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
			Header:  header,
			Topic:   msg.Subject(),
			Recover: c.recoverJSMsg(ctx, msg, options),
		}

		errApp := DeBodyCompression(&message)

		if errApp != nil {
			c.Log.Error("Failed to decompress message body", errApp, msg.Subject())
			return
		}

		errApp = handler(ctx, &message)

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

		} else if options.Delay > 0 {
			err = msg.NakWithDelay(options.Delay)
		} else {
			err = msg.Nak()
		}

		if err != nil {
			c.Log.Error("Failed to nack message", err, msg.Subject)
			return
		}
	}
}

func (c *Connection) recoverJSMsg(ctx context.Context, msg jetstream.Msg, _ *SubJSOption) func(r any, stack []byte) *response.AppError {
	return func(r any, stack []byte) *response.AppError {
		c.Log.ErrorPanic(request.GetReqCtx(ctx), fmt.Sprintf("Recovered from panic in NATS JetStream handler: %s", msg.Subject()), r, string(stack))
		return response.ServerError(fmt.Sprintf("panic in NATS JetStream handler: %s", msg.Subject()))
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
	handler = c.applyMiddlewares(handler, options.Config.IsNoRecovery)
	if options.Config.Group != "" {
		sub, err = c.conn.ChanQueueSubscribe(topic, options.Config.Group, chanMsg)
	} else {
		sub, err = c.conn.ChanSubscribe(topic, chanMsg)
	}

	if err != nil {
		c.Log.Error("Failed to subscribe to topic", err, topic)
		return response.ServerError(fmt.Sprintf("failed to subscribe to topic %s: %s", topic, err.Error()))
	}
	handlerMsg := c.natHandlerMsg(handler)

	if options.Concurrent > 1 {
		eg, _ := errgroup.WithContext(context.Background())
		eg.SetLimit(int(options.Concurrent))
		go func() {
			for range options.Concurrent {
				eg.Go(func() error {
					for msg := range chanMsg {
						handlerMsg(msg)
					}
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
				handlerMsg(msg)
			}
		}()
	}

	c.subscription = append(c.subscription, sub)
	return nil
}

func (c *Connection) SubscribeNor(topic string, options queue.Options[SubOption], handler queue.HandlerFunc) *response.AppError {
	var err error
	var sub *nats.Subscription

	handler = c.applyMiddlewares(handler, options.Config.IsNoRecovery)
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

func (c *Connection) recoverNatMsg(ctx context.Context, msg *nats.Msg) func(r any, stack []byte) *response.AppError {
	return func(r any, stack []byte) *response.AppError {
		c.Log.ErrorPanic(request.GetReqCtx(ctx), fmt.Sprintf("Recovered from panic in NATS handler: %s", msg.Subject), r, string(stack))
		return response.ServerError(fmt.Sprintf("panic in NATS handler: %s", msg.Subject))
	}
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
			Header:  header,
			Topic:   msg.Subject,
			Recover: c.recoverNatMsg(ctx, msg),
		}
		errApp := DeBodyCompression(&message)

		if errApp != nil {
			c.Log.Error("Failed to decompress message body", errApp, msg.Subject)
			return
		}

		errApp = handler(ctx, &message)
		var err error
		if msg.Reply != "" {
			if errApp != nil {
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
func (c *Connection) applyMiddlewares(handler queue.HandlerFunc, isNoRecovery bool) queue.HandlerFunc {
	middlewares := queue.DefaultMiddlewares()

	if isNoRecovery {
		middlewares = c.Middlewares()
	} else {
		middlewares = append(middlewares, c.Middlewares()...)
	}

	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

func buildCtx(header nats.Header) context.Context {
	ctx := context.Background()

	if requestID := header.Get(string(constants.REQUEST_ID_KEY)); requestID != "" {
		ctx = context.WithValue(ctx, constants.REQUEST_ID_KEY, requestID)
	}

	reqCtxStr := header.Get(string(constants.REQUEST_CONTEXT_KEY))
	if reqCtxStr == "" {
		reqCtx := request.BuildRequestContext(nil, &request.UserInfo[any]{})
		return context.WithValue(ctx, constants.REQUEST_CONTEXT_KEY, reqCtx)
	}

	reqCtxBytes, err := base64.StdEncoding.DecodeString(reqCtxStr)
	if err != nil {
		return ctx
	}

	reqCtx := &request.ReqContext{}
	if err := sonic.Unmarshal(reqCtxBytes, reqCtx); err != nil {
		return ctx
	}

	return context.WithValue(ctx, constants.REQUEST_CONTEXT_KEY, reqCtx)
}
