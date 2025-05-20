package queue

import (
	"context"
	"sync"

	"github.com/tuan-dd/go-pkg/common/response"
)

type (
	Message struct {
		ID      string
		Body    []byte
		Headers *map[string]any
	}

	QueueServer struct {
		ErrorFunc ErrorFunc

		// internal
		mu          sync.RWMutex
		middlewares []Middleware
	}

	QueueClient struct {
		ErrorFunc ErrorFunc
		// internal
		mu          sync.RWMutex
		middlewares []Middleware
	}

	Options[T any] struct {
		AutoAck bool
		Config  T
	}

	ErrorFunc   func(ctx context.Context, msg *Message, err error)
	HandlerFunc func(ctx context.Context, msg *Message) *response.AppError

	Middleware func(HandlerFunc) HandlerFunc

	Server[T any] interface {
		Subscribe(topic string, options Options[T], handler HandlerFunc) *response.AppError
		Publish(ctx context.Context, topic string, msg *Message) *response.AppError
		Shutdown() *response.AppError
		Use(m Middleware) *response.AppError
	}

	Client interface {
		Publish(ctx context.Context, topic string, msg *Message) *response.AppError
		Shutdown() *response.AppError
		Use(m Middleware) *response.AppError
	}
)

// func (fn HandlerFunc) ProcessTask(ctx context.Context, msg *Message) error {
// 	return fn(ctx, msg)
// }

func (q *QueueClient) Middlewares() []Middleware {
	return q.middlewares
}

func (q *QueueServer) Middlewares() []Middleware {
	return q.middlewares
}

func (c *QueueServer) Use(mill Middleware) *response.AppError {
	if len(c.middlewares) > 10 {
		return response.ServerError("too many middlewares")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.middlewares = append(c.middlewares, mill)
	return nil
}

func (c *QueueClient) Use(mill Middleware) *response.AppError {
	if len(c.middlewares) > 10 {
		return response.ServerError("too many middlewares")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.middlewares = append(c.middlewares, mill)

	return nil
}
