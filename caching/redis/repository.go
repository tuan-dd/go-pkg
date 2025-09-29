package redisClient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"github.com/tuan-dd/go-common/response"
	typeCustom "github.com/tuan-dd/go-common/type-custom"
)

func (r *CacheClient) buildCacheKey(key string) string {
	return fmt.Sprintf("%s:%s", r.serviceName, key)
}

func (r *CacheClient) buildCacheListKey(key []string) []string {
	return lo.Map(key, func(k string, _ int) string {
		return fmt.Sprintf("%s:%s", r.serviceName, k)
	})
}

func (r *CacheClient) Get(ctx context.Context, key string) ([]byte, bool, *response.AppError) {
	byteValue, err := r.client.Get(ctx, r.buildCacheKey(key)).Bytes()

	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, response.ServerError("failed to get value: " + err.Error())
	}
	return byteValue, true, nil
}

func (r *CacheClient) Set(ctx context.Context, key string, value any, expireTime time.Duration) (bool, *response.AppError) {
	err := r.client.Set(ctx, r.buildCacheKey(key), value, expireTime).Err()

	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, response.ServerError("failed to set value: " + err.Error())
	}
	return true, nil
}

func (r *CacheClient) Del(ctx context.Context, key string) *response.AppError {
	return response.ConvertError(r.client.Del(ctx, r.buildCacheKey(key)).Err())
}

