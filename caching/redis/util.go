package redisClient

import (
	"context"
	"errors"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
	"github.com/tuan-dd/go-common/response"
	typeCustom "github.com/tuan-dd/go-common/type-custom"
)

func GetPtr[T any](ctx context.Context, client typeCustom.EngineCaching, key string) (*T, bool, *response.AppError) {
	byteValue, is, errApp := client.Get(ctx, key)

	if is && errApp == nil {
		value := new(T)
		if err := sonic.Unmarshal(byteValue, value); err != nil {
			return value, false, response.ServerError("failed to unmarshal value: " + err.Error())
		}
		return value, true, nil
	}

	return nil, false, nil
}

func GetList[T any](ctx context.Context, client typeCustom.EngineCaching, key string) ([]*T, bool, *response.AppError) {
	byteValue, is, errApp := client.Get(ctx, key)

	if is && errApp == nil {
		var values []*T
		if err := sonic.Unmarshal(byteValue, &values); err != nil {
			return nil, false, response.ServerError("failed to unmarshal value: " + err.Error())
		}
		return values, true, nil
	}

	return nil, false, errApp
}

func Get[T any](ctx context.Context, client typeCustom.EngineCaching, key string) (T, bool, *response.AppError) {
	byteValue, is, errApp := client.Get(ctx, key)

	if is && errApp == nil {
		var value T
		if err := sonic.Unmarshal(byteValue, &value); err != nil {
			return value, false, response.ServerError("failed to unmarshal value: " + err.Error())
		}
		return value, true, nil
	}

	return *new(T), false, nil
}

func SetMarshal(ctx context.Context, client typeCustom.EngineCaching, key string, value any, ttl time.Duration) (bool, *response.AppError) {
	bytes, err := sonic.Marshal(value)
	if err != nil {
		return false, response.ServerError("failed to marshal value: " + err.Error())
	}
	return client.Set(ctx, key, bytes, ttl)
}

// Helper functions for list operations with marshalling
func LPushMarshal(ctx context.Context, client typeCustom.EngineCaching, key string, values ...any) (int64, *response.AppError) {
	marshalledValues := make([]any, len(values))
	for i, value := range values {
		bytes, err := sonic.Marshal(value)
		if err != nil {
			return 0, response.ServerError("failed to marshal value: " + err.Error())
		}
		marshalledValues[i] = bytes
	}
	return client.LPush(ctx, key, marshalledValues...)
}

func RPushMarshal(ctx context.Context, client typeCustom.EngineCaching, key string, values ...any) (int64, *response.AppError) {
	marshalledValues := make([]any, len(values))
	for i, value := range values {
		bytes, err := sonic.Marshal(value)
		if err != nil {
			return 0, response.ServerError("failed to marshal value: " + err.Error())
		}
		marshalledValues[i] = bytes
	}
	return client.RPush(ctx, key, marshalledValues...)
}

func LPopUnmarshal[T any](ctx context.Context, client typeCustom.EngineCaching, key string) (T, bool, *response.AppError) {
	value, errApp := client.LPop(ctx, key)
	if errors.Is(errApp, redis.Nil) {
		return *new(T), false, nil
	}
	if errApp != nil {
		return *new(T), false, errApp
	}

	var result T
	if err := sonic.Unmarshal([]byte(value), &result); err != nil {
		return *new(T), false, response.ServerError("failed to unmarshal value: " + err.Error())
	}
	return result, true, nil
}

func RPopUnmarshal[T any](ctx context.Context, client typeCustom.EngineCaching, key string) (T, bool, *response.AppError) {
	value, errApp := client.RPop(ctx, key)
	if errors.Is(errApp, redis.Nil) {
		return *new(T), false, nil
	}
	if errApp != nil {
		return *new(T), false, errApp
	}

	var result T
	if err := sonic.Unmarshal([]byte(value), &result); err != nil {
		return *new(T), false, response.ServerError("failed to unmarshal value: " + err.Error())
	}
	return result, true, nil
}

func LRangeUnmarshal[T any](ctx context.Context, client typeCustom.EngineCaching, key string, start, stop int64) ([]T, *response.AppError) {
	values, errApp := client.LRange(ctx, key, start, stop)
	if errApp != nil {
		return nil, errApp
	}

	results := make([]T, len(values))
	for i, value := range values {
		if err := sonic.Unmarshal([]byte(value), &results[i]); err != nil {
			return nil, response.ServerError("failed to unmarshal value: " + err.Error())
		}
	}
	return results, nil
}

// Helper functions for set operations with marshalling
func SAddMarshal(ctx context.Context, client typeCustom.EngineCaching, key string, members ...any) (int64, *response.AppError) {
	marshalledMembers := make([]any, len(members))
	for i, member := range members {
		bytes, err := sonic.Marshal(member)
		if err != nil {
			return 0, response.ServerError("failed to unmarshal value: " + err.Error())
		}
		marshalledMembers[i] = bytes
	}
	return client.SAdd(ctx, key, marshalledMembers...)
}

