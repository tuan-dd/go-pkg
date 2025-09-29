package ws_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tuan-dd/go-common/request"
	"github.com/tuan-dd/go-common/response"
	"github.com/tuan-dd/go-pkg/ws"
)

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

func createTestServer() (*http.ServeMux, *ws.Board) {
	mux := http.NewServeMux()
	board := ws.NewBoard(&ws.Config{
		WriteWait:       10 * time.Second,
		PongWait:        60 * time.Second,
		MaxMessageSize:  1024 * 1024, // 1 MB
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}, logger)

	return mux, board
}

func HandleMessage(msg []byte) *response.AppError {
	// Simulate message handling
	if len(msg) == 0 {
		return nil
	}
	logger.Info("Message handled successfully", "msg", string(msg))
	return nil
}

func TestSendMsgClient(t *testing.T) {
	mux, board := createTestServer()
	unique := "test-unique-id"
	fmt.Println("Starting test server with unique ID:", unique)

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		client, appErr := board.UpgradeWebSocket(w, r)
		if appErr != nil {
			logger.Error("Failed to upgrade WebSocket", "error", appErr)
			return
		}
		appErr = client.Process(HandleMessage)
		if appErr != nil {
			logger.Error("Failed to process WebSocket message", "error", appErr)
			return
		}
	})
	time.Sleep(10 * time.Second)
	clients := board.GetClients(unique)
	client := clients[0]

	server := httptest.NewServer(mux)
	defer server.Close()
	time.Sleep(1 * time.Second)

	for i := 0; i < 10; i++ {
		msg := fmt.Sprintf("Test message %d", i)
		client.MsgBytes <- []byte(msg)
	}
}

func main() {
	mux, board := createTestServer()
	unique := "test-unique-id"
	fmt.Println("Starting test server with unique ID:", unique)

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		client, appErr := board.UpgradeWebSocket(w, r)
		if appErr != nil {
			logger.Error("Failed to upgrade WebSocket", "error", appErr)
			return
		}
		appErr = client.Process(HandleMessage)
		if appErr != nil {
			logger.Error("Failed to process WebSocket message", "error", appErr)
			return
		}
	})
	time.Sleep(10 * time.Second)
	clients := board.GetClients(unique)
	client := clients[0]

	server := httptest.NewServer(mux)
	defer server.Close()

	for i := 0; i < 10; i++ {
		msg := fmt.Sprintf("Test message %d", i)
		client.MsgBytes <- []byte(msg)
	}
}
