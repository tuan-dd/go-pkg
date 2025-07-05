package natsQueue

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"sync"
	"time"

	"github.com/klauspost/compress/gzip"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	appLogger "github.com/tuan-dd/go-pkg/app-logger"
	"github.com/tuan-dd/go-pkg/common/queue"
	"github.com/tuan-dd/go-pkg/common/response"
)

type (
	QueueConfig struct {
		Host     string        `mapstructure:"HOST"`
		Port     int           `mapstructure:"PORT"`
		Username string        `mapstructure:"USERNAME"`
		Token    string        `mapstructure:"TOKEN"`
		Seed     string        `mapstructure:"SEED"`
		Password string        `mapstructure:"PASSWORD"`
		Topics   []TopicConfig `mapstructure:"TOPICS"`
	}

	TopicConfig struct {
		Name        string        `mapstructure:"NAME"`
		Description string        `mapstructure:"DESCRIPTION"`
		Subjects    []string      `mapstructure:"SUBJECTS"`
		MaxMsgs     int64         `mapstructure:"MAX_MSGS"`
		MaxAge      time.Duration `mapstructure:"MAX_AGE"` // "24h", "30m"
		MaxBytes    int64         `mapstructure:"MAX_BYTES"`
		Storage     int           `mapstructure:"STORAGE"`   // "file" hoặc "memory"
		Retention   int           `mapstructure:"RETENTION"` // "limits", "interest", "work queue"
		Replicas    int           `mapstructure:"REPLICAS"`
	}

	BasicJSOption struct {
		Group          string
		AckPolicy      jetstream.AckPolicy
		MaxDeliver     int
		AckWait        time.Duration
		PriorityPolicy jetstream.PriorityPolicy
		DeliverPolicy  jetstream.DeliverPolicy
		FilterSubject  string
		MaxAckPending  int
		Delay          time.Duration
		IsNoRecovery   bool
	}
	SubJSOption struct {
		BasicJSOption
		Group           string
		PullMaxMessages int
	}

	SubOption struct {
		IsNoRecovery bool
		Group        string
	}

	SubChanOption struct {
		IsNoRecovery bool
		Group        string
		ChanNumber   int8
	}

	PubOption struct {
		Topic   string
		Durable time.Duration
	}

	PubJsOption struct {
		Topic   string
		Durable time.Duration
	}

	Connection struct {
		queue.QueueServer
		conn         *nats.Conn
		subscription []*nats.Subscription

		// For JetStream
		subscriptionJS    []jetstream.ConsumeContext
		subscriptionJSMsg []jetstream.MessagesContext
		mapTopic          map[string]jetstream.Stream
		js                jetstream.JetStream
		Log               *appLogger.Logger
	}
	NatsKey         string
	CompressionType string
)

const (
	NatsMSGID   NatsKey = "Nats-Msg-Id"
	Compression NatsKey = "Nats-Compression"

	S2Compression CompressionType = "s2"
	// ProtobufCompression CompressionType = "protobuf"
	GzipCompression CompressionType = "gzip"
	NoCompression   CompressionType = "none"
)

var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(nil, gzip.BestSpeed)
		return w
	},
}

func getGzipWriter(dst io.Writer) *gzip.Writer {
	w := gzipPool.Get().(*gzip.Writer)
	w.Reset(dst)
	return w
}

