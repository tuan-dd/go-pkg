package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/tuan-dd/go-common/pagination"
	"github.com/tuan-dd/go-common/response"
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
		Page           int32
		Limit          int32
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

func (r *Repository[T]) CreateOne(ctx context.Context, entity *T) (*T, *response.AppError) {
	tableName := r.TableName()
	t := reflect.TypeOf(entity)
	v := reflect.ValueOf(entity)

	columns := make([]string, 0, r.fieldInsertCount)
	values := make([]any, 0, r.fieldInsertCount)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Tag.Get("db") == "-" {
			continue
		}

		columns = append(columns, field.Tag.Get("db"))
		values = append(values, v.Field(i).Interface())
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *",
		tableName, strings.Join(columns, ","), strings.Repeat("?, ", r.fieldInsertCount)+"?")

	row := r.db.QueryRowContext(ctx, query, values...)

	var res T
	err := row.Scan(&res)
	if err != nil {
		return nil, response.ConvertDatabaseError(err)
	}

	return &res, nil
}

func (r *Repository[T]) CreateMany(ctx context.Context, entities ...*T) ([]T, *response.AppError) {
	if len(entities) == 0 {
		return nil, response.ConvertDatabaseError(fmt.Errorf("entities is empty"))
	}
	tableName := r.TableName()
	t := reflect.TypeOf(entities[0])
	numberRecords := len(entities) * r.fieldInsertCount

	columns := make([]string, 0, r.fieldInsertCount)
	values := make([]any, 0, numberRecords)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Tag.Get("db") == "-" {
			continue
		}
		columns = append(columns, field.Tag.Get("db"))
	}

	var valueStrings bytes.Buffer

	for _, entity := range entities {
		v := reflect.ValueOf(entity)
		value := make([]any, 0, r.fieldInsertCount)
		valueStrings.WriteString("(")
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.Tag.Get("db") == "-" {
				continue
			}
			value = append(value, v.Field(i).Interface())
		}
		valueStrings.WriteString("),")
		values = append(values, value)
	}

	valueStrings.Truncate(valueStrings.Len() - 1)

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s RETURNING *",
		tableName, strings.Join(columns, ","), valueStrings.String())

	rows, err := r.db.QueryContext(ctx, query, values...)
	if err != nil {
		return nil, response.ConvertDatabaseError(err)
	}

	res := make([]T, 0, numberRecords)

	for {
		var entity T
		err := rows.Scan(&entity)
		if err == sql.ErrNoRows {
			break
		} else if err != nil {
			return nil, response.ConvertDatabaseError(err)
		}

		res = append(res, entity)
	}

	return res, nil
}

func (r *Repository[T]) CreateWithOnConflicting(ctx context.Context, conflictString EUpdateConflictString, entities ...*T) ([]T, *response.AppError) {
	if len(entities) == 0 {
		return nil, response.ConvertDatabaseError(fmt.Errorf("entities is empty"))
	}
	tableName := r.TableName()
	t := reflect.TypeOf(entities[0])
	numberRecords := len(entities) * r.fieldInsertCount

	columns := make([]string, 0, r.fieldInsertCount)
	values := make([]any, 0, numberRecords)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Tag.Get("db") == "-" {
			continue
		}
		columns = append(columns, field.Tag.Get("db"))
	}

	var valueStrings bytes.Buffer

	for _, entity := range entities {
		v := reflect.ValueOf(entity)
		value := make([]any, 0, r.fieldInsertCount)
		valueStrings.WriteString("(")
		for i := 0; i < t.NumField(); i++ {
			value = append(value, v.Field(i).Interface())
		}
		valueStrings.WriteString("),")
		values = append(values, value)
	}

	valueStrings.Truncate(valueStrings.Len() - 1)

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s RETURNING * ON CONFLICT %s", conflictString,
		tableName, strings.Join(columns, ","), valueStrings.String())

	rows, err := r.db.QueryContext(ctx, query, values...)
	if err != nil {
		return nil, response.ConvertDatabaseError(err)
	}

	res := make([]T, 0, numberRecords)

	for {
		var entity T
		err := rows.Scan(&entity)
		if err == sql.ErrNoRows {
			break
		} else if err != nil {
			return nil, response.ConvertDatabaseError(err)
		}

		res = append(res, entity)
	}

	return res, nil
}

func (r *Repository[T]) UpdateMany(ctx context.Context, entity *map[string]any, option *UpdateOption) (int64, *response.AppError) {
	tableName := r.TableName()
	t := reflect.TypeOf(entity)
	v := reflect.ValueOf(entity)

	var updateString bytes.Buffer
	params := make([]any, 0, r.fieldInsertCount)
	for i := range t.NumField() {
		field := t.Field(i)
		if !r.fields[field.Name] {
			continue
		}

		updateString.WriteString(fmt.Sprintf("%s = ?,", field.Name))
		params = append(params, v.Field(i).Interface())
	}

	// REMOVE LAST COMMA
	updateString.Truncate(updateString.Len() - 1)

	// ADD WHERE CLAUSE
	params = append(params, option.Params...)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		tableName, updateString.String(), option.Condition)

	result, err := r.db.ExecContext(ctx, query, params...)
	if err != nil {
		return 0, response.ConvertDatabaseError(err)
	}

	return convertError(result.RowsAffected())
}

