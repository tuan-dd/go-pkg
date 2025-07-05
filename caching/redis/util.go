package redisClient

import (
	"context"
	"errors"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
)

func GetPtr[T any](client *CacheClient, key string) (*T, bool, error) {
	byteValue, is, err := client.Get(key)

	if is && err == nil {
		value := new(T)
		if err := sonic.Unmarshal(byteValue, value); err != nil {
			return value, false, err
		}
		return value, true, nil
	}

	return nil, false, nil
}

func GetList[T any](client *CacheClient, key string) ([]*T, bool, error) {
	byteValue, is, err := client.Get(key)

	if is && err == nil {
		var values []*T
		if err := sonic.Unmarshal(byteValue, &values); err != nil {
			return nil, false, err
		}
		return values, true, nil
	}

	return nil, false, err
}

func Get[T any](client *CacheClient, key string) (T, bool, error) {
	byteValue, is, err := client.Get(key)

	if is && err == nil {
		var value T
		if err := sonic.Unmarshal(byteValue, &value); err != nil {
			return value, false, err
		}
		return value, true, nil
	}

	return *new(T), false, nil
}

// Helper functions for list operations with marshalling
func LPushMarshal[T any](client *CacheClient, key string, values ...T) (int64, error) {
	marshalledValues := make([]any, len(values))
	for i, value := range values {
		bytes, err := sonic.Marshal(value)
		if err != nil {
			return 0, err
		}
		marshalledValues[i] = bytes
	}
	return client.LPush(key, marshalledValues...)
}

func RPushMarshal[T any](client *CacheClient, key string, values ...T) (int64, error) {
	marshalledValues := make([]any, len(values))
	for i, value := range values {
		bytes, err := sonic.Marshal(value)
		if err != nil {
			return 0, err
		}
		marshalledValues[i] = bytes
	}
	return client.RPush(key, marshalledValues...)
}

func LPopUnmarshal[T any](client *CacheClient, key string) (T, bool, error) {
	value, err := client.LPop(key)
	if errors.Is(err, redis.Nil) {
		return *new(T), false, nil
	}
	if err != nil {
		return *new(T), false, err
	}

	var result T
	if err := sonic.Unmarshal([]byte(value), &result); err != nil {
		return *new(T), false, err
	}
	return result, true, nil
}

func RPopUnmarshal[T any](client *CacheClient, key string) (T, bool, error) {
	value, err := client.RPop(key)
	if errors.Is(err, redis.Nil) {
		return *new(T), false, nil
	}
	if err != nil {
		return *new(T), false, err
	}

	var result T
	if err := sonic.Unmarshal([]byte(value), &result); err != nil {
		return *new(T), false, err
	}
	return result, true, nil
}

func LRangeUnmarshal[T any](client *CacheClient, key string, start, stop int64) ([]T, error) {
	values, err := client.LRange(key, start, stop)
	if err != nil {
		return nil, err
	}

	results := make([]T, len(values))
	for i, value := range values {
		if err := sonic.Unmarshal([]byte(value), &results[i]); err != nil {
			return nil, err
		}
	}
	return results, nil
}

// Helper functions for set operations with marshalling
func SAddMarshal[T any](client *CacheClient, key string, members ...T) (int64, error) {
	marshalledMembers := make([]any, len(members))
	for i, member := range members {
		bytes, err := sonic.Marshal(member)
		if err != nil {
			return 0, err
		}
		marshalledMembers[i] = bytes
	}
	return client.SAdd(key, marshalledMembers...)
}

func SMembersUnmarshal[T any](client *CacheClient, key string) ([]T, error) {
	members, err := client.SMembers(key)
	if err != nil {
		return nil, err
	}

	results := make([]T, len(members))
	for i, member := range members {
		if err := sonic.Unmarshal([]byte(member), &results[i]); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func SIsMemberMarshal[T any](client *CacheClient, key string, member T) (bool, error) {
	bytes, err := sonic.Marshal(member)
	if err != nil {
		return false, err
	}
	return client.SIsMember(key, bytes)
}

func SRemMarshal[T any](client *CacheClient, key string, members ...T) (int64, error) {
	marshalledMembers := make([]any, len(members))
	for i, member := range members {
		bytes, err := sonic.Marshal(member)
		if err != nil {
			return 0, err
		}
		marshalledMembers[i] = bytes
	}
	return client.SRem(key, marshalledMembers...)
}

func SPopUnmarshal[T any](client *CacheClient, key string, count int64) ([]T, error) {
	members, err := client.SPop(key, count)
	if err != nil {
		return nil, err
	}

	results := make([]T, len(members))
	for i, member := range members {
		if err := sonic.Unmarshal([]byte(member), &results[i]); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func SRandMemberUnmarshal[T any](client *CacheClient, key string, count int64) ([]T, error) {
	members, err := client.SRandMember(key, count)
	if err != nil {
		return nil, err
	}

	results := make([]T, len(members))
	for i, member := range members {
		if err := sonic.Unmarshal([]byte(member), &results[i]); err != nil {
			return nil, err
		}
	}
	return results, nil
}

// Additional utility functions for hash operations with marshalling
func HSetMarshal[T any](client *CacheClient, key, field string, value T) error {
	bytes, err := sonic.Marshal(value)
	if err != nil {
		return err
	}
	return client.client.HSet(context.Background(), key, field, bytes).Err()
}

func HGetUnmarshal[T any](client *CacheClient, key, field string) (T, bool, error) {
	value, err := client.client.HGet(context.Background(), key, field).Result()
	if errors.Is(err, redis.Nil) {
		return *new(T), false, nil
	}
	if err != nil {
		return *new(T), false, err
	}

	var result T
	if err := sonic.Unmarshal([]byte(value), &result); err != nil {
		return *new(T), false, err
	}
	return result, true, nil
}

func HMSetMarshal[T any](client *CacheClient, key string, fields map[string]T) error {
	marshalledFields := make(map[string]any)
	for field, value := range fields {
		bytes, err := sonic.Marshal(value)
		if err != nil {
			return err
		}
		marshalledFields[field] = bytes
	}
	return client.client.HMSet(context.Background(), key, marshalledFields).Err()
}

func HGetAllUnmarshal[T any](client *CacheClient, key string) (map[string]T, error) {
	values, err := client.client.HGetAll(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}

	results := make(map[string]T)
	for field, value := range values {
		var result T
		if err := sonic.Unmarshal([]byte(value), &result); err != nil {
			return nil, err
		}
		results[field] = result
	}
	return results, nil
}

// Batch operations helper
func SetMultipleMarshal[T any](client *CacheClient, keyValues map[string]T, expire time.Duration) error {
	pipe := client.client.Pipeline()
	for key, value := range keyValues {
		bytes, err := sonic.Marshal(value)
		if err != nil {
			return err
		}
		pipe.Set(context.Background(), key, bytes, expire)
	}
	_, err := pipe.Exec(context.Background())
	return err
}

func GetMultipleUnmarshal[T any](client *CacheClient, keys []string) (map[string]T, error) {
	pipe := client.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))

	for i, key := range keys {
		cmds[i] = pipe.Get(context.Background(), key)
	}

	_, err := pipe.Exec(context.Background())
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	results := make(map[string]T)
	for i, cmd := range cmds {
		value, err := cmd.Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return nil, err
		}

		var result T
		if err := sonic.Unmarshal([]byte(value), &result); err != nil {
			return nil, err
		}
		results[keys[i]] = result
	}

	return results, nil
}
