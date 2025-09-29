package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bytedance/sonic/decoder"
	"github.com/tuan-dd/go-common/response"
)

const (
	MethodGet     Method = "GET"
	MethodHead    Method = "HEAD"
	MethodPost    Method = "POST"
	MethodPut     Method = "PUT"
	MethodPatch   Method = "PATCH" // RFC 5789
	MethodDelete  Method = "DELETE"
	MethodConnect Method = "CONNECT"
	MethodOptions Method = "OPTIONS"
	MethodTrace   Method = "TRACE"
)

type Method string

func Get[T any](ctx context.Context, cl *Client, option *FetchOp) (*T, *response.AppError) {
	if option == nil {
		option = &FetchOp{}
	}
	option.Method = MethodGet
	return executeReq[T](ctx, cl, option)
}

// Post performs a POST request
func Post[T any](ctx context.Context, cl *Client, option *FetchOp) (*T, *response.AppError) {
	option = buildOptions(option)
	option.Method = MethodPost
	return executeReq[T](ctx, cl, option)
}

// Put performs a PUT request
func Put[T any](ctx context.Context, cl *Client, option *FetchOp) (*T, *response.AppError) {
	option = buildOptions(option)
	option.Method = MethodPut
	return executeReq[T](ctx, cl, option)
}

func Delete(ctx context.Context, cl *Client, option *FetchOp) *response.AppError {
	option = buildOptions(option)

	option.Method = MethodDelete
	_, errApp := executeReq[any](ctx, cl, option)
	if errApp != nil {
		return errApp
	}

	return nil
}

func Do[T any](ctx context.Context, cl *Client, option *FetchOp) (*T, *response.AppError) {
	option = buildOptions(option)
	if option.Method == "" {
		option.Method = MethodGet
	}

	return executeReq[T](ctx, cl, option)
}

// buildReq constructs an HTTP request based on the provided FetchOp options
func buildReq(ctx context.Context, cl *Client, option *FetchOp) (*http.Request, *response.AppError) {
	option = buildOptions(option)

	rel := &url.URL{Path: option.Path}
	if option.Query != nil {
		rel.RawQuery = option.Query.Encode()
	}

	u := cl.baseURL.ResolveReference(rel)

	var reader io.Reader
	if len(option.Body) > 0 {
		reader = bytes.NewReader(option.Body)
	}

	req, err := http.NewRequestWithContext(ctx, string(option.Method), u.String(), reader)
	if err != nil {
		cl.log.Error("Error creating request", err)
		return nil, response.ServerError(fmt.Sprintf("Error creating request: %s", err.Error()))
	}

	// Set default headers
	for key, values := range cl.defaultHeaders {
		if len(values) > 0 {
			req.Header.Set(key, values[0])
		}
	}

	// Set custom headers
	if option.Headers != nil {
		for key, values := range *option.Headers {
			if len(values) > 0 {
				req.Header.Set(key, values[0])
			}
		}
	}

	return req, nil
}

// buildOptions initializes the FetchOp with default values if not set
func executeReq[T any](ctx context.Context, cl *Client, option *FetchOp) (*T, *response.AppError) {
	req, errApp := buildReq(ctx, cl, option)
	if errApp != nil {
		return nil, errApp
	}
	ctx = context.WithValue(ctx, reqCtxKey, req)
	resp, err := cl.fetch(ctx, req)
	if err != nil {
		return nil, response.ServerError(fmt.Sprintf("Error making request: %s", err.Error()))
	}
	defer resp.Body.Close()

	ctx = context.WithValue(ctx, resCtxKey, resp)

	// Process response body
	res := new(T)
	if IsSuccessStatus(resp.StatusCode) && (resp.ContentLength > 0 || resp.ContentLength == -1) {
		dec := decoder.NewStreamDecoder(resp.Body)
		if err := dec.Decode(res); err != nil {
			return nil, response.ServerError(fmt.Sprintf("Error decoding response body: %s", err.Error()))
		}
		ctx = context.WithValue(ctx, bodyCtxKey, res)
	} else if resp.ContentLength > 0 {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, response.ServerError(fmt.Sprintf("Error reading response body: %s", err.Error()))
		}
		ctx = context.WithValue(ctx, bodyCtxKey, bodyBytes)
	}

	// Handle the response (status code validation, etc.)
	errApp = cl.hs(ctx, resp, option.HandleResponse)
	if errApp != nil {
		return nil, errApp
	}

	return res, nil
}

// Fetch performs a request and returns the response without parsing the body.
// It is useful for cases where you want to handle the response body manually.
func (h *Client) Fetch(ctx context.Context, option *FetchOp) (*http.Response, *response.AppError) {
	req, errApp := buildReq(ctx, h, option)
	if errApp != nil {
		return nil, errApp
	}
	ctx = context.WithValue(ctx, reqCtxKey, req)
	resp, err := h.fetch(ctx, req)
	if err != nil {
		return nil, response.ServerError(fmt.Sprintf("Failed to fetch: %v", err))
	}

	return resp, h.hs(ctx, resp, option.HandleResponse)
}
