package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/tuan-dd/go-common/response"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

type SQLConfig struct {
	Host            string `mapstructure:"DB_HOST"`
	Port            int    `mapstructure:"DB_PORT"`
	Username        string `mapstructure:"DB_USERNAME"`
	Password        string `mapstructure:"DB_PASSWORD"`
	DBname          string `mapstructure:"DB_DBNAME"`
	LogEnabled      bool   `mapstructure:"LOG_ENABLED"`
	SSLMode         string `mapstructure:"SSL_MODE"`
	RDBMS           string `mapstructure:"RDBMS"`
	MaxConnIdleTime uint32 `mapstructure:"MAX_CONN_IDLE_TIME"`
	MaxConnLifetime uint64 `mapstructure:"MAX_CONN_LIFE_TIME"`
	MaxConns        uint8  `mapstructure:"MAX_CONNS"`
	MaxIdleConns    uint8  `mapstructure:"MAX_IDLE_CONNS"`
}

type Connection struct {
	db    *sql.DB
	cfg   *SQLConfig
	RDBMS string
}

func connect(dsn string, cfg *SQLConfig) (*Connection, *response.AppError) {
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatal(err)
		return nil, response.ConvertDatabaseError(err)
	}

	if cfg.MaxConnIdleTime > 0 {
		poolConfig.MaxConnIdleTime = time.Duration(cfg.MaxConnIdleTime) * time.Minute
	}
	if cfg.MaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = time.Duration(cfg.MaxConnLifetime) * time.Minute
	}
	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = int32(cfg.MaxConns)
	}
	// MaxIdleConns is the maximum number of connections in the idle connection pool
	if cfg.MaxIdleConns > 0 {
		poolConfig.MinIdleConns = int32(cfg.MaxIdleConns)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatal(err)
		return nil, response.ConvertDatabaseError(err)
	}
	db := stdlib.OpenDBFromPool(pool)

	return &Connection{
		RDBMS: cfg.RDBMS,
		db:    db,
		cfg:   cfg,
	}, nil
}

func (c *Connection) DB() *sql.DB {
	return c.db
}

func (c *Connection) Close() *response.AppError {
	err := c.db.Close()
	if err != nil {
		return response.ConvertDatabaseError(err)
	}
	return nil
}

func (c *Connection) HealthCheck(ctx context.Context) *response.AppError {
	if err := c.db.PingContext(ctx); err != nil {
		return response.ConvertDatabaseError(fmt.Errorf("%w: %v", ErrHealthCheckFailed, err))
	}
	return nil
}
