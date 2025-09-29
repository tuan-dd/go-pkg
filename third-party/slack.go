package thirdPkg

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/tuan-dd/go-common/response"
	typeCustom "github.com/tuan-dd/go-common/type-custom"
)

type SlackConnection struct {
	webHook     string
	serviceName string
	http        *http.Client
	log         typeCustom.Logger
}

type SlackConfig struct {
	WebHook     string `mapstructure:"WEBHOOK"`
	ServiceName string `mapstructure:"SERVICE_NAME"` // e.g., "smm-service"
}

// var baseURL = "https://hooks.slack.com/services/"

func NewSlackConnection(config *SlackConfig) (*SlackConnection, *response.AppError) {
	return &SlackConnection{
		webHook:     config.WebHook,
		serviceName: config.ServiceName,
		http: &http.Client{
			Timeout: 30 * time.Second, // Set a timeout for the HTTP client
		},
		log: slog.Default(),
	}, nil
}

func (s *SlackConnection) SendMsgStr(ctx context.Context, message string) *response.AppError {
	if message == "" {
		return nil
	}

	body := map[string]string{
		"text": message,
	}

	bodyBytes, err := sonic.Marshal(body)
	if err != nil {
		return response.ServerError("Failed to marshal message: " + err.Error())
	}
	res, errApp := s.http.Post(s.webHook, "application/json", bytes.NewBuffer(bodyBytes))

	if errApp != nil {
		return response.ServerError("Failed to send message to Slack: " + errApp.Error())
	}

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return response.ServerError("Slack API returned non-OK status: " + res.Status + ", response: " + string(bodyBytes))
	}

	return nil
}

type Msg struct {
	Text        string `json:"text"`
	Blocks      []any  `json:"blocks,omitempty"`
	Attachments []any  `json:"attachments,omitempty"`
	ThreadTs    string `json:"thread_ts,omitempty"`
	MrkdwnIn    string `json:"mrkdwn_in,omitempty"`
}

func (s *SlackConnection) SendMsg(ctx context.Context, msg *Msg) *response.AppError {
	if msg == nil {
		return nil
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return response.ServerError("Failed to marshal message" + err.Error())
	}

	res, errApp := s.http.Post(s.webHook, "application/json", bytes.NewBuffer(msgBytes))
	if errApp != nil {
		return response.ServerError("Failed to send message to Slack: " + errApp.Error())
	}
	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return response.ServerError("Slack API returned non-OK status: " + res.Status + ", response: " + string(bodyBytes))
	}
	return nil
}
