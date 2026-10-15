package database

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // "pgx5://" driver
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"golang_crud/migrations"
)

// RunMigrations applies all pending "up" migrations from the embedded files.
// It is safe to call on every startup: if nothing is pending it does nothing.
func RunMigrations(dsn string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, toMigrateURL(dsn))
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// golang-migrate's pgx v5 driver expects the "pgx5://" URL scheme.
func toMigrateURL(dsn string) string {
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(dsn, prefix) {
			return "pgx5://" + strings.TrimPrefix(dsn, prefix)
		}
	}
	return dsn
}
