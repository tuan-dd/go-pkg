package natsQueue

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appLogger "github.com/tuan-dd/go-pkg/app-logger"
	"github.com/tuan-dd/go-pkg/common/queue"
	"github.com/tuan-dd/go-pkg/common/response"
	"github.com/tuan-dd/go-pkg/settings"
)

var cfg = &QueueConfig{
	Host:  "18.139.147.231",
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

var logger, _ = appLogger.NewLogger(
	&appLogger.LoggerConfig{
		Level: "debug",
	}, &settings.ServerSetting{
		Environment: "test",
	})

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
	return &queue.Header{
		"content-type":     "text/plain",
		"timestamp":        time.Now().Format(time.RFC3339),
		"Nats-Compression": "gzip",
	}
}

func baseHeaderCompressionS2() *queue.Header {
	return &queue.Header{
		"content-type":     "text/plain",
		"timestamp":        time.Now().Format(time.RFC3339),
		"Nats-Compression": "s2",
	}
}

func endTest(conn *Connection) {
	for _, stream := range cfg.Topics {
		conn.DeleteStream(stream.Name)
	}
	conn.Shutdown()
}

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
		logger.Info("Normal Subscribe received", data)
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
		logger.Info("Channel Subscribe received", data)
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
			ID:      fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			Body:    []byte(testMsg.content),
			Headers: baseHeader(),
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
				return response.QueryInvalid("Message body cannot be empty")
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
			BasicJSOption: BasicJSOption{
				Group:         "js-manage-group",
				AckPolicy:     jetstream.AckExplicitPolicy,
				MaxDeliver:    3,
				AckWait:       5 * time.Second,
				DeliverPolicy: jetstream.DeliverAllPolicy,
				MaxAckPending: 10,
			},
		},
	}

	err = conn.Subscribe("TEST_STREAM", options, func(ctx context.Context, msg *queue.Message) *response.AppError {
		mu.Lock()
		middlewareOrder = append(middlewareOrder, "handler")
		mu.Unlock()

		data := string(msg.Body)
		messageTracker.AddMessage(data)
		logger.Info("Final handler 4", data)

		// Simulate some processing time
		time.Sleep(50 * time.Millisecond)

		return nil
	})
	require.NoError(t, ConvertAppErrorToError(err), "Should be able to subscribe with middleware")

	// Wait for subscription to be ready
	time.Sleep(1 * time.Second)

	// Test with valid message
	validMsg := &queue.Message{
		ID:      "middleware-test-1",
		Body:    []byte("Valid middleware test message"),
		Headers: baseHeader(),
	}

	_, err = conn.Publish(context.Background(), "TEST_STREAM.jetstream", validMsg)
	require.NoError(t, ConvertAppErrorToError(err), "Should be able to publish valid message")

	// Test with empty message (should trigger validation error)
	invalidMsg := &queue.Message{
		ID:      "middleware-test-2",
		Body:    []byte(""),
		Headers: baseHeader(),
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

	logger.Info("Middleware execution order:", finalOrder)
	logger.Info("Test completed successfully - middleware chain working correctly")

	// Test middleware limit
	for i := range 9 {
		dummyMiddleware := func(next queue.HandlerFunc) queue.HandlerFunc {
			return next
		}
		conn.Use(dummyMiddleware)
		if i < 8 {
			require.NoError(t, ConvertAppErrorToError(err), "Should be able to add middleware %d", i)
		} else {
			require.Error(t, err, "Should reject middleware when limit exceeded")
			errMsg := err.Error()
			assert.Contains(t, errMsg, "too many middlewares")
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

// Test 5: Async publishing capabilities
func TestAsyncPublishing(t *testing.T) {
	// Skip if running in CI or if NATS server is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup JetStream connection
	conn, err := NewConnectWithJetStream(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")
	require.NotNil(t, conn, "Connection should not be nil")
	defer endTest(conn)

	asyncTracker := NewMessageTracker()

	// Subscribe to async test topic
	asyncOptions := queue.Options[SubJSOption]{
		Config: SubJSOption{
			BasicJSOption: BasicJSOption{
				Group:         "async-test-group",
				AckPolicy:     jetstream.AckExplicitPolicy,
				MaxDeliver:    1,
				AckWait:       5 * time.Second,
				DeliverPolicy: jetstream.DeliverAllPolicy,
			},
		},
	}

	err = conn.Subscribe("TEST_ASYNC_STREAM", asyncOptions, func(ctx context.Context, msg *queue.Message) *response.AppError {
		asyncTracker.AddMessage(string(msg.Body))
		logger.Info(fmt.Sprintf("Processed async message: %s", string(msg.Body)))
		return nil
	})
	require.NoError(t, ConvertAppErrorToError(err), "Should be able to subscribe for async test")

	// Wait for subscription to be ready
	time.Sleep(1 * time.Second)

	// Test multiple async publishing scenarios
	t.Run("ConcurrentAsyncPublish", func(t *testing.T) {
		// Publish multiple messages asynchronously
		var futures []jetstream.PubAckFuture
		messageCount := 10

		for i := range messageCount {
			msg := &queue.Message{
				ID:      fmt.Sprintf("async-msg-%d", i),
				Body:    fmt.Appendf(nil, "Async message %d", i),
				Headers: baseHeader(),
			}

			future, err := conn.PublishAsync(context.Background(), "TEST_ASYNC_STREAM.async", msg)
			require.NoError(t, ConvertAppErrorToError(err), "Async publish should not fail for message %d", i)
			futures = append(futures, future)
		}

		// Wait for all async publishes to complete
		successCount := 0
		for i, future := range futures {
			select {
			case <-future.Ok():
				logger.Info(fmt.Sprintf("Async message %d published successfully", i))
				successCount++
			case err := <-future.Err():
				t.Errorf("Async publish %d failed: %v", i, err)
			case <-time.After(5 * time.Second):
				t.Errorf("Async publish %d timed out", i)
			}
		}

		// Verify all publishes succeeded
		assert.Equal(t, messageCount, successCount, "All async publishes should succeed")

		// Wait for message processing
		time.Sleep(3 * time.Second)

		// Verify all messages were received
		assert.Equal(t, messageCount, asyncTracker.GetCount(),
			"Should receive all %d async messages", messageCount)
	})

	logger.Info("Async publishing test completed successfully")
	logger.Info(fmt.Sprintf("Total async messages processed: %d", asyncTracker.GetCount()))
}

// Test 6: SubscribeMessage functionality with pull-based consumers
func TestSubscribeMessage(t *testing.T) {
	// Skip if running in CI or if NATS server is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup JetStream connection
	conn, err := NewConnectWithJetStream(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")
	require.NotNil(t, conn, "Connection should not be nil")
	defer endTest(conn)

	t.Run("SingleConsumerPullMessages", func(t *testing.T) {
		messageTracker := NewMessageTracker()

		// Configure options for pull-based consumer
		options := queue.Options[SubJSOption]{
			Concurrent: 1, // Single consumer
			Config: SubJSOption{
				BasicJSOption: BasicJSOption{
					Group:         "pull-single-group",
					AckPolicy:     jetstream.AckExplicitPolicy,
					MaxDeliver:    3,
					AckWait:       5 * time.Second,
					DeliverPolicy: jetstream.DeliverAllPolicy,
					MaxAckPending: 10,
				},
				PullMaxMessages: 50, // Pull up to 50 messages at a time
			},
		}

		err = conn.SubscribeMessage("TEST_STREAM", options, func(ctx context.Context, msg *queue.Message) *response.AppError {
			data := string(msg.Body)
			messageTracker.AddMessage(fmt.Sprintf("Pull-Single: %s", data))
			logger.Info("SubscribeMessage Single received -1", data)
			return nil
		})

		require.NoError(t, ConvertAppErrorToError(err), "SubscribeMessage should not fail")

		// Wait for subscription to be ready
		time.Sleep(1 * time.Second)

		// Publish test messages
		messageCount := 10
		for i := range messageCount {
			msg := &queue.Message{
				ID:      fmt.Sprintf("pull-msg-%d", i),
				Body:    []byte(fmt.Sprintf("Pull message %d", i)),
				Headers: baseHeader(),
			}

			_, err = conn.Publish(context.Background(), "TEST_STREAM.pull", msg)
			require.NoError(t, ConvertAppErrorToError(err), "Publishing message should not fail")
			time.Sleep(10 * time.Millisecond) // Small delay between messages
		}

		// Wait for message processing
		time.Sleep(3 * time.Second)

		// Verify message reception
		assert.Equal(t, messageCount, messageTracker.GetCount(), "Should receive all messages via pull")

		// Verify message content
		messages := messageTracker.GetMessages()
		for i := range messageCount {
			expectedMsg := fmt.Sprintf("Pull-Single: Pull message %d", i)
			assert.Contains(t, messages, expectedMsg, "Should contain message %d", i)
		}

		logger.Info("Single consumer pull test completed successfully")
	})

	t.Run("PullMessagesWithErrorHandling", func(t *testing.T) {
		errorTracker := NewMessageTracker()
		var processedCount int32
		var errorCount int32

		// Configure options with error handling
		options := queue.Options[SubJSOption]{
			Concurrent: 2,
			Config: SubJSOption{
				BasicJSOption: BasicJSOption{
					Group:         "pull-error-group",
					AckPolicy:     jetstream.AckExplicitPolicy,
					MaxDeliver:    2, // Limited retries
					AckWait:       3 * time.Second,
					DeliverPolicy: jetstream.DeliverAllPolicy,
					MaxAckPending: 5,
					Delay:         500 * time.Millisecond,
				},
				PullMaxMessages: 10,
			},
		}

		err = conn.SubscribeMessage("TEST_STREAM", options, func(ctx context.Context, msg *queue.Message) *response.AppError {
			data := string(msg.Body)
			logger.Info("message", data)
			// Simulate error for specific messages
			if string(msg.Body) == "error-message" {
				atomic.AddInt32(&errorCount, 1)
				errorTracker.AddMessage(fmt.Sprintf("Error: %s", data))
				return response.ServerError("Simulated processing error")
			}

			atomic.AddInt32(&processedCount, 1)
			errorTracker.AddMessage(fmt.Sprintf("Success: %s", data))
			return nil
		})
		require.NoError(t, ConvertAppErrorToError(err), "Error handling SubscribeMessage should not fail")

		// Wait for subscription to be ready
		time.Sleep(1 * time.Second)

		// Publish mix of successful and error messages
		testMessages := []string{
			"success-message-1",
			"error-message",
			"success-message-2",
			"success-message-3",
		}

		for i, msgContent := range testMessages {
			msg := &queue.Message{
				ID:      fmt.Sprintf("error-test-msg-%d", i),
				Body:    []byte(msgContent),
				Headers: baseHeader(),
			}

			_, err = conn.Publish(context.Background(), "TEST_STREAM.error-test", msg)
			require.NoError(t, ConvertAppErrorToError(err), "Publishing error test message should not fail")
			time.Sleep(50 * time.Millisecond)
		}

		// Wait for message processing and retries
		time.Sleep(8 * time.Second)

		// Verify results
		messages := errorTracker.GetMessages()
		successMessages := 0
		errorMessages := 0

		for _, msg := range messages {
			if strings.Contains(msg, "Success:") {
				successMessages++
			} else if strings.Contains(msg, "Error:") {
				errorMessages++
			}
		}

		// Should have 3 successful messages
		assert.Equal(t, 3, successMessages, "Should process 3 successful messages")

		// Should have error attempts (including retries)
		assert.GreaterOrEqual(t, errorMessages, 1, "Should have at least 1 error attempt")
		assert.LessOrEqual(t, errorMessages, 2, "Should not exceed MaxDeliver attempts")

		logger.Info("Error handling pull test completed successfully")
		logger.Info(fmt.Sprintf("Processed: %d, Errors: %d", atomic.LoadInt32(&processedCount), atomic.LoadInt32(&errorCount)))
	})

	logger.Info("SubscribeMessage test suite completed successfully")
}

func TestCompress(t *testing.T) {
	// Skip if running in CI or if NATS server is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup JetStream connection
	conn, err := NewConnectWithJetStream(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")
	require.NotNil(t, conn, "Connection should not be nil")
	defer endTest(conn)

	t.Run("js-compression", func(t *testing.T) {
		messageTracker := NewMessageTracker()
		normalTracker := NewMessageTracker()

		// Configure options for pull-based consumer
		options := queue.Options[SubJSOption]{
			Concurrent: 1, // Single consumer
			Config: SubJSOption{
				BasicJSOption: BasicJSOption{
					Group:         "js-compression-group",
					AckPolicy:     jetstream.AckExplicitPolicy,
					MaxDeliver:    3,
					AckWait:       5 * time.Second,
					DeliverPolicy: jetstream.DeliverAllPolicy,
					MaxAckPending: 10,
				},
			},
		}

		normalOptions := queue.Options[SubOption]{
			Config: SubOption{
				Group: "normal-compression-group",
			},
		}

		err = conn.SubscribeMessage("COMPRESSION", options, func(ctx context.Context, msg *queue.Message) *response.AppError {
			data := string(msg.Body)
			messageTracker.AddMessage(data)
			logger.Info("js consumer", data)
			return nil
		})

		err = conn.SubscribeNor("COMPRESSION.>", normalOptions, func(ctx context.Context, msg *queue.Message) *response.AppError {
			data := string(msg.Body)
			normalTracker.AddMessage(fmt.Sprintf("Normal: %s", data))
			logger.Info("Normal consumer", data)
			return nil
		})

		require.NoError(t, ConvertAppErrorToError(err), "SubscribeMessage should not fail")

		// Wait for subscription to be ready
		time.Sleep(1 * time.Second)
		messageCount := 2
		for range messageCount {
			msg := &queue.Message{
				Body:    fmt.Appendf(nil, "message gzip"),
				Headers: baseHeaderCompressionGzip(),
			}
			msg2 := &queue.Message{
				Body:    fmt.Appendf(nil, "message s2"),
				Headers: baseHeaderCompressionS2(),
			}

			_, err = conn.Publish(context.Background(), "COMPRESSION.gzip", msg)
			require.NoError(t, ConvertAppErrorToError(err), "Publishing compressed messages should not fail")

			_, err = conn.Publish(context.Background(), "COMPRESSION.s2", msg2)
			require.NoError(t, ConvertAppErrorToError(err), "Publishing compressed messages should not fail")
		}

		// Wait for message processing
		time.Sleep(3 * time.Second)

		// Verify message content
		messages := messageTracker.GetMessages()
		assert.Equal(t, 4, len(messages), "Should receive 2 messages")
		assert.Contains(t, messages, "message gzip", "Should contain gzip message")
		assert.Contains(t, messages, "message s2", "Should contain s2 message")

		normalMessages := normalTracker.GetMessages()
		assert.Equal(t, 4, len(normalMessages), "Normal consumer should receive 2 messages")
		assert.Contains(t, normalMessages, "Normal: message gzip", "Normal consumer should receive gzip message")
		assert.Contains(t, normalMessages, "Normal: message s2", "Normal consumer should receive s2 message")

		logger.Info("Compression test completed successfully")
	})
}

func TestDeleteStream(t *testing.T) {
	conn, err := NewConnectWithJetStream(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")

	err = conn.DeleteAllStreams()
	require.NoError(t, ConvertAppErrorToError(err), "Failed to delete all streams")
}
