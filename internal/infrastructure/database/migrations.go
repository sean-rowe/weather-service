package database

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations executes all pending database migrations.
//
// Parameters:
//   - db: Active database connection
//   - logger: Zap logger for migration logging
//
// Returns:
//   - error: Migration execution error or validation error
func RunMigrations(db *sql.DB, logger *zap.Logger) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create source driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if dirty {
		logger.Warn("database migrations are dirty",
			zap.Uint("version", version))
		
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("failed to force migration version: %w", err)
		}
	}

	logger.Info("running database migrations",
		zap.Uint("current_version", version))

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	newVersion, _, err := m.Version()
	if err != nil {
		return fmt.Errorf("failed to get new migration version: %w", err)
	}

	logger.Info("database migrations completed",
		zap.Uint("version", newVersion))

	return nil
}

// MigrateDown rolls back the last migration.
//
// Parameters:
//   - db: Active database connection
//   - logger: Zap logger for migration logging
//
// Returns:
//   - error: Rollback error or version retrieval error
func MigrateDown(db *sql.DB, logger *zap.Logger) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create source driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	version, _, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	logger.Info("rolling back migration",
		zap.Uint("current_version", version))

	if err := m.Steps(-1); err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	newVersion, _, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get new migration version: %w", err)
	}

	logger.Info("migration rolled back",
		zap.Uint("version", newVersion))

	return nil
}

// MigrateToVersion migrates to a specific version.
//
// Parameters:
//   - db: Active database connection
//   - targetVersion: Target migration version
//   - logger: Zap logger for migration logging
//
// Returns:
//   - error: Migration error or version validation error
func MigrateToVersion(db *sql.DB, targetVersion uint, logger *zap.Logger) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create source driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	currentVersion, _, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	logger.Info("migrating to version",
		zap.Uint("current_version", currentVersion),
		zap.Uint("target_version", targetVersion))

	if err := m.Migrate(targetVersion); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to migrate to version %d: %w", targetVersion, err)
	}

	logger.Info("migration completed",
		zap.Uint("version", targetVersion))

	return nil
}