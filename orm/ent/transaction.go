package entOrm

import (
	"context"
	"log/slog"

	"github.com/casbin/ent-adapter/ent"
	"github.com/tuan-dd/go-pkg/common/response"
)

func WithTransaction[T any](ctx context.Context, client *ent.Client, fn func(context.Context, Tx) (*T, *response.AppError)) (*T, *response.AppError) {
	tx, err := client.Tx(ctx)
	if err != nil {
		return nil, response.ServerError("failed to start transaction: " + err.Error())
	}

	var result *T
	var fnErr *response.AppError

	defer func() {
		if p := recover(); p != nil {
			// Panic occurred, rollback and convert to AppError
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				slog.Error("failed to rollback transaction after panic",
					slog.Any("panic", p),
					slog.Any("rollback_error", rollbackErr))
			}
			result = nil
			fnErr = response.ServerError("transaction failed due to panic")
			return
		}

		if fnErr != nil {
			// Business logic error, rollback
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				fnErr = response.ServerError("transaction rollback failed: " + rollbackErr.Error())
				slog.Error("failed to rollback transaction", slog.Any("error", rollbackErr))
				result = nil
			}
		} else {
			if commitErr := tx.Commit(); commitErr != nil {
				slog.Error("failed to commit transaction", slog.Any("error", commitErr))
				fnErr = response.ServerError("transaction commit failed: " + commitErr.Error())
				result = nil
			}
		}
	}()

	result, fnErr = fn(ctx, tx)
	return result, fnErr
}

type Tx interface {
	Client() *ent.Client
	OnRollback(f ent.RollbackHook)
	OnCommit(f ent.CommitHook)
}

type Fn func(context.Context, Tx) (any, *response.AppError)

// WithTransactionVoid is a helper for transactions that don't return data
func WithTransactionVoid(ctx context.Context, client *ent.Client, fn func(context.Context, Tx) *response.AppError) *response.AppError {
	result, err := WithTransaction(ctx, client, func(ctx context.Context, tx Tx) (*struct{}, *response.AppError) {
		if fnErr := fn(ctx, tx); fnErr != nil {
			return nil, fnErr
		}
		return &struct{}{}, nil
	})
	if err != nil {
		return err
	}

	_ = result // result is always &struct{}{} if successful
	return nil
}

// WithTransactionMultiple handles transactions that return multiple values
func WithTransactionMultiple[T1, T2 any](
	ctx context.Context,
	client *ent.Client,
	fn func(context.Context, Tx) (*T1, *T2, *response.AppError),
) (*T1, *T2, *response.AppError) {
	type result struct {
		v1 *T1
		v2 *T2
	}

	res, err := WithTransaction(ctx, client, func(ctx context.Context, tx Tx) (*result, *response.AppError) {
		v1, v2, fnErr := fn(ctx, tx)
		if fnErr != nil {
			return nil, fnErr
		}
		return &result{v1: v1, v2: v2}, nil
	})
	if err != nil {
		return nil, nil, err
	}

	return res.v1, res.v2, nil
}

// SafeRollback safely rolls back a transaction with error logging
func SafeRollback(tx *ent.Tx, originalErr *response.AppError) *response.AppError {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		slog.Error("failed to rollback transaction",
			slog.Any("original_error", originalErr),
			slog.Any("rollback_error", rollbackErr))
		return response.ServerError("transaction rollback failed: " + rollbackErr.Error())
	}
	return originalErr
}

// SafeCommit safely commits a transaction with error handling
func SafeCommit(tx *ent.Tx) *response.AppError {
	if commitErr := tx.Commit(); commitErr != nil {
		slog.Error("failed to commit transaction", slog.Any("error", commitErr))
		return response.ServerError("transaction commit failed: " + commitErr.Error())
	}
	return nil
}
