package migrations

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
)

//go:embed *.sql
var migrationsDir embed.FS

// RunMigrations applies the database migrations using the provided DSN.
func RunMigrations(dsn string, flow bool) error {
	d, err := iofs.New(migrationsDir, ".")
	if err != nil {
		return fmt.Errorf("failed to return an iofs driver: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, dsn)
	if err != nil {
		return fmt.Errorf("failed to get a new migrate instance: %w", err)
	}
	if flow {
		// If flow is true, apply migrations in a forward direction
		if err := m.Up(); err != nil {
			if !errors.Is(err, migrate.ErrNoChange) {
				return fmt.Errorf("failed to apply migrations to the DB: %w", err)
			}
		}
		return nil

	} // If flow is false, apply migrations in a backward direction
	if err := m.Down(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("failed to rollback migrations in the DB: %w", err)
		}
	}
	return nil
}