func (r *Repository[T]) UpdateOne(ctx context.Context, entity *map[string]any, option *Where) *response.AppError {
	tableName := r.TableName()
	t := reflect.TypeOf(entity)
	v := reflect.ValueOf(entity)

	var updateString bytes.Buffer
	params := make([]any, 0, r.fieldInsertCount)
	for i := range t.NumField() {
		field := t.Field(i)
		if !r.fields[field.Name] {
			continue
		}

		updateString.WriteString(fmt.Sprintf("%s = ?,", field.Tag.Get("db")))
		params = append(params, v.Field(i).Interface())
	}

	// REMOVE LAST COMMA
	updateString.Truncate(updateString.Len() - 1)

	// ADD WHERE CLAUSE
	params = append(params, option.Params...)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s LIMIT 1",
		tableName, updateString.String(), option.Condition)

	_, err := r.db.ExecContext(ctx, query, params...)
	if err != nil {
		return response.ConvertDatabaseError(err)
	}

	return nil
}

func (r *Repository[T]) UpdateById(ctx context.Context, id string, entity *map[string]any) *response.AppError {
	tableName := r.TableName()
	t := reflect.TypeOf(entity)
	v := reflect.ValueOf(entity)

	var updateString bytes.Buffer
	params := make([]any, 0, r.fieldInsertCount)
	for i := range t.NumField() {
		field := t.Field(i)
		if !r.fields[field.Name] {
			continue
		}

		updateString.WriteString(fmt.Sprintf("%s = ?,", field.Tag.Get("db")))
		params = append(params, v.Field(i).Interface())
	}

	// REMOVE LAST COMMA
	updateString.Truncate(updateString.Len() - 1)

	// ADD WHERE CLAUSE
	params = append(params, id)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ? LIMIT 1",
		tableName, updateString.String())

	_, err := r.db.ExecContext(ctx, query, params...)
	if err != nil {
		return response.ConvertDatabaseError(err)
	}

	return nil
}

func (r *Repository[T]) UpdateReturning(ctx context.Context, entity *map[string]any, option *UpdateOption) ([]T, *response.AppError) {
	tableName := r.TableName()
	t := reflect.TypeOf(entity)
	v := reflect.ValueOf(entity)

	var updateString bytes.Buffer
	params := make([]any, 0, r.fieldInsertCount)
	for i := range t.NumField() {
		field := t.Field(i)
		if !r.fields[field.Name] {
			continue
		}

		updateString.WriteString(fmt.Sprintf("%s = ?,", field.Name))
		params = append(params, v.Field(i).Interface())
	}

	// REMOVE LAST COMMA
	updateString.Truncate(updateString.Len() - 1)

	// ADD WHERE CLAUSE
	params = append(params, option.Params...)

	returningString := "*"

	if len(option.Select) > 0 {
		returningString = strings.Join(option.Select, ",")
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s RETURNING %s",
		tableName, updateString.String(), option.Condition, returningString)

	rows, err := r.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, response.ConvertDatabaseError(err)
	}

	defer func() {
		CloseRows(rows)
	}()

	var res []T

	for rows.Next() {
		var entity T
		err := rows.Scan(&entity)
		if err != nil {
			return nil, response.ConvertDatabaseError(err)
		}
		res = append(res, entity)
	}

	return res, nil
}

func (r *Repository[T]) UpdateOneReturning(ctx context.Context, entity *map[string]any, option *UpdateOption) (*T, *response.AppError) {
	tableName := r.TableName()
	t := reflect.TypeOf(entity)
	v := reflect.ValueOf(entity)

	var updateString bytes.Buffer
	params := make([]any, 0, r.fieldInsertCount)
	for i := range t.NumField() {
		field := t.Field(i)
		if !r.fields[field.Name] {
			continue
		}

		updateString.WriteString(fmt.Sprintf("%s = ?,", field.Name))
		params = append(params, v.Field(i).Interface())
	}

	// REMOVE LAST COMMA
	updateString.Truncate(updateString.Len() - 1)

	// ADD WHERE CLAUSE
	params = append(params, option.Params...)

	returningString := "*"

	if len(option.Select) > 0 {
		returningString = strings.Join(option.Select, ",")
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s LIMIT 1 RETURNING %s",
		tableName, updateString.String(), option.Condition, returningString)

	row, err := r.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, response.ConvertDatabaseError(err)
	}

	var res T
	if !row.Next() {
		return nil, response.QueryNotFoundErr(string(response.DatabaseNotFound))
	}
	err = row.Scan(&res)
	if err != nil {
		return nil, response.ConvertDatabaseError(err)
	}

	return &res, nil
}

