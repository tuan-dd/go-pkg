package redisClient

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type EngineCaching interface {
	Get(key string) ([]byte, bool, error)
	Set(key string, value any, expire time.Duration) (bool, error)
	Del(key string) error
}

func (r *CacheClient) Get(key string) ([]byte, bool, error) {
	byteValue, err := r.client.Get(context.Background(), key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, err
	}
	if err != nil {
		return nil, false, err
	}
	return byteValue, true, nil
}

func Get[T any](key string, cl EngineCaching) (*T, error) {
	byteValue, _, err := cl.Get(key)
	if errors.Is(err, redis.Nil) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	result := new(T)

	return result, json.Unmarshal(byteValue, result)
}

func (r *CacheClient) Set(key string, value any, expireTime time.Duration) (bool, error) {
	err := r.client.Set(context.Background(), key, value, expireTime).Err()
	if errors.Is(err, redis.Nil) {
		return false, err
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *CacheClient) Del(key string) error {
	return r.client.Del(context.Background(), key).Err()
}
