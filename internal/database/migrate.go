package database

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	migmysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/gorm"

	"github.com/shaia/Synapse/pkg/logger"
)

//go:embed migrations/mysql
var mysqlMigrationsFS embed.FS

// RunMigrations executes all pending versioned SQL migrations.
//
// Strategy:
//   - MySQL (production): uses golang-migrate with embedded SQL files under
//     internal/database/migrations/mysql/. The schema_migrations table tracks
//     which migrations have run. CREATE TABLE IF NOT EXISTS in migration 001
//     makes it safe to run against databases previously managed by GORM
//     AutoMigrate — existing tables and rows are untouched.
//   - SQLite (development): no-op; the caller falls back to GORM AutoMigrate.
func RunMigrations(db *gorm.DB, driver string) error {
	if driver != "mysql" {
		return nil // SQLite uses AutoMigrate — nothing to do here.
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("migrations: get sql.DB: %w", err)
	}

	// Sub-FS rooted at migrations/mysql so the iofs driver sees *.sql directly.
	sub, err := fs.Sub(mysqlMigrationsFS, "migrations/mysql")
	if err != nil {
		return fmt.Errorf("migrations: sub fs: %w", err)
	}

	srcDriver, err := iofs.New(sub, ".")
	if err != nil {
		return fmt.Errorf("migrations: iofs source: %w", err)
	}

	dbDriver, err := migmysql.WithInstance(sqlDB, &migmysql.Config{})
	if err != nil {
		return fmt.Errorf("migrations: mysql driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", srcDriver, "mysql", dbDriver)
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
