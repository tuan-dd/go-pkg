package http

import (
	"context"
	"net/http"
	"time"

	typeCustom "github.com/tuan-dd/go-common/type-custom"
)

type (
	Option func(*Client)
	ctxKey string
)

const (
	bodyCtxKey        ctxKey = "body"
	reqCtxKey         ctxKey = "req"
	resCtxKey         ctxKey = "res"
	retryNumberCtxKey ctxKey = "retry_number"
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

func WithRetryConfig(retryFunc RetryFunc, retryCount int8) Option {
	return func(c *Client) {
		c.retryFunc = retryFunc
		c.retryCount = retryCount
	}
}

func WithHandleResponse(handleResponse HandleResponse) Option {
	return func(c *Client) {
		c.handleResponse = handleResponse
	}
}

func WithLogger(log typeCustom.Logger) Option {
	return func(c *Client) {
		c.log = log
	}
}

/**
 * GetResBody retrieves the response pointer body from the context.
 * It returns the body as type T and a boolean indicating if the retrieval was successful.
 * If statusCode error, it will return body []byte not parsed.
 */
func GetResBody[T any](ctx context.Context) (T, bool) {
	body, ok := ctx.Value(bodyCtxKey).(T)
	return body, ok
}

func GetRes(ctx context.Context) *http.Response {
	res, _ := ctx.Value(resCtxKey).(*http.Response)
	return res
}

func GetReq(ctx context.Context) *http.Request {
	req, _ := ctx.Value(reqCtxKey).(*http.Request)
	return req
}

func GetRetryNumber(ctx context.Context) int8 {
	retryNumber, _ := ctx.Value(retryNumberCtxKey).(int8)
	return retryNumber
}
