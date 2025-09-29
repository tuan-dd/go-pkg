package test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tuan-dd/go-common/response"
	typeCustom "github.com/tuan-dd/go-common/type-custom"
	redisClient "github.com/tuan-dd/go-pkg/caching/redis"
)

func connectRedisTest() (typeCustom.EngineCaching, *response.AppError) {
	cfg := redisClient.CacheConfig{
		Host:     "18.136.192.113",
		Port:     6379,
		Username: "myredis",
		Password: "redisabc123",

		// // Database: 0,
		// PoolSize: 0,
	}
	engineRedis, err := redisClient.NewRedisClient(&cfg)
	if err != nil {
		return nil, response.ServerError("failed to create redis client: " + err.Error())
	}
	time.Sleep(2 * time.Second)
	return engineRedis, nil
}

func TestNewRedisClient(t *testing.T) {
	tests := []struct {
		name       string
		wantErr    bool
		wantValue  string
		redisError *response.AppError
	}{
		{
			name:      "successful connection and set/get",
			wantErr:   false,
			wantValue: "test",
		},
		{
			name:       "connection failure",
			wantErr:    true,
			redisError: response.ServerError("connection failed"),
		},
		{
			name:       "set failure",
			wantErr:    true,
			redisError: response.ServerError("set failed"),
		},
		{
			name:       "get failure",
			wantErr:    true,
			redisError: response.ServerError("get failed"),
		},
	}

	engineRedis, err := connectRedisTest()
	if err != nil {
		t.Fatalf("connectRedisTest() error = %v", err)
	}

	defer engineRedis.Shutdown()
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.redisError != nil {
				// simulate error
				err = tt.redisError
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("connectRedisTest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			_, err = engineRedis.Set(ctx, "key", "test", time.Minute)

			if (err != nil) != tt.wantErr {
				t.Errorf("engineRedis.Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			value, _, err := engineRedis.Get(ctx, "key")

			if (err != nil) != tt.wantErr {
				t.Errorf("engineRedis.Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}
			if string(value) != tt.wantValue {
				t.Errorf("engineRedis.Get() value = %v, want %v", string(value), tt.wantValue)
			}
		})
	}
}

func TestLock(t *testing.T) {
	ctx := context.Background()
	engineRedis, err := connectRedisTest()
	if err != nil {
		t.Fatalf("connectRedisTest() error = %v", err)
	}
	defer engineRedis.Shutdown()

	// Test acquiring a lock
	lockKey := "test_lock"

	lockValue, isLock, err := engineRedis.Lock(ctx, lockKey, time.Minute)
	if err != nil {
		t.Errorf("engineRedis.Lock() error = %v", err)
		return
	}

	if !isLock {
		t.Errorf("engineRedis.Lock() failed to acquire lock")
		return
	}

	if lockValue == "" {
		t.Errorf("engineRedis.Lock() returned empty lock value")
		return
	}

	// lock same key
	lockValue2, isLock2, err := engineRedis.TryLock(ctx, lockKey, &typeCustom.LockProcessOptions{
		Expire:     time.Minute,
		RetryCount: 3,
		RetryDelay: time.Second,
	})
	if err != nil {
		if err.Message != "failed to acquire lock after retries" {
			t.Errorf("engineRedis.TryLock() error = %v, expected 'failed to acquire lock after retries'", err)
			return
		}
	}

	// cant not lock same key
	if isLock2 {
		t.Errorf("engineRedis.TryLock() should not acquire lock on already locked key")
	}

	if lockValue2 != "" {
		t.Errorf("engineRedis.TryLock() returned non-empty lock value")
	}

	// unlock
	err = engineRedis.Unlock(ctx, lockKey, lockValue)
	if err != nil {
		t.Errorf("engineRedis.Unlock() error = %v", err)
		return
	}
}

func TestMultiCurrencyLock(t *testing.T) {
	engineRedis, err := connectRedisTest()
	if err != nil {
		t.Fatalf("connectRedisTest() error = %v", err)
	}
	defer engineRedis.Shutdown()

	// Test acquiring a lock
	lockKey := "multi_lock_key"
	lockValue := ""
	signal := make(chan struct{})
	// 1000 goroutine try to lock same key
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := range 10_000 {
		wg.Go(func() {
			<-signal
			value, isLock, err := engineRedis.TryLock(ctx, lockKey, &typeCustom.LockProcessOptions{
				Expire:     time.Minute,
				RetryCount: 2,
				RetryDelay: 100 * time.Millisecond,
			})
			if err != nil {
				if err.Message != "failed to acquire lock after retries" {
					t.Errorf("engineRedis.TryLock() error = %v, expected 'failed to acquire lock after retries'", err)
					return
				}
				return
			}

			// cant not lock same key
			if isLock {
				lockValue = value
				t.Logf("Goroutine %d acquired lock with value: %s", i, value)
			}
		})
	}

	time.Sleep(5 * time.Second)
	close(signal)
	wg.Wait()

	err = engineRedis.Unlock(ctx, lockKey, lockValue)
	if err != nil {
		t.Errorf("engineRedis.Unlock() error = %v, lockValue = %v", err, lockValue)
		return
	}
}
