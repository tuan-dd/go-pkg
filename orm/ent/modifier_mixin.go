package entOrm

import (
	"context"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

// -------------------------------------------------
// Mixin definition

// ModifierMixin implements the ent.Mixin for sharing
// string fields with package schemas.
type (
	ModifierMixin struct {
		// We embed the `mixin.Schema` to avoid
		// implementing the rest of the methods.
		mixin.Schema
	}
	ModifierKey struct{}
	CreatorKey  struct{}
)

const (
	CREATED_BY_COLUMN_NAME = "created_by"
	UPDATED_BY_COLUMN_NAME = "updated_by"
)

func (ModifierMixin) Fields() []ent.Field {
	return []ent.Field{
		field.String(CREATED_BY_COLUMN_NAME).
			Immutable(),
		field.String(UPDATED_BY_COLUMN_NAME).
			Optional().
			Nillable(),
	}
}

func SkipModifier(parent context.Context) context.Context {
	return context.WithValue(parent, ModifierKey{}, true)
}

func SkipCreator(parent context.Context) context.Context {
	return context.WithValue(parent, CreatorKey{}, true)
}
