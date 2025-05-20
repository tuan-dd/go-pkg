package goroutine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tuan-dd/go-pkg/common/constants"
	"github.com/tuan-dd/go-pkg/common/response"

	"golang.org/x/sync/errgroup"
)

type PromiseFunc func(ctx context.Context) (any, *response.AppError)

func PromiseAllWithCtx(ctx context.Context, promises ...PromiseFunc) (map[int]any, *response.AppError) {
	if len(promises) > 1000 {
		return nil, response.NewAppError("too many promises", constants.ParamInvalid)
	}

	results := make(map[int]any, len(promises))
	eg, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	for i, promise := range promises {
		eg.Go(func(i int, promise PromiseFunc) func() error {
			return func() error {
				result, err := promise(ctx)
				if err != nil {
					return fmt.Errorf("promise %d failed: %w", i, err)
				}
				mu.Lock()
				defer mu.Unlock()
				results[i] = result
				return nil
			}
		}(i, promise))
	}

	if err := eg.Wait(); err != nil {
		return results, response.ConvertError(err)
	}

	return results, nil
}

func PromiseAll(promises ...PromiseFunc) (map[int]any, *response.AppError) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Minute))
	defer cancel()
	return PromiseAllWithCtx(ctx, promises...)
}
