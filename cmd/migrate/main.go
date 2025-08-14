package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/sean-rowe/weather-service/internal/infrastructure/database"
	"go.uber.org/zap"
)

func main() {
	var (
		action  = flag.String("action", "up", "Migration action: up, down, version, force")
		version = flag.Uint("version", 0, "Target version for migrate or force")
		dbHost  = flag.String("host", getEnv("DB_HOST", "localhost"), "Database host")
		dbPort  = flag.String("port", getEnv("DB_PORT", "5432"), "Database port")
		dbUser  = flag.String("user", getEnv("DB_USER", "postgres"), "Database user")
		dbPass  = flag.String("password", getEnv("DB_PASSWORD", ""), "Database password")
		dbName  = flag.String("database", getEnv("DB_NAME", "weather"), "Database name")
		dbSSL   = flag.String("sslmode", getEnv("DB_SSLMODE", "disable"), "SSL mode")
	)

	flag.Parse()

	logger, err := zap.NewProduction()

	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	defer func(logger *zap.Logger) {
		err := logger.Sync()

		if err != nil {
			log.Printf("Failed to sync logger: %v", err)
		}
	}(logger)

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		*dbHost, *dbPort, *dbUser, *dbPass, *dbName, *dbSSL,
	)

	db, err := sql.Open("postgres", dsn)

	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}

	defer func(db *sql.DB) {
		err := db.Close()

		if err != nil {
			logger.Error("Failed to close database connection", zap.Error(err))
		}
	}(db)

	if err := db.Ping(); err != nil {
		logger.Fatal("Failed to ping database", zap.Error(err))
	}

	switch *action {
	case "up":
		if err := database.RunMigrations(db, logger); err != nil {
			logger.Fatal("Migration failed", zap.Error(err))
		}

		logger.Info("Migrations completed successfully")

	case "down":
		if err := database.MigrateDown(db, logger); err != nil {
			logger.Fatal("Rollback failed", zap.Error(err))
		}

		logger.Info("Rollback completed successfully")

	case "version":
		if *version == 0 {
			logger.Fatal("Version must be specified with -version flag")
		}

		if err := database.MigrateToVersion(db, *version, logger); err != nil {
			logger.Fatal("Migration to version failed",
				zap.Uint("version", *version),
				zap.Error(err))
		}

		logger.Info("Migration to version completed",
			zap.Uint("version", *version))

	case "force":
		if *version == 0 {
			logger.Fatal("Version must be specified with -version flag")
		}

		if err := database.MigrateToVersion(db, *version, logger); err != nil {
			logger.Fatal("Force migration failed",
				zap.Uint("version", *version),
				zap.Error(err))
		}

		logger.Info("Forced migration completed",
			zap.Uint("version", *version))

	default:
		logger.Fatal("Invalid action",
			zap.String("action", *action))
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}
