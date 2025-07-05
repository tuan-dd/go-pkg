package entOrm

import (
	"context"

	"github.com/tuan-dd/go-pkg/common"

	"entgo.io/ent"
)

// Define interfaces for mutations that support modifier fields
type CreateMutation interface {
	SetCreatedBy(any)
	SetOp(ent.Op)
}

type UpdateMutation interface {
	SetUpdatedBy(any)
	SetOp(ent.Op)
}

func (d ModifierMixin) Hook() ent.Hook {
	return func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			client := next

			if skip, ok := ctx.Value(ModifierKey{}).(bool); ok && skip {
				return client.Mutate(ctx, m)
			}

			userID := common.GetUserCtx[any](ctx).ID
			switch m.Op() {
			case ent.OpCreate:
				if createMutation, ok := m.(CreateMutation); ok {
					createMutation.SetOp(ent.OpCreate)
					createMutation.SetCreatedBy(userID)
				}
			case ent.OpUpdateOne, ent.OpUpdate:
				if updateMutation, ok := m.(UpdateMutation); ok {
					updateMutation.SetOp(ent.OpUpdate)
					updateMutation.SetUpdatedBy(userID)
				}
			}

			return client.Mutate(ctx, m)
		})
	}
}