func (r *Repository[T]) DeleteMany(ctx context.Context, option *UpdateOption) (int64, *response.AppError) {
	tableName := r.TableName()
	query := fmt.Sprintf("DELETE FROM %s WHERE %s", tableName, option.Condition)

	result, err := r.db.ExecContext(ctx, query, option.Params...)
	if err != nil {
		return 0, response.ConvertDatabaseError(err)
	}

	return convertError(result.RowsAffected())
}

func (r *Repository[T]) DeleteById(ctx context.Context, id any) *response.AppError {
	tableName := r.TableName()
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName)

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return response.ConvertDatabaseError(err)
	}

	return nil
}

// Query

func (r *Repository[T]) PaginationQuery(ctx context.Context, option *FindOption) ([]T, uint64, *response.AppError) {
	tableName := r.TableName()
	selects := "*"
	if len(option.Select) > 0 {
		selects = strings.Join(option.Select, ",")
	}

	take, skip := pagination.PaginationOpts(option.Page, option.Limit)

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tableName, option.Condition)

	count := r.db.QueryRowContext(ctx, countQuery, option.Params...)

	var total uint64
	err := count.Scan(&total)
	if err != nil {
		return nil, 0, response.ConvertDatabaseError(err)
	}

	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s LIMIT ? OFFSET ?", selects, tableName, option.Condition)

	if len(option.Order) > 0 {
		query = fmt.Sprintf("%s ORDER BY %s", query, strings.Join(option.Order, ", "))
	}

	option.Params = append(option.Params, take, skip)
	rows, err := r.db.QueryContext(ctx, query, option.Params...)
	if err != nil {
		return nil, 0, response.ConvertDatabaseError(err)
	}

	defer CloseRows(rows)

	res := make([]T, 0, take)
	for rows.Next() {
		var entity T
		err := rows.Scan(&entity)
		if err != nil {
			return nil, 0, response.ConvertDatabaseError(err)
		}
		res = append(res, entity)
	}

	return res, total, nil
}

func (r *Repository[T]) FindMany(ctx context.Context, option *FindOption) ([]T, *response.AppError) {
	tableName := r.TableName()
	selects := "*"
	if len(option.Select) > 0 {
		selects = strings.Join(option.Select, ",")
	}

	query := ""

	if option.IncludeDeleted {
		query = fmt.Sprintf("SELECT %s FROM %s WHERE deleted_at IS NOT NULL", selects, tableName)
	} else {
		query = fmt.Sprintf("SELECT %s FROM %s WHERE deleted_at IS NULL", selects, tableName)
	}

	if len(option.Condition) > 0 {
		query = fmt.Sprintf("%s WHERE %s", query, option.Condition)
	}

	if option.Page > 0 && option.Limit > 0 {
		take, skip := pagination.PaginationOpts(option.Page, option.Limit)

		query = fmt.Sprintf("%s LIMIT ? OFFSET ?", query)

		option.Params = append(option.Params, take, skip)
	}

	if len(option.Order) > 0 {
		query = fmt.Sprintf("%s ORDER BY %s", query, strings.Join(option.Order, ", "))
	}

	rows, err := r.db.QueryContext(ctx, query, option.Params...)
	if err != nil {
		return nil, response.ConvertDatabaseError(err)
	}

	defer func() {
		CloseRows(rows)
	}()

	res := []T{}
	for rows.Next() {
		var entity T
		err := rows.Scan(&entity)
		if err != nil {
			return nil, response.ConvertDatabaseError(err)
		}
		res = append(res, entity)
	}

	return res, nil
}

func (r *Repository[T]) FindOne(ctx context.Context, option *FindOption) (*T, *response.AppError) {
	tableName := r.TableName()
	selects := "*"
	if len(option.Select) > 0 {
		selects = strings.Join(option.Select, ",")
	}

	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s LIMIT 1", selects, tableName, option.Condition)

	row, err := r.db.QueryContext(ctx, query, option.Params...)
	if err != nil {
		return nil, response.ConvertDatabaseError(err)
	}

	defer func() {
		CloseRows(row)
	}()

	var res T
	if !row.Next() {
		return nil, response.ConvertDatabaseError(errors.New(string(response.DatabaseNotFound)))
	}

	err = row.Scan(&res)
	if err != nil {
		return nil, response.ConvertDatabaseError(err)
	}

	return &res, nil
}

func (r *Repository[T]) FindByID(ctx context.Context, id any, option *FindOption) (*T, *response.AppError) {
	tableName := r.TableName()
	selects := "*"
	if len(option.Select) > 0 {
		selects = strings.Join(option.Select, ",")
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", selects, tableName)

	row, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, response.ConvertDatabaseError(err)
	}

	defer func() {
		CloseRows(row)
	}()

	var res T
	if !row.Next() {
		return nil, response.ConvertDatabaseError(errors.New(string(response.DatabaseNotFound)))
	}

	err = row.Scan(&res)
	if err != nil {
		return nil, response.ConvertDatabaseError(err)
	}

	return &res, nil
}

func convertError[T any](data T, err error) (T, *response.AppError) {
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
