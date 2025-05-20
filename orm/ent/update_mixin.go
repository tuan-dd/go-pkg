package entOrm

import (
	"github.com/tuan-dd/go-pkg/common/utils"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

type TimeMixin struct {
	mixin.Schema
}

func (TimeMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time("created_at").
			Default(utils.UTCDate).
			Immutable().
			Annotations(
				entsql.Default("CURRENT_TIMESTAMP"),
			),
		field.Time("updated_at").UpdateDefault(utils.UTCDate).
			Optional().
			Nillable(),
	}
}
