package queue

import (
	"context"
	"maps"
	"runtime/debug"
	"sync"

	"github.com/tuan-dd/go-pkg/common/response"
)

type (
	HeaderIf interface {
		Get(key string) string
		Set(key string, value string)
		Add(key string, value string)
	}
	Message struct {
		Topic   string
		ID      string
		Body    []byte
		Headers *Header
		Recover func(r any, stack []byte) *response.AppError
	}

	Header map[string]any

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
		AutoAck    bool
		Concurrent uint8
		Config     T
	}

	PanicLogger struct {
		CID       string
		Topic     string
		TimeStamp int64
		Stack     string
	}

	ErrorFunc   func(ctx context.Context, msg *Message, err error)
	HandlerFunc func(ctx context.Context, msg *Message) *response.AppError

	Middleware func(HandlerFunc) HandlerFunc

	Server[T any] interface {
		Subscribe(topic string, options Options[T], handler HandlerFunc) *response.AppError
		Publish(ctx context.Context, topic string, msg *Message) *response.AppError
		Shutdown() *response.AppError
		Use(m Middleware)
	}

	Client interface {
		Publish(ctx context.Context, topic string, msg *Message) *response.AppError
		Shutdown() *response.AppError
		Use(m Middleware)
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

func (c *QueueServer) Use(mill Middleware) {
	if len(c.middlewares) > 10 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.middlewares = append(c.middlewares, mill)
}

func (c *QueueClient) Use(mill Middleware) {
	if len(c.middlewares) > 10 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.middlewares = append(c.middlewares, mill)
}

func (c *Header) Get(key string) any {
	value, ok := (*c)[key]
	if !ok {
		return ""
	}
	return value
}

func GetHeaderValue[T any](c *Header, key string) T {
	if value, ok := (*c)[key]; ok {
		if v, ok := value.(T); ok {
			return v
		}
	}
	return *new(T)
}

func (c *Header) Set(key string, value any) {
	(*c)[key] = value
}

func (c *Header) Add(key string, value any) {
	if _, ok := (*c)[key]; !ok {
		(*c)[key] = value
	}
}

func (c *Header) Del(key string) {
	delete(*c, key)
}

func (c *Header) Clone() *Header {
	clone := make(Header, len(*c))
	maps.Copy(clone, *c)
	return &clone
}

func NewMessage() *Message {
	return &Message{
		Headers: new(Header),
	}
}

func DefaultRecoverMiddleware(next HandlerFunc) HandlerFunc {
	return func(ctx context.Context, msg *Message) (result *response.AppError) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				if msg.Recover != nil {
					result = msg.Recover(r, stack)
				} else {
					result = response.ServerError("panic in queue handler: " + string(stack))
				}
			}
		}()
		return next(ctx, msg)
	}
}

func DefaultMiddlewares() []Middleware {
	return []Middleware{
		DefaultRecoverMiddleware,
	}
}
