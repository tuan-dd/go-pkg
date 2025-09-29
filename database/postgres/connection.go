package postgres

import (
	"fmt"

	"github.com/tuan-dd/go-common/response"

	_ "github.com/lib/pq"
)

func NewConnection(cfg *SQLConfig) (*Connection, *response.AppError) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.DBname, cfg.SSLMode,
	)
	return connect(dsn, cfg)
}
