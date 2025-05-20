package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tuan-dd/go-pkg/appLogger"
	"github.com/tuan-dd/go-pkg/common/constants"
	"github.com/tuan-dd/go-pkg/common/response"
	"github.com/tuan-dd/go-pkg/settings"
)

type (
	HandleResponse func(context.Context, *http.Response) *response.AppError

	RetryFunc func(context.Context, error) bool

	Client struct {
		log            *appLogger.Logger
		httpClient     *http.Client
		baseURL        *url.URL
		retryCount     int8
		retryInterval  time.Duration
		retryFunc      RetryFunc
		handleResponse HandleResponse
		defaultHeaders http.Header
	}

	FetchOp struct {
		Method         Method
		Path           string
		HandleResponse HandleResponse
		Body           []byte
		Query          *url.Values
		Headers        *http.Header
	}
)

var BASE_HEADERS = http.Header{
	"Content-Type": []string{"application/json"},
	"Accept":       []string{"application/json"},
}

func (h *Client) SetHeaders(headers http.Header) {
	h.defaultHeaders = headers
}

func (h *Client) GetHeaders() http.Header {
	return h.defaultHeaders
}

func (h *Client) hs(ctx context.Context, resp *http.Response, fun HandleResponse) *response.AppError {
	if fun != nil {
		return fun(ctx, resp)
	}

	return h.handleResponse(ctx, resp)
}

func (h *Client) fetch(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	if h.retryFunc == nil {
		return h.httpClient.Do(req)
	}

	for range h.retryCount {
		resp, err = h.httpClient.Do(req)
		if err == nil {
			return resp, nil
		}

		if !h.retryFunc(ctx, err) {
			break
		}

		time.Sleep(h.retryInterval)
	}

	return nil, err
}

func baseHandleResponse(ctx context.Context, res *http.Response) *response.AppError {
	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusCreated || res.StatusCode == http.StatusNoContent {
		return nil
	}

	bodyRes := make(map[string]any)
	_ = json.NewDecoder(res.Body).Decode(&bodyRes)

	defer func() {
		_ = res.Body.Close()
	}()

	switch res.StatusCode {
	case http.StatusBadRequest:
		return response.QueryInvalid("Bad Request").WithData(bodyRes)
	case http.StatusUnauthorized:
		return response.Unauthorized("Unauthorized").WithData(bodyRes)
	case http.StatusForbidden:
		return response.NewAppError("Access Denied", constants.ConflictData).WithData(bodyRes)
	case http.StatusConflict:
		return response.NewAppError("Conflict", constants.DuplicateData).WithData(bodyRes)
	case http.StatusNotFound:
		return response.NotFound("Not Found").WithData(bodyRes)
	}

	if res.StatusCode < http.StatusInternalServerError {
		return response.UnknownError("Internal Server Error").WithData(bodyRes)
	}

	return response.ServerError("Internal Server Error").WithData(bodyRes)
}

func NewHttpClient(baseURL string, opts ...Option) (*Client, *response.AppError) {
	trimmed := strings.TrimRight(baseURL, "/")
	parsed, err := url.Parse(trimmed)
	if err != nil {
		erStr := fmt.Sprintf("invalid base URL %q: %v", baseURL, err)
		return nil, response.ServerError(erStr)
	}

	c := &Client{
		baseURL:        parsed,
		handleResponse: baseHandleResponse,
		defaultHeaders: BASE_HEADERS.Clone(),
		httpClient:     &http.Client{Timeout: 10 * time.Second},
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.log == nil {
		c.log, _ = appLogger.NewLogger(&appLogger.LoggerConfig{Level: "info"}, &settings.ServerSetting{Environment: "dev", ServiceName: "http-client"})
	}

	return c, nil
}

func buildOptions(option *FetchOp) *FetchOp {
	if option == nil {
		return &FetchOp{
			Method: MethodGet,
		}
	}

	if option.Method == "" {
		option.Method = MethodGet
	}

	return option
}