// List operations
func (r *CacheClient) LPush(ctx context.Context, key string, values ...any) (int64, *response.AppError) {
	result, err := r.client.LPush(ctx, r.buildCacheKey(key), values).Result()
	if err != nil {
		return 0, response.ServerError("failed to push values: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) RPush(ctx context.Context, key string, values ...any) (int64, *response.AppError) {
	result, err := r.client.RPush(ctx, r.buildCacheKey(key), values).Result()
	if err != nil {
		return 0, response.ServerError("failed to push values: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) LPop(ctx context.Context, key string) (string, *response.AppError) {
	result, err := r.client.LPop(ctx, r.buildCacheKey(key)).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", response.ServerError("failed to pop value: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) RPop(ctx context.Context, key string) (string, *response.AppError) {
	result, err := r.client.RPop(ctx, r.buildCacheKey(key)).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", response.ServerError("failed to pop value: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) LRange(ctx context.Context, key string, start, stop int64) ([]string, *response.AppError) {
	result, err := r.client.LRange(ctx, r.buildCacheKey(key), start, stop).Result()
	if err != nil {
		return nil, response.ServerError("failed to get list range: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) LLen(ctx context.Context, key string) (int64, *response.AppError) {
	result, err := r.client.LLen(ctx, r.buildCacheKey(key)).Result()
	if err != nil {
		return 0, response.ServerError("failed to get list length: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) LRem(ctx context.Context, key string, count int64, value any) (int64, *response.AppError) {
	result, err := r.client.LRem(ctx, r.buildCacheKey(key), count, value).Result()
	if err != nil {
		return 0, response.ServerError("failed to remove list item: " + err.Error())
	}
	return result, nil
}

// Batch list operations
func (r *CacheClient) LPushBatch(ctx context.Context, key string, values []any) (int64, *response.AppError) {
	if len(values) == 0 {
		return 0, nil
	}
	result, err := r.client.LPush(ctx, r.buildCacheKey(key), values...).Result()
	if err != nil {
		return 0, response.ServerError("failed to push values: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) RPushBatch(ctx context.Context, key string, values []any) (int64, *response.AppError) {
	if len(values) == 0 {
		return 0, nil
	}
	result, err := r.client.RPush(ctx, r.buildCacheKey(key), values...).Result()
	if err != nil {
		return 0, response.ServerError("failed to push values: " + err.Error())
	}
	return result, nil
}

// Additional list manipulation methods
func (r *CacheClient) LTrim(ctx context.Context, key string, start, stop int64) *response.AppError {
	err := r.client.LTrim(ctx, r.buildCacheKey(key), start, stop).Err()
	if err != nil {
		return response.ServerError("failed to trim list: " + err.Error())
	}
	return nil
}

func (r *CacheClient) LIndex(ctx context.Context, key string, index int64) (string, *response.AppError) {
	result, err := r.client.LIndex(ctx, r.buildCacheKey(key), index).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", response.ServerError("failed to get list index: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) LSet(ctx context.Context, key string, index int64, value any) *response.AppError {
	err := r.client.LSet(ctx, r.buildCacheKey(key), index, value).Err()
	if err != nil {
		return response.ServerError("failed to set list value: " + err.Error())
	}
	return nil
}

// Set operations
func (r *CacheClient) SAdd(ctx context.Context, key string, members ...any) (int64, *response.AppError) {
	result, err := r.client.SAdd(ctx, r.buildCacheKey(key), members...).Result()
	if err != nil {
		return 0, response.ServerError("failed to add members: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SRem(ctx context.Context, key string, members ...any) (int64, *response.AppError) {
	result, err := r.client.SRem(ctx, r.buildCacheKey(key), members...).Result()
	if err != nil {
		return 0, response.ServerError("failed to remove members: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SMembers(ctx context.Context, key string) ([]string, *response.AppError) {
	result, err := r.client.SMembers(ctx, r.buildCacheKey(key)).Result()
	if err != nil {
		return nil, response.ServerError("failed to get set members: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SCard(ctx context.Context, key string) (int64, *response.AppError) {
	result, err := r.client.SCard(ctx, r.buildCacheKey(key)).Result()
	if err != nil {
		return 0, response.ServerError("failed to get set cardinality: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SIsMember(ctx context.Context, key string, member any) (bool, *response.AppError) {
	result, err := r.client.SIsMember(ctx, r.buildCacheKey(key), member).Result()
	if err != nil {
		return false, response.ServerError("failed to check membership: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SPop(ctx context.Context, key string, count int64) ([]string, *response.AppError) {
	result, err := r.client.SPopN(ctx, r.buildCacheKey(key), count).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, response.ServerError("failed to pop members: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SRandMember(ctx context.Context, key string, count int64) ([]string, *response.AppError) {
	result, err := r.client.SRandMemberN(ctx, r.buildCacheKey(key), count).Result()
	if err != nil {
		return nil, response.ServerError("failed to get random members: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SUnion(ctx context.Context, keys ...string) ([]string, *response.AppError) {
	result, err := r.client.SUnion(ctx, r.buildCacheListKey(keys)...).Result()
	if err != nil {
		return nil, response.ServerError("failed to get set union: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SInter(ctx context.Context, keys ...string) ([]string, *response.AppError) {
	result, err := r.client.SInter(ctx, r.buildCacheListKey(keys)...).Result()
	if err != nil {
		return nil, response.ServerError("failed to get set intersection: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SDiff(ctx context.Context, keys ...string) ([]string, *response.AppError) {
	result, err := r.client.SDiff(ctx, r.buildCacheListKey(keys)...).Result()
	if err != nil {
		return nil, response.ServerError("failed to get set difference: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SUnionStore(ctx context.Context, destination string, keys ...string) (int64, *response.AppError) {
	result, err := r.client.SUnionStore(ctx, destination, r.buildCacheListKey(keys)...).Result()
	if err != nil {
		return 0, response.ServerError("failed to store set union: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SInterStore(ctx context.Context, destination string, keys ...string) (int64, *response.AppError) {
	result, err := r.client.SInterStore(ctx, destination, r.buildCacheListKey(keys)...).Result()
	if err != nil {
		return 0, response.ServerError("failed to store set intersection: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SDiffStore(ctx context.Context, destination string, keys ...string) (int64, *response.AppError) {
	result, err := r.client.SDiffStore(ctx, destination, r.buildCacheListKey(keys)...).Result()
	if err != nil {
		return 0, response.ServerError("failed to store set difference: " + err.Error())
	}
	return result, nil
}

// Additional set utility functions
func (r *CacheClient) SMove(ctx context.Context, source, destination string, member any) (bool, *response.AppError) {
	result, err := r.client.SMove(ctx, r.buildCacheKey(source), r.buildCacheKey(destination), member).Result()
	if err != nil {
		return false, response.ServerError("failed to move member: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SScan(ctx context.Context, key string, cursor uint64, match string, count int64) ([]string, uint64, *response.AppError) {
	result, nextCursor, err := r.client.SScan(ctx, r.buildCacheKey(key), cursor, match, count).Result()
	if err != nil {
		return nil, 0, response.ServerError("failed to scan set: " + err.Error())
	}
	return result, nextCursor, nil
}

// Bulk set operations
func (r *CacheClient) SAddBatch(ctx context.Context, key string, members []any) (int64, *response.AppError) {
	if len(members) == 0 {
		return 0, nil
	}
	result, err := r.client.SAdd(ctx, r.buildCacheKey(key), members...).Result()
	if err != nil {
		return 0, response.ServerError("failed to add members: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SRemBatch(ctx context.Context, key string, members []any) (int64, *response.AppError) {
	if len(members) == 0 {
		return 0, nil
	}
	result, err := r.client.SRem(ctx, r.buildCacheKey(key), members...).Result()
	if err != nil {
		return 0, response.ServerError("failed to remove members: " + err.Error())
	}
	return result, nil
}

// Set with expiration
func (r *CacheClient) SAddWithExpire(ctx context.Context, key string, expire time.Duration, members ...any) (int64, *response.AppError) {
	pipe := r.client.Pipeline()
	saddCmd := pipe.SAdd(ctx, r.buildCacheKey(key), members...)
	pipe.Expire(ctx, r.buildCacheKey(key), expire)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, response.ServerError("failed to add members with expire: " + err.Error())
	}

	result, err := saddCmd.Result()
	if err != nil {
		return 0, response.ServerError("failed to add members with expire: " + err.Error())
	}
	return result, nil
}

// hash operations

func (r *CacheClient) HSet(ctx context.Context, key, field string, value any) *response.AppError {
	err := r.client.HSet(ctx, r.buildCacheKey(key), field, value).Err()
	if err != nil {
		return response.ServerError("failed to set hash field: " + err.Error())
	}
	return nil
}

func (r *CacheClient) HGet(ctx context.Context, key, field string) (string, bool, *response.AppError) {
	result, err := r.client.HGet(ctx, r.buildCacheKey(key), field).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, response.ServerError("failed to get hash field: " + err.Error())
	}
	return result, true, nil
}

func (r *CacheClient) HDel(ctx context.Context, key string, fields ...string) (int64, *response.AppError) {
	result, err := r.client.HDel(ctx, r.buildCacheKey(key), fields...).Result()
	if err != nil {
		return 0, response.ServerError("failed to delete hash fields: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) HExists(ctx context.Context, key, field string) (bool, *response.AppError) {
	result, err := r.client.HExists(ctx, r.buildCacheKey(key), field).Result()
	if err != nil {
		return false, response.ServerError("failed to check hash field existence: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) HGetAll(ctx context.Context, key string) (map[string]string, *response.AppError) {
	result, err := r.client.HGetAll(ctx, r.buildCacheKey(key)).Result()
	if err != nil {
		return nil, response.ServerError("failed to get all hash fields: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) HMSet(ctx context.Context, key string, fields map[string]any) *response.AppError {
	if len(fields) == 0 {
		return nil
	}
	err := r.client.HMSet(ctx, r.buildCacheKey(key), fields).Err()
	if err != nil {
		return response.ServerError("failed to set multiple hash fields: " + err.Error())
	}
	return nil
}

func (r *CacheClient) HMGet(ctx context.Context, key string, fields ...string) ([]any, *response.AppError) {
	if len(fields) == 0 {
		return []any{}, nil
	}

	result, err := r.client.HMGet(ctx, r.buildCacheKey(key), fields...).Result()
	if err != nil {
		return nil, response.ServerError("failed to get multiple hash fields: " + err.Error())
	}
	return result, nil
}

// Lock operations
func (r *CacheClient) Lock(ctx context.Context, key string, expire time.Duration) (string, bool, *response.AppError) {
	lockValue := generateLockValue()
	result, err := r.client.SetNX(ctx, r.buildCacheKey(key), lockValue, expire).Result()
	if err != nil {
		return "", false, response.ServerError("failed to acquire lock: " + err.Error())
	}
	if !result {
		return "", false, nil
	}
	return lockValue, true, nil
}

func (r *CacheClient) Unlock(ctx context.Context, key string, lockValue string) *response.AppError {
	script := `
        if redis.call("get", KEYS[1]) == ARGV[1] then
            return redis.call("del", KEYS[1])
        else
            return 0
        end
    `
	result, err := r.client.Eval(ctx, script, []string{r.buildCacheKey(key)}, lockValue).Result()
	if err != nil {
		return response.ServerError("failed to release lock: " + err.Error())
	}

	val, ok := result.(int64)
	if !ok {
		return response.ServerError("unexpected result type from redis")
	}
	if val == 0 {
		return response.ServerError("lock not found or value mismatch")
	}

	return nil
}

func (r *CacheClient) TryLock(ctx context.Context, key string, options *typeCustom.LockProcessOptions) (string, bool, *response.AppError) {
	for i := range options.RetryCount {
		lockValue := generateLockValue()
		success, err := r.client.SetNX(ctx, r.buildCacheKey(key), lockValue, options.Expire).Result()
		if err != nil {
			return "", false, response.ServerError("failed to acquire lock: " + err.Error())
		}
		if success {
			return lockValue, true, nil
		}
		if i < options.RetryCount-1 {
			time.Sleep(options.RetryDelay)
		}
	}
	return "", false, response.ServerError("failed to acquire lock after retries")
}

func (r *CacheClient) WithLockProcess(ctx context.Context, key string, options *typeCustom.LockProcessOptions) (string, bool, *response.AppError) {
	lockValue, acquired, err := r.TryLock(ctx, key, options)
	if err != nil {
		return "", false, err
	}
	if !acquired {
		return "", false, nil
	}
	defer r.Unlock(ctx, key, lockValue)

	err = options.ProcessFunc(ctx)
	if err != nil {
		return "", false, err
	}
	return lockValue, true, r.Unlock(ctx, key, lockValue)
}

func (r *CacheClient) OperateWithExpire(ctx context.Context, key string, value int64, expire time.Duration) (int64, *response.AppError) {
	pipe := r.client.Pipeline()
	key = r.buildCacheKey(key)
	incrCmd := pipe.IncrBy(ctx, key, value)
	pipe.Expire(ctx, key, expire)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, response.ServerError("failed to operate: " + err.Error())
	}
	result, err := incrCmd.Result()
	if err != nil {
		return 0, response.ServerError("failed to incr: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) Operate(ctx context.Context, key string, value int64) (int64, *response.AppError) {
	key = r.buildCacheKey(key)
	result, err := r.client.IncrBy(ctx, key, value).Result()
	if err != nil {
		return 0, response.ServerError("failed to operate: " + err.Error())
	}
	return result, nil
}

func (r *CacheClient) SetExpire(ctx context.Context, key string, expire time.Duration) (bool, *response.AppError) {
	result, err := r.client.Expire(ctx, r.buildCacheKey(key), expire).Result()
	if err != nil {
		return false, response.ServerError("failed to set expire: " + err.Error())
	}
	return result, nil
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
