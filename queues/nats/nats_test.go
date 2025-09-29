package natsQueue

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tuan-dd/go-common/compress"
	"github.com/tuan-dd/go-common/queue"
	"github.com/tuan-dd/go-common/request"
	"github.com/tuan-dd/go-common/response"
)

var cfg = &QueueConfig{
	Host:  "18.136.192.113",
	Port:  4223,
	Token: "ssm-secret-token",
	Topics: []TopicConfig{
		{
			Name:      "TEST_STREAM",
			Subjects:  []string{"TEST_STREAM.>"},
			Storage:   int(jetstream.MemoryStorage),
			MaxMsgs:   1000,
			Retention: 1,
			MaxAge:    time.Hour,
		},
		{
			Name:     "TEST_ASYNC_STREAM",
			Subjects: []string{"TEST_ASYNC_STREAM.>"},
			Storage:  int(jetstream.MemoryStorage),
			MaxMsgs:  1000,
			MaxAge:   time.Hour,
		},
		{
			Name:     "COMPRESSION",
			Subjects: []string{"COMPRESSION.>"},
			Storage:  int(jetstream.MemoryStorage),
			MaxMsgs:  1000,
			MaxAge:   time.Hour,
		},
	},
	// Username: "test",
	// Password: "test",
}

type CustomerLogger struct {
	*slog.Logger
}

func (l *CustomerLogger) AppErrorWithAlert(reqCtx *request.ReqContext, err *response.AppError, fields ...any) {
	l.Error("Error with alert")
}

func (l *CustomerLogger) ErrorPanic(reqCtx *request.ReqContext, msg string, err any, stack string) {
	l.Error("Error panic", "msg", msg, "err", err, "stack", stack)
}

var logger = &CustomerLogger{
	Logger: slog.Default(),
}

// Test stream configuration

// Message tracking for tests
type MessageTracker struct {
	mu       sync.Mutex
	messages []string
	errors   []error
	count    int
}

func NewMessageTracker() *MessageTracker {
	return &MessageTracker{
		messages: make([]string, 0),
		errors:   make([]error, 0),
	}
}

func (mt *MessageTracker) AddMessage(msg string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.messages = append(mt.messages, msg)
	mt.count++
}

func (mt *MessageTracker) AddError(err error) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.errors = append(mt.errors, err)
}

func (mt *MessageTracker) GetCount() int {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	return mt.count
}

func (mt *MessageTracker) GetMessages() []string {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	return append([]string{}, mt.messages...)
}

func (mt *MessageTracker) GetErrors() []error {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	return append([]error{}, mt.errors...)
}

func ConvertAppErrorToError(err *response.AppError) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("AppError: %s, Code: %d, Data: %v", err.Message, err.Code, err.Data)
}

func baseHeader() *queue.Header {
	return &queue.Header{
		"content-type": "text/plain",
		"timestamp":    time.Now().Format(time.RFC3339),
	}
}

func baseHeaderCompressionGzip() *queue.Header {
	header := queue.Header{
		"content-type": "text/plain",
		"timestamp":    time.Now().Format(time.RFC3339),
	}
	header[string(Compression)] = compress.GzipCompression
	return &header
}

func baseHeaderCompressionS2() *queue.Header {
	header := queue.Header{
		"content-type": "text/plain",
		"timestamp":    time.Now().Format(time.RFC3339),
	}
	header[string(Compression)] = string(compress.S2Compression)
	return &header
}

