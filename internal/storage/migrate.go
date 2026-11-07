package storage

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func (s *Storage) Migrate(ctx context.Context) (int, error) {
	fsys, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return 0, fmt.Errorf("open migrations: %w", err)
	}

	db := stdlib.OpenDBFromPool(s.pool)
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys)
	if err != nil {
		return 0, fmt.Errorf("create migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return 0, fmt.Errorf("apply migrations: %w", err)
	}
	return len(results), nil
}
