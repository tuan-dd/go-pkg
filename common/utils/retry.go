package utils

import (
	"context"
	"time"

	"github.com/tuan-dd/go-pkg/common/response"
)

func Retry[T any](f func() (T, error), maxRetry uint8) (T, *response.AppError) {
	var (
		result T
		err    error
	)

	if maxRetry == 0 {
		maxRetry = 3 // Default retry count if not specified
	}
	for range maxRetry {
		result, err = f()
		if errApp := response.ConvertError(err); errApp == nil {
			return result, nil
		}
	}

	return result, response.ConvertError(err)
}

type retryWithDelayOpts struct {
	MaxRetry uint8
	Delay    time.Duration
}

func DefaultConfig() *retryWithDelayOpts {
	return &retryWithDelayOpts{
		MaxRetry: 3,
		Delay:    1 * time.Second,
	}
}

// RetryWithDelay retries the function f with a delay between attempts.
func RetryWithDelay[T any](ctx context.Context, f func(ctx context.Context) (T, error), opts *retryWithDelayOpts) (T, *response.AppError) {
	if opts == nil {
		opts = DefaultConfig()
	}
	var (
		result T
		err    error
	)
	lastRetry := opts.MaxRetry - 1
	for i := range opts.MaxRetry {
		result, err = f(ctx)
		if errApp := response.ConvertError(err); errApp == nil {
			return result, nil
		}

		if i < lastRetry {
			select {
			case <-ctx.Done():
				return result, response.ConvertError(ctx.Err())
			case <-time.After(opts.Delay):
			}
		}
	}

	return result, response.ConvertError(err)
}
