package natsQueue

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tuan-dd/go-common/queue"
	"github.com/tuan-dd/go-common/response"
)

func TestConnectWithJetStream(t *testing.T) {
	// Skip if running in CI or if NATS server is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup JetStream connection
	conn, err := NewConnectWithJetStream(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")
	require.NotNil(t, conn, "Connection should not be nil")
	defer conn.Shutdown()
	defer conn.DeleteAllStreams()

	logger.Info("JetStream connection established successfully")
}

// Test 1: Comprehensive Publish and Subscribe test with all subscription methods
func TestJetStreamPublishAndSubscribe(t *testing.T) {
	// Skip if running in CI or if NATS server is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup JetStream connection
	conn, err := NewConnectWithJetStream(cfg, logger)
	logger.Info("Creating NATS JetStream connection", "err", err)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")
	require.NotNil(t, conn, "Connection should not be nil")
	defer endTest(conn)

	// Setup message trackers for different subscription methods
	jsTracker := NewMessageTracker()
	jsManageTracker := NewMessageTracker()

	// Test JetStream Subscribe with consumer groups
	jsOptions := queue.Options[SubJSOption]{
		Config: SubJSOption{
			Group:         "js-test-group",
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxDeliver:    3,
			AckWait:       5 * time.Second,
			DeliverPolicy: jetstream.DeliverAllPolicy,
			MaxAckPending: 10,
			Delay:         1 * time.Second,
		},
	}

	err = conn.Subscribe("TEST_STREAM", jsOptions, func(ctx context.Context, msg *queue.Message) *response.AppError {
		data := string(msg.Body)
		jsTracker.AddMessage(fmt.Sprintf("JS-Subscribe: %s", data))
		logger.Info("JetStream Subscribe received", "data", data)
		return nil
	})
	require.NoError(t, ConvertAppErrorToError(err), "JetStream subscribe should not fail")

	// Test JetStream SubscribeManage
	jsManageOptions := queue.Options[SubJSOption]{
		Config: SubJSOption{
			Group:         "js-manage-group",
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxDeliver:    3,
			AckWait:       5 * time.Second,
			DeliverPolicy: jetstream.DeliverAllPolicy,
			MaxAckPending: 10,
		},
	}

	err = conn.SubscribeManage("TEST_STREAM", jsManageOptions, func(ctx context.Context, msg *queue.Message) *response.AppError {
		data := string(msg.Body)
		jsManageTracker.AddMessage(fmt.Sprintf("JS-Manage: %s", data))
		logger.Info("JetStream SubscribeManage received", "data", data)
		return nil
	})
	require.NoError(t, ConvertAppErrorToError(err), "JetStream SubscribeManage should not fail")

	// Wait for subscriptions to be ready
	time.Sleep(1 * time.Second)

	// Test publishing to different topics
	testMessages := []struct {
		topic   string
		content string
		useJS   bool
	}{
		{"TEST_STREAM.jetstream", "JetStream message 1", true},
		{"TEST_STREAM.jetstream", "JetStream message 2", true},
		{"TEST_STREAM.jetstream", "JetStream don't exist", false},
	}

	for _, testMsg := range testMessages {
		msg := &queue.Message{
			ID:     fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			Body:   []byte(testMsg.content),
			Header: baseHeader(),
		}
		if testMsg.useJS {
			_, err = conn.Publish(context.Background(), testMsg.topic, msg)
		} else {
			// err = conn.PublishNor(context.Background(), testMsg.topic, msg)
		}
		require.NoError(t, ConvertAppErrorToError(err), "Publishing message should not fail for topic: %s", testMsg.topic)
	}

	// Wait for message processing
	time.Sleep(3 * time.Second)

	// Verify message reception
	assert.Equal(t, 2, jsTracker.GetCount(), "JetStream subscribe should receive 2 messages")
	assert.Equal(t, 2, jsManageTracker.GetCount(), "JetStream SubscribeManage should receive 2 messages")

	// Verify message content
	jsMessages := jsTracker.GetMessages()
	assert.Contains(t, jsMessages, "JS-Subscribe: JetStream message 1")
	assert.Contains(t, jsMessages, "JS-Subscribe: JetStream message 2")

	logger.Info("Test completed successfully - JetStream subscription methods working")
}

// Test 3: Error handling and retry mechanisms
func TestErrorHandlingAndRetryMechanisms(t *testing.T) {
	// Skip if running in CI or if NATS server is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup JetStream connection
	conn, err := NewConnectWithJetStream(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")
	require.NotNil(t, conn, "Connection should not be nil")
	defer endTest(conn)

	// Track message processing attempts
	type MessageAttempt struct {
		MessageID string
		Attempt   int
		Success   bool
		Error     string
		Timestamp time.Time
	}

	var attempts []MessageAttempt
	var attemptsMu sync.Mutex

	// Configure options with retry settings
	options := queue.Options[SubJSOption]{
		Config: SubJSOption{
			Group:         "retry-test-group",
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxDeliver:    3, // Allow up to 3 delivery attempts
			AckWait:       2 * time.Second,
			DeliverPolicy: jetstream.DeliverAllPolicy,
			MaxAckPending: 5,
			Delay:         500 * time.Millisecond, // Delay between retries

		},
	}

	// Simulated failure scenarios
	failureScenarios := map[string]struct {
		shouldFail    bool
		failOnAttempt int
		errorMessage  string
		recoverAfter  int
	}{
		"msg-always-fail":    {true, 1, "permanent failure", 999},
		"msg-recover-second": {true, 1, "temporary failure", 2},
		"msg-recover-third":  {true, 1, "retry needed", 3},
		"msg-success":        {false, 0, "", 0},
	}

	err = conn.Subscribe("TEST_STREAM", options, func(ctx context.Context, msg *queue.Message) *response.AppError {
		messageID := string(msg.Body)

		attemptsMu.Lock()

		// Count current attempts for this message
		currentAttempt := 1
		for _, attempt := range attempts {
			if attempt.MessageID == messageID {
				currentAttempt++
			}
		}

		attempt := MessageAttempt{
			MessageID: messageID,
			Attempt:   currentAttempt,
			Timestamp: time.Now(),
		}

		// Check if this message should fail
		if scenario, exists := failureScenarios[messageID]; exists {
			if scenario.shouldFail && currentAttempt < scenario.recoverAfter {
				attempt.Success = false
				attempt.Error = scenario.errorMessage
				attempts = append(attempts, attempt)
				attemptsMu.Unlock()

				logger.Warn(fmt.Sprintf("Simulated failure for %s on attempt %d: %s",
					messageID, currentAttempt, scenario.errorMessage))

				return response.ServerError(scenario.errorMessage)
			}
		}

		// Success case
		attempt.Success = true
		attempts = append(attempts, attempt)
		attemptsMu.Unlock()

		logger.Info(fmt.Sprintf("Successfully processed %s on attempt %d", messageID, currentAttempt))
		return nil
	})
	require.NoError(t, ConvertAppErrorToError(err), "Should be able to subscribe for retry test")

	// Wait for subscription to be ready
	time.Sleep(1 * time.Second)

	// Publish test messages for retry scenarios
	testMessages := []string{
		"msg-always-fail",
		"msg-recover-second",
		"msg-recover-third",
		"msg-success",
	}

	for i, msgID := range testMessages {
		header := baseHeader()
		header.Add("test-scenario", msgID)
		header.Add("publish-time", time.Now().Format(time.RFC3339))
		msg := &queue.Message{
			ID:     fmt.Sprintf("retry-test-%d", i),
			Body:   []byte(msgID),
			Header: header,
		}

		err = conn.PublishNor(context.Background(), "TEST_STREAM.retry", msg)
		require.NoError(t, ConvertAppErrorToError(err), "Should be able to publish message: %s", msgID)

		// Small delay between messages
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all processing attempts (including retries)
	time.Sleep(1 * time.Second)

	// Analyze results
	attemptsMu.Lock()
	finalAttempts := append([]MessageAttempt{}, attempts...)
	attemptsMu.Unlock()

	// Group attempts by message ID
	messageAttempts := make(map[string][]MessageAttempt)
	for _, attempt := range finalAttempts {
		messageAttempts[attempt.MessageID] = append(messageAttempts[attempt.MessageID], attempt)
	}

	// Verify retry behavior for each scenario
	for msgID, scenario := range failureScenarios {
		attempts, exists := messageAttempts[msgID]
		require.True(t, exists, "Should have attempts for message: %s", msgID)

		if scenario.shouldFail && scenario.recoverAfter <= 3 {
			// Should eventually succeed after retries
			assert.GreaterOrEqual(t, len(attempts), scenario.recoverAfter,
				"Should have at least %d attempts for %s", scenario.recoverAfter, msgID)

			// Last attempt should be successful
			lastAttempt := attempts[len(attempts)-1]
			assert.True(t, lastAttempt.Success,
				"Final attempt should succeed for %s", msgID)
		} else if scenario.shouldFail && scenario.recoverAfter > 3 {
			// Should fail permanently after max retries
			assert.LessOrEqual(t, len(attempts), 3,
				"Should not exceed max retries for %s", msgID)

			// All attempts should fail
			for _, attempt := range attempts {
				assert.False(t, attempt.Success,
					"All attempts should fail for %s", msgID)
			}
		} else {
			// Should succeed on first try
			assert.Equal(t, 1, len(attempts),
				"Should only need one attempt for %s", msgID)
			assert.True(t, attempts[0].Success,
				"First attempt should succeed for %s", msgID)
		}
	}

	logger.Info("Error handling and retry test completed successfully")
	logger.Info(fmt.Sprintf("Total message attempts tracked: %d", len(finalAttempts)))

	// Log summary of retry attempts
	for msgID, attempts := range messageAttempts {
		successCount := 0
		for _, attempt := range attempts {
			if attempt.Success {
				successCount++
			}
		}
		logger.Info(fmt.Sprintf("Message %s: %d attempts, %d successes",
			msgID, len(attempts), successCount))
	}
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
			Group:         "async-test-group",
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxDeliver:    1,
			AckWait:       5 * time.Second,
			DeliverPolicy: jetstream.DeliverAllPolicy,
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
				ID:     fmt.Sprintf("async-msg-%d", i),
				Body:   fmt.Appendf(nil, "Async message %d", i),
				Header: baseHeader(),
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

// Test 6: Compression support in JetStream
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
				Group:         "js-compression-group",
				AckPolicy:     jetstream.AckExplicitPolicy,
				MaxDeliver:    3,
				AckWait:       5 * time.Second,
				DeliverPolicy: jetstream.DeliverAllPolicy,
				MaxAckPending: 10,
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
			// logger.Info("js consumer", "data", data)
			return nil
		})

		prePublish := 4
		var mu sync.Mutex
		err = conn.SubscribeNor("COMPRESSION.>", normalOptions, func(ctx context.Context, msg *queue.Message) *response.AppError {
			data := string(msg.Body)
			normalTracker.AddMessage(fmt.Sprintf("Normal: %s", data))
			mu.Lock()
			if prePublish > 0 {
				prePublish--
				_, err = conn.Publish(context.Background(), "COMPRESSION.retry", msg)
			}
			mu.Unlock()
			logger.Info("msg consumer", "data", data[len(data)-20:])
			return nil
		})

		require.NoError(t, ConvertAppErrorToError(err), "SubscribeMessage should not fail")

		// Wait for subscription to be ready
		// time.Sleep(100 * time.Millisecond)
		messageCount := 2
		for range messageCount {
			msg := &queue.Message{
				Body:   []byte(strings.Repeat("large message content with random data gzip ", 10000) + " test gzip"),
				Header: baseHeaderCompressionGzip(),
			}
			msg2 := &queue.Message{
				Body:   []byte(strings.Repeat("large message content with random data s2 ", 10000) + " test s2"),
				Header: baseHeaderCompressionS2(),
			}

			_, err = conn.Publish(context.Background(), "COMPRESSION.gzip", msg)
			require.NoError(t, ConvertAppErrorToError(err), "Publishing compressed messages should not fail")

			_, err = conn.Publish(context.Background(), "COMPRESSION.s2", msg2)
			require.NoError(t, ConvertAppErrorToError(err), "Publishing compressed messages should not fail")
		}

		// Wait for message processing
		time.Sleep(1000 * time.Millisecond)

		// Verify message content
		messages := messageTracker.GetMessages()
		assert.Equal(t, 8, len(messages), "Should receive 2 messages")
		// assert.Contains(t, messages, "message gzip", "data gzip")
		// assert.Contains(t, messages, "message s2", "data s2")

		normalMessages := normalTracker.GetMessages()
		assert.Equal(t, 8, len(normalMessages), "Normal consumer should receive 8 messages")
		// assert.Contains(t, normalMessages, "Normal: message gzip", "Normal consumer should receive gzip message")
		// assert.Contains(t, normalMessages, "Normal: message s2", "Normal consumer should receive s2 message")

		logger.Info("Compression test completed successfully")
	})
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

	t.Run("JSSubscribeMessage", func(t *testing.T) {
		messageTracker := NewMessageTracker()

		// Configure options for pull-based consumer
		options := queue.Options[SubJSOption]{
			Concurrent: 1, // Single consumer
			Config: SubJSOption{
				Group:         "Concurrent",
				AckPolicy:     jetstream.AckExplicitPolicy,
				MaxDeliver:    3,
				AckWait:       5 * time.Second,
				DeliverPolicy: jetstream.DeliverAllPolicy,
				MaxAckPending: 10,
			},
		}
		start := time.Now()
		err = conn.SubscribeMessage("TEST_STREAM", options, func(ctx context.Context, msg *queue.Message) *response.AppError {
			data := string(msg.Body)
			messageTracker.AddMessage(data)
			time.Sleep(200 * time.Millisecond) // Simulate processing delay
			if data == "Pull message 49" {
				slog.Info("SubscribeMessage duration", "test", fmt.Sprintf("Duration: %d ms", time.Since(start).Milliseconds()))
			}
			return nil
		})

		require.NoError(t, ConvertAppErrorToError(err), "SubscribeMessage should not fail")

		// Publish test messages
		messageCount := 50
		for i := range messageCount {
			msg := &queue.Message{
				ID:     fmt.Sprintf("pull-msg-%d", i),
				Body:   fmt.Appendf(nil, "Pull message %d", i),
				Header: baseHeader(),
			}
			_, err = conn.Publish(context.Background(), "TEST_STREAM.pull", msg)
			require.NoError(t, ConvertAppErrorToError(err), "Publishing message should not fail")
		}

		// Wait for message processing
		time.Sleep(12 * time.Second)

		// Verify message reception
		assert.Equal(t, messageCount, messageTracker.GetCount(), "Should receive all messages via pull")

		// Verify message content
		messages := messageTracker.GetMessages()
		for i := range messageCount {
			expectedMsg := fmt.Sprintf("Pull message %d", i)
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
				Group:         "pull-error-group",
				AckPolicy:     jetstream.AckExplicitPolicy,
				MaxDeliver:    2, // Limited retries
				AckWait:       3 * time.Second,
				DeliverPolicy: jetstream.DeliverAllPolicy,
				MaxAckPending: 5,
				Delay:         500 * time.Millisecond,
			},
		}

		err = conn.SubscribeMessage("TEST_STREAM", options, func(ctx context.Context, msg *queue.Message) *response.AppError {
			data := string(msg.Body)
			logger.Info("message", "data", data)
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
				ID:     fmt.Sprintf("error-test-msg-%d", i),
				Body:   []byte(msgContent),
				Header: baseHeader(),
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

// test each consume with each subject
func TestEachConsumeWithEachSubject(t *testing.T) {
	conn, err := NewConnectWithJetStream(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")
	require.NotNil(t, conn, "Connection should not be nil")
	defer endTest(conn)

	subjects := []string{"TEST_STREAM.subject1", "TEST_STREAM.subject2.oke", "TEST_STREAM.subject3"}
	tracker := NewMessageTracker()

	optionsSubject1 := queue.Options[SubJSOption]{
		Config: SubJSOption{
			Group:           "js-subject1-group",
			AckPolicy:       jetstream.AckExplicitPolicy,
			MaxDeliver:      1,
			AckWait:         5 * time.Second,
			DeliverPolicy:   jetstream.DeliverAllPolicy,
			MaxAckPending:   1,
			IncludeSubjects: []string{"TEST_STREAM.subject1"},
		},
	}

	err = conn.Subscribe("TEST_STREAM", optionsSubject1, func(ctx context.Context, msg *queue.Message) *response.AppError {
		data := string(msg.Topic)
		tracker.AddMessage(fmt.Sprintf("Subject1: %s", data))
		slog.Info("Subject1 handler", "data", data)
		return nil
	})
	if err != nil {
		t.Error("Subscribe error", err)
	}
	optionsSubject2 := queue.Options[SubJSOption]{
		Config: SubJSOption{
			Group:           "js-subject2-group",
			AckPolicy:       jetstream.AckExplicitPolicy,
			MaxDeliver:      1,
			AckWait:         5 * time.Second,
			DeliverPolicy:   jetstream.DeliverAllPolicy,
			MaxAckPending:   1,
			IncludeSubjects: []string{"TEST_STREAM.subject2.oke"},
		},
	}

	err = conn.Subscribe("TEST_STREAM", optionsSubject2, func(ctx context.Context, msg *queue.Message) *response.AppError {
		data := string(msg.Topic)
		tracker.AddMessage(fmt.Sprintf("Subject2: %s", data))
		slog.Info("Subject2 handler", "data", data)
		return nil
	})
	if err != nil {
		t.Error("Subscribe error", err)
	}
	optionsSubject3 := queue.Options[SubJSOption]{
		Config: SubJSOption{
			Group:           "js-subject3-group",
			AckPolicy:       jetstream.AckExplicitPolicy,
			MaxDeliver:      1,
			AckWait:         5 * time.Second,
			DeliverPolicy:   jetstream.DeliverAllPolicy,
			MaxAckPending:   1,
			IncludeSubjects: []string{"TEST_STREAM.subject3"},
		},
	}

	err = conn.Subscribe("TEST_STREAM", optionsSubject3, func(ctx context.Context, msg *queue.Message) *response.AppError {
		data := string(msg.Topic)
		tracker.AddMessage(fmt.Sprintf("Subject3: %s", data))
		slog.Info("Subject3 handler", "data", data)
		return nil
	})
	if err != nil {
		t.Error("Subscribe error", err)
	}

	for _, subject := range subjects {
		msg := &queue.Message{
			ID:     fmt.Sprintf("each-subject-%s", subject),
			Body:   fmt.Appendf(nil, "Message for %s", subject),
			Header: baseHeader(),
		}

		err = conn.PublishNor(context.Background(), subject, msg)
		require.NoError(t, ConvertAppErrorToError(err), "Publishing message should not fail for subject: %s", subject)
	}

	// Wait for message processing
	time.Sleep(1 * time.Second)
}

func TestShutdownTestConsumeAllMsg(t *testing.T) {
	conn, err := NewConnectWithJetStream(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")
	require.NotNil(t, conn, "Connection should not be nil")
	defer endTest(conn)
	optionsSubject1 := queue.Options[SubJSOption]{
		Concurrent: 2,
		Config: SubJSOption{
			Group:         "js-drain-group",
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxDeliver:    2,
			AckWait:       5 * time.Second,
			DeliverPolicy: jetstream.DeliverAllPolicy,
			MaxAckPending: 5,
		},
	}

	err = conn.Subscribe("TEST_STREAM", optionsSubject1, func(ctx context.Context, msg *queue.Message) *response.AppError {
		data := string(msg.Body)
		slog.Info("drain handler", "data", data)
		time.Sleep(2 * time.Second)
		slog.Info("drain handler after sleep", "data", data)
		return nil
	})
	if err != nil {
		t.Error("Subscribe error", err)
	}

	// send 20 msg
	for idx := range 10 {
		subject := "TEST_STREAM.drain"
		msg := &queue.Message{
			Body:   fmt.Appendf(nil, "Message for %d", idx),
			Header: baseHeader(),
		}

		err = conn.PublishNor(context.Background(), subject, msg)
		require.NoError(t, ConvertAppErrorToError(err), "Publishing message should not fail for subject: %s", subject)
	}
	time.Sleep(3 * time.Second)
}

func TestHealthCheck(t *testing.T) {
	conn, err := NewConnectWithJetStream(cfg, logger)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")
	require.NotNil(t, conn, "Connection should not be nil")
	defer endTest(conn)

	err = conn.HealthCheckJS()
	require.NoError(t, ConvertAppErrorToError(err), "Health check should not fail")

	subject := queue.Options[SubJSOption]{
		Config: SubJSOption{
			Group:           "ListenerSubject",
			AckPolicy:       jetstream.AckExplicitPolicy,
			MaxDeliver:      1,
			AckWait:         5 * time.Second,
			DeliverPolicy:   jetstream.DeliverAllPolicy,
			MaxAckPending:   1,
			IncludeSubjects: []string{"TEST_STREAM.*"},
		},
	}

	err = conn.Subscribe("TEST_STREAM", subject, func(ctx context.Context, msg *queue.Message) *response.AppError {
		data := string(msg.Topic)
		slog.Info("Subject handler", "data", data)
		return nil
	})
	require.NoError(t, ConvertAppErrorToError(err), "Subscribe should not fail")
	ctx := context.Background()
	for idx := range 10 {
		subject := "TEST_STREAM.test"
		msg := &queue.Message{
			Body:   fmt.Appendf(nil, "Message for %d", idx),
			Header: baseHeader(),
		}

		err = conn.PublishNor(ctx, subject, msg)
		require.NoError(t, ConvertAppErrorToError(err), "Publishing message should not fail for subject: %s", subject)
	}
	time.Sleep(3 * time.Second)
}

func endTest(conn *Connection) {
	for _, stream := range cfg.Topics {
		conn.DeleteStream(stream.Name)
	}
	err := conn.Shutdown()
	if err != nil {
		slog.Error("Error during shutdown", "error", err)
	}
	slog.Info("End test completed, all streams deleted and connection shut down")
}
