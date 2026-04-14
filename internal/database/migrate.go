package database

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	migpg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/shaia/Synapse/pkg/logger"
)

//go:embed migrations/postgres
var pgMigrationsFS embed.FS

// RunMigrations executes all pending versioned SQL migrations for PostgreSQL.
//
// Strategy:
//   - Uses golang-migrate with embedded SQL files under
//     internal/database/migrations/postgres/.
//   - The schema_migrations table tracks which migrations have run.
//   - CREATE TABLE IF NOT EXISTS in migration 001 makes it safe to run
//     against databases previously managed by GORM AutoMigrate.
//
// dsn must be the full PostgreSQL DSN.
func RunMigrations(dsn string) error {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("migrations: open db: %w", err)
	}
	defer sqlDB.Close()

	sub, err := fs.Sub(pgMigrationsFS, "migrations/postgres")
	if err != nil {
		return fmt.Errorf("migrations: sub fs: %w", err)
	}

	srcDriver, err := iofs.New(sub, ".")
	if err != nil {
		return fmt.Errorf("migrations: iofs source: %w", err)
	}

	dbDriver, err := migpg.WithInstance(sqlDB, &migpg.Config{})
	if err != nil {
		return fmt.Errorf("migrations: postgres driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", srcDriver, "postgres", dbDriver)
	if err != nil {
		return fmt.Errorf("migrations: create migrator: %w", err)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("database: migrations up to date")
			return nil
		}
		return fmt.Errorf("migrations: run: %w", err)
	}

	version, _, _ := m.Version()
	logger.Info("database: migrations applied", "version", version)
	return nil
}
