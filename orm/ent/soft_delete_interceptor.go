package entOrm

import (
	"context"
	"time"

	"github.com/tuan-dd/go-common/request"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"github.com/casbin/ent-adapter/ent/hook"
	"github.com/casbin/ent-adapter/ent/predicate"
)

// Hooks of the SoftDeleteMixin.

type (
	DeleteMutation interface {
		SetDeletedAt(time.Time)
		SetOp(ent.Op)
		SetDeletedBy(any)
	}
	SoftDeleteKey struct{}

	DeleteQuery interface {
		Where(...predicate.CasbinRule) DeleteQuery
	}
)

func (d SoftDeleteMixin) Hooks() []ent.Hook {
	return []ent.Hook{
		hook.On(
			func(next ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
					if skip, _ := ctx.Value(SoftDeleteKey{}).(bool); skip {
						return next.Mutate(ctx, m)
					}
					userID := request.GetUserCtx(ctx).ID
					if deleteMutation, ok := m.(DeleteMutation); ok {
						deleteMutation.SetOp(ent.OpDelete)
						deleteMutation.SetDeletedAt(time.Now().UTC())
						deleteMutation.SetDeletedBy(userID)
					}

					return next.Mutate(ctx, m)
				})
			},
			ent.OpDeleteOne|ent.OpDelete,
		),
	}
}

// Interceptors of the SoftDeleteMixin.
func (d SoftDeleteMixin) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{
		ent.TraverseFunc(func(ctx context.Context, q ent.Query) error {
			// Skip soft-delete, means include soft-deleted entities.
			if skip, _ := ctx.Value(SoftDeleteKey{}).(bool); skip {
				return nil
			}

			if deleteQuery, ok := q.(DeleteQuery); ok {
				deleteQuery.Where(sql.FieldIsNull(SOFT_DELETE_AT_COLUMN_NAME))
			}
			return nil
		}),
	}
}
