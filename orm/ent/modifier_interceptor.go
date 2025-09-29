package entOrm

import (
	"context"

	"entgo.io/ent"
	"github.com/casbin/ent-adapter/ent/hook"
	"github.com/tuan-dd/go-common/request"
)

// Define interfaces for mutations that support modifier fields
type CreateMutation interface {
	SetCreatedBy(string)
}

type UpdateMutation interface {
	SetUpdatedBy(string)
}

func (d ModifierMixin) Hooks() []ent.Hook {
	return []ent.Hook{
		hook.On(func(next ent.Mutator) ent.Mutator {
			return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
				userID := request.GetUserCtx(ctx).ID
				switch m.Op() {
				case ent.OpCreate:
					if skip, ok := ctx.Value(CreatorKey{}).(bool); ok && skip {
						return next.Mutate(ctx, m)
					}
					if createMutation, ok := m.(CreateMutation); ok {
						createMutation.SetCreatedBy(userID)
					}
				case ent.OpUpdate, ent.OpUpdateOne:
					if skip, ok := ctx.Value(ModifierKey{}).(bool); ok && skip {
						return next.Mutate(ctx, m)
					}
					if updateMutation, ok := m.(UpdateMutation); ok {
						updateMutation.SetUpdatedBy(userID)
					}
				}

				return next.Mutate(ctx, m)
			})
		}, ent.OpUpdateOne|ent.OpCreate|ent.OpUpdate),
	}
}
