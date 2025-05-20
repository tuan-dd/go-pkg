package goroutine

import (
	"context"
	"sync"

	"github.com/tuan-dd/go-pkg/common/response"

	"golang.org/x/sync/errgroup"
)

type (
	TaskIf[T any] interface {
		Process() (*T, *response.AppError)
	}

	WorkerTask[T any] struct {
		Concurrency       int
		StopWhenErrorFlag bool
		WaitFlag          bool
		mu                sync.Mutex
		data              []*T
		TaskChain         chan TaskIf[T]
	}
)

func (wt *WorkerTask[T]) Run(ctx context.Context) ([]*T, *response.AppError) {
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(wt.getConcurrency())
	// defer close(wt.TaskChain)

	process := wt.processNoSkipError
	if !wt.StopWhenErrorFlag {
		process = wt.processSkipError
	}

	if wt.WaitFlag {
		process(ctx, eg)
		err := eg.Wait()
		return wt.data, response.ConvertError(err)
	}
	go process(ctx, eg)

	return nil, nil
}

func (wt *WorkerTask[T]) getConcurrency() int {
	if wt.Concurrency > 100 || wt.Concurrency == 0 {
		return 10
	}
	return wt.Concurrency
}

func (wt *WorkerTask[T]) processSkipError(_ context.Context, g *errgroup.Group) {
	for task := range wt.TaskChain {
		g.Go(func() error {
			res, err := task.Process()
			wt.atomicAppend(res)
			if err != nil {
				return err
			}
			return nil
		})
	}
}

func (wt *WorkerTask[T]) atomicAppend(res *T) {
	wt.mu.Lock()
	wt.data = append(wt.data, res)
	wt.mu.Unlock()
}

func (wt *WorkerTask[T]) processNoSkipError(ctx context.Context, g *errgroup.Group) {
	for task := range wt.TaskChain {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return nil
			default:
				res, err := task.Process()
				wt.atomicAppend(res)
				if err != nil {
					return err
				}
				return nil
			}
		})
	}
}
