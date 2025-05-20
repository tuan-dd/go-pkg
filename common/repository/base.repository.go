package repository

import (
	"context"
	"database/sql"
	"log"
	"reflect"

	"github.com/tuan-dd/go-pkg/common/response"
)

type (
	EUpdateConflictString string
	PaginationQuery       struct {
		CountSQL    string
		QuerySQL    string
		Params      []any
		Page        int
		Take        int
		ItemMappers func(*sql.Rows) any
	}

	Where struct {
		Condition string
		Params    []any
	}

	FindOption struct {
		Where
		Page           uint
		Limit          uint
		Order          []string
		Select         []string
		IncludeDeleted bool
	}

	UpdateOption struct {
		Where
		Set    map[string]any
		Select []string
		Limit  uint
	}

	IBaseEntity interface {
		TableName() string
	}

	RepositoryQueryIf[T IBaseEntity] interface {
		IBaseEntity
		PaginationQuery(ctx context.Context, option *FindOption) ([]T, int, *response.AppError)
		FindOne(ctx context.Context, option *FindOption) (*T, *response.AppError)
		FindByID(ctx context.Context, id any, option *FindOption) (*T, *response.AppError)
		FindMany(ctx context.Context, option *FindOption) ([]T, *response.AppError)
		CountBy(ctx context.Context, cond *FindOption) int
	}

	RepositoryCommandIf[T IBaseEntity] interface {
		IBaseEntity
		CreateOne(ctx context.Context, entity *T) (*T, *response.AppError)
		CreateMany(ctx context.Context, entities ...*T) []*T
		CreateWithOnConflicting(ctx context.Context, conflictString EUpdateConflictString, entities ...*T) ([]T, *response.AppError)
		UpdateMany(ctx context.Context, entity *map[string]any, option *UpdateOption) (int, *response.AppError)
		UpdateOne(ctx context.Context, entity *map[string]any, option *Where) *response.AppError
		UpdateById(ctx context.Context, id string, entity *map[string]any) *response.AppError
		UpdateReturning(ctx context.Context, entity *map[string]any, option *UpdateOption) ([]T, *response.AppError)
		UpdateOneReturning(ctx context.Context, entity *map[string]any, option *Where) (*T, *response.AppError)
		DeleteMany(ctx context.Context, option *UpdateOption) *response.AppError
		DeleteByID(ctx context.Context, id string) *response.AppError
	}

	Repository[T IBaseEntity] struct {
		IBaseEntity
		fields           map[string]bool
		fieldInsertCount int
		db               *sql.DB
		createdAt        bool
		updatedAt        bool
	}
)

const (
	DoNothing EUpdateConflictString = "DO NOTHING"
	DoUpdate  EUpdateConflictString = "DO UPDATE"
)

func NewRepository[T IBaseEntity](db *sql.DB, entity T) *Repository[T] {
	t := reflect.TypeOf(entity)
	fields := make(map[string]bool, t.NumField())
	fieldInsertCount := 0
	for i := range t.NumField() {
		field := t.Field(i)

		fields[field.Name] = true
		if field.Tag.Get("db") == "-" {
			fields[field.Name] = false
		}
		fieldInsertCount++
	}

	return &Repository[T]{
		db:               db,
		fields:           fields,
		fieldInsertCount: fieldInsertCount,
		createdAt:        fields["created_at"],
		updatedAt:        fields["updated_at"],
	}
}

func (r *Repository[T]) DB() *sql.DB {
	return r.db
}

func ConvertError[T any](data T, err error) (T, *response.AppError) {
	if err != nil {
		return data, response.ConvertDatabaseError(err)
	}

	return data, nil
}

func CloseRows(rows *sql.Rows) {
	err := rows.Close()
	if err != nil {
		log.Printf("Failed to close rows: %v", err)
	}
}