func putGzipWriter(w *gzip.Writer) {
	if w != nil {
		_ = w.Close()
		gzipPool.Put(w)
	}
}

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func getBuffer() *bytes.Buffer {
	b := bufPool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

func putBuffer(b *bytes.Buffer) {
	bufPool.Put(b)
}

func NewConnection(cfg *QueueConfig, log *appLogger.Logger) (*Connection, *response.AppError) {
	var opts []nats.Option
	dns := fmt.Sprintf("nats://%s:%d", cfg.Host, cfg.Port)

	// Configure authentication
	if cfg.Seed != "" {
		opts = append(opts, nats.UserJWTAndSeed(cfg.Token, cfg.Seed))
	} else if cfg.Token != "" {
		opts = append(opts, nats.Token(cfg.Token))
	} else {
		dns = fmt.Sprintf("nats://%s:%s@%s:%d", cfg.Username, cfg.Password, cfg.Host, cfg.Port)
	}

	conn, err := nats.Connect(dns, opts...)
	if err != nil {
		log.Error("Failed to connect to NATS", err)
		return nil, response.ServerError(fmt.Sprintf("failed to connect to NATS: %v", err))
	}

	log.Info("Successfully connected to NATS server")
	return &Connection{
		conn: conn,
		Log:  log,
	}, nil
}

func NewConnectWithJetStream(cfg *QueueConfig, log *appLogger.Logger) (*Connection, *response.AppError) {
	var conn *nats.Conn
	var err error
	if len(cfg.Topics) == 0 {
		return nil, response.ServerError("topic is nil or empty")
	}
	if cfg.Token != "" {
		dns := fmt.Sprintf("nats://%s:%d", cfg.Host, cfg.Port)
		conn, err = nats.Connect(dns, nats.Token(cfg.Token))

	} else {
		dns := fmt.Sprintf("nats://%s:%s@%s:%d", cfg.Username, cfg.Password, cfg.Host, cfg.Port)
		conn, err = nats.Connect(dns)
	}
	if err != nil {
		log.Error("Failed to connect to NATS", err)
		return nil, response.ServerError("failed to connect nats " + err.Error())
	}

	js, err := jetstream.New(conn)
	if err != nil {
		log.Error("Failed to create jetstream", err)
		return nil, response.ServerError("failed to create jetstream " + err.Error())
	}

	mapTopic := make(map[string]jetstream.Stream, len(cfg.Topics))

	for _, topic := range cfg.Topics {
		stream, err := js.Stream(context.Background(), topic.Name)
		if err != nil {
			stream, err = js.CreateStream(context.Background(), jetstream.StreamConfig{
				Name:        topic.Name,
				Description: topic.Description,
				Subjects:    topic.Subjects,
				MaxMsgs:     topic.MaxMsgs,
				Compression: jetstream.S2Compression,
				MaxAge:      topic.MaxAge,
				MaxBytes:    topic.MaxBytes,
				Storage:     jetstream.StorageType(topic.Storage),
				Retention:   jetstream.RetentionPolicy(topic.Retention),
				Replicas:    topic.Replicas,
			})
			if err != nil {
				return nil, response.ServerError(fmt.Sprintf("failed to create jetstream stream %s: %s", topic.Name, err.Error()))
			}
		}
		mapTopic[topic.Name] = stream
	}

	log.Info("Successfully connected to NATS JetStream server")
	return &Connection{conn: conn, js: js, Log: log, mapTopic: mapTopic}, nil
}

func (c *Connection) Shutdown() *response.AppError {
	defer func() {
		if r := recover(); r != nil {
			c.Log.Warn("Recovered from panic during shutdown", r, string(debug.Stack()))
		}
	}()
	// First, unsubscribe from all subscriptions to stop receiving new messages

	if len(c.subscriptionJSMsg) > 0 {
		for _, sub := range c.subscriptionJSMsg {
			sub.Stop()
		}
	}

	if len(c.subscriptionJS) > 0 {
		for _, sub := range c.subscriptionJS {
			sub.Stop()
		}
	}

	for _, sub := range c.subscription {
		if err := sub.Unsubscribe(); err != nil {
			c.Log.Error("Failed to unsubscribe", err)
			// Continue with shutdown even if unsubscribe fails
		}
	}

	// Drain the connection to process remaining messages gracefully
	if err := c.conn.Drain(); err != nil {
		c.Log.Error("Nats drain error", err)
		// Force close if drain fails
		c.conn.Close()
		return response.ServerError(fmt.Sprintf("failed to drain nats connection: %s", err.Error()))
	}

	// Connection is automatically closed after successful drain
	return nil
}

func buildConsumerConfig(cfg BasicJSOption) jetstream.ConsumerConfig {
	return jetstream.ConsumerConfig{
		Durable:        cfg.Group,
		AckPolicy:      jetstream.AckExplicitPolicy,
		MaxDeliver:     cfg.MaxDeliver,
		AckWait:        cfg.AckWait,
		PriorityPolicy: cfg.PriorityPolicy,
		DeliverPolicy:  cfg.DeliverPolicy,
		FilterSubject:  cfg.FilterSubject,
		MaxAckPending:  cfg.MaxAckPending,
	}
}

func (c *Connection) DeleteAllStreams() *response.AppError {
	streams := c.js.StreamNames(context.Background())

	for streamName := range streams.Name() {
		if err := c.js.DeleteStream(context.Background(), streamName); err != nil {
			c.Log.Error(fmt.Sprintf("Failed to delete stream %s", streamName), err)
			return response.ServerError(fmt.Sprintf("failed to delete stream %s: %s", streamName, err.Error()))
		}
		c.Log.Info(fmt.Sprintf("Deleted stream %s successfully", streamName))
	}
	c.Log.Info("All streams deleted successfully")

	c.subscriptionJS = nil
	return nil
}

func (c *Connection) DeleteStream(name string) *response.AppError {
	if _, ok := c.mapTopic[name]; !ok {
		return response.ServerError(fmt.Sprintf("stream %s not found", name))
	}
	if err := c.js.DeleteStream(context.Background(), name); err != nil {
		c.Log.Error(fmt.Sprintf("Failed to delete stream %s", name), err)
		return response.ServerError(fmt.Sprintf("failed to delete stream %s: %s", name, err.Error()))
	}

	return nil
}
