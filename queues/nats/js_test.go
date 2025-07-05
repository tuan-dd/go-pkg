package natsQueue

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tuan-dd/go-pkg/common/queue"
	"github.com/tuan-dd/go-pkg/common/response"
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
	logger.Info("Creating NATS JetStream connection", err)
	require.NoError(t, ConvertAppErrorToError(err), "Failed to create NATS JetStream connection")
	require.NotNil(t, conn, "Connection should not be nil")
	defer endTest(conn)

	// Setup message trackers for different subscription methods
	jsTracker := NewMessageTracker()
	jsManageTracker := NewMessageTracker()

	// Test JetStream Subscribe with consumer groups
	jsOptions := queue.Options[SubJSOption]{
		Config: SubJSOption{
			BasicJSOption: BasicJSOption{
				Group:         "js-test-group",
				AckPolicy:     jetstream.AckExplicitPolicy,
				MaxDeliver:    3,
				AckWait:       5 * time.Second,
				DeliverPolicy: jetstream.DeliverAllPolicy,
				MaxAckPending: 10,
				Delay:         1 * time.Second,
			},
		},
	}

	err = conn.Subscribe("TEST_STREAM", jsOptions, func(ctx context.Context, msg *queue.Message) *response.AppError {
		data := string(msg.Body)
		jsTracker.AddMessage(fmt.Sprintf("JS-Subscribe: %s", data))
		logger.Info("JetStream Subscribe received", data)
		return nil
	})
	require.NoError(t, ConvertAppErrorToError(err), "JetStream subscribe should not fail")

	// Test JetStream SubscribeManage
	jsManageOptions := queue.Options[SubJSOption]{
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

	err = conn.SubscribeManage("TEST_STREAM", jsManageOptions, func(ctx context.Context, msg *queue.Message) *response.AppError {
		data := string(msg.Body)
		jsManageTracker.AddMessage(fmt.Sprintf("JS-Manage: %s", data))
		logger.Info("JetStream SubscribeManage received", data)
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
			ID:      fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			Body:    []byte(testMsg.content),
			Headers: baseHeader(),
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
			BasicJSOption: BasicJSOption{
				Group:         "retry-test-group",
				AckPolicy:     jetstream.AckExplicitPolicy,
				MaxDeliver:    3, // Allow up to 3 delivery attempts
				AckWait:       2 * time.Second,
				DeliverPolicy: jetstream.DeliverAllPolicy,
				MaxAckPending: 5,
				Delay:         500 * time.Millisecond, // Delay between retries
			},
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
			ID:      fmt.Sprintf("retry-test-%d", i),
			Body:    []byte(msgID),
			Headers: header,
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
