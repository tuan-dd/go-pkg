package entOrm

import (
	"context"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/schema/mixin"
)

type SortType string

const (
	SortAsc  SortType = "ASC"
	SortDesc SortType = "DESC"
)

type OrderMixin struct {
	mixin.Schema
	DefaultField string
	DefaultSort  SortType
}

// Interceptors of the OrderMixin.
func (d OrderMixin) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{
		ent.TraverseFunc(func(ctx context.Context, q ent.Query) error {
			if query, ok := q.(interface {
				Order(o ...func(*sql.Selector)) ent.Query
			}); ok {
				query.Order(d.applyDefaultOrder)
			}
			return nil
		}),
	}
}

func (d OrderMixin) applyDefaultOrder(s *sql.Selector) {
	// Validate field name
	if d.DefaultField == "" {
		return
	}

	if len(s.OrderColumns()) == 0 {
		if d.DefaultSort == SortDesc {
			s.OrderBy(sql.Desc(d.DefaultField))
		} else {
			s.OrderBy(sql.Asc(d.DefaultField))
		}
	}
}
