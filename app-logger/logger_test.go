package appLogger

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tuan-dd/go-common/request"
	"github.com/tuan-dd/go-common/response"
)

func TestFileLogging(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	tests := []struct {
		name           string
		config         *LoggerConfig
		logActions     func(*Logger)
		expectedInFile []string
		notInFile      []string
	}{
		{
			name: "only errors should be in file",
			config: &LoggerConfig{
				Level:       "debug",
				LogToFile:   true,
				LogFilePath: logFile,
				MaxSize:     10,
				MaxAge:      7,
				MaxBackups:  3,
				Compress:    false,
			},
			logActions: func(logger *Logger) {
				logger.Debug("debug message")
				logger.Info("info message")
				logger.Warn("warn message")
				logger.Error("error message", response.ServerError("test error"))
				logger.AppErrorWithAlert(&request.ReqContext{}, response.ServerError("app error message"))
			},
			expectedInFile: []string{
				"error message",
				"app error message",
			},
			notInFile: []string{
				"debug message",
				"info message",
				"warn message",
			},
		},
		{
			name: "file logging disabled",
			config: &LoggerConfig{
				Level:     "debug",
				LogToFile: false,
			},
			logActions: func(logger *Logger) {
				logger.Error("this should not be in file", nil)
			},
			expectedInFile: []string{},
			notInFile:      []string{"this should not be in file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing log file
			os.Remove(logFile)

			serverConfig := &ServerSetting{
				Environment: "test",
				ServiceName: "test-service",
			}
			logger, errApp := NewLogger(tt.config, serverConfig)
			if errApp != nil {
				t.Fatalf("Failed to create logger: %v", errApp)
			}
			// Execute log actions
			tt.logActions(logger)

			// Force sync
			logger.logger.Sync()

			if !tt.config.LogToFile {
				// Check file doesn't exist
				if _, err := os.Stat(logFile); !os.IsNotExist(err) {
					t.Error("Log file should not exist when LogToFile is false")
				}
				return
			}

			// Check if file was created
			if _, err := os.Stat(logFile); os.IsNotExist(err) {
				if len(tt.expectedInFile) > 0 {
					t.Error("Expected log file to be created")
				}
				return
			}
			// Read file content
			content, err := readLogFile(logFile)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			// Check expected content is in file
			for _, expected := range tt.expectedInFile {
				if !strings.Contains(content, expected) {
					t.Errorf("Expected '%s' to be in log file, but it wasn't found", expected)
				}
			}

			// Check unwanted content is not in file
			for _, notWanted := range tt.notInFile {
				if strings.Contains(content, notWanted) {
					t.Errorf("Expected '%s' NOT to be in log file, but it was found", notWanted)
				}
			}

			// Verify JSON format for file logs
			if tt.config.LogToFile && len(tt.expectedInFile) > 0 {
				validateJSONFormat(t, logFile)
			}
		})
	}
}

func TestFileRotation(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "rotation_test.log")

	config := &LoggerConfig{
		Level:       "error",
		LogToFile:   true,
		LogFilePath: logFile,
		MaxSize:     1, // 1MB - small size to trigger rotation
		MaxAge:      1,
		MaxBackups:  2,
		Compress:    false,
	}

	serverConfig := &ServerSetting{
		Environment: "test",
		ServiceName: "test-service",
	}

	logger, errApp := NewLogger(config, serverConfig)
	if errApp != nil {
		t.Fatalf("Failed to create logger: %v", errApp)
	}

	// Write many error logs to trigger rotation
	largeData := strings.Repeat("a", 1000)
	for range 10 {
		logger.Error("large error message", response.ServerError(largeData))
	}

	for i := range 2 {
		logger.ErrorPanic(request.GetReqCtx(context.Background()), "test panic message "+strconv.Itoa(i), response.ServerError("test panic"), "test")
	}

	_ = logger.logger.Sync()

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Expected log file to exist")
	}
}

func TestFilePermissions(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "permission_test.log")

	config := &LoggerConfig{
		Level:       "error",
		LogToFile:   true,
		LogFilePath: logFile,
	}

	serverConfig := &ServerSetting{
		Environment: "test",
		ServiceName: "test-service",
	}

	logger, errApp := NewLogger(config, serverConfig)
	if errApp != nil {
		t.Fatalf("Failed to create logger: %v", errApp)
	}

	logger.Error("test error", nil)
	logger.logger.Sync()

	// Check file permissions
	fileInfo, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	mode := fileInfo.Mode()
	t.Logf("File permissions: %v", mode)
}

func TestDefaultLogFileName(t *testing.T) {
	tempDir := t.TempDir()

	// Change working directory to temp dir
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)

	config := &LoggerConfig{
		Level:       "error",
		LogToFile:   true,
		LogFilePath: "", // Empty path should use default
	}

	serverConfig := &ServerSetting{
		Environment: "test",
		ServiceName: "test-service",
	}

	logger, errApp := NewLogger(config, serverConfig)
	if errApp != nil {
		t.Fatalf("Failed to create logger: %v", errApp)
	}

	logger.Error("test error with default file name", nil)
	logger.logger.Sync()

	// Check if default file was created
	defaultFile := filepath.Join(tempDir, "app.log")
	if _, err := os.Stat(defaultFile); os.IsNotExist(err) {
		t.Error("Expected default log file 'app.log' to be created")
	}
}

// Helper functions
func readLogFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func validateJSONFormat(t *testing.T, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try to parse as JSON
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			t.Errorf("Line %d is not valid JSON: %s", lineNum, line)
		}

		// Check required fields
		if _, ok := logEntry["level"]; !ok {
			t.Errorf("Line %d missing 'level' field", lineNum)
		}
		if _, ok := logEntry["msg"]; !ok {
			t.Errorf("Line %d missing 'msg' field", lineNum)
		}
		if _, ok := logEntry["ts"]; !ok {
			t.Errorf("Line %d missing 'ts' field", lineNum)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading log file: %v", err)
	}
}

type MockAlertService struct{}

func (m *MockAlertService) SendMsgStr(ctx context.Context, alert string) *response.AppError {
	fmt.Println("Mock alert sent:", alert)
	return nil
}

func TestAlert(t *testing.T) {
	logger, _ := NewLogger(&LoggerConfig{}, &ServerSetting{
		Environment: "test",
		ServiceName: "test-service",
	}, WithAlertService(&MockAlertService{}))

	reqCtx := request.ReqContext{CID: "test-cid", RequestTimestamp: 1755665587322}
	// ctx := context.Background()
	// ctx = context.WithValue(ctx, constants.REQUEST_CONTEXT_KEY, reqCtx)

	logger.ErrorPanic(&reqCtx, "GET", "/test/path", "error message")
}
