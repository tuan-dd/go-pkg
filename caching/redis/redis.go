package redisClient

import (
	"context"
	"fmt"
	"log/slog"

	redisV9 "github.com/redis/go-redis/v9"
	"github.com/tuan-dd/go-common/response"
)

type CacheConfig struct {
	Host     string `mapstructure:"CACHE_HOST"`
	Port     int    `mapstructure:"CACHE_PORT"`
	Username string `mapstructure:"CACHE_USERNAME"`
	Password string `mapstructure:"CACHE_PASSWORD"`
	PoolSize int    `mapstructure:"CACHE_POOL_SIZE"`
}

type CacheClient struct {
	cfg         *CacheConfig
	client      *redisV9.Client
	serviceName string
}

type option func(*CacheClient)

func WithServiceName(name string) option {
	return func(c *CacheClient) {
		c.serviceName = name
	}
}

func NewRedisClient(cfg *CacheConfig, opts ...option) (*CacheClient, *response.AppError) {
	redis := &CacheClient{
		cfg:         cfg,
		serviceName: "unknown",
	}

	for _, opt := range opts {
		opt(redis)
	}

	urlRedis := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	redis.client = redisV9.NewClient(&redisV9.Options{
		Addr:     urlRedis,
		Username: cfg.Username,
		Password: cfg.Password,
		PoolSize: cfg.PoolSize,
	})

	_, err := redis.client.Ping(context.Background()).Result()
	if err != nil {
		return nil, response.ServerError("failed to connect redis " + err.Error())
	}
	slog.Info("redis connect success")
	return redis, nil
}

func (r *CacheClient) Client() *redisV9.Client {
	if r.client == nil {
		return nil
	}
	return r.client
}
