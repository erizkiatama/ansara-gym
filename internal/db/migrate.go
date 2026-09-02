package db

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// NewMigrationProvider returns a goose provider bound to the existing pool.
// The caller owns database lifetime; do not call Provider.Close (it closes *sql.DB).
func NewMigrationProvider(database *sqlx.DB, log *slog.Logger) (*goose.Provider, error) {
	fsys, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations fs: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		database.DB,
		fsys,
		goose.WithDisableGlobalRegistry(true),
		goose.WithSlog(log),
	)
	if err != nil {
		return nil, fmt.Errorf("goose provider: %w", err)
	}
	return provider, nil
}
