package entOrm

import (
	"context"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

type SoftDeleteMixin struct {
	mixin.Schema
}

const (
	SOFT_DELETE_AT_COLUMN_NAME = "deleted_at"
	SOFT_DELETE_BY_COLUMN_NAME = "deleted_by"
)

func (SoftDeleteMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time(SOFT_DELETE_AT_COLUMN_NAME).
			Optional().
			Nillable(),
		field.String(SOFT_DELETE_BY_COLUMN_NAME).
			Optional().
			Nillable().Annotations(),
	}
}

// SkipSoftDelete returns a new context that skips the soft-delete interceptor/mutators.
func SkipSoftDelete(parent context.Context) context.Context {
	return context.WithValue(parent, SoftDeleteMixin{}, true)
}