// Test 1: Normal Publish and Subscribe
func TestNormalPublishAndSubscribe(t *testing.T) {
	// Skip if running in CI or if NATS server is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup normal NATS connection for non-JetStream tests
	normalConn, err := NewConnection(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create normal NATS connection")
	require.NotNil(t, normalConn, "Normal connection should not be nil")
	defer endTest(normalConn)

	// Setup message trackers for different subscription methods
	normalTracker := NewMessageTracker()
	channelTracker := NewMessageTracker()

	// Test Normal Subscribe
	normalOptions := queue.Options[SubOption]{
		Config: SubOption{
			Group: "normal-group",
		},
	}

	err = normalConn.SubscribeNor("test.normal", normalOptions, func(ctx context.Context, msg *queue.Message) *response.AppError {
		data := string(msg.Body)
		normalTracker.AddMessage(fmt.Sprintf("Normal: %s", data))
		if strings.Contains(data, "panic") {
			panic("Simulated panic in normal handler") // Simulate a panic for testing
		}
		logger.Info("Normal Subscribe received", "data", data)
		return nil
	})
	require.NoError(t, ConvertAppErrorToError(err), "Normal subscribe should not fail")

	// Test Channel Subscribe
	channelOptions := queue.Options[SubChanOption]{
		Config: SubChanOption{
			Group:      "channel-group",
			ChanNumber: 5,
		},
	}

	err = normalConn.SubscribeChanNor("test.channel", channelOptions, func(ctx context.Context, msg *queue.Message) *response.AppError {
		data := string(msg.Body)
		channelTracker.AddMessage(fmt.Sprintf("Channel: %s", data))
		if strings.Contains(data, "panic") {
			panic("Simulated panic in channel handler") // Simulate a panic for testing
		}
		logger.Info("Channel Subscribe received", "data", data)
		return nil
	})
	require.NoError(t, ConvertAppErrorToError(err), "Channel subscribe should not fail")

	// Wait for subscriptions to be ready
	time.Sleep(1 * time.Second)

	// Test publishing to different topics
	testMessages := []struct {
		topic   string
		content string
	}{
		{"test.normal", "panic message 1"},
		{"test.normal", "Normal message 2"},
		{"test.normal", "Normal message 3"},
		{"test.channel", "panic message 1"},
		{"test.channel", "Channel message 2"},
		{"test.channel", "Channel message 3"},
	}

	for _, testMsg := range testMessages {
		msg := &queue.Message{
			ID:     fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			Body:   []byte(testMsg.content),
			Header: baseHeader(),
		}

		err = normalConn.PublishNor(context.Background(), testMsg.topic, msg)
		require.NoError(t, ConvertAppErrorToError(err), "Publishing message should not fail for topic: %s", testMsg.topic)
	}

	// Wait for message processing
	time.Sleep(3 * time.Second)

	// Verify message reception
	assert.Equal(t, 3, normalTracker.GetCount(), "Normal subscribe should receive 3 messages")
	assert.Equal(t, 3, channelTracker.GetCount(), "Channel subscribe should receive 3 messages")

	// Verify message content
	normalMessages := normalTracker.GetMessages()
	assert.Contains(t, normalMessages, "Normal: panic message 1")
	assert.Contains(t, normalMessages, "Normal: Normal message 2")
	assert.Contains(t, normalMessages, "Normal: Normal message 3")

	channelMessages := channelTracker.GetMessages()
	assert.Contains(t, channelMessages, "Channel: panic message 1")
	assert.Contains(t, channelMessages, "Channel: Channel message 2")
	assert.Contains(t, channelMessages, "Channel: Channel message 3")

	logger.Info("Test completed successfully - normal subscription methods working")
}

// Test 2: Middleware functionality test
func TestMiddlewareSupport(t *testing.T) {
	// Skip if running in CI or if NATS server is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup connection
	conn, err := NewConnectWithJetStream(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")
	require.NotNil(t, conn, "Connection should not be nil")
	defer endTest(conn)

	// Track middleware execution order
	var middlewareOrder []string
	var mu sync.Mutex

	// Logging middleware
	loggingMiddleware := func(next queue.HandlerFunc) queue.HandlerFunc {
		return func(ctx context.Context, msg *queue.Message) *response.AppError {
			logger.Info("Logging-1")
			mu.Lock()
			middlewareOrder = append(middlewareOrder, "logging-start")
			mu.Unlock()

			result := next(ctx, msg)

			mu.Lock()
			middlewareOrder = append(middlewareOrder, "logging-end")
			mu.Unlock()

			logger.Info("Finish-5")
			return result
		}
	}

	// Validation middleware
	validationMiddleware := func(next queue.HandlerFunc) queue.HandlerFunc {
		return func(ctx context.Context, msg *queue.Message) *response.AppError {
			logger.Info("validation-2")
			mu.Lock()
			middlewareOrder = append(middlewareOrder, "validation")
			mu.Unlock()

			// Simulate validation logic
			if len(msg.Body) == 0 {
				mu.Lock()
				middlewareOrder = append(middlewareOrder, "validation-error")
				mu.Unlock()
				return response.QueryInvalidErr("Message body cannot be empty")
			}

			return next(ctx, msg)
		}
	}

	// Metrics middleware
	metricsMiddleware := func(next queue.HandlerFunc) queue.HandlerFunc {
		return func(ctx context.Context, msg *queue.Message) *response.AppError {
			logger.Info("Metrics-3")
			start := time.Now()

			mu.Lock()
			middlewareOrder = append(middlewareOrder, "metrics-start")
			mu.Unlock()

			result := next(ctx, msg)

			duration := time.Since(start)

			mu.Lock()
			middlewareOrder = append(middlewareOrder, fmt.Sprintf("metrics-end-%dms", duration.Milliseconds()))
			mu.Unlock()

			return result
		}
	}

	// Register middlewares in order
	conn.Use(loggingMiddleware)
	// require.NoError(t, ConvertAppErrorToError(err), "Should be able to add logging middleware")

	conn.Use(validationMiddleware)
	// require.NoError(t, ConvertAppErrorToError(err), "Should be able to add validation middleware")
	conn.Use(metricsMiddleware)
	// require.NoError(t, ConvertAppErrorToError(err), "Should be able to add metrics middleware")

	// Track final handler execution
	messageTracker := NewMessageTracker()

	// Subscribe with middleware chain
	options := queue.Options[SubJSOption]{
		Config: SubJSOption{
			Group:         "js-manage-group",
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxDeliver:    3,
			AckWait:       5 * time.Second,
			DeliverPolicy: jetstream.DeliverAllPolicy,
			MaxAckPending: 10,
		},
	}

	err = conn.Subscribe("TEST_STREAM", options, func(ctx context.Context, msg *queue.Message) *response.AppError {
		mu.Lock()
		middlewareOrder = append(middlewareOrder, "handler")
		mu.Unlock()

		data := string(msg.Body)
		messageTracker.AddMessage(data)
		logger.Info("Final handler 4", "data", data)

		// Simulate some processing time
		time.Sleep(50 * time.Millisecond)

		return nil
	})
	require.NoError(t, ConvertAppErrorToError(err), "Should be able to subscribe with middleware")

	// Wait for subscription to be ready
	time.Sleep(1 * time.Second)

	// Test with valid message
	validMsg := &queue.Message{
		ID:     "middleware-test-1",
		Body:   []byte("Valid middleware test message"),
		Header: baseHeader(),
	}

	_, err = conn.Publish(context.Background(), "TEST_STREAM.jetstream", validMsg)
	require.NoError(t, ConvertAppErrorToError(err), "Should be able to publish valid message")

	// Test with empty message (should trigger validation error)
	invalidMsg := &queue.Message{
		ID:     "middleware-test-2",
		Body:   []byte(""),
		Header: baseHeader(),
	}

	_, err = conn.Publish(context.Background(), "TEST_STREAM.jetstream", invalidMsg)
	require.NoError(t, ConvertAppErrorToError(err), "Should be able to publish invalid message")

	// Wait for all messages to be processed
	time.Sleep(1 * time.Second)

	// Verify middleware execution
	mu.Lock()
	finalOrder := append([]string{}, middlewareOrder...)
	mu.Unlock()

	// Should have processed at least the valid message
	assert.GreaterOrEqual(t, messageTracker.GetCount(), 1, "Should process at least one valid message")

	// Verify middleware order for valid message (middlewares execute in reverse order)
	assert.Contains(t, finalOrder, "logging-start")
	assert.Contains(t, finalOrder, "validation")
	assert.Contains(t, finalOrder, "metrics-start")
	assert.Contains(t, finalOrder, "handler")
	assert.Contains(t, finalOrder, "logging-end")
	assert.Contains(t, finalOrder, "validation-error")

	// Check that metrics middleware recorded timing
	var hasMetricsEnd bool
	for _, order := range finalOrder {
		if len(order) > 11 && order[:11] == "metrics-end" {
			hasMetricsEnd = true
			break
		}
	}
	assert.True(t, hasMetricsEnd, "Metrics middleware should record timing")

	logger.Info("Middleware execution order:", "order", finalOrder)
	logger.Info("Test completed successfully - middleware chain working correctly")

	// Test middleware limit
	for i := range 10 {
		dummyMiddleware := func(next queue.HandlerFunc) queue.HandlerFunc {
			return next
		}
		conn.Use(dummyMiddleware)
		if i < 8 {
			assert.Equal(t, i+4, len(conn.Middlewares()), "Should allow adding middleware up to limit")
		} else {
			assert.Equal(t, 11, len(conn.Middlewares()), "Should not allow adding more than 10 middlewares")
		}
	}
}

// Test 4: Connection resilience and edge cases
func TestConnectionResilienceAndEdgeCases(t *testing.T) {
	// Skip if running in CI or if NATS server is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup JetStream connection
	conn, err := NewConnectWithJetStream(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")
	require.NotNil(t, conn, "Connection should not be nil")
	defer endTest(conn)

	// Test publishing with various edge cases
	t.Run("PublishingEdgeCases", func(t *testing.T) {
		edgeCases := []struct {
			name        string
			msg         *queue.Message
			expectError bool
		}{
			{
				name:        "nil message",
				msg:         nil,
				expectError: true,
			},
			{
				name: "empty body",
				msg: &queue.Message{
					ID:   "empty-body",
					Body: []byte{},
				},
				expectError: false,
			},
			{
				name: "large message over 1MB",
				msg: &queue.Message{
					ID:   "large-msg-over",
					Body: make([]byte, 1024*1024), // 1MB
				},
				expectError: true,
			},
			{
				name: "large message under 1MB",
				msg: &queue.Message{
					ID:   "large-msg-under",
					Body: make([]byte, 1024*950), // 950KB
				},
				expectError: false,
			},
			{
				name: "special characters",
				msg: &queue.Message{
					ID:   "special-chars",
					Body: []byte("Test with special chars: üñíçødé 测试 🚀"),
				},
				expectError: false,
			},
		}
		for _, tc := range edgeCases {
			t.Run(tc.name, func(t *testing.T) {
				err := conn.PublishNor(context.Background(), "TEST_STREAM.edge-cases", tc.msg)
				if tc.expectError {
					assert.Error(t, ConvertAppErrorToError(err), "Should fail for case: %s", tc.name)
				} else {
					assert.NoError(t, ConvertAppErrorToError(err), "Should succeed for case: %s", tc.name)
				}
			})
		}
	})

	logger.Info("Connection resilience and edge cases test completed successfully")
}

func TestDeleteStream(t *testing.T) {
	conn, err := NewConnectWithJetStream(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")

	err = conn.DeleteAllStreams()
	require.NoError(t, ConvertAppErrorToError(err), "Failed to delete all streams")
}
