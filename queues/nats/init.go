package natsQueue

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tuan-dd/go-common/queue"
	"github.com/tuan-dd/go-common/response"
	typeCustom "github.com/tuan-dd/go-common/type-custom"
)

type (
	QueueConfig struct {
		Host        string        `mapstructure:"HOST"`
		Port        int           `mapstructure:"PORT"`
		Username    string        `mapstructure:"USERNAME"`
		Token       string        `mapstructure:"TOKEN"`
		Seed        string        `mapstructure:"SEED"`
		NorSubjects []string      `mapstructure:"NOR_SUBJECTS"`
		Password    string        `mapstructure:"PASSWORD"`
		Topics      []TopicConfig `mapstructure:"TOPICS"`
	}

	TopicConfig struct {
		Name         string        `mapstructure:"NAME"`
		UpdateStream bool          `mapstructure:"UPDATE_STREAM"`
		Description  string        `mapstructure:"DESCRIPTION"`
		Subjects     []string      `mapstructure:"SUBJECTS"`
		MaxMsgs      int64         `mapstructure:"MAX_MSGS"`
		MaxAge       time.Duration `mapstructure:"MAX_AGE"` // "24h", "30m"
		MaxBytes     int64         `mapstructure:"MAX_BYTES"`
		Storage      int           `mapstructure:"STORAGE"`   // "file" hoặc "memory"
		Retention    int           `mapstructure:"RETENTION"` // "limits", "interest", "work queue"
		Replicas     int           `mapstructure:"REPLICAS"`
	}

	SubJSOption struct {
		Group           string
		AckPolicy       jetstream.AckPolicy
		MaxDeliver      int
		AckWait         time.Duration
		PriorityPolicy  jetstream.PriorityPolicy
		DeliverPolicy   jetstream.DeliverPolicy
		IncludeSubjects []string
		MaxAckPending   int
		Delay           time.Duration
		IsNoRecovery    bool

		// push
		DeliverSubject string
		FlowControl    bool
		IdleHeartbeat  time.Duration
		DeliverGroup   string // load balancing between consumers in the same group
		BackOff        []time.Duration

		// Pull
		PullMaxMessages    int
		MaxRequestBatch    int
		MaxRequestMaxBytes int
		MaxWaiting         int

		// Fanout/PUSH extras
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
		Log               typeCustom.LoggerWithAlert
		errCh             chan error
		wg                *sync.WaitGroup
	}
	NatsKey string
)

const (
	NatsMSGID   NatsKey = "Nats-Msg-Id"
	Compression NatsKey = "Nats-Compression"
)

func baseOptionsShutdown() ([]nats.Option, chan error, *sync.WaitGroup) {
	var opts []nats.Option
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	opts = append(opts, nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		errCh <- err
	}), nats.ClosedHandler(func(_ *nats.Conn) {
		wg.Done()
	}))

	return opts, errCh, &wg
}

func NewConnection(cfg *QueueConfig, log typeCustom.LoggerWithAlert) (*Connection, *response.AppError) {
	opts, errCh, wg := baseOptionsShutdown()
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

	return &Connection{
		conn:  conn,
		Log:   log,
		errCh: errCh,
		wg:    wg,
	}, nil
}

