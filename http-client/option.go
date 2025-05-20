package http

import (
	"time"

	"github.com/tuan-dd/go-pkg/appLogger"
)

type (
	Option func(*Client)
)

func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

func WithHeader(key, value string) Option {
	return func(c *Client) {
		c.defaultHeaders.Set(key, value)
	}
}

func WithRetryConfig(retryFunc RetryFunc, retryCount int8, retryInterval int) Option {
	return func(c *Client) {
		c.retryFunc = retryFunc
		c.retryCount = retryCount
		c.retryInterval = time.Duration(retryInterval) * time.Second
	}
}

func WithHandleResponse(handleResponse HandleResponse) Option {
	return func(c *Client) {
		c.handleResponse = handleResponse
	}
}

func WithLogger(log *appLogger.Logger) Option {
	return func(c *Client) {
		c.log = log
	}
}
