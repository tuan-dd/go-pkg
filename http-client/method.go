package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/tuan-dd/go-pkg/common/response"
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

type ctxKey string

const BodyCtxKey ctxKey = "body"

func Get[T any](ctx context.Context, cl *Client, option *FetchOp) (*T, *response.AppError) {
	option = buildOptions(option)

	rel := &url.URL{Path: option.Path}
	if option.Query != nil {
		queryString := option.Query.Encode()
		rel.RawQuery = queryString
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

	for key, value := range cl.defaultHeaders {
		req.Header.Set(key, value[0])
	}

	if option.Headers != nil {
		for key, value := range *option.Headers {
			req.Header.Set(key, value[0])
		}
	}

	resp, err := cl.fetch(ctx, req)
	if err != nil {
		cl.log.Error("Error making request", err)
		return nil, response.ServerError(err.Error())
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			cl.log.Error("Error closing response body", err)
		}
	}()

	res := new(T)
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return nil, response.ServerError(err.Error())
	}

	// Handle the response
	ctx = context.WithValue(ctx, BodyCtxKey, res)
	errApp := cl.hs(ctx, resp, option.HandleResponse)
	if errApp != nil {
		return nil, errApp
	}

	return res, nil
}

func Post[T any](ctx context.Context, cl *Client, option *FetchOp) (*T, *response.AppError) {
	option = buildOptions(option)

	rel := &url.URL{Path: option.Path}
	if option.Query != nil {
		queryString := option.Query.Encode()
		rel.RawQuery = queryString
	}

	u := cl.baseURL.ResolveReference(rel)

	var reader io.Reader
	if len(option.Body) > 0 {
		reader = bytes.NewReader(option.Body)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), reader)
	if err != nil {
		cl.log.Error("Error creating request", err)
		return nil, response.ServerError(fmt.Sprintf("Error creating request: %s", err.Error()))
	}

	for key, value := range cl.defaultHeaders {
		req.Header.Set(key, value[0])
	}

	if option.Headers != nil {
		for key, value := range *option.Headers {
			req.Header.Set(key, value[0])
		}
	}

	resp, err := cl.fetch(ctx, req)
	if err != nil {
		cl.log.Error("Error making request", err)
		return nil, response.ServerError(err.Error())
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			cl.log.Error("Error closing response body", err)
		}
	}()

	res := new(T)
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return nil, response.ServerError(err.Error())
	}

	// Handle the response
	ctx = context.WithValue(ctx, BodyCtxKey, res)
	errApp := cl.hs(ctx, resp, option.HandleResponse)
	if errApp != nil {
		return nil, errApp
	}

	return res, nil
}

func Put[T any](ctx context.Context, cl *Client, option *FetchOp) (*T, *response.AppError) {
	option = buildOptions(option)

	rel := &url.URL{Path: option.Path}
	if option.Query != nil {
		queryString := option.Query.Encode()
		rel.RawQuery = queryString
	}

	u := cl.baseURL.ResolveReference(rel)

	var reader io.Reader
	if len(option.Body) > 0 {
		reader = bytes.NewReader(option.Body)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), reader)
	if err != nil {
		cl.log.Error("Error creating request", err)
		return nil, response.ServerError(fmt.Sprintf("Error creating request: %s", err.Error()))
	}

	for key, value := range cl.defaultHeaders {
		req.Header.Set(key, value[0])
	}

	if option.Headers != nil {
		for key, value := range *option.Headers {
			req.Header.Set(key, value[0])
		}
	}

	resp, err := cl.fetch(ctx, req)
	if err != nil {
		cl.log.Error("Error making request", err)
		return nil, response.ServerError(err.Error())
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			cl.log.Error("Error closing response body", err)
		}
	}()

	res := new(T)
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return nil, response.ServerError(err.Error())
	}

	// Handle the response
	ctx = context.WithValue(ctx, BodyCtxKey, res)
	errApp := cl.hs(ctx, resp, option.HandleResponse)
	if errApp != nil {
		return nil, errApp
	}

	return res, nil
}

func Delete(ctx context.Context, cl *Client, option *FetchOp) *response.AppError {
	option = buildOptions(option)

	rel := &url.URL{Path: option.Path}
	if option.Query != nil {
		queryString := option.Query.Encode()
		rel.RawQuery = queryString
	}

	u := cl.baseURL.ResolveReference(rel)

	var reader io.Reader
	if len(option.Body) > 0 {
		reader = bytes.NewReader(option.Body)
	}

	req, err := http.NewRequestWithContext(ctx, string(option.Method), u.String(), reader)
	if err != nil {
		cl.log.Error("Error creating request", err)
		return response.ServerError(fmt.Sprintf("Error creating request: %s", err.Error()))
	}

	for key, value := range cl.defaultHeaders {
		req.Header.Set(key, value[0])
	}

	if option.Headers != nil {
		for key, value := range *option.Headers {
			req.Header.Set(key, value[0])
		}
	}

	resp, err := cl.fetch(ctx, req)
	if err != nil {
		cl.log.Error("Error making request", err)
		return response.ServerError(err.Error())
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			cl.log.Error("Error closing response body", err)
		}
	}()

	res := make(map[string]any)
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return response.ServerError(err.Error())
	}
	ctx = context.WithValue(ctx, BodyCtxKey, res)
	errApp := cl.hs(ctx, resp, option.HandleResponse)
	if errApp != nil {
		return errApp
	}

	return nil
}

func Do[T any](ctx context.Context, cl *Client, option *FetchOp) (*T, *response.AppError) {
	option = buildOptions(option)

	rel := &url.URL{Path: option.Path}
	if option.Query != nil {
		queryString := option.Query.Encode()
		rel.RawQuery = queryString
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

	for key, value := range cl.defaultHeaders {
		req.Header.Set(key, value[0])
	}

	if option.Headers != nil {
		for key, value := range *option.Headers {
			req.Header.Set(key, value[0])
		}
	}

	resp, err := cl.fetch(ctx, req)
	if err != nil {
		cl.log.Error("Error making request", err)
		return nil, response.ServerError(err.Error())
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			cl.log.Error("Error closing response body", err)
		}
	}()

	res := new(T)
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return nil, response.ServerError(err.Error())
	}

	// Handle the response
	ctx = context.WithValue(ctx, BodyCtxKey, res)
	errApp := cl.hs(ctx, resp, option.HandleResponse)
	if errApp != nil {
		return nil, errApp
	}

	return res, nil
}