func SMembersUnmarshal[T any](ctx context.Context, client typeCustom.EngineCaching, key string) ([]T, *response.AppError) {
	members, errApp := client.SMembers(ctx, key)
	if errApp != nil {
		return nil, errApp
	}

	results := make([]T, len(members))
	for i, member := range members {
		if err := sonic.Unmarshal([]byte(member), &results[i]); err != nil {
			return nil, response.ServerError("failed to unmarshal value: " + err.Error())
		}
	}
	return results, nil
}

func SIsMemberMarshal(ctx context.Context, client typeCustom.EngineCaching, key string, member any) (bool, *response.AppError) {
	bytes, err := sonic.Marshal(member)
	if err != nil {
		return false, response.ServerError("failed to unmarshal value: " + err.Error())
	}
	return client.SIsMember(ctx, key, bytes)
}

func SRemMarshal(ctx context.Context, client typeCustom.EngineCaching, key string, members ...any) (int64, *response.AppError) {
	marshalledMembers := make([]any, len(members))
	for i, member := range members {
		bytes, err := sonic.Marshal(member)
		if err != nil {
			return 0, response.ServerError("failed to unmarshal value: " + err.Error())
		}
		marshalledMembers[i] = bytes
	}
	return client.SRem(ctx, key, marshalledMembers...)
}

func SPopUnmarshal[T any](ctx context.Context, client typeCustom.EngineCaching, key string, count int64) ([]T, *response.AppError) {
	members, errApp := client.SPop(ctx, key, count)
	if errApp != nil {
		return nil, errApp
	}

	results := make([]T, len(members))
	for i, member := range members {
		if err := sonic.Unmarshal([]byte(member), &results[i]); err != nil {
			return nil, response.ServerError("failed to unmarshal value: " + err.Error())
		}
	}
	return results, nil
}

func SRandMemberUnmarshal[T any](ctx context.Context, client typeCustom.EngineCaching, key string, count int64) ([]T, *response.AppError) {
	members, errApp := client.SRandMember(ctx, key, count)
	if errApp != nil {
		return nil, errApp
	}

	results := make([]T, len(members))
	for i, member := range members {
		if err := sonic.Unmarshal([]byte(member), &results[i]); err != nil {
			return nil, response.ServerError("failed to unmarshal value: " + err.Error())
		}
	}
	return results, nil
}

// Additional utility functions for hash operations with marshalling
func HSetMarshal(ctx context.Context, client typeCustom.EngineCaching, key, field string, value any) *response.AppError {
	bytes, err := sonic.Marshal(value)
	if err != nil {
		return response.ServerError("failed to marshal value: " + err.Error())
	}
	return client.HSet(ctx, key, field, bytes)
}

func HGetUnmarshal[T any](ctx context.Context, client typeCustom.EngineCaching, key, field string) (T, bool, *response.AppError) {
	value, isFound, errApp := client.HGet(ctx, key, field)
	if !isFound {
		return *new(T), false, nil
	}

	if errApp != nil {
		return *new(T), false, errApp
	}

	var result T
	if err := sonic.Unmarshal([]byte(value), &result); err != nil {
		return *new(T), false, response.ServerError("failed to unmarshal value: " + err.Error())
	}
	return result, true, nil
}

func HMSetMarshal(ctx context.Context, client typeCustom.EngineCaching, key string, fields map[string]any) *response.AppError {
	marshalledFields := make(map[string]any)
	for field, value := range fields {
		bytes, err := sonic.Marshal(value)
		if err != nil {
			return response.ServerError("failed to marshal value: " + err.Error())
		}
		marshalledFields[field] = bytes
	}
	return client.HMSet(ctx, key, marshalledFields)
}

func HMGetMarshal[T any](ctx context.Context, client typeCustom.EngineCaching, key string, fields ...string) (map[string]T, *response.AppError) {
	values, errApp := client.HMGet(ctx, key, fields...)
	if errApp != nil {
		return nil, errApp
	}

	results := make(map[string]T)
	for i, value := range values {
		if value == nil {
			continue
		}
		var result T
		if err := sonic.Unmarshal([]byte(value.(string)), &result); err != nil {
			return nil, response.ServerError("failed to unmarshal value: " + err.Error())
		}
		results[fields[i]] = result
	}
	return results, nil
}

func HGetAllUnmarshal[T any](ctx context.Context, client typeCustom.EngineCaching, key string) (map[string]T, *response.AppError) {
	values, errApp := client.HGetAll(ctx, key)
	if errApp != nil {
		return nil, errApp
	}

	results := make(map[string]T)
	for field, value := range values {
		var result T
		if err := sonic.Unmarshal([]byte(value), &result); err != nil {
			return nil, response.ServerError("failed to unmarshal value: " + err.Error())
		}
		results[field] = result
	}
	return results, nil
}
