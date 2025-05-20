package entOrm

import (
	"context"

	"github.com/tuan-dd/go-pkg/common"
	"github.com/tuan-dd/go-pkg/common/utils"

	"entgo.io/ent"
)

func (d ModifierMixin) Hook() ent.Hook {
	return func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			client := next

			if skip, ok := ctx.Value(ModifierKey{}).(bool); ok && skip {
				return client.Mutate(ctx, mutation)
			}

			userID := common.GetUserCtx[any](ctx).ID
			switch mutation.Op() {
			case ent.OpCreate:
				utils.CallMethod("SetOp", mutation, ent.OpCreate)
				utils.CallMethod("SetCreatedBy", mutation, userID)
				client = utils.CallMethodWithValue[mutateClient]("Client", mutation)
			case ent.OpUpdateOne, ent.OpUpdate:
				utils.CallMethod("SetOp", mutation, ent.OpUpdate)
				utils.CallMethod("SetUpdatedBy", mutation, userID)
				client = utils.CallMethodWithValue[mutateClient]("Client", mutation)
			}

			return client.Mutate(ctx, mutation)
		})
	}
}
