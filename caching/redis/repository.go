package redisClient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
	"github.com/tuan-dd/go-pkg/common/response"
)

type EngineCaching interface {
	Get(key string) ([]byte, bool, error)
	Set(key string, value any, expire time.Duration) (bool, error)
	Del(key string) error

	// List operations
	LPush(key string, values ...any) (int64, error)
	RPush(key string, values ...any) (int64, error)
	LPop(key string) (string, error)
	RPop(key string) (string, error)
	LRange(key string, start, stop int64) ([]string, error)
	LLen(key string) (int64, error)
	LRem(key string, count int64, value any) (int64, error)

	// Batch list operations
	LPushBatch(key string, values []any) (int64, error)
	RPushBatch(key string, values []any) (int64, error)

	// Additional list manipulation methods
	LTrim(key string, start, stop int64) error
	LIndex(key string, index int64) (string, error)
	LSet(key string, index int64, value any) error

	// Set operations
	SAdd(key string, members ...any) (int64, error)
	SRem(key string, members ...any) (int64, error)
	SMembers(key string) ([]string, error)
	SCard(key string) (int64, error)
	SIsMember(key string, member any) (bool, error)
	SPop(key string, count int64) ([]string, error)
	SRandMember(key string, count int64) ([]string, error)
	SUnion(keys ...string) ([]string, error)
	SInter(keys ...string) ([]string, error)
	SDiff(keys ...string) ([]string, error)
	SUnionStore(destination string, keys ...string) (int64, error)
	SInterStore(destination string, keys ...string) (int64, error)
	SDiffStore(destination string, keys ...string) (int64, error)

	// Additional set utility functions
	SMove(source, destination string, member any) (bool, error)
	SScan(key string, cursor uint64, match string, count int64) ([]string, uint64, error)

	// Bulk set operations
	SAddBatch(key string, members []any) (int64, error)
	SRemBatch(key string, members []any) (int64, error)

	// Set with expiration
	SAddWithExpire(key string, expire time.Duration, members ...any) (int64, error)

	// Lock operations
	Lock(key string, expire time.Duration) (bool, error)
	Unlock(key string, lockValue string) (bool, error)
	TryLock(key string, expire time.Duration, retryCount int, retryDelay time.Duration) (string, bool, error)

	Shutdown() *response.AppError
}

