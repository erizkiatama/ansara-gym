package db

import (
	"context"
	"fmt"

	"github.com/erizkiatama/ansara-gym/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// Open connects with the pgx stdlib driver and pings immediately.
// Pool caps come from config so we never inherit database/sql's unlimited default.
func Open(ctx context.Context, cfg config.DatabaseConfig) (*sqlx.DB, error) {
	database, err := sqlx.ConnectContext(ctx, "pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	database.SetMaxOpenConns(cfg.MaxOpenConns)
	database.SetMaxIdleConns(cfg.MaxIdleConns)
	database.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	database.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return database, nil
}
