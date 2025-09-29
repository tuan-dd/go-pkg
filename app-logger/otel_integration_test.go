package appLogger

// TestIntegrationWithOtelCollector requires the OpenTelemetry collector to be running.
// It sends logs to the collector specified at localhost:4319.

// removed to avoid dependency on internal obs package

// func TestIntegrationWithOtelCollector(t *testing.T) {
// 	// 1. Initialize OpenTelemetry using the custom obs package
// 	otelConfig := obs.OtelConfig{
// 		Endpoint:   "localhost:4319", // As per docker-compose.dev.yml otelcol service ports for gRPC
// 		Insecure:   true,
// 		SvcName:    "test-integration-logger",
// 		SvcVersion: "1.0.0",
// 		Env:        "development",
// 	}
// 	providers, err := obs.InitOTel(context.Background(), otelConfig)
// 	if err != nil {
// 		t.Fatalf("Failed to initialize OpenTelemetry: %v", err)
// 	}
// 	defer providers.Shutdown(context.Background())

// 	// 2. Create an OTelCore with the LoggerProvider from the obs package
// 	core := NewOTelCore(providers.LoggerProvider, otelConfig.SvcName, zapcore.DebugLevel)

// 	// 3. Create a zap.Logger with the new core
// 	logger := zap.New(core)

// 	// 4. Log messages
// 	logger.Info("This is an informational message from the integration test.", zap.String("test-id", "otel-integration"))
// 	logger.Warn("This is a warning message.", zap.Int("user-id", 12345))
// 	logger.Error("This is an error message.", zap.Error(context.DeadlineExceeded))

// 	// Give some time for the logs to be exported
// 	t.Log("Logs sent. Waiting for 5 seconds to allow for export...")
// 	time.Sleep(5 * time.Second)

// 	t.Log("Test finished. Check your Loki or collector's output for the logs.")
// }