func NewConnectWithJetStream(cfg *QueueConfig, log typeCustom.LoggerWithAlert) (*Connection, *response.AppError) {
	var conn *nats.Conn
	opts, errCh, wg := baseOptionsShutdown()
	dns := fmt.Sprintf("nats://%s:%d", cfg.Host, cfg.Port)
	var err error
	if len(cfg.Topics) == 0 {
		return nil, response.ServerError("topic is nil or empty")
	}

	if cfg.Seed != "" {
		opts = append(opts, nats.UserJWTAndSeed(cfg.Token, cfg.Seed))
	} else if cfg.Token != "" {
		opts = append(opts, nats.Token(cfg.Token))
	} else {
		dns = fmt.Sprintf("nats://%s:%s@%s:%d", cfg.Username, cfg.Password, cfg.Host, cfg.Port)
	}

	conn, err = nats.Connect(dns, opts...)
	if err != nil {
		log.Error("Failed to connect to NATS", err)
		return nil, response.ServerError(fmt.Sprintf("failed to connect to NATS: %v", err))
	}

	js, err := jetstream.New(conn)
	if err != nil {
		log.Error("Failed to create jetstream", err)
		return nil, response.ServerError("failed to create jetstream " + err.Error())
	}

	mapTopic := make(map[string]jetstream.Stream, len(cfg.Topics))

	for _, topic := range cfg.Topics {
		var stream jetstream.Stream
		streamCfg := jetstream.StreamConfig{
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
		}
		var err error
		if topic.UpdateStream {
			stream, err = js.UpdateStream(context.Background(), streamCfg)
		} else {
			stream, err = js.Stream(context.Background(), topic.Name)
		}

		if err != nil && errors.Is(err, jetstream.ErrStreamNotFound) {
			stream, err = js.CreateStream(context.Background(), streamCfg)
		}

		if err != nil {
			return nil, response.ServerError(fmt.Sprintf("failed to create or update stream %s: %s", topic.Name, err.Error()))
		}
		mapTopic[topic.Name] = stream
	}

	log.Info("Successfully connected to NATS JetStream server")
	return &Connection{
		conn:     conn,
		js:       js,
		Log:      log,
		mapTopic: mapTopic,
		errCh:    errCh,
		wg:       wg,
	}, nil
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
			sub.Drain()
		}
	}

	if len(c.subscriptionJS) > 0 {
		for _, sub := range c.subscriptionJS {
			sub.Drain()
		}
	}

	for _, sub := range c.subscription {
		if err := sub.Drain(); err != nil {
			c.Log.Error("Failed to unsubscribe", err)
			// Continue with shutdown even if unsubscribe fails
		}
	}

	// Drain the connection to process remaining messages gracefully
	if err := c.conn.Drain(); err != nil {
		c.Log.Error("Nats drain error", err)
		return response.ServerError(fmt.Sprintf("failed to drain nats connection: %s", err.Error()))
	}

	c.wg.Wait()
	select {
	case e := <-c.errCh:
		return response.ServerError("error reported during shutdown: " + e.Error())
	default:
	}
	c.conn.Close()
	// Connection is automatically closed after successful drain
	return nil
}

func buildConsumerConfig(cfg SubJSOption) jetstream.ConsumerConfig {
	jscfg := jetstream.ConsumerConfig{
		Durable:        cfg.Group,
		AckPolicy:      cfg.AckPolicy,
		MaxDeliver:     cfg.MaxDeliver,
		AckWait:        cfg.AckWait,
		PriorityPolicy: cfg.PriorityPolicy,
		DeliverPolicy:  cfg.DeliverPolicy,
		MaxAckPending:  cfg.MaxAckPending,
		DeliverGroup:   cfg.DeliverGroup,

		// Push
		DeliverSubject: cfg.DeliverSubject,
		FlowControl:    cfg.FlowControl,
		IdleHeartbeat:  cfg.IdleHeartbeat,
		BackOff:        cfg.BackOff,

		// Pull
		MaxRequestBatch:    cfg.MaxRequestBatch,
		MaxRequestMaxBytes: cfg.MaxRequestMaxBytes,
		MaxWaiting:         cfg.MaxWaiting,
	}
	if len(cfg.IncludeSubjects) > 1 {
		jscfg.FilterSubjects = cfg.IncludeSubjects
	}
	if len(cfg.IncludeSubjects) == 1 {
		jscfg.FilterSubject = cfg.IncludeSubjects[0]
	}
	return jscfg
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

func (c *Connection) HealthCheckJS() *response.AppError {
	if c.conn == nil || c.js == nil {
		return response.ServerError("NATS JetStream connection is nil")
	}
	if c.conn.IsClosed() {
		return response.ServerError("NATS connection is closed")
	}
	for _, sub := range c.subscriptionJS {
		select {
		case <-sub.Closed():
			return response.ServerError("NATS JetStream subscription for consumer %s on stream %s is closed")
		default:
			// still open
		}
	}
	c.Log.Info("NATS JetStream health check passed")
	return nil
}
