package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/tuan-dd/go-pkg/common/response"
	"github.com/tuan-dd/go-pkg/common/utils"
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

func NewConnection(cfg *SQLConfig) (*Connection, *response.AppError) {
	return connect(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), cfg)
}

func connect(dns string, cfg *SQLConfig) (*Connection, *response.AppError) {
	config := mysql.NewConfig()

	config.Addr = dns
	config.DBName = cfg.DBname
	config.Passwd = cfg.Password
	config.Net = "tcp"
	config.AllowCleartextPasswords = true
	config.AllowNativePasswords = true
	config.ParseTime = true
	config.User = cfg.Username

	dsn := config.FormatDSN()

	db, err := utils.RetryWithDelay(context.Background(), func(ctx context.Context) (*sql.DB, error) {
		return sql.Open("mysql", dsn)
	}, nil)
	if err != nil {
		log.Fatalln(err)
		return nil, response.ConvertDatabaseError(err)
	}

	if cfg.MaxConnIdleTime > 0 {
		db.SetConnMaxIdleTime(time.Duration(cfg.MaxConnIdleTime) * time.Minute)
	}
	if cfg.MaxConnLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.MaxConnLifetime) * time.Minute)
	}
	if cfg.MaxConns > 0 {
		db.SetMaxOpenConns(int(cfg.MaxConns))
	}

	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(int(cfg.MaxIdleConns))
	}

	conn := &Connection{
		RDBMS: cfg.RDBMS,
		db:    db,
		cfg:   cfg,
	}

	errApp := conn.HealthCheck(context.Background())
	if errApp != nil {
		return nil, errApp
	}

	slog.Info("mysql connect success")

	return conn, nil
}

func (c *Connection) DB() *sql.DB {
	return c.db
}

func (c *Connection) Shutdown() *response.AppError {
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
