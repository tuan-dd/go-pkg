package entOrm

import (
	"context"
	"time"

	"github.com/tuan-dd/go-pkg/common/utils"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"github.com/casbin/ent-adapter/ent/hook"
)

// Hooks of the SoftDeleteMixin.

type (
	mutateClient interface {
		Mutate(context.Context, ent.Mutation) (ent.Value, error)
	}
	SoftDeleteKey struct{}
)

func (d SoftDeleteMixin) Hooks() []ent.Hook {
	return []ent.Hook{
		hook.On(
			func(next ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
					// Skip soft-delete, means delete the entity permanently.
					if skip, _ := ctx.Value(SoftDeleteKey{}).(bool); skip {
						return next.Mutate(ctx, m)
					}

					utils.CallMethod("SetOp", m, ent.OpUpdate)
					utils.CallMethod("SetDeletedAt", m, time.Now().UTC())

					client := utils.CallMethodWithValue[mutateClient]("Client", m)

					return client.Mutate(ctx, m)
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
			utils.CallMethod("Where", q, sql.FieldIsNull(SOFT_DELETE_AT_COLUMN_NAME))
			return nil
		}),
	}
}
