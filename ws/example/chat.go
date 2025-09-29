package main

import (
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
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

func HandleMessage(msg []byte) *response.AppError {
	// Simulate message handling
	if len(msg) == 0 {
		return nil
	}
	logger.Info("Message handled successfully", "msg", string(msg))
	return nil
}

func createTestServer() (*http.ServeMux, *ws.Board) {
	mux := http.NewServeMux()
	board := ws.NewBoard(&ws.Config{
		WriteWait:       20 * time.Second,
		PongWait:        15 * time.Second,
		MaxMessageSize:  1024 * 1024, // 1 MB
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}, logger)

	return mux, board
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "home.html")
}

func main() {
	mux, board := createTestServer()

	mux.HandleFunc("/", serveHome)
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Starting test server with unique ID:")
		client, appErr := board.UpgradeWebSocket(w, r)
		if appErr != nil {
			logger.Error("Failed to upgrade WebSocket", "error", appErr)
			return
		}
		appErr = client.Process(HandleMessage)

		for i := range 10 {
			msg := fmt.Sprintf("Hello from server! %d", i+1)
			go sendMsg(client, msg)
		}

		if appErr != nil {
			logger.Error("Failed to process WebSocket message", "error", appErr)
			return
		}
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	fmt.Println("Server is running at http://localhost:8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Println("Server error:", err)
	}
	defer server.Close()
}

func sendMsg(client *ws.Client, msg string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Recovered from panic in sendMsg", r)
		}
	}()
	for {
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
		msgSend := fmt.Sprintf("%s %s", msg, time.Now().Format(time.RFC3339))
		msgBytes := []byte(msgSend)

		select {
		case <-client.Done:
			return
		default:
			client.MsgBytes <- msgBytes
		}
	}
}