func (r *CacheClient) Get(key string) ([]byte, bool, error) {
	byteValue, err := r.client.Get(context.Background(), key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return byteValue, true, nil
}

func (r *CacheClient) Set(key string, value any, expireTime time.Duration) (bool, error) {
	valueBytes, err := sonic.Marshal(value)
	if err != nil {
		return false, err
	}

	err = r.client.Set(context.Background(), key, valueBytes, expireTime).Err()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *CacheClient) Del(key string) error {
	return r.client.Del(context.Background(), key).Err()
}

// List operations
func (r *CacheClient) LPush(key string, values ...any) (int64, error) {
	result, err := r.client.LPush(context.Background(), key, values).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (r *CacheClient) RPush(key string, values ...any) (int64, error) {
	result, err := r.client.RPush(context.Background(), key, values).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (r *CacheClient) LPop(key string) (string, error) {
	result, err := r.client.LPop(context.Background(), key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return result, nil
}

func (r *CacheClient) RPop(key string) (string, error) {
	result, err := r.client.RPop(context.Background(), key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return result, nil
}

func (r *CacheClient) LRange(key string, start, stop int64) ([]string, error) {
	result, err := r.client.LRange(context.Background(), key, start, stop).Result()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *CacheClient) LLen(key string) (int64, error) {
	result, err := r.client.LLen(context.Background(), key).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (r *CacheClient) LRem(key string, count int64, value any) (int64, error) {
	result, err := r.client.LRem(context.Background(), key, count, value).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// Batch list operations
func (r *CacheClient) LPushBatch(key string, values []any) (int64, error) {
	if len(values) == 0 {
		return 0, nil
	}
	result, err := r.client.LPush(context.Background(), key, values...).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (r *CacheClient) RPushBatch(key string, values []any) (int64, error) {
	if len(values) == 0 {
		return 0, nil
	}
	result, err := r.client.RPush(context.Background(), key, values...).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// Additional list manipulation methods
func (r *CacheClient) LTrim(key string, start, stop int64) error {
	err := r.client.LTrim(context.Background(), key, start, stop).Err()
	if err != nil {
		return err
	}
	return nil
}

func (r *CacheClient) LIndex(key string, index int64) (string, error) {
	result, err := r.client.LIndex(context.Background(), key, index).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return result, nil
}

func (r *CacheClient) LSet(key string, index int64, value any) error {
	err := r.client.LSet(context.Background(), key, index, value).Err()
	if err != nil {
		return err
	}
	return nil
}

// Set operations
func (r *CacheClient) SAdd(key string, members ...any) (int64, error) {
	result, err := r.client.SAdd(context.Background(), key, members...).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (r *CacheClient) SRem(key string, members ...any) (int64, error) {
	result, err := r.client.SRem(context.Background(), key, members...).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (r *CacheClient) SMembers(key string) ([]string, error) {
	result, err := r.client.SMembers(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *CacheClient) SCard(key string) (int64, error) {
	result, err := r.client.SCard(context.Background(), key).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (r *CacheClient) SIsMember(key string, member any) (bool, error) {
	result, err := r.client.SIsMember(context.Background(), key, member).Result()
	if err != nil {
		return false, err
	}
	return result, nil
}

func (r *CacheClient) SPop(key string, count int64) ([]string, error) {
	result, err := r.client.SPopN(context.Background(), key, count).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *CacheClient) SRandMember(key string, count int64) ([]string, error) {
	result, err := r.client.SRandMemberN(context.Background(), key, count).Result()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *CacheClient) SUnion(keys ...string) ([]string, error) {
	result, err := r.client.SUnion(context.Background(), keys...).Result()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *CacheClient) SInter(keys ...string) ([]string, error) {
	result, err := r.client.SInter(context.Background(), keys...).Result()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *CacheClient) SDiff(keys ...string) ([]string, error) {
	result, err := r.client.SDiff(context.Background(), keys...).Result()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *CacheClient) SUnionStore(destination string, keys ...string) (int64, error) {
	result, err := r.client.SUnionStore(context.Background(), destination, keys...).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (r *CacheClient) SInterStore(destination string, keys ...string) (int64, error) {
	result, err := r.client.SInterStore(context.Background(), destination, keys...).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (r *CacheClient) SDiffStore(destination string, keys ...string) (int64, error) {
	result, err := r.client.SDiffStore(context.Background(), destination, keys...).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// Additional set utility functions
func (r *CacheClient) SMove(source, destination string, member any) (bool, error) {
	result, err := r.client.SMove(context.Background(), source, destination, member).Result()
	if err != nil {
		return false, err
	}
	return result, nil
}

func (r *CacheClient) SScan(key string, cursor uint64, match string, count int64) ([]string, uint64, error) {
	result, nextCursor, err := r.client.SScan(context.Background(), key, cursor, match, count).Result()
	if err != nil {
		return nil, 0, err
	}
	return result, nextCursor, nil
}

// Bulk set operations
func (r *CacheClient) SAddBatch(key string, members []any) (int64, error) {
	if len(members) == 0 {
		return 0, nil
	}
	result, err := r.client.SAdd(context.Background(), key, members...).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (r *CacheClient) SRemBatch(key string, members []any) (int64, error) {
	if len(members) == 0 {
		return 0, nil
	}
	result, err := r.client.SRem(context.Background(), key, members...).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// Set with expiration
func (r *CacheClient) SAddWithExpire(key string, expire time.Duration, members ...any) (int64, error) {
	pipe := r.client.Pipeline()
	saddCmd := pipe.SAdd(context.Background(), key, members...)
	pipe.Expire(context.Background(), key, expire)

	_, err := pipe.Exec(context.Background())
	if err != nil {
		return 0, err
	}

	result, err := saddCmd.Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// Lock operations
func (r *CacheClient) Lock(key string, expire time.Duration) (bool, error) {
	lockValue := generateLockValue()
	result, err := r.client.SetNX(context.Background(), key, lockValue, expire).Result()
	if err != nil {
		return false, err
	}
	return result, nil
}

func (r *CacheClient) Unlock(key string, lockValue string) (bool, error) {
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	result, err := r.client.Eval(context.Background(), script, []string{key}, lockValue).Result()
	if err != nil {
		return false, err
	}
	return result.(int64) == 1, nil
}

func (r *CacheClient) TryLock(key string, expire time.Duration, retryCount int, retryDelay time.Duration) (string, bool, error) {
	for i := 0; i < retryCount; i++ {
		lockValue := generateLockValue()
		success, err := r.client.SetNX(context.Background(), key, lockValue, expire).Result()
		if err != nil {
			return "", false, err
		}
		if success {
			return lockValue, true, nil
		}
		if i < retryCount-1 {
			time.Sleep(retryDelay)
		}
	}
	return "", false, nil
}

// Helper function to generate unique lock values
func generateLockValue() string {
	bytes := make([]byte, 16)
	n, err := rand.Read(bytes)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(bytes[:n])
}

func (r *CacheClient) Shutdown() *response.AppError {
	if r.client == nil {
		return nil
	}
	err := r.client.Close()
	if err != nil {
		return response.ServerError("failed to close redis client: " + err.Error())
	}
	r.client = nil
	return nil
}
