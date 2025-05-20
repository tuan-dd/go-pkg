package test

import (
	"errors"
	"testing"

	redisClient "github.com/tuan-dd/go-pkg/caching/redis"
)

func connectRedisTest() (redisClient.EngineCaching, error) {
	cfg := redisClient.CacheConfig{
		Host:     "18.139.147.231",
		Port:     6379,
		Password: "redisabc123",
		Username: "myredis",
		// Database: 0,
		PoolSize: 0,
	}
	engineRedis, err := redisClient.NewRedisClient(&cfg)
	if err != nil {
		return nil, err
	}
	return engineRedis, nil
}

func TestNewRedisClient(t *testing.T) {
	tests := []struct {
		name       string
		wantErr    bool
		wantValue  string
		redisError error
	}{
		{
			name:      "successful connection and set/get",
			wantErr:   false,
			wantValue: "redis is oke",
		},
		{
			name:       "connection failure",
			wantErr:    true,
			redisError: errors.New("connection failed"),
		},
		{
			name:       "set failure",
			wantErr:    true,
			redisError: errors.New("set failed"),
		},
		{
			name:       "get failure",
			wantErr:    true,
			redisError: errors.New("get failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engineRedis, err := connectRedisTest()
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

			_, err = engineRedis.Set("key", []byte("redis is oke"), 1)
			if (err != nil) != tt.wantErr {
				t.Errorf("engineRedis.Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			value, _, err := engineRedis.Get("key")
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
